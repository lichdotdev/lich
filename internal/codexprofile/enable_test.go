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

func TestSetPluginOff(t *testing.T) {
	const key = "ponytail@ponytail"
	for name, tc := range map[string]struct {
		config, want string
		changed      bool
	}{
		"on":                {config: "[plugins.\"ponytail@ponytail\"]\nenabled = true\n", want: "[plugins.\"ponytail@ponytail\"]\nenabled = false\n", changed: true},
		"on with no line":   {config: "[plugins.\"ponytail@ponytail\"]\n\n[x]\n", want: "[plugins.\"ponytail@ponytail\"]\nenabled = false\n\n[x]\n", changed: true},
		"already off":       {config: "[plugins.\"ponytail@ponytail\"]\nenabled = false\n"},
		"another plugin on": {config: "[plugins.\"other@m\"]\nenabled = true\n"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, configFile)
			if err := os.WriteFile(path, []byte(tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			changed, err := SetPlugin(dir, key, false)
			if err != nil || changed != tc.changed {
				t.Fatalf("SetPlugin = (%v, %v), want (%v, nil)", changed, err, tc.changed)
			}
			got, _ := os.ReadFile(path)
			want := tc.want
			if !tc.changed {
				want = tc.config
			}
			if string(got) != want {
				t.Fatalf("config.toml = %q, want %q", got, want)
			}
		})
	}
}

func TestPluginsAndMCPServers(t *testing.T) {
	dir := t.TempDir()
	config := "[mcp_servers.youtrack]\nurl = \"https://yt/mcp\" # remote\n\n[mcp_servers.youtrack.env]\nA = \"b\"\n\n" +
		"[mcp_servers.'local']\ncommand = 'bunx'\nargs = [\"x\"]\n\n" +
		"[plugins.\"a@m\"]\nenabled = false\n\n[plugins.\"b@m\"]\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	plugins, err := Plugins(dir)
	if err != nil || len(plugins) != 2 || plugins["a@m"] || !plugins["b@m"] {
		t.Fatalf("Plugins = %v, %v; want a@m off, b@m on", plugins, err)
	}
	servers, err := MCPServers(dir)
	want := map[string]MCPServer{"youtrack": {URL: "https://yt/mcp"}, "local": {Command: "bunx"}}
	if err != nil || len(servers) != len(want) || servers["youtrack"] != want["youtrack"] || servers["local"] != want["local"] {
		t.Fatalf("MCPServers = %v, %v; want %v", servers, err, want)
	}
}
