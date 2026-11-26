package main

import "os"

func noDisplay() bool { return os.Getenv("DISPLAY") == "" }
