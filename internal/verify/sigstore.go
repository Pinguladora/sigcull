package verify

import (
	"context"
	"crypto/x509"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Pinguladora/sigcull/internal/policy"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	gsgit "github.com/sigstore/gitsign/pkg/git"
	"github.com/sigstore/gitsign/pkg/rekor"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	sigtuf "github.com/sigstore/sigstore/pkg/tuf" //nolint:staticcheck // cosign's API still needs this
)

// defaultRekorURL names the public good Rekor. The rekor client is built against
// it but never contacted: inclusion is verified strictly offline from the entry
// embedded in the signature, against the log keys in the trusted root.
const defaultRekorURL = "https://rekor.sigstore.dev"

// publicGoodRootJSON is the vendored public-good Sigstore trusted root, baked
// into the binary so the public trust source needs no network. Refresh it with
// `mise run roots:update` when Sigstore rotates keys.
//
//go:embed roots/public_good_trusted_root.json
var publicGoodRootJSON []byte

// TrustKind selects where the verification roots come from.
type TrustKind string

const (
	Public TrustKind = "public" // embedded public-good root, no network
	TUF    TrustKind = "tuf"    // BYO-TUF: live TUF from a custom mirror
	Static TrustKind = "static" // a trusted_root.json file, no network
)

// TrustParams describes how to obtain the Sigstore trusted root.
type TrustParams struct {
	Kind      TrustKind
	TrustRoot string // Static: path to trusted_root.json
	TUFMirror string // TUF: repository base URL
	TUFRoot   string // TUF: path to the root.json trust anchor
	RekorURL  string // online inclusion fallback (rarely used)
}

// trustedMaterial is the subset of the sigstore-go trusted root the verifier
// needs. Both *root.TrustedRoot and *root.LiveTrustedRoot satisfy it.
type trustedMaterial interface {
	FulcioCertificateAuthorities() []root.CertificateAuthority
	RekorLogs() map[string]*root.TransparencyLog
}

type sigstoreVerifier struct {
	git   gsgit.Verifier
	rekor rekor.Verifier
	pol   policy.Policy
}

// Name identifies the keyless authority in Check Run output.
func (v *sigstoreVerifier) Name() string { return AuthorityKeyless }

// NewKeyless builds the keyless authority from an identity policy and trust
// parameters. It reuses gitsign's library (pkg/git, pkg/rekor) with roots loaded
// from a Sigstore trusted root, so verification needs no subprocess.
func NewKeyless(ctx context.Context, pol policy.Policy, tp TrustParams) (Authority, error) {
	tm, err := loadTrustedMaterial(tp)
	if err != nil {
		return nil, err
	}

	rootPool, intermediatePool := fulcioPools(tm)
	gitVerifier, err := gsgit.NewCertVerifier(
		gsgit.WithRootPool(rootPool),
		gsgit.WithIntermediatePool(intermediatePool),
	)
	if err != nil {
		return nil, fmt.Errorf("build cert verifier: %w", err)
	}

	rekorURL := tp.RekorURL
	if rekorURL == "" {
		rekorURL = defaultRekorURL
	}
	rekorVerifier, err := rekorVerifierFrom(ctx, tm, rekorURL)
	if err != nil {
		return nil, err
	}

	return &sigstoreVerifier{pol: pol, git: gitVerifier, rekor: rekorVerifier}, nil
}

// loadTrustedMaterial resolves the trusted root for the trust source. public is
// the embedded root, static is a file, tuf is a live TUF client.
func loadTrustedMaterial(tp TrustParams) (trustedMaterial, error) {
	switch tp.Kind {
	case Public, "":
		tr, err := root.NewTrustedRootFromJSON(publicGoodRootJSON)
		if err != nil {
			return nil, fmt.Errorf("load embedded public-good root: %w", err)
		}
		return tr, nil
	case Static:
		if tp.TrustRoot == "" {
			return nil, errors.New("trust: kind static requires trustRoot")
		}
		tr, err := root.NewTrustedRootFromPath(tp.TrustRoot)
		if err != nil {
			return nil, fmt.Errorf("load trusted root %q: %w", tp.TrustRoot, err)
		}
		return tr, nil
	case TUF:
		return liveTUFTrustedRoot(tp)
	default:
		return nil, fmt.Errorf("trust: unknown kind %q", tp.Kind)
	}
}

// liveTUFTrustedRoot builds a live TUF trusted root for the BYO-TUF source.
func liveTUFTrustedRoot(tp TrustParams) (trustedMaterial, error) {
	opts := tuf.DefaultOptions()
	if tp.TUFMirror != "" {
		opts.RepositoryBaseURL = tp.TUFMirror
	}
	if tp.TUFRoot != "" {
		b, err := os.ReadFile(tp.TUFRoot)
		if err != nil {
			return nil, fmt.Errorf("read tuf root %q: %w", tp.TUFRoot, err)
		}
		opts.Root = b
	}
	lr, err := root.NewLiveTrustedRoot(opts)
	if err != nil {
		return nil, fmt.Errorf("init tuf trusted root: %w", err)
	}
	return lr, nil
}

// fulcioPools builds the Fulcio root and intermediate cert pools.
func fulcioPools(tm trustedMaterial) (rootPool, intermediatePool *x509.CertPool) {
	rootPool = x509.NewCertPool()
	intermediatePool = x509.NewCertPool()
	for _, ca := range tm.FulcioCertificateAuthorities() {
		fca, ok := ca.(*root.FulcioCertificateAuthority)
		if !ok {
			continue
		}
		if fca.Root != nil {
			rootPool.AddCert(fca.Root)
		}
		for _, ic := range fca.Intermediates {
			intermediatePool.AddCert(ic)
		}
	}
	return rootPool, intermediatePool
}

// rekorVerifierFrom builds a Rekor verifier whose public keys come from the
// trusted material, so inclusion is verified offline against the entry embedded
// in the signature.
func rekorVerifierFrom(ctx context.Context, tm trustedMaterial, rekorURL string) (rekor.Verifier, error) {
	keys := cosign.NewTrustedTransparencyLogPubKeys()
	for _, tl := range tm.RekorLogs() {
		pem, err := cryptoutils.MarshalPublicKeyToPEM(tl.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("marshal rekor public key: %w", err)
		}
		if err := keys.AddTransparencyLogPubKey(pem, sigtuf.Active); err != nil {
			return nil, fmt.Errorf("add rekor public key: %w", err)
		}
	}
	v, err := rekor.NewWithOptions(ctx, rekorURL,
		rekor.WithCosignRekorKeyProvider(func(_ context.Context) (*cosign.TrustedTransparencyLogPubKeys, error) {
			return &keys, nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("build rekor verifier: %w", err)
	}
	return v, nil
}

// Verify implements Verifier. It fails closed and never returns an error.
func (v *sigstoreVerifier) Verify(ctx context.Context, commit CommitData) CommitResult {
	res := CommitResult{SHA: commit.SHA}
	if len(commit.Signature) == 0 {
		res.Reason = ReasonUnsigned
		return res
	}

	// Verify the CMS signature and Fulcio certificate chain. No Rekor yet.
	cert, err := v.git.Verify(ctx, commit.Payload, commit.Signature, true)
	if err != nil {
		res.Reason = ReasonUnsigned
		return res
	}

	// Verify Rekor inclusion strictly offline, from the entry embedded in the
	// signature. We deliberately never fall back to gitsign's online Rekor search
	// (best-effort, rate-limited, and removed in Rekor v2), so a signature with no
	// embedded proof is rejected and verification never depends on the network.
	if _, err := v.rekor.VerifyInclusion(ctx, commit.Signature, cert); err != nil {
		res.Reason = ReasonNoTransparencyProof
		return res
	}

	sans := cryptoutils.GetSubjectAlternateNames(cert)
	issuer := (&cosign.CertExtensions{Cert: cert}).GetIssuer()
	if len(sans) == 0 || issuer == "" {
		res.Reason = ReasonParseFailure
		return res
	}
	res.Issuer = issuer

	for _, san := range sans {
		if _, matched := v.pol.Match(issuer, san); matched {
			res.OK = true
			res.Authority = AuthorityKeyless
			res.SignerSAN = san
			return res
		}
	}
	res.SignerSAN = sans[0]
	res.Reason = NotAllowlistedReason(strings.Join(sans, ", "), issuer)
	return res
}
