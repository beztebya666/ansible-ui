package api

import "testing"

func TestMapGroupsToRole(t *testing.T) {
	rules := []groupRule{
		{Group: "platform-admins", Role: "admin"},
		{Group: "devs", Role: "user"},
	}
	cases := []struct {
		name    string
		groups  []string
		def     string
		want    string
	}{
		{"ldap DN matches admin", []string{"cn=platform-admins,ou=groups,dc=x"}, "user", "admin"},
		{"oidc plain group → user", []string{"devs"}, "viewer", "user"},
		{"admin wins over user", []string{"devs", "platform-admins"}, "user", "admin"},
		{"no match → default", []string{"randoms"}, "user", "user"},
		{"no groups → default", nil, "user", "user"},
		{"case-insensitive", []string{"CN=Platform-Admins,OU=g"}, "user", "admin"},
	}
	for _, c := range cases {
		if got := mapGroupsToRole(c.groups, rules, c.def); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestClaimStrings(t *testing.T) {
	if got := claimStrings(map[string]any{"groups": []any{"a", "b"}}, "groups"); len(got) != 2 || got[0] != "a" {
		t.Errorf("[]any: got %v", got)
	}
	if got := claimStrings(map[string]any{"groups": "solo"}, "groups"); len(got) != 1 || got[0] != "solo" {
		t.Errorf("string: got %v", got)
	}
	if got := claimStrings(map[string]any{}, "groups"); got != nil {
		t.Errorf("missing: got %v", got)
	}
}
