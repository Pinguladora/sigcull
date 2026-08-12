# sigcull

A GitHub App that verifies [gitsign](https://github.com/sigstore/gitsign)
(Sigstore keyless) commit signatures and reports the result as a **Check Run**,
so it can be made a required status check through Repository Rulesets or branch
protection.

It fills the gap that Chainguard Enforce / Guardener leaves. It works on
**private repositories** and against **any Sigstore instance** (the public good
instance, a bring-your-own-TUF setup, or a private Fulcio and Rekor), not only
the public good instance.

## How it works

```
GitHub  --webhook(push, pull_request, check_run)-->  sigcull
  1. verify the webhook HMAC (X-Hub-Signature-256)
  2. mint an installation token
  3. create a Check Run (in_progress)
  4. shallow-fetch the commit range in memory (go-git)
  5. for each commit: accept it if any configured authority does (keyless / gpg / ssh / x509 / github)
  6. update the Check Run (success or failure, per-commit annotations)
```

GitHub's own `commit.verification` API object is GPG oriented and cannot
validate a Sigstore signature, so sigcull fetches the commits and verifies
them itself. See [docs/adr](docs/adr) for the design decisions.

## Status

Verification runs **fully in-process, no subprocess**. Commits are fetched with
go-git into an in-memory store, and signatures are verified using gitsign as a
library, so the runtime needs no `git` or `gitsign` binary (see
[docs/adr/0006](docs/adr/0006-in-process-verification.md)).

Trust roots come from a Sigstore trusted root:

- `public`, a trusted_root.json vendored into the binary (`go:embed`), no
  network. Refresh it with `mise run roots:update` when Sigstore rotates keys.
- `static`, a local `trusted_root.json` file, no network.
- `tuf`, bring your own TUF: a live in-library TUF client for a custom mirror.

The heavy dependency tree (cosign, sigstore-go, go-git) is compiled into the
binary, but the runtime image is a distroless-static base with no binaries.

## GitHub App setup

Create a GitHub App with:

- **Permissions:** Checks read and write, Contents read-only, Metadata
  read-only.
- **Subscribe to events:** Push, Pull request, and Check run (the last enables
  the re-run button on a check).
- **Webhook URL:** `https://<your-host>/webhook`
- **Webhook secret:** a random string, also provided to the app as
  `GITHUB_WEBHOOK_SECRET`.

Install the App on the repositories or the organization you want to check, then
add `sigcull` as a required status check in a ruleset or branch protection
rule.

## Configuration

Copy `config.example.yaml` and adjust it. Secrets are read from the environment
variables named in the file, never inlined.

Required environment variables:

- `GITHUB_WEBHOOK_SECRET`: the webhook HMAC secret.
- `GITHUB_APP_ID`: the numeric App ID.
- `GITHUB_APP_PRIVATE_KEY`: the App private key PEM contents.

Key configuration sections:

- `trust.kind`: `public`, `tuf`, or `static`. Switching between them is
  configuration only, no code change. `public` uses the embedded root (no
  network). For `tuf` (bring your own TUF), set `tufMirror` and/or `tufRoot` (a
  TUF `root.json` obtained out of band); the app uses a live in-library TUF
  client against your mirror. For `static`, set `trustRoot` to a
  `trusted_root.json` path (no network).
- `verify.authorities`: the ordered list of accepted signature sources. A commit
  passes if **any** authority accepts it. Each entry sets exactly one kind:
  - `keyless`: Sigstore / gitsign keyless signatures. `allow` is the identity
    allowlist. A commit passes if any entry matches, where the OIDC `issuer` is
    exact and the SAN is either `san` (exact) or `sanRegex` (a Go regular
    expression, automatically anchored).
  - `gpgKey`: traditional OpenPGP. Verified in-process against `keyringPath` (an
    armored or binary public keyring). The signing key must carry one of
    `allowEmails` as a UID email.
  - `sshKey`: SSH signatures. Verified in-process against the OpenSSH
    `allowedSignersPath` file. `allowPrincipals` optionally restricts which
    principals (emails) in that file are accepted; empty means any of them.
  - `x509`: non-Sigstore S/MIME. A detached CMS signature whose signing
    certificate chains to the PEM `caBundlePath` and carries one of `allowEmails`.
    It is the private-CA sibling of `keyless` (same CMS format, no Rekor).
  - `githubVerified`: commits GitHub itself cryptographically verified. With
    `webFlowOnly: true` only GitHub's own web and squash merges qualify (its
    web-flow key), not a user's vigilant-mode key that GitHub merely verifies.
    It rests on GitHub's own verification, so a spoofed committer cannot
    self-accept.
- `verify.matchCommitter`: also require the git committer to equal the signing
  identity (email SAN vs committer email, URI SAN vs committer name, or a GPG
  key's UID email). Closes the gap where an allowlisted signer commits under
  someone else's name. It does not apply to `githubVerified`, which has no signer
  identity to compare.
- `verify.showFullIdentity`: by default signer email addresses are masked in the
  Check Run output (`a***@example.com`), since a Check Run on a public repo is
  publicly visible. Set it true to show full identities (fine for private repos).
  The full identity always appears in the app logs regardless.
- `exempt.skipMergeCommits`: exempts any commit with more than one parent, which
  a pusher can craft, so keep it off unless your merges are known to be safe.
  GitHub's own merges are better handled by a `githubVerified` authority, which
  accepts rather than skips them.

## Running

```sh
export GITHUB_WEBHOOK_SECRET=...
export GITHUB_APP_ID=...
export GITHUB_APP_PRIVATE_KEY="$(cat app.private-key.pem)"
go run ./cmd/sigcull -config config.yaml
```

Or with the container image, a distroless-static build with no `git` or
`gitsign` binaries:

```sh
docker build -f Containerfile -t sigcull .
docker run --rm -p 8080:8080 \
  -e GITHUB_WEBHOOK_SECRET -e GITHUB_APP_ID -e GITHUB_APP_PRIVATE_KEY \
  -v "$PWD/config.yaml:/etc/sigcull/config.yaml:ro" \
  sigcull
```

## Testing

```sh
go test ./...
```

The tests cover policy validation and matching, committer matching, the
in-memory commit-range fetch (against a local repo), the worker pool, and
webhook HMAC rejection and dispatch. Everything runs in-process, so there is no
gitsign binary or fixture to manage. Verifying a real signed commit end to end is
best done live against a test repo with the App installed (see
[docs/RUNBOOK.md](docs/RUNBOOK.md)).

## Layout

```
cmd/sigcull/main.go     entry point and per-event orchestration
internal/config               config load, secret resolution, authority wiring
internal/verify               authorities (keyless, gpg, ssh, x509), composite, root
internal/policy               identity allowlist matching
internal/trust                trust-source config shape
internal/gitfetch             in-memory (go-git) fetch of the commit range
internal/ghapp                app auth, webhook, Check Run, githubVerified authority
tools/fetch-trusted-root      refreshes the vendored public-good root
docs/adr                      architecture decision records
docs/RUNBOOK.md               testing, BYO-TUF, and deployment guide
```

## Operating

See [docs/RUNBOOK.md](docs/RUNBOOK.md) for end-to-end testing (including the
OIDC and BYO-TUF paths) and GitHub App deployment steps.
