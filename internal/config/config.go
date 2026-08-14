// Package config loads and validates the YAML configuration and resolves the
// secrets it references from the environment. Secrets live in env vars named by
// the config, never inline in the file.
package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Pinguladora/sigcull/internal/policy"
	"github.com/Pinguladora/sigcull/internal/trust"
	"github.com/Pinguladora/sigcull/internal/verify"
	yaml "go.yaml.in/yaml/v4"
)

// Config is the raw YAML shape.
type Config struct {
	Trust     trust.Source `yaml:"trust"`
	Server    Server       `yaml:"server"`
	GithubApp GithubApp    `yaml:"githubApp"`
	Verify    Verify       `yaml:"verify"`
	Exempt    Exempt       `yaml:"exempt"`
}

// Server holds HTTP listener settings.
type Server struct {
	Addr             string `yaml:"addr"`
	WebhookSecretEnv string `yaml:"webhookSecretEnv"`
}

// GithubApp names the sources of the App credentials. The private key comes from
// either an env var (PrivateKeyEnv) or a mounted PEM file (PrivateKeyFile). A
// file is preferred on platforms that mangle multiline env values, and takes
// precedence when both are set.
type GithubApp struct {
	AppIDEnv       string `yaml:"appIDEnv"`
	PrivateKeyEnv  string `yaml:"privateKeyEnv"`
	PrivateKeyFile string `yaml:"privateKeyFile"`
}

// loadPrivateKey returns the App private key PEM, preferring a mounted file over
// the env var. Reading from a file avoids the multiline-mangling some container
// platforms apply to env values.
func (g GithubApp) loadPrivateKey() ([]byte, error) {
	if g.PrivateKeyFile != "" {
		// The path is operator config, not attacker input.
		pem, err := os.ReadFile(g.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read private key file %q: %w", g.PrivateKeyFile, err)
		}
		if len(pem) == 0 {
			return nil, fmt.Errorf("private key file %q is empty", g.PrivateKeyFile)
		}
		return pem, nil
	}
	key, err := requireEnv(g.PrivateKeyEnv, "githubApp.privateKeyEnv")
	if err != nil {
		return nil, err
	}
	pem := []byte(key)
	return pem, nil
}

// Verify holds verification-behaviour settings and the signature authorities. A
// commit passes if any authority accepts it (OR semantics), tried in order.
type Verify struct {
	// Authorities lists the accepted signature sources.
	Authorities []AuthoritySpec `yaml:"authorities"`
	// MatchCommitter also requires the git committer to equal the signing identity.
	MatchCommitter bool `yaml:"matchCommitter"`
	// ShowFullIdentity publishes full signer emails in the Check Run. The default masks them.
	ShowFullIdentity bool `yaml:"showFullIdentity"`
}

// AuthoritySpec is one entry in verify.authorities. Exactly one field must be
// set, selecting the kind of signature the authority accepts.
type AuthoritySpec struct {
	Keyless        *KeylessSpec        `yaml:"keyless"`
	GPGKey         *GPGKeySpec         `yaml:"gpgKey"`
	SSHKey         *SSHKeySpec         `yaml:"sshKey"`
	X509           *X509Spec           `yaml:"x509"`
	GitHubVerified *GitHubVerifiedSpec `yaml:"githubVerified"`
}

// KeylessSpec accepts Sigstore / gitsign keyless signatures whose identity is on
// the allowlist.
type KeylessSpec struct {
	Allow []policy.Identity `yaml:"allow"`
}

// GPGKeySpec accepts a traditional OpenPGP signature that verifies against the
// keyring at KeyringPath and whose signing key carries an allowlisted UID email.
type GPGKeySpec struct {
	KeyringPath string   `yaml:"keyringPath"`
	AllowEmails []string `yaml:"allowEmails"`
}

// SSHKeySpec accepts an SSH signature that verifies against a trusted key in the
// OpenSSH allowed_signers file at AllowedSignersPath. AllowPrincipals optionally
// restricts which principals (emails) in that file are accepted.
type SSHKeySpec struct {
	AllowedSignersPath string   `yaml:"allowedSignersPath"`
	AllowPrincipals    []string `yaml:"allowPrincipals"`
}

// X509Spec accepts a detached CMS (S-MIME) signature whose signing certificate
// chains to the CA bundle at CABundlePath and carries an allowlisted email.
type X509Spec struct {
	CABundlePath string   `yaml:"caBundlePath"`
	AllowEmails  []string `yaml:"allowEmails"`
}

// GitHubVerifiedSpec accepts a commit that GitHub itself cryptographically
// verified. With WebFlowOnly, only GitHub's own web and squash merges qualify,
// not a user's vigilant-mode key that GitHub merely verifies.
type GitHubVerifiedSpec struct {
	WebFlowOnly bool `yaml:"webFlowOnly"`
}

// count returns how many authority kinds the spec sets; exactly one is valid.
func (s AuthoritySpec) count() int {
	n := 0
	if s.Keyless != nil {
		n++
	}
	if s.GPGKey != nil {
		n++
	}
	if s.SSHKey != nil {
		n++
	}
	if s.X509 != nil {
		n++
	}
	if s.GitHubVerified != nil {
		n++
	}
	return n
}

// Exempt controls which commits are skipped rather than verified. GitHub-signed
// commits are no longer an exemption: configure a githubVerified authority to
// accept them instead.
type Exempt struct {
	SkipMergeCommits bool `yaml:"skipMergeCommits"`
}

// Resolved is the fully wired, validated configuration ready to serve. It holds
// resolved secrets and compiled artefacts, so nothing downstream re-reads the
// environment or re-compiles regexes.
type Resolved struct {
	Verifier          verify.Verifier
	Addr              string
	WebhookSecret     []byte
	PrivateKey        []byte
	AppID             int64
	Exempt            Exempt
	MatchCommitter    bool
	MaskIdentities    bool
	GitHubVerified    bool // a githubVerified authority is configured
	GitHubWebFlowOnly bool // that authority accepts only GitHub web/squash merges
}

// Load reads the YAML at path, validates it, resolves secrets from the
// environment, compiles the policy, and prepares the trust source.
func Load(path string) (*Resolved, error) {
	// The config path is an operator-provided flag, not attacker input.
	data, err := os.ReadFile(path) //nolint:gosec // G304: operator-supplied config path
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	resolved, err := c.resolve()
	return resolved, err
}

func (c Config) resolve() (*Resolved, error) {
	addr := c.Server.Addr
	if addr == "" {
		addr = ":8080"
	}

	secret, err := requireEnv(c.Server.WebhookSecretEnv, "server.webhookSecretEnv")
	if err != nil {
		return nil, err
	}

	appIDStr, err := requireEnv(c.GithubApp.AppIDEnv, "githubApp.appIDEnv")
	if err != nil {
		return nil, err
	}
	appID, err := strconv.ParseInt(strings.TrimSpace(appIDStr), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("app ID from %s is not a number: %w", c.GithubApp.AppIDEnv, err)
	}

	privateKey, err := c.GithubApp.loadPrivateKey()
	if err != nil {
		return nil, err
	}

	trustParams := verify.TrustParams{
		Kind:      verify.TrustKind(c.Trust.Kind),
		TrustRoot: c.Trust.TrustRoot,
		TUFMirror: c.Trust.TUFMirror,
		TUFRoot:   c.Trust.TUFRoot,
		RekorURL:  c.Trust.Rekor,
	}
	authorities, github, err := c.buildAuthorities(context.Background(), trustParams)
	if err != nil {
		return nil, err
	}

	resolved := &Resolved{
		Addr:           addr,
		WebhookSecret:  []byte(secret),
		AppID:          appID,
		PrivateKey:     privateKey,
		Verifier:       verify.NewComposite(authorities...),
		Exempt:         c.Exempt,
		MatchCommitter: c.Verify.MatchCommitter,
		MaskIdentities: !c.Verify.ShowFullIdentity,
	}
	if github != nil {
		resolved.GitHubVerified = true
		resolved.GitHubWebFlowOnly = github.WebFlowOnly
	}
	return resolved, nil
}

// buildAuthorities constructs the crypto verifier authorities and, separately,
// the optional githubVerified authority (which the processor evaluates as a
// fallback because it needs the per-installation GitHub client). At least one
// authority is required, otherwise every commit would fail.
func (c Config) buildAuthorities(
	ctx context.Context, tp verify.TrustParams,
) (authorities []verify.Authority, github *GitHubVerifiedSpec, err error) {
	if len(c.Verify.Authorities) == 0 {
		return nil, nil, errors.New("verify.authorities must list at least one authority")
	}
	for i, spec := range c.Verify.Authorities {
		if n := spec.count(); n != 1 {
			return nil, nil, fmt.Errorf("verify.authorities[%d]: set exactly one authority kind, got %d", i, n)
		}
		if spec.GitHubVerified != nil {
			if github != nil {
				return nil, nil, errors.New("verify.authorities: at most one githubVerified authority")
			}
			github = spec.GitHubVerified
			continue
		}
		a, err := spec.build(ctx, tp)
		if err != nil {
			return nil, nil, fmt.Errorf("verify.authorities[%d]: %w", i, err)
		}
		authorities = append(authorities, a)
	}
	return authorities, github, nil
}

// build turns one crypto authority spec into a verify.Authority. The count is
// validated by the caller, so exactly one field is set here.
func (s AuthoritySpec) build(ctx context.Context, tp verify.TrustParams) (verify.Authority, error) {
	switch {
	case s.Keyless != nil:
		a, err := s.buildKeyless(ctx, tp)
		return a, err
	case s.GPGKey != nil:
		a, err := verify.NewGPGKey(s.GPGKey.KeyringPath, s.GPGKey.AllowEmails)
		if err != nil {
			return nil, fmt.Errorf("gpgKey: %w", err)
		}
		return a, nil
	case s.SSHKey != nil:
		a, err := verify.NewSSHKey(s.SSHKey.AllowedSignersPath, s.SSHKey.AllowPrincipals)
		if err != nil {
			return nil, fmt.Errorf("sshKey: %w", err)
		}
		return a, nil
	case s.X509 != nil:
		a, err := verify.NewX509(s.X509.CABundlePath, s.X509.AllowEmails)
		if err != nil {
			return nil, fmt.Errorf("x509: %w", err)
		}
		return a, nil
	default:
		return nil, errors.New("empty authority, set one of: keyless, gpgKey, sshKey, x509, githubVerified")
	}
}

// buildKeyless compiles the keyless allowlist and constructs the authority.
func (s AuthoritySpec) buildKeyless(ctx context.Context, tp verify.TrustParams) (verify.Authority, error) {
	pol, err := policy.Compile(s.Keyless.Allow)
	if err != nil {
		return nil, fmt.Errorf("keyless: %w", err)
	}
	a, err := verify.NewKeyless(ctx, pol, tp)
	if err != nil {
		return nil, fmt.Errorf("keyless: %w", err)
	}
	return a, nil
}

// requireEnv reads the named env var, failing if the config field or the value
// is empty.
func requireEnv(envName, field string) (string, error) {
	if envName == "" {
		return "", fmt.Errorf("config %s must name an environment variable", field)
	}
	v := os.Getenv(envName)
	if v == "" {
		return "", fmt.Errorf("environment variable %s (from %s) is not set", envName, field)
	}
	return v, nil
}
