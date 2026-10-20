package main

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/native/lichclient"
)

// The sidebar's backend settings. Width and open state are global; a fold is
// scoped to its project, which the key spells out: the root and pinned
// stand-in keys exist in every project.
const (
	sidebarWidthKey     = "lich.sidebar.width"
	sidebarOpenKey      = "lich.sidebar.open"
	sidebarCollapsedKey = "lich.sidebar.collapsed."
)

// The sidebar's width bounds in rem: the floor is the header row's
// natural width, the cap short of crowding the terminal.
const (
	sidebarMinRem     = 14
	sidebarMaxRem     = 30
	sidebarDefaultRem = 15
)

// sidebarLayout is the sidebar's persisted layout.
type sidebarLayout struct {
	widthRem float64
	open     bool
}

func clampSidebarWidth(rem float64) float64 {
	return min(sidebarMaxRem, max(sidebarMinRem, rem))
}

// parseSidebarWidth reads a stored width: garbage, negatives
// and an unset value fall back to the default.
func parseSidebarWidth(raw string) float64 {
	rem, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsInf(rem, 0) || math.IsNaN(rem) || rem <= 0 {
		return sidebarDefaultRem
	}
	return clampSidebarWidth(rem)
}

// parseBoolSetting reads a stored bool: only what a write emits counts,
// so a foreign value cannot pin a toggle.
func parseBoolSetting(raw string, fallback bool) bool {
	switch raw {
	case "true":
		return true
	case "false":
		return false
	}
	return fallback
}

func loadSidebarLayout(ctx context.Context, c *lichclient.Client) (sidebarLayout, error) {
	width, err := c.GetSetting(ctx, sidebarWidthKey, "")
	if err != nil {
		return sidebarLayout{}, err
	}
	open, err := c.GetSetting(ctx, sidebarOpenKey, "")
	if err != nil {
		return sidebarLayout{}, err
	}
	return sidebarLayout{widthRem: parseSidebarWidth(width), open: parseBoolSetting(open, true)}, nil
}

func saveSidebarWidth(ctx context.Context, c *lichclient.Client, rem float64) error {
	return c.SetSetting(ctx, sidebarWidthKey, "", strconv.FormatFloat(clampSidebarWidth(rem), 'f', -1, 64))
}

func saveSidebarOpen(ctx context.Context, c *lichclient.Client, open bool) error {
	return c.SetSetting(ctx, sidebarOpenKey, "", strconv.FormatBool(open))
}

func loadCollapsed(ctx context.Context, c *lichclient.Client, projectID, blockKey string) (bool, error) {
	raw, err := c.GetSetting(ctx, sidebarCollapsedKey+blockKey, projectID)
	return parseBoolSetting(raw, false), err
}

// saveCollapsed writes unfolded as "": the store has no delete, and GetSetting
// answers "" for an unset key, so unfolded stays stored by not being stored.
func saveCollapsed(ctx context.Context, c *lichclient.Client, projectID, blockKey string, collapsed bool) error {
	value := ""
	if collapsed {
		value = "true"
	}
	return c.SetSetting(ctx, sidebarCollapsedKey+blockKey, projectID, value)
}

// moveCollapsed carries a fold from one block key to another, dropping it for
// an empty to. A folder is keyed by its name, so without this a rename would
// unfold the block under the user and strand the old entry.
func moveCollapsed(ctx context.Context, c *lichclient.Client, projectID, from, to string) (bool, error) {
	collapsed, err := loadCollapsed(ctx, c, projectID, from)
	if err != nil {
		return false, err
	}
	if err := saveCollapsed(ctx, c, projectID, from, false); err != nil {
		return false, err
	}
	if to == "" {
		return false, nil
	}
	return collapsed, saveCollapsed(ctx, c, projectID, to, collapsed)
}
