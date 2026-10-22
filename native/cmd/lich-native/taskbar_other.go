//go:build !windows

package main

func claimTaskbarIdentity(string) error { return nil }
