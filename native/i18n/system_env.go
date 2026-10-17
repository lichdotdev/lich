//go:build !windows && !(darwin && cgo)

package i18n

import "os"

func systemTag() string { return tagFromEnv(os.Getenv) }
