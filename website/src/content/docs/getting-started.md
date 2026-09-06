---
title: Getting started
description: Install the GitHub App and gate your first commit.
---

sigcull runs as a GitHub App. It receives a webhook on each push and pull
request, fetches the commits a branch introduces, verifies each one against your
configured authorities, and reports the outcome as a Check Run.

## Install the App

Create a GitHub App with Checks read and write, Contents read-only, and Metadata
read-only. Subscribe to the Push, Pull request, and Check run events. Point the
webhook at your deployment and set a webhook secret.

## Configure an authority

A commit passes if any configured authority accepts it. The simplest policy is a
single keyless authority that trusts one Sigstore identity.

```yaml
verify:
  authorities:
    - keyless:
        allow:
          - issuer: https://accounts.google.com
            san: you@example.com
```

## Make it required

Add `sigcull` as a required status check in a repository ruleset or branch
protection rule. From then on, merging a pull request requires all its commits to
satisfy the policy.
