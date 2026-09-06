---
title: "ADR 0006: fully in-process verification, no subprocess"
sidebar:
  label: "ADR 0006: in-process verify"
---

Status: accepted
Date: 2026-08-09

Supersedes the CLI approach in ADR 0001 and the CLI-vs-library split in ADR 0003.

## Context

v1 shelled out to two binaries: `git` (to shallow-fetch the commit range into a
temp checkout) and `gitsign verify` (to verify each commit). The `static` trust
source additionally used an in-library verifier behind a build tag (ADR 0003),
and BYO-TUF shelled out to `gitsign initialize`. So the running service needed
`git` and `gitsign` on the host, wrote to a temp filesystem, and spawned
processes per event.

We want zero subprocess and no filesystem dependency, so the app can run as a
single static binary in a minimal image.

## Decision

Everything runs in-process:

- **Fetch** uses go-git against an in-memory object store (memfs + memory
  storage). It fetches the range by SHA (GitHub allows arbitrary-SHA wants for
  token clients), passing the installation token via HTTP basic auth so it never
  appears in a URL. It returns already-extracted per-commit data.
- **Verify** uses gitsign as a library (`pkg/git`, `pkg/rekor`) with roots from
  a Sigstore trusted root. There is one verifier and no build tags; the CLI
  verifier is removed.
- **Roots** come from a trusted_root: `public` is a trusted_root.json vendored
  into the repo and embedded with `go:embed` (no network, refreshed by a task
  when Sigstore rotates keys), `static` is a file, and `tuf` is an in-library
  TUF client for a custom mirror. `gitsign initialize` is gone.

The `Verifier` contract changes from `Verify(ctx, repoDir, sha)` to
`Verify(ctx, CommitData)`, decoupling verification from how commits are fetched.

## Consequences

- **Runtime image is tiny**: a distroless-static base with no `git` or `gitsign`
  binaries, no shell.
- **No temp files, no cleanup, no process management.** Fewer failure modes and
  no token-in-URL leak surface.
- **The dependency tree is now always compiled in.** cosign, sigstore-go, and
  go-git are no longer isolated behind a build tag, so the Go binary is larger
  and the module graph heavier (including transitive cloud SDKs). This is the
  accepted cost of removing the subprocesses.
- **`public` has no startup network** thanks to the embedded root, at the cost of
  staleness: the vendored root must be refreshed (a scheduled task) when Sigstore
  rotates or adds keys. `tuf` still talks to its mirror by design; use `static`
  for a fully network-free private instance.
- **Output parsing is gone.** The old fragility of parsing `gitsign verify` text
  (ADR 0001) no longer exists; the identity comes straight from the verified
  certificate, so the pinned-format risk is retired.
