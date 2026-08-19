---
title: "ADR 0004: commit exemptions must be corroborated, not trusted from commit metadata"
sidebar:
  label: "ADR 0004: exemption trust"
---

Status: accepted (partially superseded by ADR 0005)
Date: 2026-07-26

> The `skipGitHubAuthored` exemption described here was removed in ADR 0005 and
> replaced by a first-class `githubVerified` authority. The corroboration
> principle (never trust commit metadata, confirm through the GitHub API) still
> holds and carries over to that authority. `skipMergeCommits` is unchanged.

## Context

Some commits are unsigned by design, notably the merge and squash commits GitHub
creates through the web UI. To avoid failing those, the app has two exemption
settings: `skipMergeCommits` and `skipGitHubAuthored`.

The obvious implementation reads the commit's own metadata: parent count for
merges, committer name and email for GitHub-authored commits. That is unsafe. A
pusher fully controls the committer name and email, and can craft a commit
object with any number of parents. So an allowlist based on that metadata alone
lets an attacker set committer email to `noreply@github.com`, or add a second
parent, and self-exempt from signature checking. That defeats the entire
fail-closed premise of the tool.

## Decision

An exemption is only granted when it is corroborated by something the pusher
cannot forge.

- `skipGitHubAuthored`: the committer string is used only as a cheap pre-filter.
  The commit is exempted only if GitHub's API reports that GitHub verified the
  signature (`Git.GetCommit(...).Verification.Verified == true`). Only GitHub can
  produce a signature that verifies for its own `noreply` identity, so a true
  result cannot be forged. Any API error fails closed and the commit is verified
  normally.
- `skipMergeCommits`: this remains a structural check on parent count, because a
  legitimately merged local commit is not GitHub-signed and there is nothing to
  corroborate against. It is therefore an explicit operator trust decision. The
  default is off, and the config and README warn that a pusher can craft a merge
  commit to bypass the check. Operators enable it only when their merges are
  known to be safe.

## Consequences

- `skipGitHubAuthored` is safe to leave on by default.
- The GitHub-verified check costs one API call per candidate commit, made only
  for commits whose committer string looks like GitHub, so the common path is
  unaffected.
- Exemptions never widen the trust surface silently. Anything not corroborated is
  verified.
