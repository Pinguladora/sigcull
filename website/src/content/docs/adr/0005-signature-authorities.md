---
title: "ADR 0005: multiple signature authorities"
sidebar:
  label: "ADR 0005: signature authorities"
---

Status: accepted
Date: 2026-07-27
Accepted: 2026-08-09

## Context

Today the app accepts exactly one kind of signature: a gitsign (Sigstore
keyless) signature whose identity is on the allowlist. Real repositories carry
other legitimate signature sources:

- GitHub's own signatures on web merges and squash merges (the `web-flow` GPG
  key), which we currently only handle as a spoof-checked *exemption* (ADR 0004).
- Traditional OpenPGP / GPG signatures, which cosign and gitsign both recognise
  as a valid signing method.

Chainguard's policy format models this as a list of **authorities**, where a
commit is accepted if **any** authority accepts it:

```yaml
authorities:
  - keyless: { identities: [ { issuer: ... } ] }   # Sigstore
  - key:     { kms: .../web-flow.gpg }              # a specific key
github: { verified: true }
```

We want the same shape: a commit passes if it satisfies at least one configured
authority.

## Decision

Generalise from a single verifier to a set of **authorities**, evaluated with
OR semantics. The existing `Verifier` interface is the seam: each authority is a
`Verifier`, and a `CompositeVerifier` returns OK on the first authority that
accepts, otherwise aggregates the reasons.

Config. `verify.authorities` is the single source of truth: there is no
top-level `policy.allow`, so the keyless allowlist lives inside a keyless
authority entry. At least one authority is required. This is unreleased, so no
backward-compatibility shim was kept:

```yaml
verify:
  authorities:
    - keyless:                       # Sigstore / gitsign (current behaviour)
        allow:
          - { issuer: https://token.actions.githubusercontent.com, sanRegex: "..." }
    - gpgKey:                        # traditional OpenPGP
        keyringPath: /etc/sigcull/trusted-keys.asc
        # a commit passes if signed by a key in the ring whose UID email is allowed
        allowEmails: [ "dev@corp.com" ]
    - githubVerified:                # GitHub's own verification
        webFlowOnly: true            # only GitHub-authored web/squash merges
```

Authorities:

1. **keyless**, the current gitsign path. No change to behaviour when it is the
   only authority.
2. **gpgKey**, verify the commit's OpenPGP signature against a trusted keyring,
   extract the signing key and its UID email, and match against an allowlist.
   Implementation: verify in-process with `github.com/ProtonMail/go-crypto/openpgp`
   (the maintained fork; `x/crypto/openpgp` is frozen) to avoid a runtime `gpg`
   dependency. The commit's signature type is detectable from the `gpgsig`
   header (PGP vs CMS), so the composite can route to the right authority.
3. **githubVerified**, reuse the GitHub API `commit.verification.verified` check
   from ADR 0004. This promotes the spoof-safe exemption into a first-class
   authority, optionally restricted to GitHub's web-flow key so only genuine
   GitHub-authored merges qualify. Because it needs the per-installation GitHub
   client, which only exists at request time, it is not a member of the crypto
   `Composite`. The processor evaluates it as a fallback for commits that no
   crypto authority accepted. The `webFlowOnly` restriction is enforced by
   verifying the commit's signature against GitHub's vendored web-flow public
   key (embedded from https://github.com/web-flow.gpg), using the signature and
   payload the API returns. This is stronger than matching a self-reported
   issuer key ID: a forged issuer cannot pass, and a stale key only fails closed.

## Consequences

- **Result reporting** gains an "authority" dimension: the Check Run says which
  authority accepted a commit (keyless / gpg / github), and on failure lists why
  each authority rejected it. The `CommitResult` may need an `Authority` field.
- **matchCommitter** (ADR follow-on) is authority-specific: for keyless it
  compares the SAN, for gpgKey it compares the key's UID email. Document per
  authority.
- **Exemptions vs authorities** overlap: `skipGitHubAuthored` was removed, not
  deprecated (the project is unreleased). `githubVerified` replaces it: an
  authority that *accepts* GitHub-verified commits, rather than an exemption that
  *skips* verification, which is clearer and still spoof-safe. `skipMergeCommits`
  stays, since it has no authority equivalent.
- **GPG dependency**: `github.com/ProtonMail/go-crypto/openpgp` was already in the
  module graph via go-git, so the `gpgKey` authority added no new module download
  and needs no `gpg` binary, matching the no-subprocess design (ADR 0006).

This shipped as three atomic commits: the `Composite` + `Authority` seam (a pure
refactor with keyless as the sole authority), then the `gpgKey` authority, then
the `githubVerified` authority replacing the exemption.

## Alternatives considered

- **Shell out to `gpg`** for the GPG authority instead of an in-process library.
  Rejected as the default: it adds a runtime binary and process-management
  surface, though it could be a fallback for exotic key setups.
- **Keep exemptions instead of a GitHub authority.** Rejected: an authority that
  *accepts* GitHub-verified commits is clearer than one that *skips*
  verification, and both rest on the same non-spoofable API check.
