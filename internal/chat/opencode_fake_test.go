//go:build !windows

package chat

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"time"
)

// openCodeServe runs the server `acp --port` runs beside the protocol, behind
// basic auth with OPENCODE_SERVER_PASSWORD, for the fake's whole life
// (measured on 1.19.4): the question tool's requests and the shell command.
func (f *fake) openCodeServe() {
	i := slices.Index(f.args, "--port")
	if i < 0 {
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+f.args[i+1])
	if err != nil {
		return
	}
	f.answers = make(chan string, 1)
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
		call := fmt.Sprintf("call_q%d", f.asks.Load())
		fmt.Fprint(w, `[{"id":"que_1","questions":[{"question":"Which fruit?","header":"Fruit","options":[{"label":"Apple","description":"red"},{"label":"Pear","description":"soft"}]},{"question":"Which colors?","header":"Colors","options":[{"label":"Red","description":""},{"label":"Blue","description":""}],"multiple":true}],"tool":{"messageID":"m","callID":"`+call+`"}}]`)
	}))
	mux.HandleFunc("POST /question/que_1/{verb}", auth(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.answers <- r.PathValue("verb") + string(body)
		fmt.Fprint(w, "true")
	}))
	mux.HandleFunc("POST /session/{id}/shell", auth(f.openCodeShell))
	go func() { _ = http.Serve(ln, mux) }()
}

// openCodeShell runs a shell command as opencode does: a bash tool call over
// ACP, then the answer once it is done.
func (f *fake) openCodeShell(w http.ResponseWriter, r *http.Request) {
	var body struct{ Agent, Command string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Agent == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	call := "call_shell_" + r.PathValue("id")
	f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": body.Command, "kind": "execute", "status": "pending"})
	f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "status": "completed", "content": []any{
		map[string]any{"type": "content", "content": map[string]any{"type": "text", "text": "ran as " + body.Agent}},
	}})
	fmt.Fprint(w, `{"info":{"role":"assistant"},"parts":[]}`)
}

// openCodeQuestion plays opencode's question tool under ACP (measured on
// 1.19.4): the tool call names the questions, and the request that waits for
// the answer lives on the server, until it is replied to or rejected. The
// fake then writes what came.
func (f *fake) openCodeQuestion() {
	call := fmt.Sprintf("call_q%d", f.asks.Add(1))
	if os.Getenv("OPENCODE_ENABLE_QUESTION_TOOL") != "1" {
		f.update(chunk("agent_message_chunk", "octool:no question tool"))
		return
	}
	if f.answers == nil {
		f.update(chunk("agent_message_chunk", "octool:no server"))
		return
	}
	f.update(map[string]any{"sessionUpdate": "tool_call", "toolCallId": call, "title": "question", "kind": "other", "status": "pending", "rawInput": map[string]any{}})
	f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "title": "question", "kind": "other", "status": "in_progress"})
	select {
	case answer := <-f.answers:
		f.update(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": call, "title": "question", "status": "completed"})
		f.update(chunk("agent_message_chunk", "octool:"+answer))
	case <-time.After(10 * time.Second):
		f.update(chunk("agent_message_chunk", "octool:nobody answered"))
	}
}
