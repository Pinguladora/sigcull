---
title: "Security model"
description: "Fail-closed verification, spoof-safe exemptions, and the trust boundaries."
weight: 60
tags: ["security"]
---

# Security model

sigcull is a gate. Its job is to decide whether a forged or unsigned commit can
merge, so every decision is designed to fail closed.

## Fail closed

Any ambiguity is a failure. An unsigned commit, a cryptographically invalid
signature, a valid signature whose identity is not on the allowlist, or a
signature whose identity cannot be read all produce a failing Check Run. A
commit is accepted only when an authority positively verifies it.

## Committer spoofing

The committer name and email in a commit object are attacker controlled. sigcull
never trusts them for a security decision on their own.

- `githubVerified` confirms through the GitHub API that GitHub itself signed the
  commit. Only GitHub can produce a signature that verifies for its own
  identity, so a pusher cannot forge it.
- `matchCommitter` binds the committer to the verified signing identity, so an
  allowlisted signer cannot commit under someone else's name.

## Identity exposure

A Check Run on a public repository is publicly visible. Signer email addresses
are masked by default in the Check Run output, while the full identity stays in
the app logs. Set `verify.showFullIdentity` to true on private repositories
where exposure is not a concern.

## Webhook authenticity

Every delivery is rejected unless its `X-Hub-Signature-256` HMAC verifies
against the configured secret. Nothing downstream runs on a bad or missing
signature.

> [!WARNING]
> A commit signed with a custom-domain email is only as trustworthy as that
> domain's control. Prefer identities tied to an OIDC issuer or a key you manage.
