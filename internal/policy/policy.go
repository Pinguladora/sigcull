// Package policy implements the identity allowlist that a verified commit
// signature must satisfy. It is deliberately independent of any Sigstore /
// gitsign types: it matches on the (issuer, SAN) pair that the verifier
// extracts from a valid signature.
//
// Semantics: the policy passes if ANY allow entry matches, where a match
// requires the OIDC issuer to be EXACTLY equal and the SAN to be either exactly
// equal (san) or to match a regular expression (sanRegex). An empty allowlist
// therefore rejects everything, failing closed by construction.
package policy

import (
	"fmt"
	"regexp"
)

// Identity is one allowlist entry. Exactly one of SAN or SANRegex must be set.
type Identity struct {
	sanRe    *regexp.Regexp
	Issuer   string `yaml:"issuer"`
	SAN      string `yaml:"san"`
	SANRegex string `yaml:"sanRegex"`
}

// Policy passes if ANY Identity matches.
type Policy struct {
	Allow []Identity `yaml:"allow"`
}

// Compile validates every Identity and compiles the SAN regexes once, up front.
// It must be called before Match. A Policy obtained any other way has no
// compiled regexes and will not match sanRegex entries.
//
// Validation rules:
//   - Issuer is required.
//   - Exactly one of SAN / SANRegex is set (XOR).
//   - SANRegex compiles and is auto-anchored (\A(?:…)\z) so an entry cannot
//     accidentally substring-match a look-alike SAN.
//
// An empty allowlist is valid (everything fails closed) but the caller should
// warn, since it makes every commit fail.
func Compile(allow []Identity) (Policy, error) {
	compiled := make([]Identity, len(allow))
	for i, id := range allow {
		if id.Issuer == "" {
			return Policy{}, fmt.Errorf("allow[%d]: issuer is required", i)
		}
		hasSAN, hasRe := id.SAN != "", id.SANRegex != ""
		switch {
		case hasSAN && hasRe:
			return Policy{}, fmt.Errorf("allow[%d]: set only one of san or sanRegex, not both", i)
		case !hasSAN && !hasRe:
			return Policy{}, fmt.Errorf("allow[%d]: one of san or sanRegex is required", i)
		}
		if hasRe {
			re, err := regexp.Compile(`\A(?:` + id.SANRegex + `)\z`)
			if err != nil {
				return Policy{}, fmt.Errorf("allow[%d]: invalid sanRegex %q: %w", i, id.SANRegex, err)
			}
			id.sanRe = re
		}
		compiled[i] = id
	}
	return Policy{Allow: compiled}, nil
}

// Match reports whether (issuer, san) satisfies any allow entry, returning the
// first matching Identity. Issuer is always matched exactly.
func (p Policy) Match(issuer, san string) (Identity, bool) {
	for _, id := range p.Allow {
		if id.Issuer != issuer {
			continue
		}
		if id.sanRe != nil {
			if id.sanRe.MatchString(san) {
				return id, true
			}
			continue
		}
		if id.SAN == san {
			return id, true
		}
	}
	return Identity{}, false
}
