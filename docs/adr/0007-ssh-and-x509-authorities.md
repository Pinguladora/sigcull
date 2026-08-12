# ADR 0007: SSH and X.509 signature authorities

Status: accepted
Date: 2026-08-10

## Context

ADR 0005 introduced the authorities model with keyless (Sigstore), gpgKey
(OpenPGP), and githubVerified. Git itself supports exactly three commit
signature formats, selected by `gpg.format`:

- `openpgp`, covered by the gpgKey authority.
- `x509`, a CMS signature. gitsign is the Sigstore flavour of this, covered by
  keyless. Plain S/MIME (smimesign with a private CA) is the same wire format
  but was not covered.
- `ssh`, an SSHSIG signature, not covered at all.

To make the app generic across everything git can sign, two authorities close
the remaining gaps.

## Decision

Add two authorities, following the ADR 0005 pattern (each is a `Verifier` in the
crypto composite, in-process, no subprocess).

### sshKey

Verify the commit's SSH signature against a trusted key. Trust comes from an
OpenSSH `allowed_signers` file (`allowedSignersPath`), the same file `git`
consumes via `gpg.ssh.allowedSignersFile`. A commit passes if its signature
verifies against a key in that file whose principal is allowed
(`allowPrincipals`, empty meaning any principal in the file). The principal is
the reported identity (the `matchCommitter` comparison target).

Implementation: the SSHSIG format from OpenSSH's `PROTOCOL.sshsig` is parsed and
verified in-process with `golang.org/x/crypto/ssh` (already in the module graph
via go-git). There is no stdlib SSH-signature support, and x/crypto/ssh is the
Go team's canonical extension, so no third-party dependency is added. The wire
structs (`sshSigBlob`, `sshSignedData`) have a field order fixed by the spec:
`ssh.Marshal`/`Unmarshal` serialise in declaration order, so the order must not
be reordered for alignment. This is pinned with a `//nolint:govet` directive and
a comment, because a `fieldalignment` autofix would silently break interop.

### x509

Verify a detached CMS (S/MIME) signature whose signing certificate chains to a
trusted CA bundle (`caBundlePath`) and carries an allowlisted email
(`allowEmails`). It is the private-CA sibling of keyless: the same CMS wire
format, but trust is a private CA rather than Fulcio, and there is no Rekor
transparency requirement.

Implementation: `github.com/digitorus/pkcs7`. There is no stdlib CMS/PKCS7
package (the Go team declined the proposal), so the choice is a third-party
library or hand-rolled ASN.1. digitorus/pkcs7 is already a transitive dependency
of sigstore/gitsign (which keyless uses), cosign, and rekor, so using it
directly adds no new module; it only promotes an existing indirect requirement.
`VerifyWithChain` defaults to `ExtKeyUsageAny`, so an S/MIME `emailProtection`
certificate verifies without a server-auth trap.

## Consequences

- The app now covers all three git signature formats plus GitHub verification,
  so it is generic: keyless and x509 for CMS, gpgKey for OpenPGP, sshKey for
  SSH, and githubVerified as a cross-cutting trust source for any of them.
- Both authorities decline signatures that are not their format (an SSH
  authority declines a CMS blob, and vice versa), so they compose cleanly in any
  order behind the other authorities.
- `matchCommitter` extends naturally: it compares the committer against the SSH
  principal or the certificate email, as it already does for the keyless SAN and
  the GPG UID email.

## Alternatives considered

- **Hand-roll CMS with `encoding/asn1`** to avoid a third-party library.
  Rejected: it is a few hundred lines of security-sensitive ASN.1, higher risk
  than a maintained library that is already in the build.
- **Skip x509** since keyless already covers the Sigstore flavour. Rejected: the
  goal was to be generic, and plain S/MIME is a real (if niche) signing method.
