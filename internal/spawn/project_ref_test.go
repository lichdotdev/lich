package spawn

import (
	"errors"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestProjectResolvesTheCallersOwnAndOneNamed(t *testing.T) {
	svc, _, _ := filer(t)

	own, err := svc.Project(t.Context(), api.ProjectRefOptions{From: "s1"})
	if err != nil || own.ID != "p1" || own.Name != "lich" {
		t.Errorf("own = %+v, %v; want the caller's project", own, err)
	}
	named, err := svc.Project(t.Context(), api.ProjectRefOptions{From: "s1", Project: "revu"})
	if err != nil || named != (api.ProjectRef{ID: "p2", Name: "revu", Path: "/src/revu"}) {
		t.Errorf("named = %+v, %v; want revu", named, err)
	}
}

func TestProjectRefusesOneItCannotResolve(t *testing.T) {
	svc, sessions, _ := filer(t)

	if _, err := svc.Project(t.Context(), api.ProjectRefOptions{From: "s1", Project: "nope"}); err == nil {
		t.Error("resolved a project that is not open")
	}
	sessions.loadErr = errors.New("database is locked")
	if _, err := svc.Project(t.Context(), api.ProjectRefOptions{From: "s1"}); err == nil {
		t.Error("resolved a project in a workspace it could not read")
	}
}
