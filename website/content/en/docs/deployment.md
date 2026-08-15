---
title: "Deployment"
description: "Run sigcull as a container and make it a required status check."
weight: 50
tags: ["operations"]
---

# Deployment

sigcull is a single stateless binary. It needs no database, no disk, and no
subprocess, so it ships as a small distroless image and scales horizontally.

## Run

Provide the secrets as environment variables and point the App at your config.

```sh
export GITHUB_WEBHOOK_SECRET=...
export GITHUB_APP_ID=...
export GITHUB_APP_PRIVATE_KEY="$(cat app.private-key.pem)"
go run ./cmd/sigcull -config config.yaml
```

## Container

```sh
docker build -f Containerfile -t sigcull .
docker run --rm -p 8080:8080 \
  -e GITHUB_WEBHOOK_SECRET -e GITHUB_APP_ID -e GITHUB_APP_PRIVATE_KEY \
  -v "$PWD/config.yaml:/etc/sigcull/config.yaml:ro" \
  sigcull
```

The webhook endpoint is `/webhook` and a liveness endpoint is `/healthz`. Set
`LOG_LEVEL` (`debug`, `info`, `warn`, `error`) to control verbosity. Logs are
structured JSON on stdout.

> [!NOTE]
> Processing is asynchronous: sigcull answers the webhook immediately, then
> verifies in the background. Run it as an always-on instance, not scale-to-zero
> serverless, so background verification always completes.

## Make it required

Add `sigcull` as a required status check in a repository ruleset or branch
protection rule. Pull requests then cannot merge unless every non-exempt commit
satisfies the policy.
