package providers

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func onPath(bins ...string) func(string) (string, error) {
	return func(bin string) (string, error) {
		if slices.Contains(bins, bin) {
			return "/usr/bin/" + bin, nil
		}
		return "", errors.New("not found")
	}
}

func TestToolInstallAsksTheFirstPackageManagerThatHasTheTool(t *testing.T) {
	cases := []struct {
		path       []string
		tool, want string
	}{
		{[]string{"pacman"}, "gh", "sudo pacman -S --needed github-cli"},
		{[]string{"apt", "brew"}, "gh", "sudo apt install gh"},
		{[]string{"apt", "brew"}, "glab", "brew install glab"},
		{[]string{"apt"}, "glab", ""},
		{[]string{"winget"}, "git", "winget install --exact --id Git.Git"},
		{nil, "git", ""},
		{[]string{"pacman"}, "svn", ""},
		{[]string{"zypper"}, "jq", "sudo zypper install jq"},
		{[]string{"winget"}, "jq", "winget install --exact --id jqlang.jq"},
	}
	for _, c := range cases {
		s := &Service{lookPath: onPath(c.path...)}
		got, err := s.ToolInstall(context.Background(), api.BinaryOptions{Bin: c.tool})
		if err != nil || got != c.want {
			t.Errorf("%s with %v: %q, %v; want %q", c.tool, c.path, got, err, c.want)
		}
	}
}
