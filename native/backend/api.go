package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type Project = api.Project

type Session = api.Session

type DiffStats = api.DiffStats

// LoadState returns every open project with its sessions.
func (c *Client) LoadState(ctx context.Context) ([]Project, error) {
	return callMethod(ctx, c, api.StoreLoadState, api.None{})
}

// AddProject opens path as a project under id.
func (c *Client) AddProject(ctx context.Context, id, name, path string) error {
	_, err := callMethod(ctx, c, api.StoreAddProject, api.AddProjectOptions{ID: id, Name: name, Path: path})
	return err
}

// AddSession records a new session row; Start then spawns its process.
func (c *Client) AddSession(ctx context.Context, projectID, sessionID, label, kind, path string, nextSeq int) error {
	_, err := callMethod(ctx, c, api.StoreAddSession, api.AddSessionOptions{ProjectID: projectID, SessionID: sessionID, Label: label, Kind: kind, Path: path, NextSeq: nextSeq})
	return err
}

// SetActiveSession records which session a project shows.
func (c *Client) SetActiveSession(ctx context.Context, projectID, sessionID string) error {
	_, err := callMethod(ctx, c, api.StoreSetActiveSession, api.ActiveSessionOptions{ProjectID: projectID, SessionID: sessionID})
	return err
}

// SetShown tells the backend which session's card is on screen, "" for none.
func (c *Client) SetShown(ctx context.Context, sessionID string) error {
	_, err := callMethod(ctx, c, api.RelaySetShown, api.ShownSessionOptions{SessionID: sessionID})
	return err
}

// Replay asks for the tail of a session's output, which arrives on the
// Terminal stream as a replay frame: the output frames before it are in the
// tail and the ones after it are not. It reports whether one is coming, false
// for a session not running, which has no output yet.
func (c *Client) Replay(ctx context.Context, id string) (bool, error) {
	return callMethod(ctx, c, api.TerminalReplay, api.SessionIDOptions{ID: id})
}

type ConversationReported = api.SessionConversation

// Start spawns the session's process at cols x rows; a no-op when it already
// runs. resume is a provider conversation id to reopen, "" to start fresh.
func (c *Client) Start(ctx context.Context, id, projectID, cwd, kind, resume string, cols, rows int) error {
	_, err := callMethod(ctx, c, api.TerminalStart, api.StartOptions{
		ID: id, ProjectID: projectID, Cwd: cwd, Kind: kind, Resume: resume, Cols: cols, Rows: rows,
	})
	return err
}

// Fork spawns the session's process branching the provider conversation
// parent: the conversation is copied into one of its own, and the card that
// runs parent goes on untouched. A no-op when the session already runs.
func (c *Client) Fork(ctx context.Context, id, projectID, cwd, kind, parent string, cols, rows int) error {
	_, err := callMethod(ctx, c, api.TerminalStart, api.StartOptions{
		ID: id, ProjectID: projectID, Cwd: cwd, Kind: kind, Resume: parent, Fork: true, Cols: cols, Rows: rows,
	})
	return err
}

// Resize sets the session's PTY size.
func (c *Client) Resize(ctx context.Context, id string, cols, rows int) error {
	_, err := callMethod(ctx, c, api.TerminalResize, api.ResizeOptions{ID: id, Cols: cols, Rows: rows})
	return err
}

// SetVisible tells the backend whether the session is on screen, which sets
// how long it batches output before sending.
func (c *Client) SetVisible(ctx context.Context, id string, visible bool) error {
	_, err := callMethod(ctx, c, api.TerminalSetVisible, api.SetVisibleOptions{ID: id, Visible: visible})
	return err
}

// Diff returns a checkout's branch and change counts.
func (c *Client) Diff(ctx context.Context, path string) (DiffStats, error) {
	return callMethod(ctx, c, api.ProjectDiff, api.PathOptions{Path: path})
}

// DiffText returns one unified diff of a checkout's staged, unstaged and
// untracked changes against HEAD.
func (c *Client) DiffText(ctx context.Context, path string) (string, error) {
	return callMethod(ctx, c, api.ProjectDiffText, api.PathOptions{Path: path})
}

// FileLines returns lines from..to (1-based, inclusive) of rel at ref, "" for
// the working tree. The backend caps one answer at 500 lines, so a caller
// wanting more asks again from where the answer ended.
func (c *Client) FileLines(ctx context.Context, path, rel, ref string, from, to int) ([]string, error) {
	return callMethod(ctx, c, api.ProjectFileLines, api.FileLinesOptions{Path: path, Rel: rel, Ref: ref, From: from, To: to})
}

type RevertLine = api.RevertLine

type RevertResult = api.RevertResult

// RevertLines puts these changed lines of rel back to HEAD, in the index too
// when it holds them. It fails when the file moved on since the diff was
// drawn.
func (c *Client) RevertLines(ctx context.Context, path, rel string, lines []RevertLine) (RevertResult, error) {
	return callMethod(ctx, c, api.ProjectRevertLines, api.RevertLinesOptions{Path: path, Rel: rel, Lines: lines})
}

// GetSetting returns a setting's value, "" when unset. scope is a project id,
// "" for a global setting.
func (c *Client) GetSetting(ctx context.Context, key, scope string) (string, error) {
	return callMethod(ctx, c, api.StoreGetSetting, api.GetSettingOptions{Key: key, ProjectID: scope})
}

// HomeProject is the project rooted at the user's home directory, the one the
// windows pin as Home. It is not opened by asking.
func (c *Client) HomeProject(ctx context.Context) (Project, error) {
	p, err := callMethod(ctx, c, api.ProjectHome, api.None{})
	if err != nil || p == nil {
		return Project{}, err
	}
	return Project{ID: p.ID, Name: p.Name, Path: p.Path}, nil
}
