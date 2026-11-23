package icons

import "testing"

func TestFileIconsPickByNameThenLongestExtension(t *testing.T) {
	cases := []struct {
		set  FileIcons
		path string
		want string
	}{
		{DarkFileIcons, "deploy/Dockerfile", "docker"},
		{DarkFileIcons, "go.mod", "go-mod"},
		{DarkFileIcons, "web/package.json", "nodejs"},
		{DarkFileIcons, "Makefile", "makefile"},
		{DarkFileIcons, ".gitignore", "git"},
		{DarkFileIcons, "README.md", "readme"},
		{DarkFileIcons, "docs/guide.md", "markdown"},
		{DarkFileIcons, "cmd/main.go", "go"},
		{DarkFileIcons, "src/App.TSX", "react_ts"},
		{DarkFileIcons, "types/index.d.ts", "typescript-def"},
		{DarkFileIcons, "a.test.ts", "test-ts"},
		{DarkFileIcons, "a.ts", "typescript"},
		{DarkFileIcons, ".env", "tune"},
		{DarkFileIcons, "src/bashly.yaml", "bashly"},
		{DarkFileIcons, "bashly.yaml", "yaml"},
		{DarkFileIcons, "main.hcl", "hcl"},
		{LightFileIcons, "main.hcl", "hcl_light"},
		{LightFileIcons, "api/openapi.yaml", "openapi_light"},
		{LightFileIcons, ".bun-version", "bun_light"},
		{LightFileIcons, "cmd/main.go", "go"},
	}
	for _, tc := range cases {
		if got := tc.set.For(tc.path); got == nil || got != bitmap("material/"+tc.want+".png") {
			t.Errorf("%v %s: want %s", tc.set, tc.path, tc.want)
		}
	}
}

func TestFileIconsGiveNoneForAnUnknownFile(t *testing.T) {
	for _, set := range []FileIcons{DarkFileIcons, LightFileIcons} {
		for _, p := range []string{"notes.zzz", "noext", "", "trailing."} {
			if set.For(p) != nil {
				t.Errorf("%v %q: got an icon, want none", set, p)
			}
		}
	}
}

func TestEveryFileIconHasItsPNG(t *testing.T) {
	sets, err := fileIconSets()
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range sets {
		for _, names := range []map[string]string{set.FileNames, set.FileExtensions} {
			for key, name := range names {
				if _, err := bitmapFiles.Open("material/" + name + ".png"); err != nil {
					t.Errorf("%s: no PNG for %s", key, name)
				}
			}
		}
	}
}
