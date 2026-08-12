//go:build esp32_c5_devkitc_1

// This file contains the pin mappings for the Espressif ESP32-C5-DevKitC-1
// development board.
//
// ESP32-C5-DevKitC-1 is an entry-level development board based on the
// ESP32-C5, a RISC-V SoC with dual-band (2.4 & 5 GHz) Wi-Fi 6,
// Bluetooth 5 (LE) and IEEE 802.15.4 connectivity.
//
// - https://docs.espressif.com/projects/esp-dev-kits/en/latest/esp32c5/esp32-c5-devkitc-1/user_guide.html

package machine

// Onboard RGB LED (addressable, WS2812). GPIO27 is also a strapping pin.
const (
	WS2812 = GPIO27
	LED    = GPIO27
)

// BOOT button (also usable as a regular button after boot).
const (
	BUTTON = GPIO28
)

// I2C pins: the board has no dedicated I2C header, these are free GPIOs
// commonly used for I2C in Espressif examples.
const (
	SDA_PIN = GPIO2
	SCL_PIN = GPIO3
)

// UART0 pins (routed to the UART bridge / UART port of the board).
const (
	UART_TX_PIN = GPIO11
	UART_RX_PIN = GPIO12
)
