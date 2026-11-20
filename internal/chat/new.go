package chat

import (
	"context"
	"errors"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// New answers api.ChatNew: the chat's process is started again with nothing
// to resume, which every provider opens as an empty conversation, on the
// model, effort and mode it ran. The conversation it leaves stays the
// provider's, for the palette's History to reopen; the session forgets it
// before the new process starts, so a restart before the first message cannot
// bring it back.
func (s *Service) New(ctx context.Context, opts api.SessionIDOptions) (api.None, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	release, err := s.claim(c.id)
	if err != nil {
		return api.None{}, err
	}
	defer release()
	if s.running(c.id) != c {
		return api.None{}, errors.New("the chat restarted meanwhile; ask again")
	}
	if c.turnRunning() || s.term.TurnOpen(c.id) {
		return api.None{}, errBusy
	}
	carry := c.currents()
	c.stop()
	if err := s.term.BindConversation(c.id, ""); err != nil {
		return api.None{}, err
	}
	if _, err := s.start(ctx, c.id, c.projectID, c.kind, c.cwd, carry); err != nil {
		return api.None{}, err
	}
	// The window reads the conversation again, empty now.
	events.Send(s.hub.Emit, api.SessionViewEvent, api.SessionView{ID: c.id, View: api.ViewChat})
	return api.None{}, nil
}
