package main

// hotkeyCard is one session card as the sidebar lists it: shown is false
// while its block is folded under a header.
type hotkeyCard struct {
	id    string
	shown bool
}

// nextShownCard walks cards from id in step's direction, wrapping, and answers
// the first shown card pick accepts. The card on screen never answers for
// itself, and an id no longer in the list starts from the end the step comes
// from. "" is nowhere to go.
func nextShownCard(cards []hotkeyCard, id string, step int, pick func(id string) bool) string {
	count := len(cards)
	from := -1
	if step < 0 {
		from = count
	}
	for i, c := range cards {
		if c.id == id {
			from = i
		}
	}
	for offset := 1; offset <= count; offset++ {
		c := cards[((from+step*offset)%count+count)%count]
		if c.shown && c.id != id && pick(c.id) {
			return c.id
		}
	}
	return ""
}

// neighborProject steps over the tab strip's ids, the one on screen included,
// wrapping at both ends. Nothing to step from (the active id is not in the
// strip) moves onto the end the step comes from. ok is false for a strip with
// nowhere to go.
func neighborProject(ids []string, active string, step int) (id string, ok bool) {
	if len(ids) < 2 {
		return "", false
	}
	for i, candidate := range ids {
		if candidate == active {
			return ids[((i+step)%len(ids)+len(ids))%len(ids)], true
		}
	}
	if step > 0 {
		return ids[0], true
	}
	return ids[len(ids)-1], true
}
