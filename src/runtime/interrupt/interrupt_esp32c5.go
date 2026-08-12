//go:build esp32c5

package interrupt

import (
	"device/riscv"
	"errors"
	"runtime/volatile"
	"unsafe"
)

//go:extern tinygo_saved_ra
var tinygo_saved_ra uintptr

// The ESP32-C5 uses a CLIC (Core-Local Interrupt Controller) as its CPU
// interrupt controller — not the PLIC of the ESP32-C6 or the INTC of the
// ESP32-C3 (SOC_INT_CLIC_SUPPORTED in ESP-IDF).
//
// CLIC lines 0..15 are reserved for CLINT/system interrupts; external
// (peripheral) interrupts occupy CLIC lines 16..47 (CLIC_EXT_INTR_NUM_OFFSET).
// The numbers used with interrupt.New (1..31) are relative external interrupt
// numbers; the CLIC line is num + clicExtIntrNumOffset, and the same offset
// value must be written to the INTMTX mapping registers.
//
// All interrupts are configured as machine-mode, edge-triggered and
// non-vectored (shv=0) with the same priority level, so every interrupt
// enters at the mtvec base (handleInterruptASM) and is dispatched here based
// on mcause.
const (
	clicExtIntrNumOffset = 16

	// Priority byte encoding with NLBITS=3: level in the top 3 bits, low 5
	// bits all-ones. Level 0 never fires; level 1 is the default.
	clicPriorityDefault = (1 << 5) | 0x1f
	clicPriorityOff     = 0x1f
)

// CLICINT byte-granular registers (DR_REG_CLIC_CTRL_BASE). Each CLIC line i
// has four byte registers at 0x20801000 + 4*i: IP (+0), IE (+1), ATTR (+2),
// CTL (+3). Byte accesses are used (matching esp-hal and the esp-pacs PAC)
// so that programming one field cannot glitch the others — in particular the
// write-0-clears IP bit.
func clicIntIPReg(line int) *volatile.Register8 {
	return (*volatile.Register8)(unsafe.Pointer(uintptr(0x20801000 + 4*line)))
}

func clicIntIEReg(line int) *volatile.Register8 {
	return (*volatile.Register8)(unsafe.Pointer(uintptr(0x20801001 + 4*line)))
}

func clicIntAttrReg(line int) *volatile.Register8 {
	return (*volatile.Register8)(unsafe.Pointer(uintptr(0x20801002 + 4*line)))
}

func clicIntCtlReg(line int) *volatile.Register8 {
	return (*volatile.Register8)(unsafe.Pointer(uintptr(0x20801003 + 4*line)))
}

// ATTR byte fields: shv bit0, trig bits 2:1, mode bits 7:6.
const (
	clicAttrLevelMachine = 0xC0 // machine mode, level-triggered, non-vectored
)

// Enable registers a CPU interrupt.
// The ESP32-C5 has 31 usable external CPU interrupts (1..31).
func (i Interrupt) Enable() error {
	if i.num < 1 || i.num > 31 {
		return errors.New("interrupt for ESP32-C5 must be in range of 1 through 31")
	}
	// Note: no defer here. Enable is called from the runtime before the
	// scheduler has started (initTimerInterrupt), and the defer machinery
	// needs the current task, which is still nil at that point.
	mask := riscv.DisableInterrupts()

	line := int(i.num) + clicExtIntrNumOffset

	// Configure the CLIC line: machine mode, level-triggered, non-vectored,
	// default priority; then enable it. Byte writes, in the same order
	// esp-hal uses (attr, priority, enable).
	clicIntAttrReg(line).Set(clicAttrLevelMachine)
	clicIntCtlReg(line).Set(clicPriorityDefault)
	clicIntIEReg(line).Set(1)

	riscv.Asm("fence")
	riscv.EnableInterrupts(mask)
	return nil
}

// Adding pseudo function calls that is replaced by the compiler with the actual
// functions registered through interrupt.New.
//
//go:linkname callHandlers runtime/interrupt.callHandlers
func callHandlers(num int)

//go:linkname signalInterrupt runtime.signalInterrupt
func signalInterrupt()

const (
	IRQNUM_1 = 1 + iota
	IRQNUM_2
	IRQNUM_3
	IRQNUM_4
	IRQNUM_5
	IRQNUM_6
	IRQNUM_7
	IRQNUM_8
	IRQNUM_9
	IRQNUM_10
	IRQNUM_11
	IRQNUM_12
	IRQNUM_13
	IRQNUM_14
	IRQNUM_15
	IRQNUM_16
	IRQNUM_17
	IRQNUM_18
	IRQNUM_19
	IRQNUM_20
	IRQNUM_21
	IRQNUM_22
	IRQNUM_23
	IRQNUM_24
	IRQNUM_25
	IRQNUM_26
	IRQNUM_27
	IRQNUM_28
	IRQNUM_29
	IRQNUM_30
	IRQNUM_31
)

//go:inline
func callHandler(n int) {
	switch n {
	case IRQNUM_1:
		callHandlers(IRQNUM_1)
	case IRQNUM_2:
		callHandlers(IRQNUM_2)
	case IRQNUM_3:
		callHandlers(IRQNUM_3)
	case IRQNUM_4:
		callHandlers(IRQNUM_4)
	case IRQNUM_5:
		callHandlers(IRQNUM_5)
	case IRQNUM_6:
		callHandlers(IRQNUM_6)
	case IRQNUM_7:
		callHandlers(IRQNUM_7)
	case IRQNUM_8:
		callHandlers(IRQNUM_8)
	case IRQNUM_9:
		callHandlers(IRQNUM_9)
	case IRQNUM_10:
		callHandlers(IRQNUM_10)
	case IRQNUM_11:
		callHandlers(IRQNUM_11)
	case IRQNUM_12:
		callHandlers(IRQNUM_12)
	case IRQNUM_13:
		callHandlers(IRQNUM_13)
	case IRQNUM_14:
		callHandlers(IRQNUM_14)
	case IRQNUM_15:
		callHandlers(IRQNUM_15)
	case IRQNUM_16:
		callHandlers(IRQNUM_16)
	case IRQNUM_17:
		callHandlers(IRQNUM_17)
	case IRQNUM_18:
		callHandlers(IRQNUM_18)
	case IRQNUM_19:
		callHandlers(IRQNUM_19)
	case IRQNUM_20:
		callHandlers(IRQNUM_20)
	case IRQNUM_21:
		callHandlers(IRQNUM_21)
	case IRQNUM_22:
		callHandlers(IRQNUM_22)
	case IRQNUM_23:
		callHandlers(IRQNUM_23)
	case IRQNUM_24:
		callHandlers(IRQNUM_24)
	case IRQNUM_25:
		callHandlers(IRQNUM_25)
	case IRQNUM_26:
		callHandlers(IRQNUM_26)
	case IRQNUM_27:
		callHandlers(IRQNUM_27)
	case IRQNUM_28:
		callHandlers(IRQNUM_28)
	case IRQNUM_29:
		callHandlers(IRQNUM_29)
	case IRQNUM_30:
		callHandlers(IRQNUM_30)
	case IRQNUM_31:
		callHandlers(IRQNUM_31)
	}
}

//export handleInterrupt
func handleInterrupt() {
	mcause := riscv.MCAUSE.Get()
	exception := mcause&(1<<31) == 0
	// In CLIC mode the exception code field is 12 bits wide and holds the
	// CLIC line number (16..47 for external interrupts).
	interruptNumber := uint32(mcause & 0xfff)

	if !exception && interruptNumber >= clicExtIntrNumOffset {
		// Save MSTATUS & MEPC, which could be overwritten by another CPU interrupt.
		mstatus := riscv.MSTATUS.Get()
		mepc := riscv.MEPC.Get()

		// Interrupts are level-triggered: the pending flag follows the
		// peripheral's interrupt line, which the registered handler clears at
		// the source (e.g. TIMG0.INT_CLR_TIMERS). No CLIC-side acknowledge is
		// needed.

		// Keep CPU interrupts disabled while the handler runs. Unlike the
		// C3/C6 ports, interrupts are NOT re-enabled here: the Espressif
		// CLIC does not appear to raise the running interrupt level for
		// non-vectored interrupts, so enabling MSTATUS.MIE mid-handler
		// allows immediate same-level re-entry (e.g. from a bouncing button)
		// and corrupts the saved MSTATUS/MEPC/MCAUSE state.

		// Call registered interrupt handler(s).
		callHandler(int(interruptNumber - clicExtIntrNumOffset))

		// Signal to sleepTicks that an interrupt has occurred.
		signalInterrupt()

		// Zero MCAUSE so that interrupt.In() returns false once we return to
		// normal (non-interrupt) code. On mret the hardware restores the
		// previous interrupt level from mcause.mpil; zeroing it restores
		// level 0, which is correct because the interrupted code always runs
		// at level 0 (no nesting among same-level interrupts).
		riscv.MCAUSE.Set(0)

		// Restore MSTATUS & MEPC.
		riscv.MSTATUS.Set(mstatus)
		riscv.MEPC.Set(mepc)
	} else {
		handleException(mcause)
	}
}

func handleException(mcause uintptr) {
	println("*** Exception:     pc:", riscv.MEPC.Get())
	println("*** Exception:   code:", uint32(mcause&0x1f))
	println("*** Exception: mcause:", mcause)
	println("*** Exception:     ra:", tinygo_saved_ra)
	switch uint32(mcause & 0x1f) {
	case riscv.InstructionAccessFault:
		println("***    virtual address:", riscv.MTVAL.Get())
	case riscv.IllegalInstruction:
		println("***            opcode:", riscv.MTVAL.Get())
	case riscv.LoadAccessFault:
		println("***      read address:", riscv.MTVAL.Get())
	case riscv.StoreOrAMOAccessFault:
		println("***     write address:", riscv.MTVAL.Get())
	}
	for {
		riscv.Asm("wfi")
	}
}
