package store

// ProjectAddedEventName is emitted once AddProject has committed: a project
// opened new, reopened from the history, or reopened at the directory it moved
// to — the same id under another path is how a relocate reaches the store.
// Payload: ProjectAdded. It is not project-opened, which an agent's open
// announces with the whole project and which the web workspace adds a tab
// from; this one only says the row changed, and the window reads it back.
const ProjectAddedEventName = "project-added"

// ProjectClosedEventName is emitted once CloseProject has marked a project
// closed, whoever asked: the window's own tab, an agent, the CLI.
// Payload: ProjectClosed.
const ProjectClosedEventName = "project-closed"

// ProjectsReorderedEventName is emitted once ReorderProjects has written the
// tab order. Payload: ProjectsReordered.
const ProjectsReorderedEventName = "projects-reordered"

// ProjectAdded is the payload of ProjectAddedEventName: the identity as stored.
type ProjectAdded struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// ProjectClosed is the payload of ProjectClosedEventName.
type ProjectClosed struct {
	ID string `json:"id"`
}

// ProjectsReordered is the payload of ProjectsReorderedEventName: every open
// project, in the order that was stored.
type ProjectsReordered struct {
	IDs []string `json:"ids"`
}

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
