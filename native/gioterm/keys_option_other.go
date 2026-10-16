//go:build !darwin

package gioterm

// optionTypesText: elsewhere Alt is Meta, and Alt+letter is encoded as an
// escape-prefixed key.
const optionTypesText = false
