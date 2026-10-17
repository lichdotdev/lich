package claudeprofile

import (
	"os"
	"path/filepath"
	"testing"
)

// trustFixture is a machine with Personal's global config and a profile's,
// each holding the projects table given.
func trustFixture(t *testing.T, personalProjects, profileProjects string) (personalFile, profileDir string) {
	t.Helper()
	personal := machine(t)
	personalFile = filepath.Join(filepath.Dir(personal), ".claude.json")
	mustWrite(t, personalFile, `{"theme":"dark","projects":`+personalProjects+`}`)
	profileDir = t.TempDir()
	mustWrite(t, filepath.Join(profileDir, ".claude.json"),
		`{"numStartups":3,"projects":{"/own":{"allowedTools":["Bash"]}`+profileProjects+`}}`)
	return personalFile, profileDir
}

type globalConfigDoc struct {
	NumStartups int                       `json:"numStartups"`
	Projects    map[string]map[string]any `json:"projects"`
}

func readBytes(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInheritTrustCarriesPersonalsAnswerIntoAProfile(t *testing.T) {
	for _, tt := range []struct{ name, personal string }{
		{"the checkout itself", `{"/wt/app":{"hasTrustDialogAccepted":true}}`},
		{"the main checkout of a worktree", `{"/src/app":{"hasTrustDialogAccepted":true}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, profile := trustFixture(t, tt.personal, "")
			if err := InheritTrust(profile, "/wt/app", "/src/app"); err != nil {
				t.Fatal(err)
			}
			var doc globalConfigDoc
			readJSON(t, filepath.Join(profile, ".claude.json"), &doc)
			if doc.Projects["/wt/app"]["hasTrustDialogAccepted"] != true {
				t.Errorf("a folder trusted in Personal is not trusted in the profile: %v", doc.Projects)
			}
			if doc.NumStartups != 3 || doc.Projects["/own"]["allowedTools"] == nil {
				t.Errorf("the profile's own config did not survive: %+v", doc)
			}
		})
	}
}

// Every case here must leave both files as they were: a folder nobody trusted,
// a profile that already trusts it, Personal itself, and a spawn outside any
// checkout.
func TestInheritTrustWritesOnlyAnAnswerThePersonGave(t *testing.T) {
	for _, tt := range []struct {
		name, personal, profile, top, origin string
		personalOnly                         bool
	}{
		{name: "trusted nowhere", personal: `{"/wt/app":{"hasTrustDialogAccepted":false},"/x":{"hasTrustDialogAccepted":true}}`,
			top: "/wt/app", origin: "/src/app"},
		{name: "already trusted in the profile", personal: `{"/wt/app":{"hasTrustDialogAccepted":true}}`,
			profile: `,"/wt/app":{"hasTrustDialogAccepted":true}`, top: "/wt/app", origin: "/src/app"},
		{name: "main checkout trusted in the profile", personal: `{"/wt/app":{"hasTrustDialogAccepted":true}}`,
			profile: `,"/src/app":{"hasTrustDialogAccepted":true}`, top: "/wt/app", origin: "/src/app"},
		{name: "Personal", personal: `{"/src/app":{"hasTrustDialogAccepted":true}}`,
			top: "/wt/app", origin: "/src/app", personalOnly: true},
		{name: "not in a checkout", personal: `{"/src/app":{"hasTrustDialogAccepted":true}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			personalFile, profile := trustFixture(t, tt.personal, tt.profile)
			profileFile := filepath.Join(profile, ".claude.json")
			before := map[string]string{personalFile: readBytes(t, personalFile), profileFile: readBytes(t, profileFile)}
			dir := profile
			if tt.personalOnly {
				dir = ""
			}
			if err := InheritTrust(dir, tt.top, tt.origin); err != nil {
				t.Fatal(err)
			}
			for path, data := range before {
				if after := readBytes(t, path); after != data {
					t.Errorf("%s was rewritten:\n%s", path, after)
				}
			}
		})
	}
}

// A profile with no global config yet is one Claude Code has not set up; a
// file holding only a projects table would skip its onboarding.
func TestInheritTrustWritesNoConfigThatIsNotThere(t *testing.T) {
	trustFixture(t, `{"/src/app":{"hasTrustDialogAccepted":true}}`, "")
	empty := t.TempDir()
	if err := InheritTrust(empty, "/src/app", "/src/app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(empty, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("a global config was created: %v", err)
	}
}

func TestInheritTrustRefusesAConfigItCannotRead(t *testing.T) {
	personalFile, profile := trustFixture(t, `{}`, "")
	mustWrite(t, personalFile, `{"projects":`)
	if err := InheritTrust(profile, "/src/app", "/src/app"); err == nil {
		t.Error("a malformed Personal config was read as untrusted")
	}
	mustWrite(t, filepath.Join(profile, ".claude.json"), `{"projects":[]}`)
	if err := InheritTrust(profile, "/src/app", "/src/app"); err == nil {
		t.Error("a malformed profile config was read as untrusted")
	}
}
