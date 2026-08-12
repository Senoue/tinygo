//go:build esp32c5

package machine

import (
	"device/esp"
)

// GPIO matrix signal indices for I2C0 on ESP32-C5.
// Source: ESP-IDF components/soc/esp32c5/include/soc/gpio_sig_map.h
const (
	I2CEXT0_SCL_OUT_IDX = 46
	I2CEXT0_SDA_OUT_IDX = 47
)

var (
	I2C0 = &I2C{
		Bus:     esp.I2C0,
		funcSCL: I2CEXT0_SCL_OUT_IDX,
		funcSDA: I2CEXT0_SDA_OUT_IDX,
		useExt1: false,
	}
)

// enableI2C0PeriphClock enables the I2C peripheral clock via PCR.
// The ESP32-C5 has a single I2C controller; unlike the C6 the PCR fields are
// named I2C_* instead of I2C0_*.
func enableI2C0PeriphClock() {
	// Enable the APB/bus clock for the I2C registers and pulse the reset.
	esp.PCR.SetI2C_CONF_I2C_CLK_EN(1)
	esp.PCR.SetI2C_CONF_I2C_RST_EN(1)
	esp.PCR.SetI2C_CONF_I2C_RST_EN(0)

	// Select the XTAL (40 MHz) source for the I2C functional clock and
	// enable the clock gate, otherwise SCL is never driven.
	esp.PCR.SetI2C_SCLK_CONF_I2C_SCLK_SEL(i2cClkSource)
	esp.PCR.SetI2C_SCLK_CONF_I2C_SCLK_EN(1)
}

// GPIO matrix input-select register fields. The ESP32-C5 SVD names these
// SIG_IN_SEL / FUNC_IN_SEL instead of SEL / IN_SEL.
const (
	gpioInSelRoute = esp.GPIO_FUNC_IN_SEL_CFG_SIG_IN_SEL
	gpioInSelPos   = esp.GPIO_FUNC_IN_SEL_CFG_FUNC_IN_SEL_Pos
)
