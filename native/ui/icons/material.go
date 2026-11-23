package icons

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"strings"
	"sync"
)

// FileIcons is a scheme's file icons, from Material Icon Theme's manifest:
// in a light theme its light overrides replace the entries they name.
type FileIcons int

const (
	DarkFileIcons FileIcons = iota
	LightFileIcons
)

//go:embed material/material.json
var materialManifest []byte

type fileIconSet struct {
	FileNames      map[string]string `json:"fileNames"`
	FileExtensions map[string]string `json:"fileExtensions"`
}

var fileIconSets = sync.OnceValues(func() ([2]fileIconSet, error) {
	var m struct {
		fileIconSet
		Light fileIconSet `json:"light"`
	}
	if err := json.Unmarshal(materialManifest, &m); err != nil {
		return [2]fileIconSet{}, fmt.Errorf("material.json: %w", err)
	}
	light := fileIconSet{FileNames: maps.Clone(m.FileNames), FileExtensions: maps.Clone(m.FileExtensions)}
	maps.Copy(light.FileNames, m.Light.FileNames)
	maps.Copy(light.FileExtensions, m.Light.FileExtensions)
	return [2]fileIconSet{DarkFileIcons: m.fileIconSet, LightFileIcons: light}, nil
})

// For is the icon of the file at p, or nil when the manifest gives it none.
func (s FileIcons) For(p string) *Bitmap {
	sets, err := fileIconSets()
	if err != nil {
		panic(err)
	}
	name, ok := sets[s].lookup(strings.ToLower(p))
	if !ok {
		return nil
	}
	return bitmap("material/" + name + ".png")
}

// lookup matches p's name first, with its folder for an entry that names one
// (".config/babelrc.json"), then its extensions, longest first: "a.d.ts" is
// "d.ts" before "ts", and ".env" is "env".
func (set fileIconSet) lookup(p string) (string, bool) {
	base := path.Base(p)
	for _, key := range []string{path.Base(path.Dir(p)) + "/" + base, base} {
		if name, ok := set.FileNames[key]; ok {
			return name, true
		}
	}
	for i := range len(base) {
		if base[i] != '.' {
			continue
		}
		if name, ok := set.FileExtensions[base[i+1:]]; ok {
			return name, true
		}
	}
	return "", false
}
