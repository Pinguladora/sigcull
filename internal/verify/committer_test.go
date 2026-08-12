package verify

import "testing"

func TestCommitterMatches(t *testing.T) {
	tests := []struct {
		name           string
		committerEmail string
		committerName  string
		san            string
		want           bool
	}{
		{"email exact", "dev@example.com", "Dev", "dev@example.com", true},
		{"email case-insensitive", "Dev@Example.com", "Dev", "dev@example.com", true},
		{"email mismatch", "someone@example.com", "Dev", "dev@example.com", false},
		{"noreply match", "1+user@users.noreply.github.com", "user", "1+user@users.noreply.github.com", true},
		{"uri san matches committer name", "", "https://github.com/org/repo/.github/workflows/ci.yml@refs/heads/main", "https://github.com/org/repo/.github/workflows/ci.yml@refs/heads/main", true},
		{"uri san mismatch on name", "", "ci-bot", "https://github.com/org/repo/.github/workflows/ci.yml@refs/heads/main", false},
		{"email san not compared to name", "other@example.com", "dev@example.com", "dev@example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CommitterMatches(tt.committerEmail, tt.committerName, tt.san); got != tt.want {
				t.Fatalf("CommitterMatches(%q,%q,%q) = %v, want %v",
					tt.committerEmail, tt.committerName, tt.san, got, tt.want)
			}
		})
	}
}
