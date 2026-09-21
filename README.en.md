**English** | [简体中文](./README.md)

<div align="center">

<img src="https://readme-typing-svg.demolab.com?font=Noto%20Sans%20SC&weight=600&size=26&duration=3200&pause=1100&color=56E6A3&center=true&vCenter=true&width=980&height=90&lines=Watch%20the%20open%20weights%20you%20pinned%20in%20production%3BRelease%20pledges%20%C2%B7%20takedowns%20%C2%B7%20mirror%20divergence%3BKnow%20before%20the%20dependency%20breaks" alt="WeightCanary">

# WeightCanary

**A CLI that watches the availability of the open weights you depend on: Hugging Face snapshots, ModelScope/Gitee mirror comparison, a commit-friendly audit ledger**

[![CI](https://img.shields.io/github/actions/workflow/status/SuperMarioYL/weightcanary/ci.yml?branch=main&label=CI)](https://github.com/SuperMarioYL/weightcanary/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Release](https://img.shields.io/github/v/release/SuperMarioYL/weightcanary)](https://github.com/SuperMarioYL/weightcanary/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](./LICENSE)

Mirror: [Gitee](https://gitee.com/SuperMarioYL/weightcanary)

<p align="center"><picture><source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="./assets/hero-mobile-dark.svg"><source media="(max-width: 640px)" srcset="./assets/hero-mobile-light.svg"><source media="(prefers-color-scheme: dark)" srcset="./assets/hero-dark.svg"><img src="./assets/hero-light.svg" width="1000" alt="WeightCanary — a canary perched on a monitoring wire, watching weight availability on Hugging Face and ModelScope/Gitee."></picture></p>

</div>

The open weights you pinned in production can quietly 404 one morning. WeightCanary watches model availability on Hugging Face: whether the weights are still listed, whether the ModelScope/Gitee mirrors have diverged from the source — and it writes every watch run into a commit-friendly `state.json`, so git history doubles as the audit ledger. One command, one static binary, no account, no server.

## Why WeightCanary

- **Weight availability is a policy decision, not a technical guarantee.** The Muse Spark weights were publicly promised six weeks ago and still have not shipped, even after Spark 1.3 was released (as of 2026-09-21). The model id pinned in your requirements or deploy scripts will not warn you about any of that.
- **Nobody watches the mirror layer.** CN teams pull DeepSeek / Qwen / GLM weights through ModelScope and Gitee. The lag and divergence between those mirrors and Hugging Face is invisible — mirrors exist for throughput and do not advertise their own staleness, and "which file the mirror is missing" ends up being your incident.
- **Warning beats rescue.** Post-deletion torrent backups solve "get it back"; WeightCanary solves "know first": file listings, sizes and LFS sha256 values are compared every run, and git history records exactly what changed and when.

## Demo

The following is a real run (2026-09-21, live against the three default models; the full record lives in [docs/demo-results.json](docs/demo-results.json)):

```bash
$ weightcanary init
wrote models.yaml with the default watchlist:

  deepseek-ai/DeepSeek-V3.1
  Qwen/Qwen3-235B-A22B
  zai-org/GLM-4.5            (ModelScope id: ZhipuAI/GLM-4.5)

$ weightcanary watch
weightcanary report — 3 model(s) — generated 2026-09-21T00:12:42Z

deepseek-ai/DeepSeek-V3.1
  huggingface  ok     177 files  641.3 GiB  commit c0781d0
  gitee        error  0 files    0 B        gitee: HTTP 403 … — rate limited; set GITEE_TOKEN
  modelscope   ok     178 files  641.3 GiB  divergent: 3 changed, +1 extra on mirror

Qwen/Qwen3-235B-A22B
  huggingface  ok     128 files  437.9 GiB  commit 8efa617
  modelscope   ok     129 files  437.9 GiB  divergent: 1 changed, +1 extra on mirror

zai-org/GLM-4.5
  huggingface  ok     101 files  667.5 GiB  commit cbb2c7c
  modelscope   ok     102 files  667.5 GiB  divergent: 1 changed, +1 extra on mirror
```

How to read it: the DeepSeek-V3.1 weight shards and their LFS sha256 values match exactly between HF and ModelScope — what actually drifted is `README` / `LICENSE` / `.gitattributes` plus one mirror-only `configuration.json` (per-file detail is in the `--files` output). The Gitee row fails loudly when the anonymous API is rate-limited — the tool never reports a half-checked mirror as "in sync".

## Quickstart

```bash
# 1. Install (~30 s)
go install github.com/SuperMarioYL/weightcanary@latest
# or grab an archive from Releases (darwin/linux/windows × amd64/arm64)

# 2. Write the watchlist (~30 s)
weightcanary init

# 3. Add or remove models (~60 s): edit models.yaml

# 4. First snapshot + report (~60 s)
weightcanary watch

# 5. Commit the watchlist and the ledger — history is the audit trail
git add -f models.yaml && git add state.json
git commit -m "start watching model availability"
```

For hands-free watching, commit the bundled [.github/workflows/watch.yml](.github/workflows/watch.yml): GitHub Actions runs `weightcanary watch` every 6 hours and commits the refreshed `state.json` — no server involved.

To try the waters with a single model, use the minimal watchlist shipped in the repo:

```bash
weightcanary watch --config examples/models.minimal.yaml
```

## How it works

<p align="center"><picture><source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="./assets/architecture-mobile-dark.svg"><source media="(max-width: 640px)" srcset="./assets/architecture-mobile-light.svg"><source media="(prefers-color-scheme: dark)" srcset="./assets/architecture-dark.svg"><img src="./assets/architecture-light.svg" width="1000" alt="Three registry sources feed weightcanary's poller/state/report; diff and notify belong to m2."></picture></p>

```
models.yaml ──► weightcanary (single static binary)
                ├── poller   HF Hub API / ModelScope API / Gitee API → snapshot
                ├── report   terminal availability report (--files expands per-file rows)
                ├── state    state.json (the commit-friendly audit ledger)
                ├── diff     run-to-run diff and alert dedupe (m2, not implemented)
                └── notify   Feishu / DingTalk / Webhook (m2, not implemented)
schedule: cron or .github/workflows/watch.yml — no server, no database
```

One read-only REST endpoint per registry, no SDKs. Behavior verified against the live APIs:

- **Hugging Face**: `/api/models/{id}` for the commit sha, then `/tree/{sha}?recursive=true` for per-file size and `lfs.oid` (sha256). No per-file mtime is exposed, so freshness is size+sha256 comparison against the last snapshot. A missing or gated repo answers HTTP 401 (not 404) — both mean "absent" here.
- **ModelScope**: `/api/v1/models/{id}/repo/files?Recursive=true` returns the whole listing in one response; the `Sha256` of LFS weights matches HF's `lfs.oid`, so hashes compare across sites. A missing repo answers HTTP 200 with `Success=false`, not an HTTP error.
- **Gitee**: `/git/trees/{ref}?recursive=1` lists blobs, but LFS weights are ~134-byte pointer files — the real size and sha256 have to be decoded from the pointer content, fetched per small file. The anonymous API rate-limits aggressively; set `GITEE_TOKEN` for large mirrors. Listings with undecoded pointers are marked partial and labeled as such in the report — never presented as a verified comparison.

## Capabilities and integrations

<p align="center"><picture><source media="(max-width: 640px) and (prefers-color-scheme: dark)" srcset="./assets/integrations-mobile-dark.svg"><source media="(max-width: 640px)" srcset="./assets/integrations-mobile-light.svg"><source media="(prefers-color-scheme: dark)" srcset="./assets/integrations-dark.svg"><img src="./assets/integrations-light.svg" width="1000" alt="The central watch pulls Hugging Face, ModelScope and Gitee; the dashed outer ring is the m2 alert routing."></picture></p>

| Capability | Status | Notes |
| :--- | :--- | :--- |
| Snapshots across Hugging Face / ModelScope / Gitee | shipped | per-file presence, size, LFS sha256; HF rows carry the commit sha |
| HF vs mirror sync verdict | shipped | file set + size + LFS sha256 comparison with in sync / divergent detail |
| `state.json` audit ledger | shipped | every run lands on disk; commit it and git history becomes the trail |
| Scheduled snapshots (GitHub Action / cron) | shipped | `watch.yml` refreshes and commits the ledger every 6 hours |
| Takedown / divergence alerts (Feishu, DingTalk, Webhook) | m2 | first run is the baseline, then one change = one alert; card payloads verified against real bots before release |
| `report --markdown` and the release-pledge ledger | m3 | a "pledged 2026-08-10 — day N, no weights" ledger for the README |

## Configuration

`models.yaml` (written by `weightcanary init`, template in [models.example.yaml](models.example.yaml)):

```yaml
models:
  - id: deepseek-ai/DeepSeek-V3.1     # Hugging Face id, the model's primary key
    mirrors:                          # per-registry ids; omit one and that site is not watched
      modelscope: deepseek-ai/DeepSeek-V3.1
      gitee: hf-models/DeepSeek-V3.1  # the "Hugging Face 模型镜像" org's sync repo on Gitee

  - id: zai-org/GLM-4.5
    mirrors:
      modelscope: ZhipuAI/GLM-4.5     # note: ModelScope hosts GLM-4.5 under the ZhipuAI namespace
      gitee: hf-models/GLM-4.5
```

Environment variables: `HF_TOKEN` (gated repos), `GITEE_TOKEN` (private repos and higher rate limits).

Command line: `weightcanary watch -c models.yaml -s state.json [--files]`; `weightcanary report [-s state.json] [--files]` (`--markdown` belongs to m3 and is not implemented yet).

`state.json` is plain JSON: one `AvailabilityRecord` per model — HF state, each mirror's state, per-file sizes and LFS sha256 values, and the check timestamps. That file is the ledger: commit it, and "was it still there back then?" always has an answer.

## Pricing

The CLI is free and open source — everything in this repo (snapshots, comparison, the ledger, the scheduled Action) has no feature lock. The commercial path is the **hosted watcher**, launched after v0.1 ships:

| Plan | Price | Included |
| :--- | :--- | :--- |
| Hosted free tier | ¥0 | up to 5 watched models, scheduled refresh, Feishu/DingTalk alerts |
| Hosted Pro | ¥128/mo (~$19, per workspace) | unlimited models, team alert routing (Feishu / DingTalk / Slack / Webhook), private ledgers, longer retention |

Once a team receives its first real alert from the free CLI, pasting `models.yaml` into the hosted page hands the whole pipeline over in under 10 minutes — no Actions or secrets to maintain. Until the hosted service launches, this pricing is the roadmap commitment; the repo will never hold features hostage to force an upgrade.

## Roadmap

- [x] **m1 — snapshots and the availability report** (current): pollers for all three registries, the `state.json` ledger, `watch` / `report`, the scheduled refresh Action.
- [ ] **m2 — change detection and alerts**: run-to-run diff (takedowns, mirror divergence), first-run baseline, one-change-one-alert dedupe, Feishu / DingTalk / Webhook cards (payloads published after verification with real bots).
- [ ] **m3 — the pledge ledger**: a `pledges:` config block (promised date, artifacts, source link), the "pledged — day N, no weights" ledger, `report --markdown` for README badges and ledgers.
- [ ] **Hosted watcher**: free tier up to 5 models; Pro ¥128/mo. See "Pricing" above.

## Known limitations

- **Gitee anonymous rate limiting**: decoding LFS pointers for a large mirror (over a hundred weight shards) can exhaust the anonymous budget; the listing is then marked partial and labeled in the report. Set `GITEE_TOKEN` for reliable full runs.
- **No ModelScope repo-level commit sha**: mirror freshness is judged by file set + size + LFS sha256 (HF rows carry a commit sha that can gate the comparison).
- **Gated repos**: HF answers 401 for both missing and gated repos, reported uniformly as absent; set `HF_TOKEN` to watch your own gated repos.
- **Network position**: the HF poller talks to huggingface.co directly. hf-mirror.com only mirrors the `/resolve/` download paths; its `/api/*` behavior is geo-dependent (not verified in this build), so it cannot serve as an API-level fallback.
- **Metadata only, never mirrors weights**: the comparison covers file listings, sizes and LFS sha256 values — weight bytes are never downloaded.
- **No alerts yet**: until m2 there is no automatic notification; diff `state.json` in git to see what changed between runs.

## License

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
