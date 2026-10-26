package main

// wheelNotch is the pixels Gio reports for one notch of a mouse wheel: AppKit
// counts a wheel without precise deltas in lines, which Gio scales by 10.
const wheelNotch float32 = 10
