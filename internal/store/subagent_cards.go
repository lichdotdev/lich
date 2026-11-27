package store

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/lichdotdev/lich/api"
)

// subagentCardsKey turns a provider's subagent cards off for the literal
// subagentCardsOff; anything else, an absent key included, leaves them on,
// which is what every subagent did before the setting existed.
func subagentCardsKey(providerID string) string {
	return "subagent." + providerID + ".cards"
}

const subagentCardsOff = "off"

// subagentBringsKey holds the items a provider's cards bring, a JSON array of
// api.SubagentCardItem IDs. Absent is none: a card starts lean.
func subagentBringsKey(providerID string) string {
	return "subagent." + providerID + ".brings"
}

// SubagentCards reports whether this provider's subagents open as cards. An
// unreadable value leaves them on.
func (s *Service) SubagentCards(providerID string) bool {
	return s.globalSetting(subagentCardsKey(providerID)) != subagentCardsOff
}

// SetSubagentCards turns this provider's subagent cards on or off.
func (s *Service) SetSubagentCards(providerID string, on bool) error {
	if on {
		return s.DeleteGlobalSetting(subagentCardsKey(providerID))
	}
	_, err := s.SetSetting(context.Background(), api.SetSettingOptions{Key: subagentCardsKey(providerID), Value: subagentCardsOff})
	return err
}

// SubagentBrings is what this provider's cards bring, sorted. An unreadable
// value brings nothing, the lean start.
func (s *Service) SubagentBrings(providerID string) []string {
	raw := s.globalSetting(subagentBringsKey(providerID))
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	slices.Sort(items)
	return items
}

// SetSubagentBring turns one item this provider's cards bring on or off.
func (s *Service) SetSubagentBring(providerID, item string, on bool) error {
	items := slices.DeleteFunc(s.SubagentBrings(providerID), func(it string) bool { return it == item })
	if on {
		items = append(items, item)
	}
	slices.Sort(items)
	raw, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("encode what %q cards bring: %w", providerID, err)
	}
	_, err = s.SetSetting(context.Background(), api.SetSettingOptions{Key: subagentBringsKey(providerID), Value: string(raw)})
	return err
}
