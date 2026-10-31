# Changelog

All notable changes to WeightCanary are documented here, newest first.
Releases are tagged `vX.Y.Z` on GitHub Releases.

## [v0.2.0] - 2026-10-31

Quiet correctness release — no new features, three fixes against the central
promise that the ledger records what is really there.

### Fixed

- **Hugging Face: follow tree pagination.** The `/tree` API pages at 1000
  entries and advertises the next page with a `Link: …; rel="next"` header;
  the poller read only page 1, so a repo with more entries was recorded
  silently truncated — marked `ok`, no partial flag, and every file beyond
  page 1 counted as missing from any mirror. The cursor is now followed until
  the listing ends (verified live: `facebook/mms-1b-all` went from a silent
  999-file snapshot to the full multi-page listing).
- **Gitee: a rejected access token is an error, not an absence.** HTTP 401 on
  the repo lookup (what Gitee answers when `GITEE_TOKEN` is stale — a public
  repo answers 200 anonymously) used to render every watched repo as
  `absent — repo missing or private`, a phantom takedown for the whole
  watchlist. It now surfaces as an error row naming `GITEE_TOKEN`; 404 alone
  still means absent.

### Added

- Version lockstep test pinning `VERSION`, the `weightcanary --version`
  constant and `web/site.json` `meta.source_version` together, plus this
  changelog.

## [v0.1.0] - 2026-09-21

### Added

- First release: Hugging Face / ModelScope / Gitee snapshot pollers
  (per-file presence, size, LFS sha256), the commit-friendly `state.json`
  ledger, the `watch` / `report` CLI, and the scheduled GitHub Action that
  keeps the ledger fresh.

[v0.2.0]: https://github.com/SuperMarioYL/weightcanary/releases/tag/v0.2.0
[v0.1.0]: https://github.com/SuperMarioYL/weightcanary/releases/tag/v0.1.0
