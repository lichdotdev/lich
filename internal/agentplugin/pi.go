package agentplugin

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/providers"
)

//go:embed pi/lich.ts
var piExtension embed.FS

func PiExtensionPath() (string, error) {
	dir, err := pluginScriptDir(providers.Pi)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lich.ts"), nil
}

func PiExtensionInstalled() bool {
	path, err := PiExtensionPath()
	if err != nil {
		return false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	_, ok := versionOf(body, jsComment)
	return ok
}

func (s *Service) piInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	body, err := piExtension.ReadFile("pi/lich.ts")
	if err != nil {
		return fmt.Errorf("read Pi extension: %w", err)
	}
	body = bytes.ReplaceAll(body, []byte("v0.0.0"), []byte("v"+version))
	body = bytes.ReplaceAll(body, []byte(`"0.0.0"`), []byte(`"`+version+`"`))
	path, err := PiExtensionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := s.installSkill(version, filepath.Join(filepath.Dir(path), "skills"), themeSkill); err != nil {
		return err
	}
	return writeFile(path, body, 0o644)
}

func (s *Service) piInstalledVersion() (string, bool) {
	path, err := PiExtensionPath()
	if err != nil {
		return "", false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return versionOf(body, jsComment)
}
