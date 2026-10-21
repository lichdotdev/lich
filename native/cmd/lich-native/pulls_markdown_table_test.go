package main

import (
	"slices"
	"testing"
)

func TestFitColumnsSharesTheRoomOnlyWhenTheTableOverflows(t *testing.T) {
	if got := fitColumns([]int{100, 50}, []int{40, 20}, 200); !slices.Equal(got, []int{100, 50}) {
		t.Errorf("fitting table = %v, want its natural widths", got)
	}
	if got := fitColumns([]int{30, 400, 170}, []int{30, 100, 70}, 400); !slices.Equal(got, []int{30, 250, 120}) {
		t.Errorf("overflowing table = %v, want each column its longest word and the rest shared out", got)
	}
}
