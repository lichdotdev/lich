package providers

import "path/filepath"

// The vendors' own install commands, each from the page named beside it (read
// 2026-11-19). Only what installs as the user without a password is here: the
// apt, dnf and pacman routes, and anything that asks for an administrator,
// are left to the install page.

// curlPipe is `curl <args> | <shell>`, the vendor's script run as the vendor
// spells it.
func curlPipe(shell, curl string, dir installDir) recipe {
	return recipe{needs: []string{shell, "curl"}, args: []string{shell, "-c", curl + " | " + shell}, dir: dir}
}

// powershell runs a vendor's PowerShell one-liner with no profile and no
// prompt to answer.
func powershell(command string, dir installDir) recipe {
	return recipe{needs: []string{"powershell"}, args: []string{"powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command}, dir: dir}
}

// The folders the scripts write their binary into (read 2026-11-19): every
// unix script but opencode's uses ~/.local/bin, which no shell is sure to
// have on PATH; noDir is a script that was not read for one.
var (
	localBin    = installDir{rel: filepath.Join(".local", "bin")}
	opencodeBin = installDir{rel: filepath.Join(".opencode", "bin")}
	noDir       = installDir{}
)

func npmGlobal(pkg string) recipe {
	return recipe{needs: []string{"npm"}, args: []string{"npm", "install", "-g", pkg}}
}

// unixRecipes are the installs Linux and macOS share.
var unixRecipes = map[string]recipe{
	// https://code.claude.com/docs/en/setup
	Claude: curlPipe("bash", "curl -fsSL https://claude.ai/install.sh", localBin),
	// https://learn.chatgpt.com/docs/codex/cli
	Codex: curlPipe("sh", "curl -fsSL https://chatgpt.com/codex/install.sh", localBin),
	// https://antigravity.google/docs/getting-started?tab=cli
	Antigravity: curlPipe("bash", "curl -fsSL https://antigravity.google/cli/install.sh", localBin),
	// https://opencode.ai/docs/
	OpenCode: curlPipe("bash", "curl -fsSL https://opencode.ai/v2/install", opencodeBin),
	// https://github.com/can1357/oh-my-pi (README)
	OMP: curlPipe("sh", "curl -fsSL https://omp.sh/install", localBin),
	// https://cursor.com/docs/cli/installation
	Cursor: curlPipe("bash", "curl https://cursor.com/install -fsS", localBin),
	// https://kiro.dev/downloads/
	Kiro: curlPipe("bash", "curl -fsSL https://cli.kiro.dev/install", localBin),
}

func with(base map[string]recipe, id string, r recipe) map[string]recipe {
	m := map[string]recipe{id: r}
	for k, v := range base {
		if k != id {
			m[k] = v
		}
	}
	return m
}

// recipes is each provider's unattended install, by OS and provider id.
var recipes = map[string]map[string]recipe{
	// Crush has no install script; its README lists npm for Linux.
	// https://github.com/charmbracelet/crush
	"linux": with(unixRecipes, Crush, npmGlobal("@charmland/crush")),
	// https://github.com/charmbracelet/crush
	"darwin": with(unixRecipes, Crush, recipe{needs: []string{"brew"}, args: []string{"brew", "install", "charmbracelet/tap/crush"}}),
	"windows": {
		// https://code.claude.com/docs/en/setup
		Claude: powershell("irm https://claude.ai/install.ps1 | iex", noDir),
		// https://github.com/openai/codex (README)
		Codex: powershell("irm https://chatgpt.com/codex/install.ps1 | iex", noDir),
		// https://antigravity.google/docs/getting-started?tab=cli
		Antigravity: powershell("irm https://antigravity.google/cli/install.ps1 | iex", installDir{env: "LOCALAPPDATA", rel: filepath.Join("agy", "bin")}),
		// https://github.com/can1357/oh-my-pi (README)
		OMP: powershell("irm https://omp.sh/install.ps1 | iex", installDir{env: "LOCALAPPDATA", rel: "omp"}),
		// https://github.com/charmbracelet/crush
		Crush: npmGlobal("@charmland/crush"),
		// https://cursor.com/docs/cli/installation
		Cursor: powershell("irm 'https://cursor.com/install?win32=true' | iex", installDir{env: "LOCALAPPDATA", rel: "cursor-agent"}),
	},
}

// installGaps is why a provider has no recipe on an OS, by OS and provider
// id; docs/ceilings.md and the README's provider table say the same.
var installGaps = map[string]map[string]string{
	"windows": {
		// https://opencode.ai/docs/: "Windows package managers are not
		// supported", and the page offers a zip.
		OpenCode: "opencode installs on Windows from the zip on its page",
		// https://kiro.dev/cli/ gives Windows the same curl | bash line, which
		// runs only under bash (WSL or Git Bash).
		Kiro: "Kiro CLI's Windows install is its curl | bash line, which needs bash (WSL or Git Bash)",
	},
}
