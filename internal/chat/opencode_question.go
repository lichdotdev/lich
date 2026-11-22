package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/terminal"
)

// opencode's question tool runs under ACP once lich switches it on, but its
// questions reach no ACP client: the tool call names them, and the request
// that waits for the answer lives on the HTTP server `opencode acp` runs
// beside the protocol (GET /question, POST /question/{id}/reply and /reject,
// basic auth as opencode; measured on 1.19.4).
const (
	openCodeQuestionTool = "question"
	openCodeUser         = "opencode"
)

// openCodeFindTries and openCodeFindGap bound the wait for the request behind
// a question tool call, which the server may file a moment after the call is
// reported.
const (
	openCodeFindTries = 20
	openCodeFindGap   = 100 * time.Millisecond
)

// newOpenCodeServer picks a free port on the loopback and a password for an
// opencode chat's server; nil when no port could be had, and the chat then
// runs without the question tool.
func newOpenCodeServer() *terminal.OpenCodeServer {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Warn("chat: no port for opencode's server", "err", err)
		return nil
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		slog.Warn("chat: free opencode's port", "err", err)
	}
	return &terminal.OpenCodeServer{Port: port, Password: rand.Text()}
}

type openCodeRequest struct {
	ID        string `json:"id"`
	Questions []struct {
		Question string `json:"question"`
		Header   string `json:"header"`
		Options  []struct {
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"options"`
		Multiple bool `json:"multiple"`
	} `json:"questions"`
	Tool struct {
		CallID string `json:"callID"`
	} `json:"tool"`
}

func (r openCodeRequest) question() api.ChatQuestion {
	q := api.ChatQuestion{Questions: []api.ChatAsk{}}
	for _, in := range r.Questions {
		ask := api.ChatAsk{Header: in.Header, Text: in.Question, Multi: in.Multiple}
		for _, o := range in.Options {
			ask.Options = append(ask.Options, api.ChatOffer{Label: o.Label, Description: o.Description})
		}
		q.Questions = append(q.Questions, ask)
	}
	return q
}

// openCodeAnswers is the reply's answers: per question, the labels picked and
// the person's own words, which opencode takes as one more label.
func openCodeAnswers(r api.ChatQReplied) [][]string {
	answers := make([][]string, len(r.Answers))
	for i, a := range r.Answers {
		answers[i] = append([]string{}, a.Picked...)
		if a.Other != "" {
			answers[i] = append(answers[i], a.Other)
		}
	}
	return answers
}

// openCodeQuestion asks the person a question opencode's question tool call
// carries, once per call. A call read back from a loaded conversation asks
// nothing: its turn is over.
func (d *acpDriver) openCodeQuestion(t acpToolCall) {
	if d.server == nil || t.Title != openCodeQuestionTool || t.Status != "in_progress" || d.c.conv.replaying() {
		return
	}
	d.mu.Lock()
	asked := d.asked[t.ID]
	d.asked[t.ID] = true
	d.mu.Unlock()
	if !asked {
		go d.serveOpenCodeQuestion(t.ID)
	}
}

func (d *acpDriver) serveOpenCodeQuestion(callID string) {
	req, err := d.findOpenCodeRequest(callID)
	if err != nil {
		d.c.conv.notice("The agent asked a question lich could not read from opencode: " + err.Error())
		return
	}
	d.c.question("opencode-"+req.ID, req.question(), func(r api.ChatQReplied) error {
		if r.Declined {
			return d.openCodeCall(http.MethodPost, "/question/"+req.ID+"/reject", map[string]any{}, nil)
		}
		return d.openCodeCall(http.MethodPost, "/question/"+req.ID+"/reply", map[string]any{"answers": openCodeAnswers(r)}, nil)
	})
}

// findOpenCodeRequest is the pending request behind tool call callID.
func (d *acpDriver) findOpenCodeRequest(callID string) (openCodeRequest, error) {
	for range openCodeFindTries {
		var pending []openCodeRequest
		if err := d.openCodeCall(http.MethodGet, "/question", nil, &pending); err != nil {
			return openCodeRequest{}, err
		}
		for _, r := range pending {
			if r.Tool.CallID == callID {
				return r, nil
			}
		}
		time.Sleep(openCodeFindGap)
	}
	return openCodeRequest{}, fmt.Errorf("no pending question for tool call %s", callID)
}

// openCodeCall makes one request of opencode's server for the chat's
// directory, decoding the answer into out when it is not nil.
func (d *acpDriver) openCodeCall(method, path string, body any, out any) error {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	return d.openCodeDo(ctx, method, path, body, out)
}

func (d *acpDriver) openCodeDo(ctx context.Context, method, path string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}
	u := "http://127.0.0.1:" + strconv.Itoa(d.server.Port) + path + "?directory=" + url.QueryEscape(d.c.cwd)
	req, err := http.NewRequestWithContext(ctx, method, u, payload)
	if err != nil {
		return err
	}
	req.SetBasicAuth(openCodeUser, d.server.Password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("opencode answered %s %s with %d: %s", method, path, resp.StatusCode, bytes.TrimSpace(data))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}
