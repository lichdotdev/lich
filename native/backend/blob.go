package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ErrNoBlob is a side of a diff with no file: the old side of an added one,
// the new side of a deleted one.
var ErrNoBlob = errors.New("not in this revision")

// Blob reads rel, a file of the checkout at path, through /blob for an image
// or PDF preview. ref is "" for the working tree, "HEAD" for the commit under
// it, or an object id, which the backend reads from GitHub when the clone
// does not hold it. A side with no file is ErrNoBlob; a file the backend
// refuses (a type with no preview, one above its size limit) is a
// RemoteError whose Message is the backend's own sentence.
func (c *Client) Blob(ctx context.Context, path, rel, ref string) ([]byte, error) {
	query := url.Values{"path": {path}, "rel": {rel}, "ref": {ref}}
	u := c.url("http", "/blob") + "&" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("blob %s: %w", rel, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("blob %s: %w", rel, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("blob %s: %w", rel, err)
	}
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNoBlob
	case resp.StatusCode != http.StatusOK:
		return nil, &RemoteError{Method: "blob", Message: strings.TrimSpace(string(data)), Status: resp.StatusCode}
	}
	return data, nil
}
