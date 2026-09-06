---
title: "ADR 0002: fetch commits and verify locally, do not trust the GitHub API verification object"
sidebar:
  label: "ADR 0002: fetch, verify locally"
---

Status: accepted
Date: 2026-07-26

## Context

GitHub's REST API exposes a `commit.verification` object with a `verified`
boolean and a `reason`. It would be convenient to read that instead of fetching
commits ourselves.

## Decision

We ignore `commit.verification` and instead shallow-fetch the commit range into a
throwaway checkout, then run `gitsign verify` against the real commit objects.

## Consequences

- `commit.verification` is GPG and S/MIME oriented. It does not understand a
  Sigstore CMS signature carried in the commit's `gpgsig` header, so it cannot
  attest to a gitsign signature. Relying on it would silently accept or reject the
  wrong thing.
- We need the raw commit object anyway, because `gitsign verify` operates on a
  git repository and reads the signature from the object itself.
- The fetch uses an installation access token embedded in the clone URL
  (`https://x-access-token:<token>@github.com/<owner>/<repo>.git`). This works on
  private repositories, which is a primary goal. The token is scrubbed from every
  log line, and the checkout is a temp directory removed on every exit path.
- Force-pushes can make the range base unreachable. We fall back to verifying the
  shallow window of the head and mark the result truncated, surfaced as a note on
  the Check Run, rather than failing the whole run.

See also ADR 0001 for why verification itself shells out to gitsign in v1.
