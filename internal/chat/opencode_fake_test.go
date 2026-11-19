//go:build !windows

package chat

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"time"
)

// openCodeQuestion plays opencode's question tool under ACP (measured on
// 1.19.4): the tool call names the questions, and the request waits on the
// server `acp --port` runs, behind basic auth with OPENCODE_SERVER_PASSWORD,
// until it is replied to or rejected. The fake then writes what came.
func (f *fake) openCodeQuestion() {
	got := make(chan string, 1)
	f.asks++
	call := fmt.Sprintf("call_q%d", f.asks)
	if os.Getenv("OPENCODE_ENABLE_QUESTION_TOOL") != "1" {
		f.update(chunk("agent_message_chunk", "octool:no question tool"))
		return
	}
	i := slices.Index(f.args, "--port")
	ln, err := net.Listen("tcp", "127.0.0.1:"+f.args[i+1])
	if err != nil {
		f.update(chunk("agent_message_chunk", "octool:"+err.Error()))
		return
	}
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if user, pw, ok := r.BasicAuth(); !ok || user != "opencode" || pw != os.Getenv("OPENCODE_SERVER_PASSWORD") || r.URL.Query().Get("directory") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /question", auth(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"id":"que_1","questions":[{"question":"Which fruit?","header":"Fruit","options":[{"label":"Apple","description":"red"},{"label":"Pear","description":"soft"}]},{"question":"Which colors?","header":"Colors","options":[{"label":"Red","description":""},{"label":"Blue","description":""}],"multiple":true}],"tool":{"messageID":"m","callID":"`+call+`"}}]`)
	}))
	mux.HandleFunc("POST /question/que_1/{verb}", auth(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- r.PathValue("verb") + string(body)
		fmt.Fprint(w, "true")
	}))
	srv := &http.Server{Handler: mux}
	// Close, not the listener alone: a kept-alive connection would go on
	// serving this question to the next one's reads.
	defer srv.Close()
	go func() { _ = srv.Serve(ln) }()
	f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": "question", "kind": "other", "status": "pending", "rawInput": map[string]any{}})
	f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "title": "question", "kind": "other", "status": "in_progress"})
	select {
	case answer := <-got:
		f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "title": "question", "status": "completed"})
		f.update(chunk("agent_message_chunk", "octool:"+answer))
	case <-time.After(10 * time.Second):
		f.update(chunk("agent_message_chunk", "octool:nobody answered"))
	}
}
