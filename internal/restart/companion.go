package restart

import (
	"fmt"
	"os"
	"path/filepath"
)

// NativeEnv pins the native window to launch, by path. `go run` and a native
// build of one's own have no lich-native beside the lich binary.
const NativeEnv = "LICH_NATIVE"

// NativeBinary is the native window's executable, shipped beside lich's.
const NativeBinary = "lich-native"

// NativeExecutable is the native window's binary: the one NativeEnv pins,
// else lich-native beside this lich binary. A missing one is an error that
// says how to get one.
func NativeExecutable(env func(string) string) (string, error) {
	return Companion(env, NativeEnv, NativeBinary, "build it from native/ (native/slice.sh)")
}

// Companion is the executable pinned by envName, else binary beside this lich
// binary; howToBuild finishes the sentence a missing one is reported with.
func Companion(env func(string) string, envName, binary, howToBuild string) (string, error) {
	if pinned := env(envName); pinned != "" {
		if _, err := os.Stat(pinned); err != nil {
			return "", fmt.Errorf("%s=%s: %w", envName, pinned, err)
		}
		return pinned, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find the lich binary: %w", err)
	}
	beside := filepath.Join(filepath.Dir(exe), executableName(binary))
	if _, err := os.Stat(beside); err != nil {
		return "", fmt.Errorf("no %s beside %s: %s, or point %s at one", binary, exe, howToBuild, envName)
	}
	return beside, nil
}

// executableName is binary's file name on this OS.
func executableName(binary string) string { return binary + exeExt }
