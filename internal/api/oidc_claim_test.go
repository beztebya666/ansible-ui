package api

import "testing"

func TestClaimSatisfies(t *testing.T) {
	claims := map[string]any{
		"hd":     "example.com",
		"email":  "a@example.com",
		"groups": []any{"eng", "admins"},
		"active": true,
		"empty":  "",
	}
	cases := []struct {
		rule string
		want bool
	}{
		{"", true},                     // no restriction
		{"hd=example.com", true},       // string claim equals
		{"hd=other.com", false},        // string claim differs
		{"groups=eng", true},           // array claim contains
		{"groups=ops", false},          // array claim missing value
		{"hd", true},                   // bare key present + truthy
		{"active", true},               // bool truthy
		{"empty", false},               // present but empty string → falsey
		{"missing", false},             // claim absent
		{"missing=x", false},           // absent with value
	}
	for _, c := range cases {
		if got := claimSatisfies(claims, c.rule); got != c.want {
			t.Errorf("claimSatisfies(%q) = %v, want %v", c.rule, got, c.want)
		}
	}
}
