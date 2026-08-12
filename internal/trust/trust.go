// Package trust holds the trust-source configuration shape. Resolving it into a
// Sigstore trusted root happens in the verify package, which owns the sigstore
// dependencies. There is no shell-out: public uses an embedded root, static a
// file, and tuf an in-library TUF client.
package trust

// Kind selects where the roots of trust come from.
type Kind string

const (
	Public Kind = "public" // embedded public-good root, no network
	TUF    Kind = "tuf"    // BYO-TUF: live TUF from a custom mirror
	Static Kind = "static" // a trusted_root.json file, no network
)

// Source is the trust configuration as loaded from YAML.
type Source struct {
	Kind      Kind   `yaml:"kind"`
	TUFMirror string `yaml:"tufMirror"` // for Kind==TUF
	TUFRoot   string `yaml:"tufRoot"`   // TUF root.json trust anchor
	TrustRoot string `yaml:"trustRoot"` // for Kind==Static: trusted_root.json
	Rekor     string `yaml:"rekor"`     // optional Rekor URL for the online fallback
}
