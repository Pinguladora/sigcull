---
title: "ADR 0001: single-call gitsign verify plus in-process policy matching"
sidebar:
  label: "ADR 0001: single-call verify"
---

Status: accepted
Date: 2026-07-26

## Context

We verify each commit's gitsign (Sigstore keyless) signature and enforce an
allowlist of accepted identities, expressed as `{issuer, san | sanRegex}`
entries. Verification in v1 shells out to the `gitsign verify` binary.

`gitsign verify` requires, for keyless flows, both an identity matcher
(`--certificate-identity` or `--certificate-identity-regexp`) and an issuer
matcher (`--certificate-oidc-issuer` or `--certificate-oidc-issuer-regexp`). It
cannot run with no identity constraint. Its exit code is 0 for a valid signature
and non-zero otherwise. On success it prints, to stdout, a summary line:

```
gitsign: Good signature from [<san> ...](<issuer>)
```

That gives us two ways to enforce a multi-entry allowlist.

## Options considered

**A. Loop and delegate.** For each allowlist entry, invoke `gitsign verify` with
that entry's exact issuer and SAN matchers. The commit passes if any invocation
exits 0. gitsign does all matching, so there is no output to parse.

**B. Single call plus parse.** Invoke `gitsign verify` once with permissive
matchers (`--certificate-identity-regexp '.*' --certificate-oidc-issuer-regexp
'.*'`). This validates the certificate chain, Rekor inclusion, and SCT without
constraining the identity. Then parse the printed SAN and issuer and match them
against the allowlist in Go.

## Decision

We chose **B, single call plus parse**.

## Consequences

Reasons B wins:

- **It can name the offending identity.** An acceptance criterion requires that a
  valid signature from a non-allowlisted identity fails with an annotation naming
  the offending SHA and reason. Under A, every failed invocation looks the same,
  so a non-zero result cannot tell "unsigned" apart from "validly signed by
  someone not on the list", and there is no identity to report. Under B a
  successful exit gives us the real signer, so the annotation can say exactly who
  signed and that they are not allowed.
- **One subprocess per commit** instead of up to N (N being the allowlist size).
- **Policy logic stays in Go**, next to its tests, rather than being spread
  across process invocations.

Cost and how we contain it:

- B depends on gitsign's human-readable output format. We treat that as a pinned
  contract: the gitsign version is fixed in the Containerfile, and a golden-output
  unit test in `internal/verify` locks the parse to the observed v0.14.0 format
  (`Good signature from [%v](%s)`, SANs as a bracketed Go slice, to stdout).
- **Fail closed on parse miss.** If gitsign exits 0 but we cannot extract a SAN
  and issuer, the commit fails with `ReasonParseFailure` rather than passing. A
  format drift can therefore cause a false failure, never a false pass.

When the v2 native `sigstore-go` verifier lands behind the same `Verifier`
interface, it will return the identity directly and this parsing goes away, but
the fail-closed and identity-naming behaviour stays the same.
