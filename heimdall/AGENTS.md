# AGENTS.md — Heimdall (read this first)

> Audience: an AI coding agent (e.g. MiniMax via the *pi* agent) picking up this repo on a fresh
> machine with **no prior conversation context**. This file is self-sufficient: it tells you what
> Heimdall is, how it's built, how every file fits together, how to run and verify it, the
> non-obvious decisions you must not break, and how to make common changes safely.
>
> **Reconstructing the repo from docs only?** Use the three-file set together: this `AGENTS.md`
> (the map), `IMPLEMENTATION_APPROACH.md` (the spec + per-class contracts), and **`FULL_SOURCE.md`**
> (the exact, byte-faithful content of every file + the wrapper-bootstrap step).

---

## 1. What Heimdall is

Heimdall is a **Java 21 end-to-end data-propagation test framework** for the *Giftwrapper* banking
platform. For each test scenario it publishes JSON to a Kafka topic and then asserts the data
propagates correctly through the pipeline:

```
publish → Kafka_topic_1 → cbs-denormaliser → Kafka_topic_2 → cbs-kafka-aerospike-consumer
        → Aerospike → cbs_web_services (REST API)
```

Per scenario it: **(1)** publishes the input JSON to Kafka, **(2)** waits a fixed interval,
**(3)** runs an AQL query and compares the Aerospike record against a golden JSON file, **(4)** runs
a curl request and compares the API response against a golden JSON file. Scenarios are defined in a
**CSV manifest** (one row per scenario). Heimdall is a **pure client against configurable
endpoints** — you point it at running services via `test-data/config.json`; it does not own the
services. Results are written as CSV + a self-contained HTML report + JUnit-XML.

It is itself a test suite, so **it ships with no automated tests of its own** (see §9).

---

## 2. Quick start

**Prerequisites**
- **JDK 21** — the Gradle toolchain auto-provisions one (via the foojay resolver) if the host has an
  older JDK, *provided the first build has network access*. The Gradle launcher itself needs JDK 17+.
- **Docker** — only for the default Aerospike `aql` execution mode (`docker-run`) and the optional
  local environment. Not needed if `aerospike.executor=host` with a local `aql`.
- **bash + curl** — for the API phase (standard on macOS/Linux/CI).

**Commands** (the developer UX is the `Makefile`):
```bash
make build     # ./gradlew clean build   — compile (must be warning-free)
make run       # ./gradlew runScenarios  — run all scenarios against configured endpoints
make report    # open build/heimdall-report/heimdall-report.html
make clean     # remove build output
# optional local env (replace __REPLACE_ME__ images in docker/.env first):
make up        # docker compose up -d
make down      # docker compose down -v
```
Override inputs: `make run CONFIG=path/to/config.json CSV=path/to/scenarios.csv`.

**First build downloads** the Gradle distribution, a JDK 21, and Maven dependencies — give it
network and a minute.

**Exit codes** (of `HeimdallMain`): `0` all scenarios pass · `1` ≥1 FAIL/ERROR · `2` framework/config
error. Note: through `make run`/Gradle, any non-zero app exit surfaces as "BUILD FAILED" (Gradle
collapses it), so via `make` you observe **0 = pass, non-zero = failure**; the precise 1-vs-2 is only
seen running `HeimdallMain` directly. Reports are always written before exit.

---

## 3. Repository layout

**Single Gradle module, single package `com.giftwrapper.heimdall`.** ~14 small, cohesive classes.

```
heimdall/
├─ build.gradle.kts        # single module: Java 21 toolchain, deps (inlined), runScenarios task
├─ settings.gradle.kts     # rootProject + foojay toolchain resolver (no submodules)
├─ gradle.properties · gradlew · gradlew.bat · gradle/wrapper/   # Gradle 8.10.2 wrapper
├─ Makefile                # developer UX (build/run/report/up/down/clean)
├─ src/main/java/com/giftwrapper/heimdall/*.java   # ALL source (one package)
├─ src/main/resources/logback.xml                  # JSON logging config
├─ test-data/              # config.json, scenarios.csv, input/ aql/ api/ expected/  (sample + the self-check)
├─ docker/                 # OPTIONAL local env: docker-compose.yml + .env (cbs images = __REPLACE_ME__)
├─ AGENTS.md               # this file
├─ README.md               # user-facing run guide
├─ IMPLEMENTATION_APPROACH.md   # full design + rationale (decisions D1–D19). NOTE: §6 describes the
│                               #   ORIGINAL 9-module layout (historical) — §15 records the collapse.
├─ TECH_DEBT.md            # parked decisions (P-01…P-20), testing stance, §D structure note
├─ CLAUDE.md               # session bootstrap (Claude Code); locked decisions
└─ first_prompt.rtf        # the original user requirements (reference only)
```

### Source files and their responsibility
| File | Responsibility |
|------|----------------|
| `HeimdallMain.java` | Entry point. Loads config + scenarios, wires collaborators, runs, cleans up, writes reports, computes exit code. |
| `Config.java` | All config records (`Root`, `Profile`, `Kafka`, `ProducerCfg`, `Wait`, `Aerospike`, `Api`, `Auth`, `Validation`, `Arr`, `TestData`, `Cleanup`, `Report`, `Execution`) **plus** `Config.load(Path)` (read → `${ENV:default}` substitution → bind → validate). |
| `Model.java` | Domain types: enums `Phase{KAFKA,AEROSPIKE,API}`, `Status{PASS,FAIL,SKIPPED,ERROR}`; records `Scenario`, `PhaseResult`, `ScenarioResult`, `ValidationOutcome`, `ApiResponse`. |
| `ScenarioCsvLoader.java` | Parse/validate the CSV manifest → `List<Scenario>` (resolves file paths, phase-skip on empty cells, dup/bad-ref detection). Also `readInput(Path)` → `PublishPayload` (object⇒1 msg; array⇒N msgs in order). |
| `KafkaPublisher.java` | Idempotent producer; publishes a scenario's messages with prerequisite semantics (ordered, same-partition key, all-or-nothing). `AutoCloseable`; one instance reused for the run. |
| `Aql.java` | The Aerospike phase: builds the `aql` command for one of 3 modes, runs it, defensively parses `-o json` output into record bin-maps; also `cleanup(...)` (end-of-run truncate). |
| `Validator.java` | json-unit comparison (lenient/strict, array opts); prunes `ignorePaths` from both sides via Jayway JsonPath; `shapeAerospikeActual(...)` reconciles the record list with the golden's shape. |
| `CurlApiExecutor.java` | Runs one curl command via `bash -c`, captures body + HTTP status; honours `curlBinary`/`baseUrlEnv`. |
| `Reporters.java` | Writes CSV, self-contained HTML (filter/sort/expand, no CDN), and JUnit-XML; gated by report toggles. |
| `ScenarioRunner.java` | The §4 orchestration per scenario; assembles `ScenarioResult`; applies PII masking to snapshots/diffs. |
| `PiiMasker.java` | Shared masker: field-name masking of JSON + value-scrubbing of free text. |
| `Json.java` | Shared Jackson `ObjectMapper` (+ helpers). Configured so it can bind into package-private config records. |
| `ProcessRunner.java` | Runs an external process with a hard timeout, draining stdout/stderr on separate threads. |
| `HeimdallException.java` | The single exception type (config/load errors → exit 2; phase errors are caught → ERROR). |

---

## 4. End-to-end execution flow (per scenario, sequential)

Implemented in `ScenarioRunner.runOne(Scenario)`:
1. `correlationId = scenarioNumber + "-" + runId`; set MDC (`correlationId`, `scenarioNumber`, `phase`).
2. **KAFKA**: `ScenarioCsvLoader.readInput()` → publish via `KafkaPublisher.publish()`.
   - Array input ⇒ **one message per element, in order, same partition key, all-or-nothing** (these
     are *prerequisites*: the downstream app emits the final Aerospike record only once all arrive).
   - If Kafka does not PASS ⇒ AEROSPIKE + API are **SKIPPED**; scenario ends.
3. **WAIT**: fixed interval `wait.waitMsFor(tableName)` (no polling).
4. **AEROSPIKE** (if both query + golden present): `Aql.query()` → records; read golden;
   `Validator.shapeAerospikeActual()` → `Validator.compare()`. A FAIL here does **not** skip API.
5. **API** (if both curl + golden present): `CurlApiExecutor.execute()` → `ApiResponse`; compare body
   to golden. PASS requires HTTP 2xx **and** body match.
6. **assemble**: overall = ERROR if any phase ERROR, else FAIL if any FAIL, else PASS (first offending
   phase in execution order is the `failurePhase`).
7. After all scenarios: `Aql.cleanup()` once (best-effort), then `Reporters.write()`.

Snapshots (published payload / AQL+result / curl+response) and diffs are **PII-masked** when the
`PhaseResult` is built; comparison runs on **unmasked** values so PII fields are genuinely validated.

---

## 5. Configuration & inputs

### `test-data/config.json` (profiles, with `${ENV:default}` substitution)
Key fields: `kafka.bootstrapServers`; `wait.defaultMs` + `wait.perTable`; `aerospike.executor`
(`docker-run` default | `docker-exec` | `host`), `host` (macOS: `host.docker.internal`), `namespace`,
`timeoutMs`; `api.baseUrlEnv` (env var substituted into the curl, default `API_BASE`), `api.curlBinary`;
`validation.mode` (`lenient` default | `strict`), `validation.ignorePaths` (JSONPath, e.g. `$..lastUpdatedTs`),
`validation.array.{ignoreOrder,ignoreExtraItems}`; `report.{csv,html,junitXml,maskPii,piiFields}`;
`cleanup.{enabled,strategy,sets}`; `execution.failExitCodeOnScenarioFailure`.

### CSV manifest (`test-data/scenarios.csv`) — columns, in order
`ScenarioNumber, TableName, TopicName, input_json_file_name, aerospike_query_file_name,
aerospike_output_json_file_name, api_endpoint_files, api_output_json_file_name`.
- `input_json_file_name`: object ⇒ 1 message; **JSON array ⇒ 1 message per element (prerequisites)**.
- A phase is run only if **both** of its columns are filled (query+golden, or curl+golden); an empty
  cell skips that phase. The loader fails fast if a referenced file is missing or a phase is half-specified.

### `test-data/` layout
`input/` (input JSON), `aql/` (one AQL query per file, run verbatim), `api/` (one curl command per file),
`expected/aerospike/` + `expected/api/` (golden JSON). Resolved under dirs from `config.json → testData`.

---

## 6. Non-obvious decisions you MUST NOT break

These were deliberate fixes/choices. Reverting them reintroduces real bugs:

1. **Recursive `ignorePaths` (`$..field`)**: json-unit 3.4.1's `whenIgnoringPaths` does **not** honour
   recursive descent — and `config.json` ships paths like `$..lastUpdatedTs`. `Validator` therefore
   **prunes ignorePaths from both expected and actual via Jayway JsonPath** before comparing. Do not
   switch back to json-unit's `whenIgnoringPaths`.
2. **`runScenarios` must launch with the Java 21 toolchain** (`javaLauncher.set(javaToolchains…21)` in
   `build.gradle.kts`). Without it the task runs on the Gradle JVM (possibly 17) and fails with
   `UnsupportedClassVersionError`.
3. **`max.block.ms` is bounded** to `ackTimeoutMs` in the producer config so an unreachable broker
   fails fast instead of blocking ~60s.
4. **Jackson + package-private records**: config records are package-private; `Json` sets creator +
   field visibility to `ANY` so Jackson can bind them. Keep that, or make the records public.
5. **Name choices avoiding clashes** (single package): the AQL class is `Aql` (the config record is
   `Aerospike`); the Kafka producer config record is `ProducerCfg` (Kafka's interface is `Producer`);
   in `Validator`, Jayway's `Option`/`Configuration` are fully-qualified to avoid clashing with
   json-unit's. A record component cannot be named `wait` (clashes with `Object.wait()`) — it's
   `waitConfig` with `@JsonProperty("wait")`.
6. **AQL is passed as a single process argument** (no shell) → no quoting/escaping needed and no
   injection risk. **curl runs via `bash -c`** so shell env expansion (`${API_BASE}`) works; appended
   flags (`-s -o <tmp> -w '%{http_code}'`) win over duplicates; the temp body file is always deleted.
7. **PII masking discipline**: compare on unmasked data; mask only when building report/log output
   (`PiiMasker.maskJsonString` for JSON snapshots, `maskValuesIn` for diffs/free text).
8. **Lint exclusions** in `build.gradle.kts`: `-auxiliaryclass` (we group several types per file in
   one package on purpose) and `-serial` (the one exception is never serialized cross-JVM). The build
   must stay **warning-free**.

---

## 7. How to make common changes (there are no SPIs — wire directly)

The original `ServiceLoader`/SPI machinery was removed for simplicity; behaviour is wired directly.
- **New AQL executor mode** → add a case to the `switch` in `Aql.buildCommand(...)`.
- **New report format** → add a `writeXxx(...)` method in `Reporters`, a toggle field in the `Report`
  config record (+ `config.json`), and a branch in `Reporters.write(...)`.
- **Different validation** → edit `Validator.compare(...)` / `buildConfiguration(...)`.
- **API auth** (currently none) → add header logic in `CurlApiExecutor` (append `-H` flags) keyed off
  `api.auth.type`.
- **New config field** → add it to the relevant record in `Config.java` and to `config.json`.
- **New dependency** → add to `dependencies {}` in `build.gradle.kts` (versions are inlined, no catalog).

---

## 8. Conventions
- **Java 21**, single package, types **package-private by default** (only `HeimdallMain` is public).
- **Errors**: throw `HeimdallException` for config/manifest/load problems (→ exit 2). Phase code
  catches its own failures and records an ERROR `PhaseResult` (→ exit 1); never let a phase exception
  abort the whole run.
- **Logging**: SLF4J + Logback JSON (`logback.xml`); `ScenarioRunner` sets/clears MDC keys
  `correlationId`/`scenarioNumber`/`phase`. Override level with `-Dheimdall.log.level=DEBUG`.
- Keep the build warning-free; keep masking applied on every output path.

---

## 9. Testing & verification (IMPORTANT — no shipped tests)

Heimdall *is* an E2E test suite, so by decision **D18** it ships **no automated tests**. Dev-time
tests were used during development and removed before delivery. To verify any change:

1. `make build` — must be **BUILD SUCCESSFUL with zero warnings**.
2. **No-services smoke test**: `make run` against the sample data with nothing running. Expected:
   every scenario FAILs at the KAFKA phase (broker unreachable, ~5s fail-fast), downstream SKIPPED,
   **all three reports written**, PII masked (no raw `pan`/`accountNo` in any report), exit non-zero.
   This exercises the entire plumbing except live-service PASS.
3. **Full self-check** (needs real services): the shipped `test-data/scenarios.csv` has `HD_001`
   (single object, PASS-capable), `HD_002` (array/prerequisite, PASS-capable), `HD_003` (deliberately
   wrong Aerospike golden → must **FAIL at AEROSPIKE**, no API columns). Point `config.json` at running
   Kafka/Aerospike/API and align the golden files to your data, then `make run`.

If you add dev-time tests: add `src/test/java/...`, add `testImplementation("org.junit.jupiter:junit-jupiter:5.10.3")`
to `build.gradle.kts`, and **remove them before final delivery** (D18). Tracked in `TECH_DEBT.md` (T1/T2).

---

## 10. What's not done / parked (don't assume it exists)

See `TECH_DEBT.md` for the full list (P-01…P-19 + §D). Highlights:
- **Real tables to onboard**: `INVM`, `INCT`, `CUSVAA`. v1 ships **generic placeholder** test data.
- **Service images** are `__REPLACE_ME__` in `docker/.env` (P-02); the optional compose won't start
  the cbs-* services until you set real coordinates.
- Parked: full E2E from Oracle/GoldenGate (entry is `Kafka_topic_1` only), authentication (none),
  polling/await (fixed wait instead), native Aerospike Java client (uses `aql`), parallel execution
  (sequential), per-scenario cleanup (end-of-run only), JSON-schema validation, GoCD pipeline,
  observability exporters.
- **Removed** (TECH_DEBT §D): ServiceLoader SPIs (edit classes directly now), `buildSrc`, version
  catalog, and two unused deps (`micrometer`, `networknt`) — re-add the deps only if you take up the
  related parked items.

---

## 11. Environment notes for the new machine
- The build self-provisions JDK 21 on first run **if it has network**; otherwise install JDK 21 and it
  will be used by the toolchain.
- The Gradle wrapper (8.10.2) downloads on first `./gradlew` run.
- `docker-run` AQL mode needs Docker running; on macOS the container reaches the host Aerospike via
  `host.docker.internal` (already the config default).
- The `.gradle/` and `build/` directories are generated and git-ignored; safe to delete.

---

## 12. Glossary
- **Giftwrapper** — the banking platform under test (a read-wrapper over the Core Banking System).
- **CBS** — Core Banking System; the `cbs-*` services are its components.
- **AQL** — Aerospike Query Language; queries are run verbatim via the `aql` CLI tool.
- **Golden file** — the expected JSON a phase's actual result is compared against.
- **Prerequisite** — an element of a JSON-array input; all must be published (ordered, all-or-nothing)
  before the downstream app emits the final, converged Aerospike record.
- **Snapshot** — the masked payload/result captured per phase for the report's failure bundle.

> For the *why* behind decisions, read `IMPLEMENTATION_APPROACH.md` §2 (decisions D1–D19). For run
> instructions aimed at humans, `README.md`. This `AGENTS.md` is the agent-oriented map of the code.
