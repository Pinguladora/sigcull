---
title: "Documentation"
description: "Deploy sigcull, configure signature authorities, and make it a required check."
weight: 1
bookFlatSection: false
---

# Documentation

sigcull is a GitHub App that verifies the signature on every commit and turns
the result into a required Check Run, so unsigned or unauthorized commits cannot
merge into a protected branch. It works on private repositories and against any
Sigstore instance.

![sigcull checks every commit in the range through five stages: the GitHub event, the HMAC-verified webhook, a shallow fetch, per-commit verification with gitsign and the other authorities, and a Check Run that passes or fails with annotations.](images/pipeline.png "How sigcull verifies a commit")

Start with [Getting started]({{< relref "getting-started" >}}) for a first
signed commit, then reach for:

- [Authorities]({{< relref "authorities" >}}) for the signature formats sigcull accepts.
- [Configuration]({{< relref "configuration" >}}) for the full policy reference.
- [Trust sources]({{< relref "trust-sources" >}}) for public, TUF, and static roots.
- [Deployment]({{< relref "deployment" >}}) for running the App.
- [Security model]({{< relref "security-model" >}}) for the trust boundaries.
