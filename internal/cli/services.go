package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// A project's services on both surfaces: `lich service` and the *_service MCP
// tools. Both name a service by its name within a project, the caller's own
// unless one is named, because the id is nothing a person or an agent has seen;
// the methods underneath are the window's own (api.Services*).

// serviceEdit is what an edit changes; a nil field keeps the saved value.
type serviceEdit struct {
	name, command *string
	startWithLich *bool
}

// projectServices is the project a call is aimed at and the services saved in it.
func projectServices(ctx context.Context, c *client, project string) (api.ProjectRef, []api.Service, error) {
	ref, err := callMethod(ctx, c, api.SpawnProject, api.ProjectRefOptions{From: c.sessionID(), Project: project}, shortCall)
	if err != nil {
		return api.ProjectRef{}, nil, err
	}
	all, err := callMethod(ctx, c, api.ServicesList, api.None{}, shortCall)
	if err != nil {
		return api.ProjectRef{}, nil, err
	}
	own := []api.Service{}
	for _, s := range all {
		if s.ProjectID == ref.ID {
			own = append(own, s)
		}
	}
	return ref, own, nil
}

// findService matches the way services.Save keeps names apart: case aside.
func findService(ref api.ProjectRef, list []api.Service, name string) (api.Service, error) {
	names := make([]string, 0, len(list))
	for _, s := range list {
		if strings.EqualFold(s.Name, strings.TrimSpace(name)) {
			return s, nil
		}
		names = append(names, fmt.Sprintf("%q", s.Name))
	}
	if len(names) == 0 {
		return api.Service{}, fmt.Errorf("%s has no services", ref.Name)
	}
	return api.Service{}, fmt.Errorf("no service named %q in %s. Its services: %s", name, ref.Name, strings.Join(names, ", "))
}

func createService(ctx context.Context, c *client, project, name, command string, startWithLich bool) (api.Service, error) {
	ref, err := callMethod(ctx, c, api.SpawnProject, api.ProjectRefOptions{From: c.sessionID(), Project: project}, shortCall)
	if err != nil {
		return api.Service{}, err
	}
	return callMethod(ctx, c, api.ServicesSave, api.SaveServiceOptions{
		ProjectID: ref.ID, Name: name, Command: command, StartWithLich: startWithLich,
	}, shortCall)
}

func editService(ctx context.Context, c *client, project, name string, edit serviceEdit) (api.Service, error) {
	ref, list, err := projectServices(ctx, c, project)
	if err != nil {
		return api.Service{}, err
	}
	s, err := findService(ref, list, name)
	if err != nil {
		return api.Service{}, err
	}
	opts := api.SaveServiceOptions{
		ID: s.ID, ProjectID: s.ProjectID, Name: s.Name, Command: s.Command, StartWithLich: s.StartWithLich,
	}
	if edit.name != nil {
		opts.Name = *edit.name
	}
	if edit.command != nil {
		opts.Command = *edit.command
	}
	if edit.startWithLich != nil {
		opts.StartWithLich = *edit.startWithLich
	}
	return callMethod(ctx, c, api.ServicesSave, opts, shortCall)
}

// actOnService runs one of the id-keyed methods (start, stop, delete) on the
// service name names, and answers with the service as it is afterwards; a
// deleted one as it was.
func actOnService(ctx context.Context, c *client, project, name string, m api.Method[api.ServiceIDOptions, api.None]) (api.Service, error) {
	ref, list, err := projectServices(ctx, c, project)
	if err != nil {
		return api.Service{}, err
	}
	s, err := findService(ref, list, name)
	if err != nil {
		return api.Service{}, err
	}
	if _, err := callMethod(ctx, c, m, api.ServiceIDOptions{ID: s.ID}, shortCall); err != nil {
		return api.Service{}, err
	}
	if m.Name == api.ServicesDelete.Name {
		return s, nil
	}
	_, list, err = projectServices(ctx, c, project)
	if err != nil {
		return api.Service{}, err
	}
	return findService(ref, list, s.Name)
}

// actOnProject runs StartAll or StopAll on a project and answers with its
// services as they are afterwards.
func actOnProject(ctx context.Context, c *client, project string, m api.Method[api.ProjectIDOptions, api.None]) ([]api.Service, error) {
	ref, _, err := projectServices(ctx, c, project)
	if err != nil {
		return nil, err
	}
	if _, err := callMethod(ctx, c, m, api.ProjectIDOptions{ID: ref.ID}, shortCall); err != nil {
		return nil, err
	}
	_, list, err := projectServices(ctx, c, project)
	return list, err
}

// serviceOutput is the service's console at the moment of the call: what the
// run still going has printed so far, or else what its last run printed.
func serviceOutput(ctx context.Context, c *client, project, name string) (string, error) {
	ref, list, err := projectServices(ctx, c, project)
	if err != nil {
		return "", err
	}
	s, err := findService(ref, list, name)
	if err != nil {
		return "", err
	}
	m := api.ServicesOutput
	if s.State == api.ServiceRunning || s.State == api.ServiceStopping {
		m = api.ServicesLiveOutput
	}
	out, err := callMethod(ctx, c, m, api.ServiceIDOptions{ID: s.ID}, shortCall)
	return string(out), err
}

// serviceVerbs are the `lich service` verbs that run one id-keyed method on
// one service, and projectVerbs the ones that run a method on the project.
var (
	serviceVerbs = map[string]api.Method[api.ServiceIDOptions, api.None]{
		"delete": api.ServicesDelete, "start": api.ServicesStart, "stop": api.ServicesStop,
		"force-stop": api.ServicesForceStop, "restart": api.ServicesRestart,
	}
	projectVerbs = map[string]api.Method[api.ProjectIDOptions, api.None]{
		"start-all": api.ServicesStartAll, "stop-all": api.ServicesStopAll,
	}
)

// service is `lich service`: whatever the window's Services footer does to a
// project's services, from a terminal.
func (c *client) service(args []string) error {
	flags := newFlagSet("service")
	project := flags.String("project", "", "project the service is in, by name or by directory path; defaults to the caller's own")
	name := flags.String("name", "", "edit: the service's new name")
	command := flags.String("command", "", "edit: the service's new command")
	startWithLich := flags.Bool("start-with-lich", false, "add, edit: start the service whenever lich starts with its project open")
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	ctx := context.Background()
	verb, rest := flags.Arg(0), flags.Args()
	if len(rest) > 0 {
		rest = rest[1:]
	}
	serviceMethod, isServiceVerb := serviceVerbs[verb]
	projectMethod, isProjectVerb := projectVerbs[verb]

	switch {
	case verb == "list" && len(rest) == 0:
		_, list, err := projectServices(ctx, c, *project)
		return c.printServices(list, err, *asJSON)
	case isProjectVerb && len(rest) == 0:
		list, err := actOnProject(ctx, c, *project, projectMethod)
		return c.printServices(list, err, *asJSON)
	case verb == "output" && len(rest) == 1:
		out, err := serviceOutput(ctx, c, *project, rest[0])
		return c.printOutput(out, err, *asJSON)
	case isServiceVerb && len(rest) == 1:
		s, err := actOnService(ctx, c, *project, rest[0], serviceMethod)
		return c.printService(s, err, *asJSON)
	case verb == "add" && len(rest) == 2:
		s, err := createService(ctx, c, *project, rest[0], rest[1], *startWithLich)
		return c.printService(s, err, *asJSON)
	case verb == "edit" && len(rest) == 1:
		s, err := editService(ctx, c, *project, rest[0], setEdit(flags, *name, *command, *startWithLich))
		return c.printService(s, err, *asJSON)
	}
	return usageError("service")
}

// setEdit keeps only the edit flags given on the command line: an absent one
// leaves the saved value alone, where its zero value would clear it.
func setEdit(flags *flag.FlagSet, name, command string, startWithLich bool) serviceEdit {
	var edit serviceEdit
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "name":
			edit.name = &name
		case "command":
			edit.command = &command
		case "start-with-lich":
			edit.startWithLich = &startWithLich
		}
	})
	return edit
}

func (c *client) printService(s api.Service, err error, asJSON bool) error {
	if err == nil && asJSON {
		return c.emit(s)
	}
	return c.printServices([]api.Service{s}, err, false)
}

func (c *client) printOutput(out string, err error, asJSON bool) error {
	if err != nil {
		return err
	}
	if asJSON {
		return c.emit(out)
	}
	fmt.Fprint(c.stdout, out)
	return nil
}

func (c *client) printServices(list []api.Service, err error, asJSON bool) error {
	if err != nil {
		return err
	}
	if asJSON {
		return c.emit(list)
	}
	if len(list) == 0 {
		fmt.Fprintln(c.stdout, "No services.")
		return nil
	}
	fmt.Fprintln(c.stdout, "service\tstate\tcommand")
	for _, s := range list {
		fmt.Fprintf(c.stdout, "%s\t%s\t%s\n", s.Name, s.State, s.Command)
	}
	return nil
}

func serviceJSON(v any) (string, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode the service: %w", err)
	}
	return string(out), nil
}

const (
	serviceProjectUsage = "The project the service is in, by name or by directory path. " +
		"Leave it out for the project this session runs in."
	serviceNameUsage = "The service, by its name in the project."
)

func projectActionTool(name, description string, m api.Method[api.ProjectIDOptions, api.None]) mcpTool {
	return mcpTool{
		Name: name, Description: description,
		Schema: schema(map[string]any{"project": property("string", serviceProjectUsage)}),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			list, err := actOnProject(ctx, c, args.text("project"), m)
			if err != nil {
				return "", err
			}
			return serviceJSON(list)
		},
	}
}

// serviceByName is the schema of a tool that names one service and nothing else.
func serviceByName() map[string]any {
	return schema(map[string]any{
		"project": property("string", serviceProjectUsage),
		"service": property("string", serviceNameUsage),
	}, "service")
}

func serviceActionTool(name, description string, m api.Method[api.ServiceIDOptions, api.None]) mcpTool {
	return mcpTool{
		Name: name, Description: description, Schema: serviceByName(),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			s, err := actOnService(ctx, c, args.text("project"), args.text("service"), m)
			if err != nil {
				return "", err
			}
			return serviceJSON(s)
		},
	}
}

// serviceTools are the project services as MCP tools, appended to mcpTools.
var serviceTools = []mcpTool{
	{
		Name: "list_services",
		Description: "List a project's services: commands saved once (a dev server, a container, a " +
			"tunnel) that lich runs on a console of their own, outside the session list. Each " +
			"carries its name, command, whether it starts with lich, and its state: `running`, " +
			"`stopping`, `stopped` (exited 0 or was asked to stop) or `failed` (exited on its own " +
			"with another code, in exitCode).",
		Schema:   schema(map[string]any{"project": property("string", serviceProjectUsage)}),
		ReadOnly: true,
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			_, list, err := projectServices(ctx, c, args.text("project"))
			if err != nil {
				return "", err
			}
			return serviceJSON(list)
		},
	},
	{
		Name: "create_service",
		Description: "Save a new service in a project: a command lich runs in the project's " +
			"directory until it exits. It is not started; start_service does that. Its name must " +
			"be free in the project.",
		Schema: schema(map[string]any{
			"project":         property("string", serviceProjectUsage),
			"name":            property("string", "The service's name, unique in the project."),
			"command":         property("string", "The shell command the service runs."),
			"start_with_lich": property("boolean", "Start the service whenever lich starts with its project open."),
		}, "name", "command"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			s, err := createService(ctx, c, args.text("project"), args.text("name"), args.text("command"), args.flag("start_with_lich"))
			if err != nil {
				return "", err
			}
			return serviceJSON(s)
		},
	},
	{
		Name: "edit_service",
		Description: "Change a saved service's name, command or start_with_lich; what is left out " +
			"stays as it is. A running service keeps the command it started with until its next start.",
		Schema: schema(map[string]any{
			"project":         property("string", serviceProjectUsage),
			"service":         property("string", serviceNameUsage),
			"name":            property("string", "The service's new name."),
			"command":         property("string", "The service's new command."),
			"start_with_lich": property("boolean", "Start the service whenever lich starts with its project open."),
		}, "service"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			edit, err := mcpEdit(args)
			if err != nil {
				return "", err
			}
			s, err := editService(ctx, c, args.text("project"), args.text("service"), edit)
			if err != nil {
				return "", err
			}
			return serviceJSON(s)
		},
	},
	serviceActionTool("delete_service",
		"Delete a saved service. A running one is refused: stop it first.", api.ServicesDelete),
	serviceActionTool("start_service",
		"Start a service on its own console in its project's directory. One already running is left alone.",
		api.ServicesStart),
	serviceActionTool("stop_service",
		"Ask a running service to stop, as Ctrl+C at its console does. The answer may still read "+
			"`stopping`: list_services shows when it is down.", api.ServicesStop),
	serviceActionTool("force_stop_service",
		"Kill a running service without waiting for it to agree: for one that ignores stop_service "+
			"or takes too long over it.", api.ServicesForceStop),
	serviceActionTool("restart_service",
		"Stop a running service and start it again once it exits, picking up an edited command; "+
			"one that is not running is started.", api.ServicesRestart),
	projectActionTool("start_all_services",
		"Start every service of a project that is not running.", api.ServicesStartAll),
	projectActionTool("stop_all_services",
		"Ask every running service of a project to stop, as stop_service does.", api.ServicesStopAll),
	{
		Name: "read_service_output",
		Description: "Read a service's console as it is right now, terminal escapes included: what " +
			"a running one has printed so far, or what a stopped or failed one printed in its last " +
			"run. It is one read, not a stream: call it again to see what came since.",
		Schema:   serviceByName(),
		ReadOnly: true,
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			out, err := serviceOutput(ctx, c, args.text("project"), args.text("service"))
			if err != nil {
				return "", err
			}
			if out == "" {
				return "The service has printed nothing yet.", nil
			}
			return out, nil
		},
	},
}

// mcpEdit keeps only the edit arguments the call carried, as setEdit does for flags.
func mcpEdit(args mcpArgs) (serviceEdit, error) {
	var edit serviceEdit
	if _, ok := args["name"]; ok {
		name := args.text("name")
		edit.name = &name
	}
	if _, ok := args["command"]; ok {
		command := args.text("command")
		edit.command = &command
	}
	if _, ok := args["start_with_lich"]; ok {
		start := args.flag("start_with_lich")
		edit.startWithLich = &start
	}
	if edit == (serviceEdit{}) {
		return edit, errors.New("name what to change: name, command or start_with_lich")
	}
	return edit, nil
}
