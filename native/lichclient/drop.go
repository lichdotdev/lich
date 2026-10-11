package lichclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// UploadDrop stores one dropped file's bytes as a copy kept under session
// sessionID (internal/drop Upload) and answers with the copy's path and the
// line that goes under it at the prompt. name is the file's base name.
func (c *Client) UploadDrop(ctx context.Context, sessionID, name string, body io.Reader) (Attachment, error) {
	u, err := url.Parse(c.url("http", "/drop"))
	if err != nil {
		return Attachment{}, fmt.Errorf("drop upload: %w", err)
	}
	q := u.Query()
	q.Set("session", sessionID)
	q.Set("name", name)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return Attachment{}, fmt.Errorf("drop upload: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Attachment{}, fmt.Errorf("drop upload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Attachment{}, fmt.Errorf("drop upload: %s (HTTP %d)", strings.TrimSpace(string(msg)), resp.StatusCode)
	}
	var a Attachment
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return Attachment{}, fmt.Errorf("drop upload: decode: %w", err)
	}
	if a.Path == "" {
		return Attachment{}, errors.New("drop upload: no path in response")
	}
	return a, nil
}
