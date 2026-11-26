package main

// consoleValue is the program's value as the engine's C side reads it, JSON
// made in the page: WKWebView hands back only what a property list holds, and
// fails a node, a date or a map that the console shows.
const consoleValue = "JSON.stringify(v)"
