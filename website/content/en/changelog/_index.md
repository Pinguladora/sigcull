---
title: "Changelog"
description: "Notable changes to sigcull."
---

# Changelog

Notable changes to sigcull. The project follows conventional commits, so
releases are cut from the commit history.

## Unreleased

- Every git commit signature format is accepted through composable authorities:
  Sigstore keyless, OpenPGP, SSH, and X.509, plus GitHub's own verification.
- Verification runs fully in process. Commits are fetched in memory and verified
  with libraries, so the runtime needs no git or gitsign binary.
- The App private key can be mounted as a file, not only read from an
  environment variable.
- Logs are structured JSON on stdout, with a configurable `LOG_LEVEL`.
