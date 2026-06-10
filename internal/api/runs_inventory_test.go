package api

import (
	"testing"

	"github.com/nikiv/ansible-ui/internal/model"
)

func TestTemplateInventoryAllowed(t *testing.T) {
	sp := func(s string) *string { return &s }
	cases := []struct {
		name    string
		tpl     *model.Template
		invID   string
		allowed bool
	}{
		{"no restriction → anything", &model.Template{}, "inv_x", true},
		{"no restriction, blank inv", &model.Template{}, "", true},
		{"restricted: default allowed", &model.Template{InventoryID: sp("inv_def"), InventoryIDs: []string{"inv_a"}}, "inv_def", true},
		{"restricted: in set", &model.Template{InventoryID: sp("inv_def"), InventoryIDs: []string{"inv_a", "inv_b"}}, "inv_b", true},
		{"restricted: not in set → blocked", &model.Template{InventoryID: sp("inv_def"), InventoryIDs: []string{"inv_a"}}, "inv_evil", false},
		{"restricted but blank inv → allowed (resolved elsewhere)", &model.Template{InventoryIDs: []string{"inv_a"}}, "", true},
		{"restricted, no default, in set", &model.Template{InventoryIDs: []string{"inv_a"}}, "inv_a", true},
		{"restricted, no default, not in set", &model.Template{InventoryIDs: []string{"inv_a"}}, "inv_z", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := templateInventoryAllowed(c.tpl, c.invID); got != c.allowed {
				t.Fatalf("templateInventoryAllowed(%v, %q) = %v, want %v", c.tpl.InventoryIDs, c.invID, got, c.allowed)
			}
		})
	}
}
