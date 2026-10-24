package api

var (
	ServicesList      = define[None, []Service]("services.List")
	ServicesSave      = define[SaveServiceOptions, Service]("services.Save")
	ServicesDelete    = define[ServiceIDOptions, None]("services.Delete")
	ServicesStart     = define[ServiceIDOptions, None]("services.Start")
	ServicesStop      = define[ServiceIDOptions, None]("services.Stop")
	ServicesForceStop = define[ServiceIDOptions, None]("services.ForceStop")
	ServicesRestart   = define[ServiceIDOptions, None]("services.Restart")
	ServicesStartAll  = define[ProjectIDOptions, None]("services.StartAll")
	ServicesStopAll   = define[ProjectIDOptions, None]("services.StopAll")
	ServicesOutput    = define[ServiceIDOptions, []byte]("services.Output")
)

// The states a Service is in. A service that is not running is stopped when it
// exited on its own with 0 or because it was asked to (Stop, or Ctrl+C typed at
// its console), and failed when it exited on its own with anything else.
const (
	ServiceStopped  = "stopped"
	ServiceRunning  = "running"
	ServiceStopping = "stopping"
	ServiceFailed   = "failed"
)

// ServiceIDOptions names a service by its id.
type ServiceIDOptions struct {
	ID string `json:"id"`
}

// ServiceConfig is a project service as the user saved it: a command lich runs
// in the project's directory, on its own PTY, until it exits. StartWithLich
// starts it whenever the lich backend starts.
type ServiceConfig struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	StartWithLich bool   `json:"startWithLich"`
}

// SaveServiceOptions writes a service: a new one when ID is "", the one ID
// names otherwise. A changed command reaches a running service at its next start.
type SaveServiceOptions struct {
	ID            string `json:"id"`
	ProjectID     string `json:"projectId"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	StartWithLich bool   `json:"startWithLich"`
}

// Service is a service and where its process is. Terminal is the id its PTY
// streams under on /ws while it runs, the one terminal.Write, Resize, Replay
// and SetVisible take. ExitCode is set once a run exited with a status of its
// own (nil while it runs, before it ever ran, or when it was killed). StartedAt
// and StoppedAt are unix milliseconds, 0 for never.
type Service struct {
	ServiceConfig
	ProjectName string `json:"projectName"`
	Terminal    string `json:"terminal"`
	State       string `json:"state"`
	ExitCode    *int   `json:"exitCode,omitempty"`
	StartedAt   int64  `json:"startedAt"`
	StoppedAt   int64  `json:"stoppedAt"`
}
