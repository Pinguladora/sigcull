---
title: "Authorities"
description: "The signature formats sigcull accepts, evaluated with OR semantics."
weight: 20
tags: ["policy"]
---

# Authorities

An authority is one accepted source of a valid signature. A commit passes if
**any** configured authority accepts it, so you can mix the formats your team
uses. sigcull covers every signature format git can produce, plus GitHub's own
verification.

> [!NOTE]
> Authorities are tried in order. The first one that accepts a commit wins, and
> the Check Run reports which authority that was.

## keyless

Sigstore keyless signatures produced by gitsign. Each signature is verified
against a Fulcio certificate chain and a Rekor inclusion proof, then the signer
identity is matched against your allowlist.

```yaml
- keyless:
    allow:
      - issuer: https://accounts.google.com
        san: you@example.com
      - issuer: https://token.actions.githubusercontent.com
        sanRegex: "^https://github.com/acme/.+/.github/workflows/.+@refs/heads/main$"
```

The OIDC `issuer` is matched exactly. The subject is either `san` (exact) or
`sanRegex` (a Go regular expression, automatically anchored).

## gpgKey

Traditional OpenPGP signatures, verified in process against a trusted keyring.
The signing key must carry one of the allowed UID emails.

```yaml
- gpgKey:
    keyringPath: /etc/sigcull/trusted-keys.asc
    allowEmails:
      - dev@acme.com
```

## sshKey

SSH signatures (SSHSIG), verified against an OpenSSH `allowed_signers` file.
`allowPrincipals` optionally restricts which principals in that file are
accepted.

```yaml
- sshKey:
    allowedSignersPath: /etc/sigcull/allowed_signers
    allowPrincipals:
      - dev@acme.com
```

## x509

Detached CMS (S/MIME) signatures whose signing certificate chains to a private
certificate authority. This is the non-Sigstore sibling of keyless.

```yaml
- x509:
    caBundlePath: /etc/sigcull/ca-bundle.pem
    allowEmails:
      - dev@acme.com
```

## githubVerified

Commits GitHub itself cryptographically verified, such as web and squash merges.
With `webFlowOnly`, only GitHub's own web-flow key qualifies, not a user's
vigilant-mode key that GitHub merely verifies.

```yaml
- githubVerified:
    webFlowOnly: true
```

> [!WARNING]
> `githubVerified` rests on GitHub's own verification, confirmed through the API,
> so a spoofed committer cannot self-accept. See the
> [security model]({{< relref "security-model" >}}).
