package policy

import "testing"

func TestCompileValidation(t *testing.T) {
	tests := []struct {
		name    string
		allow   []Identity
		wantErr bool
	}{
		{"exact san ok", []Identity{{Issuer: "https://x", SAN: "a@b.com"}}, false},
		{"regex san ok", []Identity{{Issuer: "https://x", SANRegex: ".*@b.com"}}, false},
		{"empty allowlist ok", nil, false},
		{"missing issuer", []Identity{{SAN: "a@b.com"}}, true},
		{"both san and regex", []Identity{{Issuer: "https://x", SAN: "a", SANRegex: "b"}}, true},
		{"neither san nor regex", []Identity{{Issuer: "https://x"}}, true},
		{"bad regex", []Identity{{Issuer: "https://x", SANRegex: "("}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.allow)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Compile err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	pol, err := Compile([]Identity{
		{Issuer: "https://token.actions.githubusercontent.com", SANRegex: "^https://github.com/Pinguladora/.+@refs/heads/main$"},
		{Issuer: "https://accounts.google.com", SAN: "pinguladora@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		issuer string
		san    string
		want   bool
	}{
		{"exact san match", "https://accounts.google.com", "pinguladora@example.com", true},
		{"exact san wrong issuer", "https://token.actions.githubusercontent.com", "pinguladora@example.com", false},
		{"regex match", "https://token.actions.githubusercontent.com", "https://github.com/Pinguladora/repo/.github/workflows/ci.yml@refs/heads/main", true},
		{"regex no match branch", "https://token.actions.githubusercontent.com", "https://github.com/Pinguladora/repo/.github/workflows/ci.yml@refs/heads/dev", false},
		{"regex anchored not substring", "https://token.actions.githubusercontent.com", "prefixhttps://github.com/Pinguladora/repo@refs/heads/mainsuffix", false},
		{"unknown identity", "https://evil.example", "attacker@evil.example", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := pol.Match(tt.issuer, tt.san); ok != tt.want {
				t.Fatalf("Match(%q,%q) = %v, want %v", tt.issuer, tt.san, ok, tt.want)
			}
		})
	}
}

func TestEmptyPolicyFailsClosed(t *testing.T) {
	pol, err := Compile(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pol.Match("https://accounts.google.com", "anyone@example.com"); ok {
		t.Fatal("empty policy should match nothing")
	}
}
