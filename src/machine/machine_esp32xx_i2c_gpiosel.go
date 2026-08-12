//go:build (esp32c3 || esp32c6 || esp32s3) && !m5stamp_c3

package machine

import "device/esp"

// GPIO matrix input-select register fields (older-style SVD field names).
const (
	gpioInSelRoute = esp.GPIO_FUNC_IN_SEL_CFG_SEL
	gpioInSelPos   = esp.GPIO_FUNC_IN_SEL_CFG_IN_SEL_Pos
)
