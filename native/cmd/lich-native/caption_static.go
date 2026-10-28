//go:build !linux

package main

import "context"

// followCaption has nothing to follow: the platform's buttons never move.
func followCaption(context.Context, func(captionLayout)) {}
