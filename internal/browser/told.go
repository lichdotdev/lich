package browser

import "slices"

// tell keeps list as the one the agent heard last. A read the agent asked
// for always answers with the whole list, so one it lost from its context
// comes back by asking.
func (s *Service) tell(sess *session, list []Interactive) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess.told = list
}

// leaveOutTold drops from an action's page answer the element list the
// agent's last page answer gave it: 98 of 323 page answers repeated it whole,
// 13% of what they weighed (measured 2026-11-07).
func (s *Service) leaveOutTold(sess *session, info *PageInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(info.Interactive) > 0 && slices.EqualFunc(sess.told, info.Interactive, sameShown) {
		info.Interactive, info.InteractiveSame = nil, true
		return
	}
	sess.told = info.Interactive
}

// sameShown is whether a and b read the same in a page answer, which never
// shows an element's box.
func sameShown(a, b Interactive) bool {
	return a.Index == b.Index && a.Tag == b.Tag && a.Type == b.Type && a.Role == b.Role &&
		a.Text == b.Text && a.Href == b.Href && sameFlag(a.Filled, b.Filled) && sameFlag(a.Checked, b.Checked)
}

func sameFlag(a, b *bool) bool {
	return (a == nil) == (b == nil) && (a == nil || *a == *b)
}
