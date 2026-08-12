//go:build esp32c5

package machine

import (
	"device/esp"
	"device/riscv"
	"errors"
	"runtime/interrupt"
	"runtime/volatile"
	"sync"
)

// UART on the ESP32-C5.
//
// The C5 has a newer UART IP than the C6: several configuration registers
// grew a _SYNC suffix (CONF0_SYNC, CLKDIV_SYNC, RS485_CONF_SYNC, ...) and the
// clock source selection moved fully into the PCR block.

const cpuInterruptFromUART = 7

var (
	DefaultUART = UART0

	UART0  = &_UART0
	_UART0 = UART{Bus: esp.UART0, Buffer: NewRingBuffer()}
	UART1  = &_UART1
	_UART1 = UART{Bus: esp.UART1, Buffer: NewRingBuffer()}

	onceUart            = sync.Once{}
	errSamePins         = errors.New("UART: invalid pin combination")
	errWrongUART        = errors.New("UART: unsupported UARTn")
	errWrongBitSize     = errors.New("UART: invalid data size")
	errWrongStopBitSize = errors.New("UART: invalid bit size")
)

type UART struct {
	Bus                  *esp.UART_Type
	Buffer               *RingBuffer
	ParityErrorDetected  bool // set when parity error detected
	DataErrorDetected    bool // set when data corruption detected
	DataOverflowDetected bool // set when data overflow detected in UART FIFO buffer or RingBuffer
}

const (
	defaultDataBits = 8
	defaultStopBit  = 1
	defaultParity   = ParityNone

	uartInterrupts = esp.UART_INT_ENA_RXFIFO_FULL_INT_ENA |
		esp.UART_INT_ENA_PARITY_ERR_INT_ENA |
		esp.UART_INT_ENA_FRM_ERR_INT_ENA |
		esp.UART_INT_ENA_RXFIFO_OVF_INT_ENA |
		esp.UART_INT_ENA_GLITCH_DET_INT_ENA

	pplClockFreq = 80e6
)

type registerSet struct {
	interruptMapReg  *volatile.Register32
	gpioMatrixSignal uint32
}

func (uart *UART) Configure(config UARTConfig) error {
	if config.BaudRate == 0 {
		config.BaudRate = 115200
	}
	if config.TX == config.RX {
		return errSamePins
	}
	switch {
	case uart.Bus == esp.UART0:
		return uart.configure(config, registerSet{
			interruptMapReg:  &esp.INTERRUPT_CORE0.UART0_INTR_MAP,
			gpioMatrixSignal: 6,
		})
	case uart.Bus == esp.UART1:
		return uart.configure(config, registerSet{
			interruptMapReg:  &esp.INTERRUPT_CORE0.UART1_INTR_MAP,
			gpioMatrixSignal: 9,
		})
	}
	return errWrongUART
}

func (uart *UART) configure(config UARTConfig, regs registerSet) error {
	initUARTClock(uart.Bus)

	// - disable TX/RX clock to make sure the UART transmitter or receiver is
	//   not at work during configuration
	uart.Bus.SetCLK_CONF_TX_SCLK_EN(0)
	uart.Bus.SetCLK_CONF_RX_SCLK_EN(0)

	uart.SetBaudRate(config.BaudRate)
	uart.SetFormat(defaultDataBits, defaultStopBit, defaultParity)

	// - set UART mode
	uart.Bus.SetRS485_CONF_SYNC_RS485_EN(0)
	uart.Bus.SetRS485_CONF_SYNC_RS485TX_RX_EN(0)
	uart.Bus.SetRS485_CONF_SYNC_RS485RXBY_TX_EN(0)
	uart.Bus.SetCONF0_SYNC_IRDA_EN(0)
	// - disable hw-flow control
	uart.Bus.SetCONF0_SYNC_TX_FLOW_EN(0)

	// synchronize values into Core Clock
	uart.Bus.SetREG_UPDATE(1)

	uart.setupPins(config, regs)
	uart.configureInterrupt(regs.interruptMapReg)
	uart.enableTransmitter()
	uart.enableReceiver()

	// Start TX/RX
	uart.Bus.SetCLK_CONF_TX_SCLK_EN(1)
	uart.Bus.SetCLK_CONF_RX_SCLK_EN(1)
	return nil
}

func (uart *UART) SetFormat(dataBits, stopBits int, parity UARTParity) error {
	if dataBits < 5 {
		return errWrongBitSize
	}
	if stopBits > 1 {
		return errWrongStopBitSize
	}
	// - data length
	uart.Bus.SetCONF0_SYNC_BIT_NUM(uint32(dataBits - 5))
	// - stop bit
	uart.Bus.SetCONF0_SYNC_STOP_BIT_NUM(uint32(stopBits))
	// - parity check
	switch parity {
	case ParityNone:
		uart.Bus.SetCONF0_SYNC_PARITY_EN(0)
	case ParityEven:
		uart.Bus.SetCONF0_SYNC_PARITY_EN(1)
		uart.Bus.SetCONF0_SYNC_PARITY(0)
	case ParityOdd:
		uart.Bus.SetCONF0_SYNC_PARITY_EN(1)
		uart.Bus.SetCONF0_SYNC_PARITY(1)
	}
	return nil
}

func initUARTClock(bus *esp.UART_Type) {
	// On ESP32-C5, the UART clock is controlled via PCR (Peripheral Clock
	// Reset), including the clock source selection:
	// 0 = XTAL, 1 = RC_FAST, 2 = PLL_F80M (uart_ll_set_sclk).
	switch bus {
	case esp.UART0:
		esp.PCR.SetUART0_CONF_UART0_CLK_EN(1)
		esp.PCR.SetUART0_CONF_UART0_RST_EN(1)
		esp.PCR.SetUART0_CONF_UART0_RST_EN(0)
		// Select PLL_F80M (80 MHz) for good baud-rate accuracy.
		esp.PCR.SetUART0_SCLK_CONF_UART0_SCLK_SEL(2)
		// Configure SCLK divisor in PCR
		esp.PCR.SetUART0_SCLK_CONF_UART0_SCLK_DIV_NUM(0)
		esp.PCR.SetUART0_SCLK_CONF_UART0_SCLK_DIV_A(0)
		esp.PCR.SetUART0_SCLK_CONF_UART0_SCLK_DIV_B(0)
		esp.PCR.SetUART0_SCLK_CONF_UART0_SCLK_EN(1)
	case esp.UART1:
		esp.PCR.SetUART1_CONF_UART1_CLK_EN(1)
		esp.PCR.SetUART1_CONF_UART1_RST_EN(1)
		esp.PCR.SetUART1_CONF_UART1_RST_EN(0)
		esp.PCR.SetUART1_SCLK_CONF_UART1_SCLK_SEL(2)
		esp.PCR.SetUART1_SCLK_CONF_UART1_SCLK_DIV_NUM(0)
		esp.PCR.SetUART1_SCLK_CONF_UART1_SCLK_DIV_A(0)
		esp.PCR.SetUART1_SCLK_CONF_UART1_SCLK_DIV_B(0)
		esp.PCR.SetUART1_SCLK_CONF_UART1_SCLK_EN(1)
	}
	// wait for Core Clock to ready for configuration
	for bus.GetREG_UPDATE() > 0 {
		riscv.Asm("nop")
	}
}

func (uart *UART) SetBaudRate(baudRate uint32) {
	// based on esp-idf
	max_div := uint32((1 << 12) - 1)
	sclk_div := (pplClockFreq + (max_div * baudRate) - 1) / (max_div * baudRate)
	clk_div := (pplClockFreq << 4) / (baudRate * sclk_div)
	uart.Bus.SetCLKDIV_SYNC_CLKDIV(clk_div >> 4)
	uart.Bus.SetCLKDIV_SYNC_CLKDIV_FRAG(clk_div & 0xf)
	// The SCLK divisor is in the PCR register.
	switch uart.Bus {
	case esp.UART0:
		esp.PCR.SetUART0_SCLK_CONF_UART0_SCLK_DIV_NUM(sclk_div - 1)
	case esp.UART1:
		esp.PCR.SetUART1_SCLK_CONF_UART1_SCLK_DIV_NUM(sclk_div - 1)
	}
}

func (uart *UART) setupPins(config UARTConfig, regs registerSet) {
	config.RX.Configure(PinConfig{Mode: PinInputPullup})
	config.TX.Configure(PinConfig{Mode: PinInputPullup})

	// link TX with GPIO signal X (technical reference manual, GPIO matrix)
	if config.InvertTX {
		config.TX.outFunc().Set(regs.gpioMatrixSignal | esp.GPIO_FUNC_OUT_SEL_CFG_FUNC_OUT_INV_SEL)
	} else {
		config.TX.outFunc().Set(regs.gpioMatrixSignal)
	}
	// link RX with GPIO signal X and route signals via GPIO matrix
	if config.InvertRX {
		inFunc(regs.gpioMatrixSignal).Set(esp.GPIO_FUNC_IN_SEL_CFG_SIG_IN_SEL | uint32(config.RX) | esp.GPIO_FUNC_IN_SEL_CFG_FUNC_IN_INV_SEL)
	} else {
		inFunc(regs.gpioMatrixSignal).Set(esp.GPIO_FUNC_IN_SEL_CFG_SIG_IN_SEL | uint32(config.RX))
	}
}

func (uart *UART) configureInterrupt(intrMapReg *volatile.Register32) {
	// Disable all UART interrupts
	uart.Bus.INT_ENA.ClearBits(0x0ffff)

	// The INTMTX mapping value is the CLIC line number (offset included).
	intrMapReg.Set(cpuInterruptFromUART + clicExtIntrNumOffset)
	onceUart.Do(func() {
		_ = interrupt.New(cpuInterruptFromUART, func(i interrupt.Interrupt) {
			UART0.serveInterrupt(0)
			UART1.serveInterrupt(1)
		}).Enable()
	})
}

func (uart *UART) serveInterrupt(num int) {
	// get interrupt status
	interrutFlag := uart.Bus.INT_ST.Get()
	if (interrutFlag & uartInterrupts) == 0 {
		return
	}

	// block UART interrupts while processing
	uart.Bus.INT_ENA.ClearBits(uartInterrupts)

	if interrutFlag&esp.UART_INT_ENA_RXFIFO_FULL_INT_ENA > 0 {
		for uart.Bus.GetSTATUS_RXFIFO_CNT() > 0 {
			b := uart.Bus.GetFIFO_RXFIFO_RD_BYTE()
			if !uart.Buffer.Put(byte(b & 0xff)) {
				uart.DataOverflowDetected = true
			}
		}
	}
	if interrutFlag&esp.UART_INT_ENA_PARITY_ERR_INT_ENA > 0 {
		uart.ParityErrorDetected = true
	}
	if 0 != interrutFlag&esp.UART_INT_ENA_FRM_ERR_INT_ENA {
		uart.DataErrorDetected = true
	}
	if 0 != interrutFlag&esp.UART_INT_ENA_RXFIFO_OVF_INT_ENA {
		uart.DataOverflowDetected = true
	}
	if 0 != interrutFlag&esp.UART_INT_ENA_GLITCH_DET_INT_ENA {
		uart.DataErrorDetected = true
	}

	// Clear the UART interrupt status
	uart.Bus.INT_CLR.SetBits(interrutFlag)
	uart.Bus.INT_CLR.ClearBits(interrutFlag)
	// Enable interrupts
	uart.Bus.INT_ENA.Set(uartInterrupts)
}

const uart_empty_thresh_default = 10

func (uart *UART) enableTransmitter() {
	uart.Bus.SetCONF0_SYNC_TXFIFO_RST(1)
	uart.Bus.SetCONF0_SYNC_TXFIFO_RST(0)
	uart.Bus.SetCONF1_TXFIFO_EMPTY_THRHD(uart_empty_thresh_default)
}

func (uart *UART) enableReceiver() {
	uart.Bus.SetCONF0_SYNC_RXFIFO_RST(1)
	uart.Bus.SetCONF0_SYNC_RXFIFO_RST(0)
	uart.Bus.SetCONF1_RXFIFO_FULL_THRHD(1)
	uart.Bus.SetINT_ENA_RXFIFO_FULL_INT_ENA(1)
	uart.Bus.SetINT_ENA_FRM_ERR_INT_ENA(1)
	uart.Bus.SetINT_ENA_PARITY_ERR_INT_ENA(1)
	uart.Bus.SetINT_ENA_GLITCH_DET_INT_ENA(1)
	uart.Bus.SetINT_ENA_RXFIFO_OVF_INT_ENA(1)
}

func (uart *UART) writeByte(b byte) error {
	for (uart.Bus.STATUS.Get()&esp.UART_STATUS_TXFIFO_CNT_Msk)>>esp.UART_STATUS_TXFIFO_CNT_Pos >= 128 {
		// Wait until there is space in the transmit buffer.
	}
	uart.Bus.FIFO.Set(uint32(b))
	return nil
}

func (uart *UART) flush() {}
