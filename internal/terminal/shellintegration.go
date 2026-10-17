package terminal

import "github.com/lichdotdev/lich/internal/shellintegration"

// SetShellIntegrationDir names the directory shellintegration.Install wrote
// the shell scripts to; "" leaves shell sessions without them.
//
// Startup wiring, called before anything spawns.
func (s *Service) SetShellIntegrationDir(dir string) {
	s.shellIntegrationDir = dir
}

// withShellIntegration starts a shell session's shell with lich's prompt
// marks (internal/shellintegration). A provider's session is left as it is:
// its program draws its own screen and runs no shell rc.
func withShellIntegration(spec ptySpec, kind, dir string) ptySpec {
	if kind != KindShell || dir == "" {
		return spec
	}
	spec.args, spec.env = shellintegration.Apply(spec.bin, spec.args, spec.env, dir)
	return spec
}
