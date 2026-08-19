---
title: Documentation
description: Deploy sigcull, configure signature authorities, and make it a required check.
---

sigcull is a GitHub App that verifies the signature on every commit and turns
the result into a required Check Run, so unsigned or unauthorized commits cannot
merge into a protected branch. It works on private repositories and against any
Sigstore instance.

Start with [Getting started](/getting-started/) for a first
signed commit, then reach for:

- [Authorities](/authorities/) for the signature formats sigcull accepts.
- [Configuration](/configuration/) for the full policy reference.
- [Trust sources](/trust-sources/) for public, TUF, and static roots.
- [Deployment](/deployment/) for running the App.
- [Security model](/security-model/) for the trust boundaries.
