//go:build esp32c5

package runtime

import (
	"device/esp"
	"device/riscv"
	"machine"
	"runtime/interrupt"
	"runtime/volatile"
	"unsafe"
)

// This is the function called on startup after the flash (the unified
// flash-mapped window) is initialized and the stack pointer has been set.
//
//export main
func main() {
	// This initialization configures the following things:
	// * It disables all watchdog timers. They might be useful at some point in
	//   the future, but will need integration into the scheduler. For now,
	//   they're all disabled.
	// * It sets the CPU frequency to 240MHz, which is the maximum speed allowed
	//   for this CPU.

	// Disable Timer Group 0 watchdog (unlock first).
	esp.TIMG0.WDTWPROTECT.Set(0x50D83AA1)
	esp.TIMG0.WDTCONFIG0.Set(0)

	// Disable Timer Group 1 watchdog (unlock first).
	esp.TIMG1.WDTWPROTECT.Set(0x50D83AA1)
	esp.TIMG1.WDTCONFIG0.Set(0)

	// Disable LP watchdog (write-protect key first).
	esp.LP_WDT.WPROTECT.Set(0x50D83AA1)
	esp.LP_WDT.CONFIG0.Set(0)

	// Disable super watchdog.
	esp.LP_WDT.SWD_WPROTECT.Set(0x50D83AA1)
	esp.LP_WDT.SetSWD_CONFIG_SWD_DISABLE(1)

	// Change CPU frequency to 240MHz.
	//
	// Unlike the ESP32-C6, the C5 does not divide the SPLL directly: the CPU
	// clock selects one of the fixed PLL taps (PLL_F160M or PLL_F240M). The
	// sequence below follows rtc_clk_cpu_freq_to_pll_240_mhz in ESP-IDF:
	//   CPU  = PLL_F240M / (CPU_DIV_NUM+1)  = 240 / 1 = 240 MHz
	//   AHB  = PLL_F240M / (AHB_DIV_NUM+1)  = 240 / 6 =  40 MHz
	//   SOC_CLK_SEL: 0=XTAL, 1=RC_FAST, 2=PLL_F160M, 3=PLL_F240M
	esp.PCR.SetCPU_FREQ_CONF_CPU_DIV_NUM(0)
	esp.PCR.SetAHB_FREQ_CONF_AHB_DIV_NUM(5)
	esp.PCR.SetSYSCLK_CONF_SOC_CLK_SEL(3)

	// Commit the new clock configuration and wait for it to take effect.
	esp.PCR.SetBUS_CLK_UPDATE_BUS_CLOCK_UPDATE(1)
	for esp.PCR.GetBUS_CLK_UPDATE_BUS_CLOCK_UPDATE() != 0 {
	}

	// Select the Timer Group 0 timer clock source.
	//
	// The shared timekeeping code (runtime_esp32xx.go) assumes the TIMG0 timer
	// counts at 40MHz: an 80MHz source divided by the prescaler of 2 set in
	// initTimer, giving 25ns/tick. Select PLL_F80M (80MHz) as the source.
	// Clock source encoding on the C5 (timer_ll_set_clock_source):
	// 0 = XTAL, 1 = RC_FAST, 2 = PLL_F80M.
	esp.PCR.SetTIMERGROUP0_CONF_TG0_CLK_EN(1)
	esp.PCR.SetTIMERGROUP0_TIMER_CLK_CONF_TG0_TIMER_CLK_SEL(2)
	esp.PCR.SetTIMERGROUP0_TIMER_CLK_CONF_TG0_TIMER_CLK_EN(1)

	clearbss()

	// Configure interrupt handler
	interruptInit()

	// Initialize main system timer used for time.Now.
	initTimer()

	// Initialize timer alarm interrupt for the scheduler.
	initTimerInterrupt()

	// Initialize the heap, call main.main, etc.
	run()

	// Fallback: if main ever returns, hang the CPU.
	exit(0)
}

func init() {
	machine.InitSerial()
}

func abort() {
	for {
		riscv.Asm("wfi")
	}
}

// The ESP32-C5 uses a CLIC as its CPU interrupt controller. External
// (peripheral) interrupts occupy CLIC lines 16..47; the relative external
// interrupt number used with interrupt.New must be offset by this value when
// writing INTMTX mapping registers.
const clicExtIntrNumOffset = 16

// CLIC configuration register (DR_REG_CLIC_BASE).
var clicIntConfig = (*volatile.Register32)(unsafe.Pointer(uintptr(0x20800000)))

// mintthresh: machine-mode interrupt level threshold CSR (standard CLIC).
const mintthresh = riscv.CSR(0x347)

// mtvt: machine-mode CLIC vector table base CSR.
const mtvt = riscv.CSR(0x307)

// interruptInit initializes the CLIC interrupt controller.
func interruptInit() {
	// Set the number of level bits (MNLBITS, bits [3:0]) to 3, giving 8
	// priority levels. This matches ESP-IDF and esp-hal (NLBITS = 3).
	clicIntConfig.ReplaceBits(3, 0xf, 0)

	// Set the trap entry address in CLIC mode (mtvec[1:0] = 3). All
	// interrupts are non-vectored (shv=0), so exceptions and interrupts all
	// enter at this address, which must be 64-byte aligned.
	riscv.MTVEC.Set((uintptr(unsafe.Pointer(&_vector_table))) | 3)

	// Set the CLIC hardware vector table. The Espressif CLIC fetches the
	// handler address from mtvt + 4*line when delivering an interrupt, so
	// this must point at a valid table of function pointers (all entries
	// point at handleInterruptASM).
	mtvt.Set(uintptr(unsafe.Pointer(&_mtvt_table)))

	// Unmask all interrupt levels. The threshold is inclusive: levels <=
	// threshold are masked, so 0 enables everything.
	mintthresh.Set(0)

	// Globally enable machine-mode interrupts (MSTATUS.MIE).
	//
	// Do not rely on the state inherited from the ROM bootloader: per-line
	// enables live in the CLIC (mie is a don't-care in CLIC mode), but
	// MSTATUS.MIE still gates all machine-mode interrupts.
	riscv.MSTATUS.SetBits(riscv.MSTATUS_MIE)
}

// CPU interrupt number (relative external interrupt number) used for the
// TIMG0 timer alarm.
const timerAlarmCPUInterrupt = 9

var interruptPending volatile.Register8

func signalInterrupt() {
	interruptPending.Set(1)
}

// initTimerInterrupt routes the TIMG0 timer 0 alarm interrupt to a CPU
// interrupt and registers a handler.
func initTimerInterrupt() {
	// Map the TIMG0 T0 peripheral interrupt to a CPU interrupt line.
	// The INTMTX mapping value is the CLIC line number, which includes the
	// external interrupt offset.
	esp.INTERRUPT_CORE0.TG0_T0_INTR_MAP.Set(timerAlarmCPUInterrupt + clicExtIntrNumOffset)

	// Enable T0 interrupt at the timer group level.
	esp.TIMG0.INT_ENA_TIMERS.SetBits(1)

	// Register the interrupt handler and enable the CPU interrupt (the CLIC
	// line is configured in interrupt.Enable).
	interrupt.New(timerAlarmCPUInterrupt, func(interrupt.Interrupt) {
		esp.TIMG0.INT_CLR_TIMERS.Set(1)
	}).Enable()
}

// sleepTicks spins until the given number of ticks have elapsed, using the
// TIMG0 alarm interrupt to avoid busy-waiting for the entire duration.
func sleepTicks(d timeUnit) {
	machine.FlushSerial()
	target := ticks() + d
	for ticks() < target {
		interruptPending.Set(0)

		esp.TIMG0.T0ALARMLO.Set(uint32(target))
		esp.TIMG0.T0ALARMHI.Set(uint32(target >> 32))

		// Enable the alarm (auto-clears when alarm fires).
		esp.TIMG0.T0CONFIG.SetBits(esp.TIMG_T0CONFIG_T0_ALARM_EN)

		for interruptPending.Get() == 0 {
			if ticks() >= target {
				return
			}
		}
	}
}

//go:extern _vector_table
var _vector_table [0]uintptr

//go:extern _mtvt_table
var _mtvt_table [0]uintptr

// The following functions mirror runtime_esp32xx.go. The ESP32-C5 SVD names
// the TIMG T0CONFIG bit-field constants with a T0_ prefix (e.g.
// TIMG_T0CONFIG_T0_EN instead of TIMG_T0CONFIG_EN), so the shared file cannot
// be reused directly.

// Initialize .bss: zero-initialized global variables.
// The .data section has already been loaded by the ROM bootloader.
func clearbss() {
	ptr := unsafe.Pointer(&_sbss)
	for ptr != unsafe.Pointer(&_ebss) {
		*(*uint32)(ptr) = 0
		ptr = unsafe.Add(ptr, 4)
	}
}

func initTimer() {
	// Configure timer 0 in timer group 0, for timekeeping.
	//   EN:       Enable the timer.
	//   INCREASE: Count up every tick (as opposed to counting down).
	//   DIVIDER:  16-bit prescaler, set to 2 for dividing the 80MHz timer
	//             clock (PLL_F80M) by two (40MHz).
	esp.TIMG0.T0CONFIG.Set(esp.TIMG_T0CONFIG_T0_EN | esp.TIMG_T0CONFIG_T0_INCREASE | 2<<esp.TIMG_T0CONFIG_T0_DIVIDER_Pos)

	// Set the timer counter value to 0.
	esp.TIMG0.T0LOADLO.Set(0)
	esp.TIMG0.T0LOADHI.Set(0)
	esp.TIMG0.T0LOAD.Set(0) // value doesn't matter.
}

func ticks() timeUnit {
	// First, update the LO and HI register pair by writing any value to the
	// register. This allows reading the pair atomically.
	esp.TIMG0.T0UPDATE.Set(0)
	// Then read the two 32-bit parts of the timer.
	return timeUnit(uint64(esp.TIMG0.T0LO.Get()) | uint64(esp.TIMG0.T0HI.Get())<<32)
}

func nanosecondsToTicks(ns int64) timeUnit {
	// Calculate the number of ticks from the number of nanoseconds. At a 80MHz
	// timer clock, that's 25 nanoseconds per tick with a timer prescaler of 2:
	// 25 = 1e9 / (80MHz / 2)
	return timeUnit(ns / 25)
}

func ticksToNanoseconds(ticks timeUnit) int64 {
	// See nanosecondsToTicks.
	return int64(ticks) * 25
}

func exit(code int) {
	abort()
}

func putchar(c byte) {
	machine.Serial.WriteByte(c)
}

func getchar() byte {
	for machine.Serial.Buffered() == 0 {
		Gosched()
	}
	v, _ := machine.Serial.ReadByte()
	return v
}

func buffered() int {
	return machine.Serial.Buffered()
}
