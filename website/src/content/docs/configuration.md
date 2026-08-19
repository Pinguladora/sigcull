---
title: Configuration
description: "The full sigcull policy reference: trust, authorities, committer matching, and exemptions."
---

sigcull reads a single YAML file. Secrets are never inlined. They are read from
the environment variables named in the file, or mounted as files.

## Secrets

Three values are required at runtime:

- `GITHUB_WEBHOOK_SECRET`: the webhook HMAC secret.
- `GITHUB_APP_ID`: the numeric App ID.
- `GITHUB_APP_PRIVATE_KEY`: the App private key PEM.

The private key can instead be mounted as a file with
`githubApp.privateKeyFile`, which avoids container platforms that mangle the
newlines in a multiline environment value.

```yaml
githubApp:
  appIDEnv: GITHUB_APP_ID
  privateKeyEnv: GITHUB_APP_PRIVATE_KEY
  # privateKeyFile: /etc/sigcull/app-key.pem   # takes precedence when set
```

## Trust

`trust.kind` selects where the verification roots come from: `public`, `tuf`, or
`static`. Switching between them is configuration only. See
[Trust sources](/trust-sources/).

## Authorities

`verify.authorities` is the ordered list of accepted signature sources. A commit
passes if any authority accepts it. Each entry sets exactly one kind. See
[Authorities](/authorities/) for the full list.

## Committer matching

`verify.matchCommitter` additionally requires the git committer to equal the
signing identity: an email subject against the committer email, a URI subject
against the committer name, or a GPG key's UID email. It closes the gap where an
allowlisted signer commits under someone else's name. It does not apply to
`githubVerified`, which has no signer identity to compare.

## Identity masking

`verify.showFullIdentity` is false by default, so signer email addresses are
masked in the Check Run output (`a***@example.com`), since a Check Run on a
public repository is publicly visible. The full identity always appears in the
app logs.

## Exemptions

```yaml
exempt:
  skipMergeCommits: false
```

:::danger
`skipMergeCommits` exempts any commit with more than one parent, which a pusher
can craft. Keep it off unless your merges are known to be safe. GitHub's own
merges are better handled by a `githubVerified` authority, which accepts rather
than skips them.
:::
