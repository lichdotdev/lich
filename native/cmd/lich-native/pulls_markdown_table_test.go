package main

import (
	"reflect"
	"slices"
	"testing"
)

func TestSplitMarkdownTablesCutsGFMTablesOutOfTheProse(t *testing.T) {
	src := "Intro\n\n| Provider | Works |\n|:--|--:|\n| Claude \\| Code | `yes` |\n| Codex |\n\nAfter\n\n```\n| not | a table |\n|---|---|\n```"
	got := splitMarkdownTables(src)
	want := []markdownSegment{
		{text: "Intro\n"},
		{table: [][]string{{"Provider", "Works"}, {"Claude | Code", "`yes`"}, {"Codex", ""}}},
		{text: "\nAfter\n\n```\n| not | a table |\n|---|---|\n```"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitMarkdownTables =\n%#v\nwant\n%#v", got, want)
	}
	if got := splitMarkdownTables("a | b\nno delimiter"); len(got) != 1 || got[0].table != nil {
		t.Errorf("a pipe without a delimiter row became %#v, want prose", got)
	}
}

func TestFitColumnsSharesTheRoomOnlyWhenTheTableOverflows(t *testing.T) {
	if got := fitColumns([]int{100, 50}, 200); !slices.Equal(got, []int{100, 50}) {
		t.Errorf("fitting table = %v, want its natural widths", got)
	}
	if got := fitColumns([]int{300, 100}, 200); !slices.Equal(got, []int{150, 50}) {
		t.Errorf("overflowing table = %v, want [150 50]", got)
	}
}
