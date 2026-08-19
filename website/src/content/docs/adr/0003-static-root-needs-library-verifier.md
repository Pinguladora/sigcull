---
title: "ADR 0003: static-root offline verification uses a library verifier, not the gitsign CLI"
sidebar:
  label: "ADR 0003: static-root verifier"
---

Status: accepted
Date: 2026-07-26

## Context

We want a `static` trust source: verify a commit against roots pinned in a local
`trusted_root.json`, fully offline, with no TUF and no network. This also unlocks
a hermetic end-to-end test that needs no live OIDC, Fulcio, or Rekor at test time.

The v1 verifier shells out to the `gitsign verify` CLI. We checked whether that
CLI can do offline static-root verification.

## What the source says

Confirmed against the gitsign source:

- `gitsign verify` always initialises a TUF client to obtain the CT-log public
  key and the Rekor public key, which involves network calls.
- `GITSIGN_FULCIO_ROOT` / `SIGSTORE_ROOT_FILE` supply only the Fulcio CA root.
  TUF is still initialised for the other material.
- The CLI hides `--ca-roots` and related flags and notes it "only supports
  reading from a TUF root at the moment". There is no way to pass a
  `trusted_root.json`.
- `GITSIGN_REKOR_MODE=offline` avoids contacting the Rekor server (the entry is
  read from the signature), but it does not remove the TUF dependency for keys.

So the CLI structurally cannot do static-root, fully-offline verification.

## Decision

The `static` trust source is served by a **native library verifier**, not the
CLI. It reuses gitsign's own verification code as a library rather than
reimplementing any crypto:

- Load the pinned roots with sigstore-go `root.NewTrustedRootFromPath` to obtain
  the Fulcio CA pool, CT-log keys, Rekor keys, and any timestamp-authority certs.
- Build `git.NewCertVerifier` with `WithRootPool`, `WithIntermediatePool`, and
  `WithTimestampCertPool`, plus a cosign `CheckOpts` that carries the CT-log keys
  (for SCT) and enables offline Rekor inclusion checking.
- Read the commit object and its CMS signature and call `CertVerifier.Verify`,
  then apply the identity `Policy` in Go, exactly as the CLI verifier does.

The trust source selects the verifier: `static` routes to the native verifier,
`public` and `tuf` stay on the CLI. This is wired now. The native implementation
pulls in the gitsign, cosign, and sigstore-go dependency trees, so it lives
behind a build tag and, until built, fails closed with an actionable error
rather than pretending to verify.

## Consequences

- The clean, mostly-standard-library default build is preserved. The heavy
  dependency tree is only compiled into the native build.
- `static` is the correct home for the hermetic offline e2e: capture one real
  signed commit plus the `trusted_root.json` valid at signing time, then verify
  forever offline with no infrastructure.
- This supersedes the earlier roadmap note that lumped static-root under a
  generic "sigstore-go v2 verifier". The mechanism is settled here: reuse
  gitsign's `pkg/git` verifier with injected roots.
