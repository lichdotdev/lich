//go:build !linux && !windows && !darwin

package main

func platformCaption() captionLayout {
	left, right := parseButtonLayout(defaultButtonLayout)
	return captionLayout{left: left, right: right, style: captionStyleGNOME}
}
