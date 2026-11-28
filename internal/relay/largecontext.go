package relay

// largeContextTokens is the context past which a sender is told an independent
// task would cost less in a new session. Every model call a session makes
// re-reads its whole context, so a task handed to one at this size pays for it
// on each of its calls. 200k is the whole standard window of Claude Code's
// models: a session past it has outgrown one, which only a 1M window or a
// configured larger one lets it do, and each of its calls reads several times
// what a new session starts with. Beta users who reported the cost were sending
// to sessions at 451k tokens a call.
const largeContextTokens = 200_000

// largeContext is session id's context in tokens when it is past
// largeContextTokens, 0 otherwise, including where its provider records none.
func (s *Service) largeContext(id string) int {
	tokens, ok := s.term.ContextTokens(id)
	if !ok || tokens <= largeContextTokens {
		return 0
	}
	return tokens
}
