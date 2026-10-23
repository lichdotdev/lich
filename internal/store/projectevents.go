package store

import "github.com/lichdotdev/lich/api"

type ProjectAdded = api.ProjectAdded

type ProjectClosed = api.ProjectClosed

type ProjectsReordered = api.ProjectsReordered

// SetProjectChanged registers what to run for each change to the project list
// once it is committed. The three events above are what it is told.
//
// It is startup wiring, called before anything serves: lich hands it the
// events hub. Without it a change costs nothing but the notice.
func (s *Service) SetProjectChanged(fn func(name string, data any)) {
	s.projectChanged = fn
}

func (s *Service) projectHasChanged(name string, data any) {
	if s.projectChanged != nil {
		s.projectChanged(name, data)
	}
}
