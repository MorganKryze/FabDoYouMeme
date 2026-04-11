# Contributing

Thanks for considering a contribution. FabDoYouMeme is a small, opinionated
project maintained by one person. This file is short on purpose — the goal is
that you read it and start, not that I cover every edge case.

## Before you code

**Small fix** (typo, obvious bug, broken link, one-line tweak): open a PR
directly. No discussion needed.

**Anything bigger** — new feature, refactor, new dependency, new game type:
open an issue first and wait for a response. This isn't gatekeeping; it's so
you don't spend a weekend on code I won't merge for reasons I could have told
you in five minutes.

## Setting up

Everything you need is in the [README](../README.md). The dev stack runs on
Docker Compose — you do not need Go or Node installed locally unless you're
working on the backend or frontend individually.

## What I look for in a PR

- **Scope.** One PR = one change. "Refactored everything while I was in
  there" makes review miserable.
- **Tests.** If it's a bug fix, add a test that would have caught it. If it's
  a feature, cover the happy path and at least one edge case.
- **Passes CI.** Run these before pushing:

  ```bash
  cd backend && go vet ./... && go test -race -count=1 ./...
  cd frontend && npm run check && npm run build
  ```

- **Follows existing patterns.** If the codebase uses `sqlc`, don't introduce
  raw SQL. If it uses Svelte 5 runes, don't bring back stores. When in doubt,
  grep first.
- **Small diffs.** A PR that touches 40 files will get asked to split.
- **Conventional Commits** for messages: `feat:`, `fix:`, `docs:`, `build:`,
  `refactor:`, `test:`, `chore:`. Match the existing history.

## What won't get merged

- Reformatting or style-only changes unrelated to a functional fix.
- New dependencies without a clear justification. Every `require` or
  `package.json` line is a long-term maintenance cost.
- Features that couple FabDoYouMeme to a specific deployment, hosting
  provider, or paid service. Self-hosting is core to the project.
- AI-generated PRs that weren't actually tested. I'm not anti-AI — I use it
  myself. I'm anti "it compiled once on my machine."

## License

FabDoYouMeme is [GPLv3](../LICENSE). By submitting a PR you agree your
contribution is released under the same license. No CLA, no DCO sign-off —
a GitHub PR is enough.

## Questions or stuck

Open a GitHub issue or discussion. For sensitive stuff, email
**<contact@libresoftware.cloud>**.

See also: [Code of Conduct](./CODE_OF_CONDUCT.md).
