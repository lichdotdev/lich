package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type SessionArtifacts = api.SessionArtifacts

type Artifact = api.Artifact

// Artifacts is every open session's artifacts, newest first.
func (c *Client) Artifacts(ctx context.Context) ([]SessionArtifacts, error) {
	return callMethod(ctx, c, api.ArtifactList, api.None{})
}

// AttachArtifact readies artifact name of session from for session
// sessionID's prompt: the path to paste and the notice that goes under it.
func (c *Client) AttachArtifact(ctx context.Context, sessionID, from, name string) (Attachment, error) {
	return callMethod(ctx, c, api.ArtifactAttach, api.AttachArtifactOptions{SessionID: sessionID, From: from, Name: name})
}

// OpenArtifact loads artifact name of session from in session sessionID's
// browser, starting one when it has none.
func (c *Client) OpenArtifact(ctx context.Context, sessionID, from, name string) error {
	_, err := callMethod(ctx, c, api.ArtifactOpen, api.AttachArtifactOptions{SessionID: sessionID, From: from, Name: name})
	return err
}
