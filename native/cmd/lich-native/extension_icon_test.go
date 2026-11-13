package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func logoPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestTheStoreDecodesASurfaceIconOnceAndDropsIt(t *testing.T) {
	m := newTestModel(nil)
	surfaces := []backend.ExtensionSurface{{ID: "main", Icon: "box"}, {ID: "other", Icon: "box"}}
	with := backend.Extension{ID: "a", Status: api.ExtensionRunning, Surfaces: surfaces, Icons: map[string][]byte{"main": logoPNG(t), "other": []byte("not a png")}}
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, with))
	main := m.extensions.surfaceImages()[extensionKey{"a", "main"}]
	if main == nil {
		t.Fatal("no image for the surface that sent one")
	}
	if _, ok := m.extensions.surfaceImages()[extensionKey{"a", "other"}]; ok {
		t.Error("an icon that does not decode has an image")
	}
	if again := m.extensions.surfaceImages()[extensionKey{"a", "main"}]; again != main {
		t.Error("the image was decoded again without a status change")
	}

	without := with
	without.Icons = nil
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, without))
	if len(m.extensions.surfaceImages()) != 0 {
		t.Error("an icon the extension no longer sends is still drawn")
	}
	m.extensionEvent(extensionEv(t, api.ExtensionStatusEvent.Name, with))
	m.extensionEvent(extensionEv(t, api.ExtensionRemovedEvent.Name, api.ExtensionIDOptions{ID: "a"}))
	if len(m.extensions.surfaceImages()) != 0 {
		t.Error("a removed extension's icon is still in the store")
	}
}

func TestSettingsDrawsTheFirstSurfaceImage(t *testing.T) {
	var store extensionStore
	e := backend.Extension{ID: "a", Surfaces: []backend.ExtensionSurface{{ID: "main", Icon: "rocket"}}, Icons: map[string][]byte{"main": logoPNG(t)}}
	store.setStatus(e)
	p := extensionsSettings{store: &store}
	if got, want := p.extensionIcon(e), store.surfaceImages()[extensionKey{"a", "main"}]; got != want {
		t.Errorf("icon = %v, want the surface's image", got)
	}
	e.Icons = nil
	store.setStatus(e)
	if got := p.extensionIcon(e); got == nil || got == icExtension {
		t.Errorf("icon = %v, want the named one once the image is gone", got)
	}
}
