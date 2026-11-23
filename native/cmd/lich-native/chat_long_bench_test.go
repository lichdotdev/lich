package main

import (
	"strconv"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// longChat is a conversation of turns turns, each a message, a tool call and a
// Markdown answer of several paragraphs and a code block.
func longChat(turns int) []backend.ChatItem {
	answer := strings.Repeat("A paragraph of the answer that wraps over a line or two of the column, with `code` and **bold** in it.\n\n", 5) +
		"```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n"
	var items []backend.ChatItem
	for i := range turns {
		n := strconv.Itoa(i)
		at := novDay(18, 9, 0) + int64(i)*1000
		items = append(items,
			backend.ChatItem{ID: "u" + n, Kind: api.ItemUser, Turn: i + 1, At: at, Text: "question " + n},
			backend.ChatItem{ID: "t" + n, Kind: api.ItemTool, Turn: i + 1, At: at, Tool: &backend.ChatToolCall{Name: "Read", Title: "main.go", State: "done"}},
			backend.ChatItem{ID: "a" + n, Kind: api.ItemAgent, Turn: i + 1, At: at, Text: answer + n},
		)
	}
	return items
}

// benchmarkLongChat frames a settled conversation once per iteration, after
// step moves it, at 50, 200 and 1000 turns: a frame that costs the same at
// every length lays out only what is in view.
func benchmarkLongChat(b *testing.B, step func(u *window, r *frameRig, i int)) {
	for _, turns := range []int{50, 200, 1000} {
		b.Run(strconv.Itoa(turns), func(b *testing.B) {
			u, r := stampFrames(b)
			u.model.chat.history("s1", longChat(turns))
			framesOf(b, u, r, step)
		})
	}
}

func framesOf(b *testing.B, u *window, r *frameRig, step func(u *window, r *frameRig, i int)) {
	for range maxSettleFrames {
		r.now = r.now.Add(frameStep)
		r.frame()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		step(u, r, i)
		r.now = r.now.Add(frameStep)
		r.frame()
	}
}

func BenchmarkLongChatIdle(b *testing.B) {
	benchmarkLongChat(b, func(*window, *frameRig, int) {})
}

// A wheel notch up and down, every frame.
func BenchmarkLongChatScroll(b *testing.B) {
	benchmarkLongChat(b, func(_ *window, r *frameRig, i int) {
		dy := float32(-120)
		if i%2 == 1 {
			dy = 120
		}
		r.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: f32.Pt(300, 300), Scroll: f32.Pt(0, dy)})
	})
}

// The agent writing its last answer, a word a frame.
func BenchmarkLongChatStreaming(b *testing.B) {
	benchmarkLongChat(b, func(u *window, _ *frameRig, i int) {
		u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "live", Kind: api.ItemAgent, Turn: 1 << 20, At: novDay(18, 17, 0), Text: strings.Repeat("word ", i+1)}})
	})
}

// One answer far taller than the view, read at its end.
func BenchmarkLongAnswer(b *testing.B) {
	u, r := stampFrames(b)
	text := strings.Repeat("A paragraph of the answer that wraps over a line or two of the column, with `code` and **bold** in it.\n\n", 2000)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u0", Kind: api.ItemUser, Turn: 1, At: novDay(18, 9, 0), Text: "question"},
		{ID: "a0", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 9, 0), Text: text},
	})
	framesOf(b, u, r, func(*window, *frameRig, int) {})
}
