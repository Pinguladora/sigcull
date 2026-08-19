---
title: Trust sources
description: Public good, bring-your-own TUF, or a static offline root.
---

`trust.kind` decides where the Sigstore roots of trust come from. All three run
fully in process, with no subprocess.

## public

The public good Sigstore instance. sigcull ships a vendored `trusted_root.json`
embedded in the binary, so this source needs no network at startup.

```yaml
trust:
  kind: public
```

## static

A local `trusted_root.json` file, read once at startup. Fully offline, for
air-gapped instances.

```yaml
trust:
  kind: static
  trustRoot: /etc/sigcull/trusted_root.json
```

## tuf

Bring your own TUF. sigcull builds a live in-library TUF client against a custom
mirror, using an out-of-band `root.json` as the trust anchor.

```yaml
trust:
  kind: tuf
  tufMirror: https://tuf.internal.example.eu
  tufRoot: /etc/sigcull/root.json
```

:::note
The identity allowlist is issuer agnostic. The set of issuers you can produce
signatures from is whatever the target Fulcio trusts, so a private instance can
mint identities the public good instance would never accept.
:::
