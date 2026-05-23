# Plan: Generate a Comprehensive `AGENTS.md` for a Spring Boot Platform (Sub-Agent Orchestration)

This document is a **plan you hand to an LLM**. The LLM acts as an **orchestrator** that
spawns **sub-agents** to analyze one Spring Boot application at a time and produces a
layered set of agent-facing docs. It is written for a model with a **~90,000-token
context window**, so no single agent ever loads the whole repo.

---

## 0. Inputs you must set before running

**Workspace layout.** This plan file lives in a **root workspace folder**; every repo to be
documented is a **sibling directory** next to it:

```
<workspace-root>/
├── agents-md-plan.md        ← this plan
├── AGENTS.md                ← root index (generated here; see §8.1)
├── application_1/           ← app repo (sibling)
├── application_2/           ← app repo (sibling)
├── ...                      ← more app repos
├── <helm-repo>/             ← infra repo (sibling)
└── <gocd-repo>/             ← infra repo (sibling)
```

The coding agent (e.g. **pi**) runs from `<workspace-root>`, acts as the **orchestrator**, and
processes **one sibling repo per pass**. `TYPE` decides the concern set (`app` → §7, `infra` →
§7B); `ARCHETYPE` decides which `app` concerns to spawn (§5). Each repo has its own **git
history** — read it (§4, §10).

```
CONTEXT_WINDOW   = 90000 tokens
WORKSPACE_ROOT   = <folder containing this plan file>
ROOT_INDEX_HOME  = WORKSPACE_ROOT     # the cross-repo root index AGENTS.md is written here
```

### Platform inventory (fill in all rows)

| # | NAME | TYPE | ARCHETYPE | ROOT (sibling dir) | BUILD/TOOL |
|---|------|------|-----------|--------------------|------------|
| 1 | application_1 | app | Kafka → transform → Kafka | `./application_1` | maven \| gradle |
| 2 | `<app>` | app | Kafka → Aerospike (writer) | `./<dir>` | maven \| gradle |
| 3 | `<app>` | app | API over Aerospike (reader) | `./<dir>` | maven \| gradle |
| 4 | `<app>` | app | Event publisher → Kafka | `./<dir>` | maven \| gradle |
| I1 | `<helm-repo>` | infra | Helm charts | `./<dir>` | Helm |
| I2 | `<gocd-repo>` | infra | GoCD pipelines | `./<dir>` | GoCD |

> Process one repo per pass. For each: set `CURRENT = {NAME, TYPE, ARCHETYPE, ROOT}`, run
> §4 → §9, then append the repo to the root index (§8.1). Repeat until every row is done.

---

## 1. Roles

- **Orchestrator** (1 instance): owns the plan. It does reconnaissance, decomposes the work,
  spawns sub-agents, and merges their outputs into the final docs. **It must never read full
  source files** — it works only from the cheap *Manifest* (§4) and from the compact *Section
  Briefs* (§6) returned by sub-agents. This is what keeps the orchestrator under budget.
- **Sub-agents** (N instances, one per concern in §7): each gets a **bounded slice** of the
  repo, answers a fixed question checklist, and returns one **Section Brief** in the exact
  schema of §6. A sub-agent sees only its slice — never the whole app.
- **Verifier** (1 instance, §9): re-checks the assembled docs against the code and flags any
  claim that is not backed by a file.

---

## 2. Hard constraints & token budget

The whole point of sub-agents here is **context isolation**. Enforce these caps:

| Agent | Input source payload | Reasoning headroom | Output cap |
|-------|----------------------|--------------------|------------|
| Orchestrator | Manifest (~3–5K) + all Briefs (N × ≤1.5K) | — | the 3 docs |
| Each sub-agent | **≤ 45K tokens of source** (~180 KB) | ~25K | **Brief ≤ 1.5K tokens** |
| Verifier | Manifest + the 3 docs | ~15K | findings list |

Rules of thumb (1 token ≈ 4 chars of code):
- **Never give one sub-agent more than ~45K tokens of file content.** If a concern's files
  exceed that, **split the concern** (§5) or instruct that sub-agent to read **signatures and
  key methods only**, not entire files.
- **Briefs are compressed, not transcripts.** A Brief is bullet facts + citations, never pasted
  code blocks longer than ~10 lines. This is why the orchestrator can hold 8–12 briefs at once.
- The orchestrator's running context = Manifest + Briefs only, so it stays well under 90K even
  with many sub-agents.

---

## 3. Output artifacts (layered, per the chosen shape)

Every repo (app or infra) gets the same **lean `AGENTS.md` + linked deep-dive** pair. One root
index ties them together.

1. **`ROOT_INDEX_HOME/AGENTS.md`** — thin **root index**. Platform-wide conventions + tables
   linking to every repo's `AGENTS.md` (apps **and** infra). Created on first pass, appended to.
2. **App repos** (`TYPE=app`):
   - `APP_ROOT/AGENTS.md` — lean operational contract (build/test/run, conventions, validate).
   - `APP_ROOT/docs/ARCHITECTURE.md` — deep-dive (architecture, domain, Kafka contract, rules, tests).
3. **Infra repos** (`TYPE=infra`):
   - `INFRA_ROOT/AGENTS.md` — lean operational contract (lint/template/validate, deploy, roll back).
   - `INFRA_ROOT/docs/DEPLOYMENT.md` — deep-dive (topology, chart catalog, GoCD pipeline catalog,
     environment matrix, secret flow, rollback).

Skeletons are in §8.

---

## 4. Phase 0 — Reconnaissance → the Manifest (orchestrator, cheap)

Goal: build a map of `CURRENT.ROOT` **without reading file bodies**. Use directory listing +
file sizes + first ~30 lines / signature scan, **plus a cheap read of git history** —
`git log --oneline -n 200` and `git log -n 20 --stat` on the busiest paths — to learn intent,
recent changes, and hotspots. Commit messages often explain *why* a rule exists (§10).

Produce a `MANIFEST` object:

```yaml
repo: application_1
type: app                  # app | infra
archetype: kafka-transform # from §0 inventory — drives concern selection (§5)
build_tool: maven
entrypoints:        # *Application.java, @SpringBootApplication
config_files:       # application.yml, application-*.yml, bootstrap.yml
build_files:        # pom.xml / build.gradle(.kts), settings
schema_files:       # *.avsc, *.proto, JSON schema
# entry/exit points present — these decide which §7 concerns to spawn:
kafka_in:           # @KafkaListener / consumer config (topics, group)
kafka_out:          # KafkaTemplate / producer config (topics)
rest_endpoints:     # @RestController / @*Mapping (the API archetype)
aerospike:          # AerospikeClient / repositories / namespace + set refs
scheduled_jobs:     # @Scheduled / triggers (often the event-publisher archetype)
source_tree:        # package -> classes with one-line role guess + token estimate
test_tree:          # unit vs integration split; Testcontainers/EmbeddedKafka markers
git_history:        # last N commit subjects; files changed most often; notable refactors
total_token_estimate_by_package:
```

The Manifest is the orchestrator's working memory and the input to decomposition. Keep it ≤ 5K
tokens. If a single file is huge, record its size so §5 can route it to a dedicated sub-agent.
Note: **app repos have no CI/CD, Helm, or k8s files** (those are the infra repos), so an app
Manifest has no infra section.

---

## 5. Phase 1 — Decompose into sub-agent tasks (orchestrator)

Split the repo **by concern, not by file count** — concerns map cleanly to AGENTS.md sections
and keep each slice coherent. Use the concern set for the repo's `TYPE`: **§7** for `app`,
**§7B** for `infra`.

Decomposition algorithm:
0. **Archetype routing**: spawn only the concerns whose entry/exit points the Manifest detected.
   An API-over-Aerospike repo spawns the REST + Aerospike-read concerns, not Kafka; a
   Kafka→Aerospike writer spawns Kafka-in + Aerospike-write, not REST. (Concern→archetype map is
   in §7.)
1. Assign each file in the Manifest to exactly one concern (§7 for apps, §7B for infra).
2. Sum the token estimate per concern.
3. If a concern ≤ 45K tokens → one sub-agent.
4. If a concern > 45K tokens → split it (e.g. "Transformation A: mappers" vs "Transformation
   B: rule engine"), or downgrade that sub-agent to **signatures + branch-bearing methods only**.
5. Emit a **task list**: `[concern, file globs, token estimate, question checklist ref]`.

Spawn sub-agents per the task list. They are independent — run them in parallel where the
framework allows.

---

## 6. The Sub-Agent Contract (identical for every sub-agent)

### 6a. Input template the orchestrator sends each sub-agent

```
ROLE: You are a senior {Spring Boot engineer | platform/DevOps engineer} analyzing ONE
  concern of {CURRENT.NAME}.   (pick the role from CURRENT.TYPE: app → Spring Boot; infra → DevOps)
CONCERN: {concern name}
YOUR FILES (read only these): {file list / globs}
TOKEN BUDGET: keep total reading ≤ 45K; if files exceed this, read signatures + the
  methods/templates that contain branching/business/deploy logic, and say so.
QUESTIONS TO ANSWER: {checklist from §7 (app) or §7B (infra) for this concern}
OUTPUT: exactly the Section Brief schema below. No preamble, no pasted full files.
GROUND RULES: §10 (cite file paths; never invent; mark unknowns as UNKNOWN).
```

### 6b. Section Brief schema (every sub-agent returns exactly this)

```markdown
## BRIEF: <concern>
### Facts
- <atomic fact> — `path/to/File.java:line`
### Commands (if any)
- `<exact command>` — purpose — source `path`
### Contracts (topics / schemas / endpoints / env vars, if any)
- name | direction/type | key fields | source `path`
### Business rules / behaviors (if any)
- <rule in one line> — trigger/condition → effect — `path:line`
### Gotchas & non-obvious coupling
- <thing that would surprise an editor> — `path`
### UNKNOWN / needs human input
- <question the code didn't answer>
```

Caps: ≤ 1.5K tokens; code excerpts ≤ 10 lines; every Fact/Rule/Contract carries a citation.

---

## 7. Sub-agent assignments — app repos (archetype-aware)

This is a **menu**. Each is one sub-agent (split per §5 if oversized); the checklist *is* the
prompt's question set. The orchestrator spawns only the concerns the archetype needs (§5 routing):

| Archetype | Concerns to spawn |
|-----------|-------------------|
| Kafka → transform → Kafka | SA-1, SA-2 (in+out), SA-3, SA-4, SA-5, SA-6 |
| Kafka → Aerospike (writer) | SA-1, SA-2 (in), SA-3, SA-4, SA-5, SA-6, SA-7 (Aerospike write) |
| API over Aerospike (reader) | SA-1, SA-2 (REST), SA-4, SA-5, SA-6, SA-7 (Aerospike read) |
| Event publisher → Kafka | SA-1, SA-2 (out + trigger), SA-3, SA-4, SA-5, SA-6 |

Any sub-agent may also run `git log -- <its files>` to learn *why* the code looks the way it does.

**SA-1 · Build, config & runtime**
Files: `pom.xml`/`build.gradle`, `application*.yml`, profiles. **App repos hold app code + build
config ONLY — no CI/CD, Helm, or k8s files; those live in the GoCD and Helm repos. Do not expect
them here and do not invent them.**
Ask: Java + Spring Boot versions; key dependencies; exact build / test / run commands; every env
var & config property the app reads + default; how it's started locally; health/readiness
endpoints. For *how this app is built into an image and deployed*, don't guess — cross-link to the
GoCD + Helm repo guides in the merged docs (§8.2 "Deployment").

**SA-2 · I/O contract — entry & exit points** (the app's boundary; tailor to the archetype)
Files: Manifest's `kafka_in`/`kafka_out`/`rest_endpoints`/`scheduled_jobs`, (de)serializers,
controllers, request/response models, error handlers.
Ask, for whichever points exist:
- **Kafka IN**: topic(s), consumer group, key/value type & deserializer, ack mode, concurrency.
- **Kafka OUT**: topic(s), key/value type & serializer, **what triggers a publish**, DLQ/error
  topic, retry/backoff, ordering/idempotency guarantees.
- **REST** (API archetype): each endpoint's method + path, request/response model, validation,
  auth, status codes, pagination.
- **Scheduled** (publisher archetype): cadence and what each job triggers.
Provide one representative inbound and one representative outbound payload.

**SA-3 · Transformation & business rules** (the "why" of this app)
Files: transformers, mappers, rule/validation/enrichment classes.
Ask: end-to-end pipeline from consume → transform → produce, as ordered steps; every business
rule and its condition→effect; validation and what happens on failure (drop / DLQ / throw);
enrichment / lookups; field-level mapping inbound→outbound.

**SA-4 · Domain model**
Files: DTOs, entities, enums, value objects, `*.avsc`/`*.proto`.
Ask: the core types and their fields; relationships; which are wire/schema types vs internal;
enums and their meaning; nullability / required fields.

**SA-5 · Cross-cutting (errors, observability, resilience, security)**
Files: exception handlers, `@ControllerAdvice`, logging config, Micrometer/metrics, tracing,
resilience4j/retry, secrets/config handling.
Ask: exception flow & what gets logged; log levels & correlation IDs; metrics & traces emitted;
retry/circuit-breaker/fallback behavior; how secrets are supplied (never print values).

**SA-6 · Tests as executable spec**
Files: `src/test` unit + integration.
Ask: how to run unit vs integration tests (exact commands); test framework & mocking strategy &
naming conventions an agent must follow; integration setup (Testcontainers / EmbeddedKafka /
fixtures); which behaviors/rules are pinned by tests (cross-reference SA-3); coverage gaps.

**SA-7 · Persistence & external integrations** (spawn when Manifest shows Aerospike/DB/cache/HTTP)
Files: `AerospikeClient` config & repositories, DB config, cache, external HTTP clients.
Ask:
- **Aerospike**: client/host config; **namespace + set(s) + key strategy + bin layout**;
  read vs write vs query patterns; secondary indexes; TTL / expiry; batch ops; failure handling.
  (For the writer archetype focus on writes/TTL; for the API archetype focus on reads/queries.)
- Other datastores / caches: schema, repository patterns, notable queries.
- External APIs called: endpoint, auth, timeouts, retry/fallback.

> SA-3 and SA-6 together give an app's *real* behavior; for Aerospike archetypes SA-7 is equally
> central — prioritize the depth of whichever concerns carry the archetype's core value.

---

## 7B. Sub-agent assignments — infrastructure repos (Helm + GoCD)

Use these when `TYPE=infra`. The Manifest (§4), 45K cap (§2/§5), Brief schema (§6), and Verifier
(§9) are reused unchanged — only the concerns differ. For infra repos the Manifest should index
`Chart.yaml`, `values*.yaml`, `templates/`, GoCD pipeline configs (YAML/JSON/`*.gocd.*`),
environment folders, and secret-management files instead of Java packages.

**Helm and GoCD are separate repos, so each pass uses a subset of the concerns below:**
- **Helm repo** → IA-1, **IA-2**, IA-4, IA-5  (skip IA-3)
- **GoCD repo** → IA-1, **IA-3**, IA-4, IA-5  (skip IA-2; IA-3 must capture that the GoCD pipelines
  consume the **app repos** (source/artifact) and the **Helm repo** (charts) as materials —
  this cross-repo wiring is the link between all three repos).

**IA-1 · Repo purpose & deployment topology**
Files: top-level README, repo layout, environment folders, cluster/namespace config.
Ask: what this repo deploys and for which apps; environments it covers (dev/stage/prod);
target clusters/namespaces; how a change here reaches a running app (the deploy path in one
paragraph).

**IA-2 · Helm charts**
Files: `Chart.yaml`, `values.yaml`, `values-<env>.yaml`, `templates/*`, subcharts.
Ask: chart-per-app vs umbrella chart; which app each chart deploys; what's templated
(Deployment, Service, ConfigMap, Secret, HPA, Ingress); how image repo/tag is wired; the
key values an agent edits per env; chart dependencies; `helm lint` / `helm template` commands.

**IA-3 · GoCD pipelines**
Files: pipeline definitions, templates, `*.gocd.yaml`/`.json`, config-repo wiring.
Ask: pipelines and their materials (which repos/branches trigger them); build vs deploy
pipelines; stage/job breakdown; the **promotion flow** across environments (auto vs manual
gates); artifacts produced/consumed; how a deploy is triggered and by whom; elastic-agent /
agent profiles; secure variables / secret references (describe, never print).

**IA-4 · Environments & secrets**
Files: per-env values/overrides, secret manifests, Vault/sealed-secret/SOPS config.
Ask: the full environment matrix (env → namespace → cluster → values file); how secrets are
sourced and injected; config that differs per env; what an agent must NOT hardcode.

**IA-5 · Change & rollback playbook**
Files: CONTRIBUTING/runbooks, Makefile/scripts, validation hooks.
Ask: how to add or modify a chart safely; how to add/modify a pipeline; pre-merge validation
commands (`helm lint`, `helm template`, GoCD config validation, schema checks); how to roll
back a release (Helm rollback / re-run pipeline); approval/ownership rules.

> IA-2 and IA-3 are the spine of an infra repo — prioritize their depth, the way SA-2/SA-3 are
> prioritized for apps.

---

## 7C. Phase 1.5 — Clarify with the human (before merge)

Every Brief surfaces gaps in its `UNKNOWN / needs human input` section (§6b). **Before writing
any doc for the repo**, the orchestrator (pi) aggregates every UNKNOWN across that repo's Briefs,
de-duplicates, and **asks YOU a single batched set of questions** — e.g. "what does rule X mean
for the business?", "is topic Y really the DLQ?", "which environment is authoritative?".

Rules:
- **Batch per repo** — don't interrupt for each Brief. Number the questions; cite the file that
  raised each.
- Prefer concrete, answerable questions ("A or B?") over open-ended ones.
- Fold answers back in as **cited facts** ("per owner, …"). Only what stays unanswered becomes an
  `UNKNOWN` under the doc's "Open questions" section.
- If the runtime can't prompt you mid-run, write the batched questions to
  `CURRENT.ROOT/docs/OPEN-QUESTIONS.md`, proceed best-effort, and flag them for review.

---

## 8. Phase 2 — Merge & synthesize (orchestrator)

Per repo, produce its two docs **in this order** (the deep-dive is generated *from* `AGENTS.md`):

**Step 1 — write `AGENTS.md` first** (the anchor). Distill the Briefs + the human's §7C answers
into the lean contract. Deduplicate facts (keep the best-cited); resolve conflicts by preferring
code/config citations over inference, noting any that remain.

**Step 2 — write the deep-dive *based on* `AGENTS.md`.** Use `AGENTS.md` as the section skeleton
and expand each section to full depth, drawing detail from the **retained Briefs** — so depth is
*not* limited to the lean file's text, but the deep-dive stays consistent with it (same commands,
topics, endpoints; more detail). Then update the root index (§8.1).

Throughout:
- **Route content**: commands / conventions / how-to-run / how-to-validate → `AGENTS.md`;
  architecture, domain, I/O contract, business rules, persistence, testing → the deep-dive.
- **Carry citations through.** Every command, topic, endpoint, and rule keeps its `path:line`
  (or commit) provenance. Remaining `UNKNOWN`s go to "Open questions."

### 8.1 `ROOT_INDEX_HOME/AGENTS.md` (root index — thin)
```markdown
# Platform — Agent Guide
One-paragraph platform purpose. Shared stack (Java/Spring Boot versions), shared conventions,
and the message-flow big picture (which app feeds which topic). Note that apps and infra live
in separate repos; links below are repo URLs/paths.

## Applications
| App | Purpose | In topic → Out topic | Guide |
|-----|---------|----------------------|-------|
| application_1 | <one line> | <in> → <out> | <app_1 repo>/AGENTS.md |

## Infrastructure
| Repo | Role | Covers | Guide |
|------|------|--------|-------|
| <helm-repo> | Helm charts | apps it deploys | <helm repo>/AGENTS.md |
| <gocd-repo> | GoCD pipelines | build + deploy | <gocd repo>/AGENTS.md |

## Deploy path (one paragraph, spans 3 repos)
How a code change flows: **app repo** (source/build) → **GoCD repo** (pipeline builds the image,
then runs Helm) → **Helm repo** (charts/values) → environment. Fill the exact trigger and
promotion steps from IA-3 once verified.

## Platform-wide conventions
- build/test commands common to all apps; commit/PR rules; how to run the stack locally
```

### 8.2 `APP_ROOT/AGENTS.md` (lean operational contract)
```markdown
# application_1 — Agent Guide
> Deep dive: ./docs/ARCHITECTURE.md

## What it does (3–4 lines)
Reads `<in-topic>`, applies <rules>, publishes to `<out-topic>`.

## Setup & commands
- Build: `<cmd>`   Run: `<cmd>`   Unit tests: `<cmd>`   Integration tests: `<cmd>`

## Deployment (NOT in this repo)
- This repo contains no CI/CD or k8s manifests. The app is built & deployed via the GoCD repo
  (<gocd repo>/AGENTS.md) using charts in the Helm repo (<helm repo>/AGENTS.md).

## How to validate a change (do this before declaring done)
- run unit + integration tests; the integration test that exercises consume→produce is `<path>`

## Conventions an agent MUST follow
- code style, package layout, test framework + naming, DTO/mapping patterns

## Kafka contract (summary)
- IN: `<topic>` group `<g>` type `<T>`   OUT: `<topic>` type `<T>`   DLQ: `<topic>`

## Gotchas
- non-obvious coupling, config that breaks things, ordering/idempotency caveats

## Open questions (unverified)
```

### 8.3 `APP_ROOT/docs/ARCHITECTURE.md` (deep-dive)
```markdown
# application_1 — Architecture
1. Overview & responsibility (bounded context)
2. Tech stack & dependencies
3. Runtime config — every env var / property + default + profile
4. Message flow — consume → transform → produce, as an ordered walkthrough
5. Kafka I/O contract — topics, groups, (de)serializers, ack, retry/backoff, DLQ, guarantees
6. Domain model — types, fields, relationships, schemas
7. Business rules & transformations — each rule: condition → effect → citation
8. Error handling, observability, resilience, security
9. Persistence / external integrations (if any)
10. Testing strategy — unit + integration, how behaviors are pinned, coverage gaps
11. Open questions / knowledge gaps
```

### 8.4 `INFRA_ROOT/AGENTS.md` (infra repo — lean operational contract)
```markdown
# <infra-repo> — Agent Guide
> Deep dive: ./docs/DEPLOYMENT.md

## What this repo does (3–4 lines)
Helm charts / GoCD pipelines that deploy <which apps> to <which environments>.

## Setup & commands
- Lint charts: `helm lint <chart>`   Render: `helm template <chart> -f values-<env>.yaml`
- Validate pipelines: `<gocd config validation cmd>`

## How to validate a change (before declaring done)
- chart change → `helm lint` + `helm template` for each affected env renders cleanly
- pipeline change → config validates; dry-run / preview if available

## Conventions an agent MUST follow
- where env values live; naming; what must NOT be hardcoded (images, secrets, namespaces)

## Deploy & rollback
- trigger a deploy: <how>   roll back: <helm rollback / re-run pipeline>

## Gotchas
- promotion gates, secret wiring, env-specific overrides that bite

## Open questions (unverified)
```

### 8.5 `INFRA_ROOT/docs/DEPLOYMENT.md` (infra repo — deep-dive)
```markdown
# <infra-repo> — Deployment & Operations
1. Purpose & deployment topology (apps × environments × clusters)
2. Helm chart catalog — each chart: app it deploys, templated resources, key values, image wiring
3. GoCD pipeline catalog — each pipeline: materials, stages/jobs, artifacts
4. Promotion flow — dev → stage → prod, gates (auto vs manual), who approves
5. Environment matrix — env → namespace → cluster → values file
6. Secret flow — how secrets are sourced & injected (no values printed)
7. Change & rollback playbook — add/modify a chart or pipeline; validation; rollback
8. Open questions / knowledge gaps
```

---

## 9. Phase 3 — Verification self-audit (Verifier sub-agent)

Spawn **one dedicated Verifier sub-agent** — its only job is to audit, never to author — with the
Manifest + the repo's drafted docs (`AGENTS.md` + its deep-dive, not full source). It must:
- Confirm **every command** in the docs appears in a real build/CI/chart/pipeline file.
- Confirm every **topic / env var / endpoint / class** (apps), **namespace / set / bin**
  (Aerospike), or **chart / values key / pipeline / namespace** (infra) referenced exists in the
  Manifest.
- Flag any claim **without a citation** or that looks inferred rather than evidenced.
- Confirm **`AGENTS.md` and the deep-dive are consistent** — no command, topic, or endpoint in
  one contradicts the other (the deep-dive must expand, not diverge from, the anchor).
- Confirm the **human's §7C answers were incorporated**, and only genuinely-open items remain
  under "Open questions."
- Apps: business rules trace to SA-3 citations and are pinned by SA-6 tests where claimed.
- Infra: the promotion flow and env→namespace matrix trace to IA-3/IA-4 citations.

Return a findings list: `OK | FIX <what> <where>`. Orchestrator applies fixes, then finalizes.

---

## 10. Ground rules (apply to every agent)

1. **Evidence or silence.** Every fact, command, topic, and rule cites `path` or `path:line`.
   No citation → don't state it.
2. **Never invent** class names, methods, topics, properties, or commands. If the code doesn't
   show it, write `UNKNOWN` and surface it under Open questions.
3. **Infer architecture, but label inference** ("Likely … based on `X`") vs. observed fact.
4. **Secrets**: describe *how* secrets are supplied; never print secret values.
5. **Compress**: Briefs and docs are bullets + citations, not pasted files.
6. Prefer config/code as source of truth over comments or naming.

---

## 11. Reusability across the platform

Iterate over every row of the §0 inventory. For each repo: set `CURRENT`, re-run **§4 → §9**
using the concern set for its `TYPE` (§7 for `app`, §7B for `infra`), write that repo's
`AGENTS.md` + deep-dive, then add one row to the matching root-index table (§8.1 — Applications
or Infrastructure). The Manifest (§4), Brief schema (§6), and Verifier (§9) stay identical, so
output is consistent across apps and infra repos alike.

---

### Execution checklist (orchestrator runs this loop once per §0 inventory row)
- [ ] §0 Pick next repo; set `CURRENT = {NAME, TYPE, ARCHETYPE, ROOT}`
- [ ] §4 Build Manifest (no file bodies; read git history)
- [ ] §5 Archetype routing + decompose into sub-agent tasks (enforce 45K cap; §7 app / §7B infra)
- [ ] §6/§7 Spawn sub-agents; collect Briefs (≤1.5K each)
- [ ] §7C Ask the human the batched UNKNOWNs; fold answers in
- [ ] §8 Merge → write AGENTS.md first, then the deep-dive *from* it
- [ ] §9 Dedicated Verifier pass → apply fixes
- [ ] §11 Append repo to root index (Applications or Infrastructure table)
- [ ] Repeat until all inventory rows are done
