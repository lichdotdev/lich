package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"testing"
)

func avatarPNGBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 96, 96))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPullsStoreReadsEachAvatarOnceEvenAcrossAChange(t *testing.T) {
	data := avatarPNGBytes(t)
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return data, http.StatusOK }}
	s := newPullsStore(context.Background(), stub.client(t), func() {})
	const url = "https://avatars.githubusercontent.com/u/1?s=96"
	waitFor(t, "the picture", func() bool { return s.avatar("/repo", url) != nil })
	s.stale()
	if s.avatar("/repo", url) == nil {
		t.Fatal("a change dropped the picture")
	}
	if got := stub.calls("project.PullRequestAvatar"); got != 1 {
		t.Errorf("reads = %d, want 1: the address changes with the picture", got)
	}
}

func TestPullsStoreAvatarWithoutAddressAsksNothing(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return nil, http.StatusOK }}
	s := newPullsStore(context.Background(), stub.client(t), func() {})
	if s.avatar("/repo", "") != nil {
		t.Error("an account deleted since has no picture")
	}
	if got := stub.calls("project.PullRequestAvatar"); got != 0 {
		t.Errorf("reads = %d, want none", got)
	}
}

func TestDecodeAuthorAvatar(t *testing.T) {
	if img, err := decodeAuthorAvatar(avatarPNGBytes(t)); err != nil || img == nil {
		t.Fatalf("png: %v", err)
	}
	if _, err := decodeAuthorAvatar([]byte("<html>")); err == nil {
		t.Error("want an error for bytes that are no image")
	}
}
