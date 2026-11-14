package cli

import (
	"fmt"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/extensions"
)

// extension checks an extension before it is published. It talks to no lich:
// an author runs it beside a build, and the catalog's checks run it in CI.
func (c *client) extension(args []string) error {
	flags := newFlagSet("extension")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 || flags.Arg(0) != "check" {
		return usageError("extension")
	}
	path := flags.Arg(1)
	m, err := extensions.Check(path)
	if err != nil {
		return fmt.Errorf("%s fails: %w", path, err)
	}
	if m.Description == "" {
		fmt.Fprintf(c.stderr, "lich: warning: %s sets no description, so the catalog will not list it\n", extension.ManifestFile)
	}
	fmt.Fprintf(c.stdout, "%s passes: %s %s\n", path, m.ID, m.Version)
	return nil
}
