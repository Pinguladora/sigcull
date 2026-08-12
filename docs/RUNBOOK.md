# Runbook

Operational guide for testing and deploying sigcull. Everything the code
can prove on its own is covered by `go test ./...`. The steps here cover what
needs a live GitHub App, OIDC, or a live Sigstore instance.

## 0. What is already proven

`go test ./...` runs the unit suite in-process with no network and no binaries:
policy and committer matching, the in-memory commit-range fetch against a local
repo, the worker pool, and webhook HMAC rejection and dispatch.

The remaining checks need a real signed commit (OIDC), a live Sigstore, or a
running host.

## 1. Pass-path end to end (a real signed commit)

Verification is in-process, so there is no fixture or gitsign binary to manage.
The realistic way to prove the pass path is live against a test repo with the App
installed:

1. Register the App and install it on a throwaway repo (section 3).
2. Set a keyless authority's `allow` to your gitsign identity. Find it by
   verifying one of your own signed commits and reading the SAN and issuer it
   reports.
3. Push a gitsign-signed commit whose identity is on the allowlist, and confirm
   the Check Run passes. Push an unsigned commit and confirm it fails.

## 2. BYO-TUF against an ephemeral Sigstore

This proves `trust.kind: tuf` against a private instance.

1. Stand up an ephemeral Sigstore with
   [`sigstore/scaffolding`](https://github.com/sigstore/scaffolding). Its setup
   brings up Fulcio, Rekor, a CT log, a TUF mirror, and a test OIDC issuer in a
   kind cluster, and exposes their URLs plus the generated `root.json`. Confirm
   the exact URLs and the `root.json` path it emits against the scaffolding
   version you use.

2. Point config at the instance:

   ```yaml
   trust:
     kind: tuf
     tufMirror: <scaffolding TUF mirror URL>
     tufRoot: <path to scaffolding root.json>
   ```

   At startup the app builds a live TUF client against `tufMirror` (using
   `tufRoot` as the trust anchor) and loads the roots in-library. No subprocess
   and no gitsign binary.

3. Sign a commit against the scaffolding instance, push it, and confirm the Check
   Run passes. Point the allowlist at the scaffolding issuer and the test
   identity, since the certificate carries the scaffolding issuer URL, not a
   public one.

Note: the app's identity allowlist is issuer-agnostic. The set of issuers you can
produce signatures from is whatever the target Fulcio trusts, so a scaffolding
instance can mint identities the public good instance would never accept.

## 3. Deploy the GitHub App

### Register the App

Create a GitHub App (org or personal) with:

- Permissions: Checks read and write, Contents read-only, Metadata read-only.
- Subscribe to events: Push, Pull request, and Check run (Check run enables the
  re-run button).
- Webhook URL: `https://<your-host>/webhook`
- Webhook secret: a random string.
- Generate and download a private key (PEM).

Install the App on the repositories or organization you want checked.

### Configure and run

Provide the secrets as environment variables named in `config.yaml`
(`config.example.yaml` is the template):

```sh
export GITHUB_WEBHOOK_SECRET=<the webhook secret>
export GITHUB_APP_ID=<numeric app id>
export GITHUB_APP_PRIVATE_KEY="$(cat app.private-key.pem)"
```

Container (distroless-static, no git or gitsign binary):

```sh
docker build -f Containerfile -t sigcull .
docker run --rm -p 8080:8080 \
  -e GITHUB_WEBHOOK_SECRET -e GITHUB_APP_ID -e GITHUB_APP_PRIVATE_KEY \
  -v "$PWD/config.yaml:/etc/sigcull/config.yaml:ro" \
  sigcull
```

For `trust.kind: static` (fully offline), mount the `trusted_root.json`
referenced by `trust.trustRoot` into the container.

The webhook endpoint is `/webhook` and a liveness endpoint is `/healthz`. The
process handles SIGINT and SIGTERM, stops accepting connections, and drains
in-flight verification before exiting.

### Expose and wire up

- Terminate TLS in front of the app and route `https://<your-host>/webhook` to
  it. For local testing, a tunnel such as `smee.io` or `ngrok` works.
- Confirm delivery: push a signed and an unsigned commit to a test repo and watch
  the Check Run appear as success and failure. The failure run names the
  offending SHA and reason.

### Make it required

In the target repository or ruleset, add `sigcull` as a required status
check (Repository Rulesets, or classic branch protection). Pull requests then
cannot merge unless every non-exempt commit satisfies the identity allowlist.

## 4. Acceptance checklist

- Commit signed by an allowlisted identity gives Check Run success.
- Unsigned commit, or a valid signature from a non-allowlisted identity, gives
  Check Run failure with an annotation naming the SHA and reason.
- Works on a private repo (installation-token clone succeeds).
- Switching `trust.kind` from `public` to `tuf` is config only, no code change.
- A bad or missing `X-Hub-Signature-256` is rejected and nothing is processed.
- A `githubVerified` authority accepts only GitHub-verified commits, confirmed
  through the API. A spoofed committer name does not self-accept.

## 5. Configuration reference

See `config.example.yaml` for the full annotated config. Key points:

- Secrets are read from the env vars named in the config, never inlined.
- `trust.kind`: `public` (embedded root), `static` (a trusted_root.json file), or
  `tuf` (live TUF from a custom mirror).
- `verify.authorities`: the accepted signature sources. A commit passes if any
  accepts it. Each entry is one of `keyless` (a Sigstore identity `allow`list,
  matching an exact OIDC `issuer` and a `san` or auto-anchored `sanRegex`),
  `gpgKey` (an OpenPGP `keyringPath` plus `allowEmails`), or `githubVerified`
  (GitHub's own verification, optionally `webFlowOnly`).
- `exempt.skipMergeCommits` exempts any commit with more than one parent, which a
  pusher can craft, so keep it off unless your merges are known safe.
