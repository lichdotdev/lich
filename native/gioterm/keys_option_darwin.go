package gioterm

// optionTypesText: on macOS Option composes text (Option+c is ç), which Cocoa
// hands over as an EditEvent beside the key press, as Ghostty's and
// Terminal.app's default macos-option-as-alt=false does. Encoding the press
// too would type the letter after the composed one.
const optionTypesText = true
