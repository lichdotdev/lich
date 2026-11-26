//go:build windows || darwin

package main

// The window shows each page's own surface here: no frames.

func drawsOffscreen() bool { return false }

func (p *page) frameShown() {}
