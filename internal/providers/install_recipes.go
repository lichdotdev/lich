package providers

// The vendors' own install commands, each from the page named beside it (read
// 2026-11-19). Only what installs as the user without a password is here: the
// apt, dnf and pacman routes, and anything that asks for an administrator,
// are left to the install page.

// curlPipe is `curl <args> | <shell>`, the vendor's script run as the vendor
// spells it.
func curlPipe(shell, curl string) recipe {
	return recipe{needs: []string{shell, "curl"}, args: []string{shell, "-c", curl + " | " + shell}}
}

// powershell runs a vendor's PowerShell one-liner with no profile and no
// prompt to answer.
func powershell(command string) recipe {
	return recipe{needs: []string{"powershell"}, args: []string{"powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command}}
}

func npmGlobal(pkg string) recipe {
	return recipe{needs: []string{"npm"}, args: []string{"npm", "install", "-g", pkg}}
}

// unixRecipes are the installs Linux and macOS share.
var unixRecipes = map[string]recipe{
	// https://code.claude.com/docs/en/setup
	Claude: curlPipe("bash", "curl -fsSL https://claude.ai/install.sh"),
	// https://learn.chatgpt.com/docs/codex/cli
	Codex: curlPipe("sh", "curl -fsSL https://chatgpt.com/codex/install.sh"),
	// https://antigravity.google/docs/install?tab=cli
	Antigravity: curlPipe("bash", "curl -fsSL https://antigravity.google/cli/install.sh"),
	// https://opencode.ai/docs/
	OpenCode: curlPipe("bash", "curl -fsSL https://opencode.ai/v2/install"),
	// https://github.com/can1357/oh-my-pi (README)
	OMP: curlPipe("sh", "curl -fsSL https://omp.sh/install"),
	// https://cursor.com/docs/cli/installation
	Cursor: curlPipe("bash", "curl https://cursor.com/install -fsS"),
	// https://kiro.dev/downloads/
	Kiro: curlPipe("bash", "curl -fsSL https://cli.kiro.dev/install"),
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
		Claude: powershell("irm https://claude.ai/install.ps1 | iex"),
		// https://github.com/openai/codex (README)
		Codex: powershell("irm https://chatgpt.com/codex/install.ps1 | iex"),
		// https://antigravity.google/docs/install?tab=cli
		Antigravity: powershell("irm https://antigravity.google/cli/install.ps1 | iex"),
		// opencode documents no Windows script; npm is one of its routes.
		// https://opencode.ai/docs/
		OpenCode: npmGlobal("@opencode/cli"),
		// https://github.com/can1357/oh-my-pi (README)
		OMP: powershell("irm https://omp.sh/install.ps1 | iex"),
		// https://github.com/charmbracelet/crush
		Crush: npmGlobal("@charmland/crush"),
		// https://cursor.com/docs/cli/installation
		Cursor: powershell("irm 'https://cursor.com/install?win32=true' | iex"),
	},
}

// installGaps is why a provider has no recipe on an OS, by OS and provider
// id; docs/ceilings.md and the README's provider table say the same.
var installGaps = map[string]map[string]string{
	"windows": {
		// https://kiro.dev/downloads/ lists the bash line for Windows too, and
		// no page shows a PowerShell install.
		Kiro: "Kiro CLI documents no Windows install lich can run",
	},
}
