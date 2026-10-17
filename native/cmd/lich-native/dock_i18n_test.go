package main

import (
	"strings"
	"testing"
)

func TestDockCountsFollowTheInterfaceLanguage(t *testing.T) {
	inPortuguese(t)
	r := searchResult{Matches: make([]searchMatch, 3), Cut: true, TooLarge: 2}
	if got := searchSummary(r, 2); got != "3+ resultados" {
		t.Errorf("cut summary = %q", got)
	}
	r.Cut = false
	if got := searchSummary(r, 1); got != "3 resultados em 1 arquivo" {
		t.Errorf("summary = %q", got)
	}
	if got := searchFootnote(r); got != "2 arquivos acima de 1 MB não foram pesquisados." {
		t.Errorf("footnote = %q", got)
	}
	if got := treeFootnote(true, []string{"build"}); got != "Ocultos: build. Esta pasta tem mais arquivos do que a árvore consegue listar." {
		t.Errorf("tree footnote = %q", got)
	}
}

func TestInjectMenuSpeaksTheInterfaceLanguage(t *testing.T) {
	inPortuguese(t)
	entries := injectEntries("a.go", "3-4", func(string) bool { return true }, func() {})
	labels := []string{entries[0].Label, entries[1].Label, entries[2].Label}
	want := []string{"Injetar arquivo", "Injetar linhas 3-4", "Comentar para a sessão 3-4…"}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestPreviewErrorsSpeakTheInterfaceLanguage(t *testing.T) {
	inPortuguese(t)
	_, err := decodePreview([]byte("not an image"))
	if err == nil || !strings.HasPrefix(err.Error(), "não foi possível decodificar esta imagem") {
		t.Errorf("undecodable = %v", err)
	}
	_, err = decodePreview([]byte("GIF89a\x10\x27\x10\x27\x00\x00\x00"))
	if err == nil || !strings.Contains(err.Error(), "10000 × 10000 passa do limite de 40 MP") {
		t.Errorf("pixel limit = %v", err)
	}
}
