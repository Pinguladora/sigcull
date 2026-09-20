# Security Policy

## Supported versions

sigcull is pre-1.0 and is released from `main`. Security fixes land on the latest
release and the current `main`. Older tags are not backported.

| Version                     | Supported |
| --------------------------- | --------- |
| latest release (and `main`) | yes       |
| older tags                  | no        |

## Reporting a vulnerability

Please report security issues **privately** through GitHub's private vulnerability
reporting rather than a public issue, pull request, or discussion:

- Open <https://github.com/Pinguladora/sigcull/security/advisories/new>, or
- Go to the repository's **Security** tab and choose **Report a vulnerability**.

This creates a private advisory visible only to the maintainers.

When reporting, please include:

- a description of the issue and its impact,
- the version, commit, or image digest affected,
- steps to reproduce or a proof of concept,
- any suggested remediation.

## What to expect

- An acknowledgement within a few days.
- An assessment and, for a confirmed issue, a fix on `main` and a new signed
  release.
- Credit in the advisory or release notes if you would like it.

## Scope

sigcull verifies commit signatures and reports the result as a Check Run.
Especially relevant are signature-verification bypasses, webhook authentication
(HMAC) flaws, installation-token handling, and trust-root or Rekor-proof
validation. Release artifacts are signed with cosign (keyless) and carry SLSA
provenance, so supply-chain or provenance weaknesses are in scope as well.
