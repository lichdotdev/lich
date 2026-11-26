package codexprofile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnablePlugin(t *testing.T) {
	const key = "lich@lich-plugin"
	for name, tc := range map[string]struct {
		config, want string
		changed      bool
	}{
		"turned off": {
			config:  "model = \"x\"\n\n[plugins.\"lich@lich-plugin\"]\nenabled = false\n\n[plugins.\"other@m\"]\nenabled = false\n",
			want:    "model = \"x\"\n\n[plugins.\"lich@lich-plugin\"]\nenabled = true\n\n[plugins.\"other@m\"]\nenabled = false\n",
			changed: true,
		},
		"indented, with a comment": {
			config:  "[plugins.\"lich@lich-plugin\"]\n  enabled = false # off for now\r\n",
			want:    "[plugins.\"lich@lich-plugin\"]\n  enabled = true\r\n",
			changed: true,
		},
		"already on":             {config: "[plugins.\"lich@lich-plugin\"]\nenabled = true\n"},
		"no switch in its table": {config: "[plugins.\"lich@lich-plugin\"]\n"},
		"another plugin off":     {config: "[plugins.\"other@m\"]\nenabled = false\n"},
		"not installed":          {config: "model = \"x\"\n"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, configFile)
			if err := os.WriteFile(path, []byte(tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			changed, err := EnablePlugin(dir, key)
			if err != nil || changed != tc.changed {
				t.Fatalf("EnablePlugin = (%v, %v), want (%v, nil)", changed, err, tc.changed)
			}
			want := tc.want
			if !tc.changed {
				want = tc.config
			}
			data, _ := os.ReadFile(path)
			if string(data) != want {
				t.Errorf("config.toml =\n%q\nwant\n%q", data, want)
			}
			if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Errorf("mode %v, want the file's own 0600 kept", info.Mode().Perm())
			}
		})
	}
}

func TestEnablePluginWithoutAConfig(t *testing.T) {
	if changed, err := EnablePlugin(t.TempDir(), "lich@lich-plugin"); changed || err != nil {
		t.Fatalf("EnablePlugin = (%v, %v), want nothing to do", changed, err)
	}
}
