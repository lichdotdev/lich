package markdown

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

func TestATableWhoseWordsOverflowTheColumnScrollsInsideIt(t *testing.T) {
	th := markdownTheme()
	path := "`model/src/main/java/br/com/sankhya/soplogistica/service/documento/ContratosCiotViagemFonte.java:88`"
	table := renderBlocks(th, "| # | Location |\n|---|---|\n| 1 | "+path+" |")[0].table
	gtx, _ := markdownFrame(400)
	grid := table.layoutGrid(gtx, th, Links{})
	if grid.Size.X <= gtx.Constraints.Max.X {
		t.Fatalf("grid is %d wide, want it wider than the %d column", grid.Size.X, gtx.Constraints.Max.X)
	}
	table.offset = 1 << 20
	d := table.layout(gtx, th, Links{})
	if d.Size.X != gtx.Constraints.Max.X {
		t.Errorf("scrolling table is %d wide, want the column's %d", d.Size.X, gtx.Constraints.Max.X)
	}
	if d.Size.Y <= grid.Size.Y {
		t.Errorf("scrolling table is %d tall, want room under its %d rows for the bar", d.Size.Y, grid.Size.Y)
	}
	if want := grid.Size.X - gtx.Constraints.Max.X; table.offset != want {
		t.Errorf("offset = %d, want it held at the table's end, %d", table.offset, want)
	}
}

func TestATableThatFitsDoesNotScroll(t *testing.T) {
	th := markdownTheme()
	table := renderBlocks(th, "| # | Location |\n|---|---|\n| 1 | `a.go:1` |")[0].table
	gtx, _ := markdownFrame(400)
	table.offset = 30
	grid := table.layoutGrid(gtx, th, Links{})
	if d := table.layout(gtx, th, Links{}); d != grid || table.offset != 0 {
		t.Errorf("fitting table = %v at offset %d, want its grid %v at 0", d, table.offset, grid)
	}
}
