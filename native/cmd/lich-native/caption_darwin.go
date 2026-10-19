package main

import "gioui.org/unit"

// trafficLightsReserve is the room macOS's traffic lights take at the
// window's left, the inset Electron's hiddenInset title bar leaves them.
const trafficLightsReserve = unit.Dp(78)

func platformCaption() captionLayout {
	return captionLayout{leftReserve: trafficLightsReserve}
}
