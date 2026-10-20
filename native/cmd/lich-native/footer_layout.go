package main

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/lichdotdev/lich/native/lichclient"
)

// footerItem is one control of the footer strip.
type footerItem struct {
	ID string
	// Example is the reading the editor previews, ExampleKey its catalog key
	// where the reading is a word rather than a value.
	Example    string
	ExampleKey string
}

var footerItems = []footerItem{
	{ID: "attach", ExampleKey: "footer.item.attach.example"},
	{ID: "files", ExampleKey: "footer.item.files.example"},
	{ID: "changes", Example: "3 · +10 −2"},
	{ID: "pr", Example: "PR #123"},
	{ID: "checkout", Example: "feature/toolbar"},
	{ID: "path", Example: "~/project"},
	// The editor shows the active session's model; this is its fallback.
	{ID: "model", ExampleKey: "footer.item.model.example"},
	{ID: "context", Example: "42%"},
	{ID: "plan", Example: "5h 28%"},
	{ID: "cost", Example: "$1.25"},
	{ID: "handsOn", Example: "12m"},
	{ID: "clock", Example: "Sep 6 · 14:30"},
}

// Footer zones: the two sides, and the pool of items on neither.
const (
	footerLeft      = "left"
	footerRight     = "right"
	footerAvailable = "available"
)

// footerLayoutKey is the global setting the layout persists under, as JSON.
const footerLayoutKey = "footer.layout"

type footerLayout struct {
	Left  []string `json:"left"`
	Right []string `json:"right"`
}

// footerVisibility is the pre-layout set of switches, which only seed a layout
// nobody has saved yet.
type footerVisibility struct {
	Model, Plan, HandsOn, Clock bool
}

var defaultFooterVisibility = footerVisibility{Model: true, Plan: true, HandsOn: true}

func defaultFooterLayout() footerLayout {
	return footerLayout{
		Left:  []string{"attach", "files", "changes", "pr"},
		Right: []string{"checkout", "model", "context", "plan", "handsOn"},
	}
}

func isFooterItem(id string) bool {
	return slices.ContainsFunc(footerItems, func(item footerItem) bool { return item.ID == id })
}

func (l footerLayout) has(id string) bool {
	return slices.Contains(l.Left, id) || slices.Contains(l.Right, id)
}

// parseFooterLayout reads a saved layout. A saved empty side is intentional;
// unknown or repeated items are dropped so a reload cannot clone a control.
func parseFooterLayout(raw string) (footerLayout, bool) {
	var value struct {
		Left, Right *[]json.RawMessage
	}
	if json.Unmarshal([]byte(raw), &value) != nil || value.Left == nil || value.Right == nil {
		return footerLayout{}, false
	}
	seen := map[string]bool{}
	clean := func(items []json.RawMessage) []string {
		out := []string{}
		for _, item := range items {
			var id string
			if json.Unmarshal(item, &id) != nil || !isFooterItem(id) || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
		return out
	}
	return footerLayout{Left: clean(*value.Left), Right: clean(*value.Right)}, true
}

// resolveFooterLayout is the layout to draw: the saved one, or one seeded
// from the visibility switches when nothing is saved.
func resolveFooterLayout(stored *footerLayout, visibility footerVisibility, showContext, cost bool) footerLayout {
	// Cost's backend flag also controls pricing. A layout saved while it was
	// off must not silently turn that work back on when another item moves.
	if stored != nil {
		if stored.has("cost") == cost {
			return *stored
		}
		target := footerAvailable
		if cost {
			target = footerRight
		}
		next, _ := moveFooterItem(*stored, "cost", target)
		return next
	}
	shown := map[string]bool{
		"model": visibility.Model, "context": showContext, "plan": visibility.Plan,
		"cost": cost, "handsOn": visibility.HandsOn, "clock": visibility.Clock,
	}
	right := []string{"checkout"}
	for _, id := range []string{"model", "context", "plan", "cost", "handsOn", "clock"} {
		if shown[id] {
			right = append(right, id)
		}
	}
	return footerLayout{Left: defaultFooterLayout().Left, Right: right}
}

// footerZone is the zone an item, or a zone's own id, belongs to.
func footerZone(l footerLayout, id string) string {
	if id == footerLeft || slices.Contains(l.Left, id) {
		return footerLeft
	}
	if id == footerRight || slices.Contains(l.Right, id) {
		return footerRight
	}
	return footerAvailable
}

// moveFooterItem serves pointer drops and the keyboard/menu alternatives
// alike: dropping on a zone appends, on an item takes that item's position.
// It reports false, with l unchanged, when nothing moved.
func moveFooterItem(l footerLayout, id, over string) (footerLayout, bool) {
	isZone := over == footerLeft || over == footerRight || over == footerAvailable
	if !isFooterItem(id) || id == over || (!isFooterItem(over) && !isZone) {
		return l, false
	}
	side := footerZone(l, over)
	without := func(items []string) []string {
		return slices.DeleteFunc(slices.Clone(items), func(item string) bool { return item == id })
	}
	next := footerLayout{Left: without(l.Left), Right: without(l.Right)}
	switch side {
	case footerLeft:
		next.Left = insertAt(next.Left, slices.Index(l.Left, over), id)
	case footerRight:
		next.Right = insertAt(next.Right, slices.Index(l.Right, over), id)
	}
	if slices.Equal(next.Left, l.Left) && slices.Equal(next.Right, l.Right) {
		return l, false
	}
	return next, true
}

// insertAt puts id at index, or appends for -1. index is taken against the
// side before id left it.
func insertAt(items []string, index int, id string) []string {
	if index < 0 || index > len(items) {
		index = len(items)
	}
	return slices.Insert(items, index, id)
}

// loadFooterLayout reads the saved layout and resolves it against the cost
// flag. The native window has no visibility switches or context toggle of its
// own, so an unsaved layout seeds from defaultFooterVisibility.
func loadFooterLayout(ctx context.Context, c *lichclient.Client, cost bool) (footerLayout, error) {
	raw, err := c.GetSetting(ctx, footerLayoutKey, "")
	if err != nil {
		return footerLayout{}, err
	}
	var stored *footerLayout
	if l, ok := parseFooterLayout(raw); ok {
		stored = &l
	}
	return resolveFooterLayout(stored, defaultFooterVisibility, true, cost), nil
}

func saveFooterLayout(ctx context.Context, c *lichclient.Client, l footerLayout) error {
	data, err := json.Marshal(footerLayout{Left: nonNil(l.Left), Right: nonNil(l.Right)})
	if err != nil {
		return err
	}
	return c.SetSetting(ctx, footerLayoutKey, "", string(data))
}

// nonNil keeps an emptied side saved as [] rather than null, which
// parseFooterLayout rejects.
func nonNil(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
