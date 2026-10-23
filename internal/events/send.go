package events

import "github.com/lichdotdev/lich/api"

// Send pushes payload as e through emit: the hub's Emit, or the copy of it a
// service was handed. The compiler holds payload to the type api declares for
// e; a None payload goes out as no data at all.
func Send[T any](emit func(name string, data any), e api.Event[T], payload T) {
	if _, empty := any(payload).(api.None); empty {
		emit(e.Name, nil)
		return
	}
	emit(e.Name, payload)
}
