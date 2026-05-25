# Heimdall — Implementation Approach (v7, replication-grade, as-built)

**Project:** Heimdall — End-to-End Data Propagation Test Framework for *Giftwrapper*
**Status:** **BUILT.** This document is a complete, self-contained specification of the *as-built*
implementation. Following it reproduces the current code **exactly**.
**Structure:** a **single Gradle module** with a **single Java package** `com.giftwrapper.heimdall`
(Java 21). **No multi-module layout, no ServiceLoader/SPIs** — those existed in earlier drafts and
were intentionally removed (see §16 / `TECH_DEBT.md` §D).

> **For the implementing agent (MiniMax + pi):** build this from scratch by following §3 (layout),
> §5 (verbatim build files), §6 (verbatim config + samples), §7 (the 14 classes, with contracts),
> §8 (execution flow), §9 (must-not-break details), §13 (step-by-step order). Then verify with §12.
> Companion doc `AGENTS.md` is the map for working with the finished code; this doc is how to build it.
> For the **exact, byte-faithful content of every file** (build from markdown alone, no repo needed),
> see **`FULL_SOURCE.md`** — it contains the complete repository as one document plus the wrapper-
> bootstrap step. The three docs `AGENTS.md` + `IMPLEMENTATION_APPROACH.md` + `FULL_SOURCE.md` are
> together sufficient to reconstruct this repo on a fresh machine.

---

## 1. Purpose & Scope

```
Oracle → GoldenGate → [ Kafka_topic_1 → cbs-denormaliser → Kafka_topic_2 →
cbs-kafka-aerospike-consumer → Aerospike → cbs_web_services (API) ]   ← Heimdall scope
```

Per scenario Heimdall: **publish** JSON to `Kafka_topic_1` → **wait** a fixed interval → run an
**AQL** query and compare the Aerospike record to a golden file → run a **curl** and compare the API
response to a golden file. Scenarios come from a **CSV manifest**. Heimdall is a **pure client
against configurable endpoints**. Oracle/GoldenGate are trusted/out of scope (entry is `Kafka_topic_1`).

---

## 2. Confirmed Decisions (unchanged from the original design)

| # | Decision |
|---|---|
| D1 | Inject at `Kafka_topic_1` (per-scenario `TopicName`) |
| D2 | Heimdall is a **pure client against fully-configurable endpoints**; docker-compose env is optional |
| D3 | JSON serde |
| D4 | **Makefile** is the developer UX (no CLI) |
| D5 | **Fixed configurable wait**, then run AQL **once** and compare (no polling) |
| D6 | **No per-entity descriptors / no PK extraction**; `TableName` is a report label |
| D7 | Aerospike: run the **AQL file verbatim** via `aql -o json` (no native Java client) |
| D8 | API: file holds **one curl command**; compare **body vs golden + assert HTTP 2xx** |
| D9 | **Validation default = lenient, configurable** to strict |
| D10 | **Sequential** execution |
| D11 | **Cleanup once, end-of-run** (truncate Aerospike ns/set) |
| D12 | Scenario execution = **plain orchestrator loop** |
| D13 | Input JSON array ⇒ **one Kafka message per element, in order** |
| D16 | **Prerequisite semantics:** all array elements published **all-or-nothing, ordered, same-partition**; final result typically **one** converged record |
| D17 | A **README** with full run instructions is delivered |
| D14 | Reporting: CSV + single-file filterable HTML + JUnit-XML |
| D15 | Auth plain/none for now |
| D18 | **No tests shipped** (it *is* an E2E suite). Dev-time tests removed before delivery. Verify = clean build + sample self-check |
| D19 | A **`TECH_DEBT.md`** tracker records parked decisions + "add tests later" |

> **As-built deltas from the original D14/D15/D12 wording:** reporters/validators/executors are
> **wired directly** (no `Reporter`/`Validator`/`QueryExecutor`/`AuthProvider` SPI); the 3 AQL modes
> are a `switch`; auth is omitted entirely (type `none`, curl runs verbatim). The HTML report uses
> **vanilla JS**, not DataTables (TECH_DEBT P-19). Behaviour is identical to the decisions above.

---

## 3. Project structure to create

Single module; all source in one package. Exact tree (generated dirs omitted):

```
heimdall/
├─ settings.gradle.kts            # rootProject name + foojay toolchain resolver (NO includes)
├─ build.gradle.kts               # single module: java plugin, JDK 21 toolchain, deps, runScenarios task
├─ gradle.properties
├─ gradlew · gradlew.bat · gradle/wrapper/{gradle-wrapper.jar,gradle-wrapper.properties}   # Gradle 8.10.2
├─ Makefile
├─ .gitignore
├─ AGENTS.md · README.md · IMPLEMENTATION_APPROACH.md · TECH_DEBT.md   # docs
├─ src/main/java/com/giftwrapper/heimdall/    # 14 .java files (one package) — see §7
│   ├─ HeimdallMain.java  Config.java  Model.java  ScenarioCsvLoader.java
│   ├─ KafkaPublisher.java  Aql.java  Validator.java  CurlApiExecutor.java
│   ├─ Reporters.java  ScenarioRunner.java  PiiMasker.java  Json.java
│   └─ ProcessRunner.java  HeimdallException.java
├─ src/main/resources/logback.xml
├─ test-data/                     # config.json, scenarios.csv, input/ aql/ api/ expected/{aerospike,api}/
└─ docker/                        # OPTIONAL: docker-compose.yml + .env (cbs images = __REPLACE_ME__)
```

---

## 4. Tech stack & exact dependencies

| Concern | Coordinate (exact version) | Scope |
|---|---|---|
| Language / build | Java 21 toolchain · Gradle wrapper **8.10.2** | — |
| JSON | `com.fasterxml.jackson.core:jackson-databind:2.17.2` | implementation |
| JSON compare | `net.javacrumbs.json-unit:json-unit:3.4.1` | implementation |
| JSONPath (keyField, ignorePaths pruning) | `com.jayway.jsonpath:json-path:2.9.0` | implementation |
| Kafka producer | `org.apache.kafka:kafka-clients:3.7.1` | implementation |
| CSV | `org.apache.commons:commons-csv:1.11.0` | implementation |
| Logging API | `org.slf4j:slf4j-api:2.0.13` | implementation |
| Logging impl | `ch.qos.logback:logback-classic:1.5.6` | runtimeOnly |
| JSON logs | `net.logstash.logback:logstash-logback-encoder:7.4` | runtimeOnly |

**Deliberately NOT used** (do not add unless taking up the related parked item): `micrometer-core`
(no metrics wired — latencies use `System.nanoTime`, P-15), `networknt json-schema-validator`
(schema validation parked, P-09), JUnit (no shipped tests, D18). Aerospike is accessed only via the
external `aql` tool (no Java client); HTTP only via the external `curl` (no HTTP-client dependency).
The foojay toolchain-resolver Gradle plugin `org.gradle.toolchains.foojay-resolver-convention:0.8.0`
is applied in `settings.gradle.kts` to auto-provision JDK 21.

---

## 5. Build files (verbatim — reproduce exactly)

### `settings.gradle.kts`
```kotlin
plugins {
    id("org.gradle.toolchains.foojay-resolver-convention") version "0.8.0"
}

rootProject.name = "heimdall"
```

### `build.gradle.kts`
```kotlin
plugins {
    java
}

group = "com.giftwrapper.heimdall"
version = "1.0.0-SNAPSHOT"

repositories {
    mavenCentral()
}

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(21))
    }
}

dependencies {
    implementation("com.fasterxml.jackson.core:jackson-databind:2.17.2")
    implementation("net.javacrumbs.json-unit:json-unit:3.4.1")
    implementation("com.jayway.jsonpath:json-path:2.9.0")
    implementation("org.apache.kafka:kafka-clients:3.7.1")
    implementation("org.apache.commons:commons-csv:1.11.0")
    implementation("org.slf4j:slf4j-api:2.0.13")
    runtimeOnly("ch.qos.logback:logback-classic:1.5.6")
    runtimeOnly("net.logstash.logback:logstash-logback-encoder:7.4")
}

tasks.withType<JavaCompile>().configureEach {
    options.encoding = "UTF-8"
    options.compilerArgs.addAll(listOf("-parameters", "-Xlint:all,-processing,-serial,-auxiliaryclass"))
}

tasks.register<JavaExec>("runScenarios") {
    group = "heimdall"
    description = "Run all scenarios from the CSV manifest against configured endpoints."
    mainClass.set("com.giftwrapper.heimdall.HeimdallMain")
    classpath = sourceSets["main"].runtimeClasspath
    javaLauncher.set(javaToolchains.launcherFor {
        languageVersion.set(JavaLanguageVersion.of(21))
    })
    systemProperty("heimdall.config", project.findProperty("config") ?: "test-data/config.json")
    systemProperty("heimdall.csv", project.findProperty("csv") ?: "test-data/scenarios.csv")
    project.findProperty("logLevel")?.let { systemProperty("heimdall.log.level", it) }
}
```

### `gradle.properties`
```
org.gradle.caching=true
org.gradle.parallel=true
org.gradle.configuration-cache=false
```

### `gradle/wrapper/gradle-wrapper.properties` (key line)
```
distributionUrl=https\://services.gradle.org/distributions/gradle-8.10.2-bin.zip
```
Generate the wrapper with any Gradle ≥ 8.10: `gradle wrapper --gradle-version 8.10.2 --distribution-type bin`.

### `Makefile`
```make
CONFIG  ?= test-data/config.json
CSV     ?= test-data/scenarios.csv
REPORT  ?= build/heimdall-report/heimdall-report.html
COMPOSE  = docker compose -f docker/docker-compose.yml --env-file docker/.env

.PHONY: help build run up down report logs clean

help: ; @echo "Heimdall targets:"; \
	echo "  make build   - compile everything (./gradlew clean build)"; \
	echo "  make run     - run all scenarios against configured endpoints"; \
	echo "  make report  - open the HTML report"; \
	echo "  make up      - start the OPTIONAL local env (docker/docker-compose.yml)"; \
	echo "  make down    - stop the local env and wipe its volumes"; \
	echo "  make logs    - tail the local env logs"; \
	echo "  make clean   - remove build output and the report"

build: ; ./gradlew clean build

run: ; ./gradlew runScenarios -Pconfig=$(CONFIG) -Pcsv=$(CSV)

up: ; $(COMPOSE) up -d

down: ; $(COMPOSE) down -v

report: ; @sh -c 'open $(REPORT) 2>/dev/null || xdg-open $(REPORT) 2>/dev/null || echo "Open $(REPORT) in a browser."'

logs: ; $(COMPOSE) logs -f

clean: ; ./gradlew clean && rm -rf build/heimdall-report
```

### `src/main/resources/logback.xml`
```xml
<configuration>
    <appender name="JSON" class="ch.qos.logback.core.ConsoleAppender">
        <encoder class="net.logstash.logback.encoder.LogstashEncoder">
            <includeMdcKeyName>correlationId</includeMdcKeyName>
            <includeMdcKeyName>scenarioNumber</includeMdcKeyName>
            <includeMdcKeyName>phase</includeMdcKeyName>
            <customFields>{"app":"heimdall"}</customFields>
        </encoder>
    </appender>
    <root level="${heimdall.log.level:-INFO}">
        <appender-ref ref="JSON"/>
    </root>
</configuration>
```

---

## 6. Configuration, CSV manifest & sample test-data (verbatim)

### `test-data/config.json`
```jsonc
{
  "activeProfile": "local",
  "profiles": {
    "local": {
      "kafka": {
        "bootstrapServers": "${KAFKA_BOOTSTRAP:localhost:9092}",
        "producer": { "acks": "all", "ackTimeoutMs": 5000, "orderedSamePartition": true, "keyField": null }
      },
      "wait": { "defaultMs": 8000, "perTable": { "INVM": 10000 } },
      "aerospike": {
        "executor": "docker-run", "toolsImage": "aerospike/aerospike-tools:11.0.0",
        "toolsContainer": "aerospike-tools", "host": "${AS_HOST:host.docker.internal}",
        "port": 3000, "aqlBinary": "aql", "outputFormat": "json", "namespace": "cbs", "timeoutMs": 15000
      },
      "api": { "curlBinary": "curl", "timeoutMs": 10000, "auth": { "type": "none" }, "baseUrlEnv": "API_BASE" },
      "validation": {
        "mode": "lenient", "ignorePaths": ["$..lastUpdatedTs", "$..auditTs"],
        "array": { "ignoreOrder": true, "ignoreExtraItems": false }, "schema": false
      },
      "testData": {
        "baseDir": "./test-data", "inputJsonDir": "input", "aqlDir": "aql",
        "aerospikeExpectedDir": "expected/aerospike", "apiRequestDir": "api", "apiExpectedDir": "expected/api"
      },
      "cleanup": { "enabled": true, "when": "end-of-run", "strategy": "truncate-namespace", "sets": [] },
      "report": {
        "outputDir": "./build/heimdall-report", "csv": true, "html": true, "junitXml": true,
        "maskPii": true, "piiFields": ["pan", "aadhaar", "accountNo"]
      },
      "execution": { "mode": "sequential", "parallelism": 1, "failExitCodeOnScenarioFailure": true }
    }
  }
}
```
`${ENV:default}` substitution runs on the raw text before JSON parsing; the default is everything
after the first `:` (so `localhost:9092` survives). Resolution order: OS env → JVM system property →
inline default → empty (warn).

### CSV manifest — columns (in order)
```
ScenarioNumber,TableName,TopicName,input_json_file_name,aerospike_query_file_name,
aerospike_output_json_file_name,api_endpoint_files,api_output_json_file_name
```
Rules: values trimmed; an **empty cell skips that phase**; a phase needs **both** of its columns
(query+golden, or curl+golden) or neither; `ScenarioNumber`/`TopicName`/`input_json_file_name` are
required; duplicate scenario numbers and missing referenced files fail the load (all problems
reported at once). File names resolve under `testData.baseDir` + the per-kind subdir.

### `test-data/scenarios.csv` (the sample self-check, Appendix-M role)
```
ScenarioNumber,TableName,TopicName,input_json_file_name,aerospike_query_file_name,aerospike_output_json_file_name,api_endpoint_files,api_output_json_file_name
HD_001,INVM,CBS-INBOUND-CHANNEL,invm_single.json,aql_invm_single.txt,expected_invm_single.json,account_details_api.txt,expected_api_invm_single.json
HD_002,INVM,CBS-INBOUND-CHANNEL,invm_prereq_array.json,aql_invm_prereq.txt,expected_invm_prereq.json,account_details_prereq_api.txt,expected_api_invm_prereq.json
HD_003,INVM,CBS-INBOUND-CHANNEL,invm_single.json,aql_invm_single.txt,expected_invm_NEGATIVE.json,,
```
- `HD_001`: single object → PASS-capable (all 3 phases).
- `HD_002`: 2-element array (prerequisites) → PASS-capable.
- `HD_003`: reuses HD_001's input/query but a **deliberately wrong** golden and **no API columns** →
  must **FAIL at the AEROSPIKE phase**.

### Sample data files (generic placeholders — real tables INVM/INCT/CUSVAA onboarded later, P-17)
`input/invm_single.json` (object; note PII fields `pan`/`accountNo`):
```json
{ "PK": "INVM-0001", "accountId": "INVM-0001", "accountNo": "100200300400", "pan": "ABCDE1234F",
  "customerName": "Placeholder Customer", "productType": "INVM", "balance": 15000.50,
  "currency": "INR", "status": "ACTIVE" }
```
`input/invm_prereq_array.json` (array of 2 prerequisites that converge to eventSeq 2 state):
```json
[ { "PK":"INVM-0002","accountId":"INVM-0002","accountNo":"100200300500","pan":"PQRSX5678Y",
    "productType":"INVM","balance":5000.00,"currency":"INR","status":"PENDING","eventSeq":1 },
  { "PK":"INVM-0002","accountId":"INVM-0002","accountNo":"100200300500","pan":"PQRSX5678Y",
    "productType":"INVM","balance":18250.75,"currency":"INR","status":"ACTIVE","eventSeq":2 } ]
```
`aql/aql_invm_single.txt`:
```
SELECT accountId, accountNo, pan, productType, balance, currency, status FROM cbs.INVM WHERE PK = 'INVM-0001'
```
`aql/aql_invm_prereq.txt`: same projection, `WHERE PK = 'INVM-0002'`.
`expected/aerospike/expected_invm_single.json`: `{accountId,accountNo,pan,productType,balance,currency,status}` matching the single input.
`expected/aerospike/expected_invm_prereq.json`: same fields with INVM-0002 / balance 18250.75 / status ACTIVE.
`expected/aerospike/expected_invm_NEGATIVE.json`: like single but `balance: 99999.99`, `status: "CLOSED"` (intentionally wrong).
`api/account_details_api.txt`:
```
curl -X GET "${API_BASE:-http://localhost:8080}/accounts/INVM-0001" -H "Accept: application/json"
```
`api/account_details_prereq_api.txt`: same for `/accounts/INVM-0002`.
`expected/api/expected_api_invm_single.json`: `{accountId,accountNo,balance,currency,status}` for INVM-0001.
`expected/api/expected_api_invm_prereq.json`: same shape for INVM-0002.

---

## 7. The 14 classes — package `com.giftwrapper.heimdall`

All types are **package-private** except `HeimdallMain` (public, has `main`). Group small types per
file as noted (this is why `-Xlint:auxiliaryclass` is disabled).

### `HeimdallException.java` — `public class HeimdallException extends RuntimeException`
Two ctors `(String)` and `(String, Throwable)`. The single exception type. Thrown for
config/manifest/load failures (bubbles to `HeimdallMain` → exit 2). Phase code also throws it but
catches it locally → records an ERROR `PhaseResult` (exit 1).

### `Json.java` — `final class Json`
Shared Jackson `ObjectMapper`: `FAIL_ON_UNKNOWN_PROPERTIES=false` **and** visibility checker with
`withCreatorVisibility(ANY).withFieldVisibility(ANY)` (so it can bind the package-private config
records). A second `PRETTY` mapper with `INDENT_OUTPUT`. Methods: `static ObjectMapper mapper()`,
`static JsonNode read(Path)`, `static JsonNode read(String)`, `static String toPretty(JsonNode)`,
`static String toCompact(JsonNode)`.

### `Config.java` — `final class Config` + 14 records
- `static Root load(Path)`: not-found/IO → `HeimdallException`; reads file; `substituteEnv`;
  `Json.mapper().readValue(text, Root.class)`; `validate`. Logs the active profile.
- `static String substituteEnv(String)`: regex `\$\{([A-Za-z0-9_]+)(?::([^}]*))?}`; env → sysprop →
  default → "" (warn when fully unresolved).
- `private static void validate(Root, Path)`: activeProfile present & in profiles;
  `kafka.bootstrapServers` and `testData.baseDir` present in the active profile.
- Records (package-private, top-level in this file):
  `Root(String activeProfile, Map<String,Profile> profiles)` → `Profile active()`;
  `Profile(Kafka kafka, @JsonProperty("wait") Wait waitConfig, Aerospike aerospike, Api api, Validation validation, TestData testData, Cleanup cleanup, Report report, Execution execution)`;
  `Kafka(String bootstrapServers, ProducerCfg producer)`;
  `ProducerCfg(String acks, int ackTimeoutMs, boolean orderedSamePartition, String keyField)`;
  `Wait(long defaultMs, Map<String,Long> perTable)` → `long waitMsFor(String table)`;
  `Aerospike(String executor, String toolsImage, String toolsContainer, String host, int port, String aqlBinary, String outputFormat, String namespace, long timeoutMs)`;
  `Api(String curlBinary, long timeoutMs, Auth auth, String baseUrlEnv)`; `Auth(String type)`;
  `Validation(String mode, List<String> ignorePaths, Arr array, boolean schema)` → `boolean isStrict()`;
  `Arr(boolean ignoreOrder, boolean ignoreExtraItems)`;
  `TestData(String baseDir, String inputJsonDir, String aqlDir, String aerospikeExpectedDir, String apiRequestDir, String apiExpectedDir)`;
  `Cleanup(boolean enabled, String when, String strategy, List<String> sets)`;
  `Report(String outputDir, boolean csv, boolean html, boolean junitXml, boolean maskPii, List<String> piiFields)`;
  `Execution(String mode, int parallelism, Boolean failExitCodeOnScenarioFailure)` → `boolean failExitCode()` (defaults true when null).

### `Model.java` — domain types (all package-private, one file)
`enum Phase { KAFKA, AEROSPIKE, API }`; `enum Status { PASS, FAIL, SKIPPED, ERROR }`;
`record Scenario(String scenarioNumber, String tableName, String topicName, Path inputJson, Path aqlQuery, Path aerospikeExpected, Path apiEndpoint, Path apiExpected)` → `hasAerospikePhase()` (aqlQuery & aerospikeExpected non-null), `hasApiPhase()`;
`record PhaseResult(Phase phase, Status status, long latencyMs, String detail, String diff, String snapshot)` → `static skipped(Phase, String)`;
`record ScenarioResult(String scenarioNumber, String tableName, String topicName, String correlationId, int msgCount, Status overall, Phase failurePhase, String failureReason, long totalLatencyMs, List<PhaseResult> phases)` → `isFailureOrError()`, `statusFor(Phase)` (SKIPPED if absent);
`record ValidationOutcome(boolean match, String diff)` → `static matched()`, `static mismatch(String)`;
`record ApiResponse(int status, String body)` → `is2xx()` (200–299).

### `ScenarioCsvLoader.java`
- Ctor `(TestData cfg)` → resolves `baseDir` absolute.
- `List<Scenario> load(Path csvPath)`: Commons-CSV `CSVFormat.DEFAULT.builder().setHeader().setSkipHeaderRecord(true).setTrim(true).setIgnoreEmptyLines(true).setIgnoreSurroundingSpaces(true)`; validate required header columns; per row validate (required fields, duplicate scenarioNumber, both-or-neither phase columns) and resolve+exist-check files; accumulate all errors → one `HeimdallException`.
- `record PublishPayload(List<JsonNode> elements, boolean array)` → `size()`.
- `static PublishPayload readInput(Path)`: array root ⇒ N elements in order (`array=true`, empty array → error); object root ⇒ 1 element (`array=false`); else error.

### `KafkaPublisher.java` — `final class KafkaPublisher implements AutoCloseable`
- Ctor `(Kafka cfg, PiiMasker masker)` builds a `KafkaProducer<String,String>` from `buildProducerProps`.
- `buildProducerProps(Kafka)`: `BOOTSTRAP_SERVERS`, `StringSerializer` key+value, `acks` (cfg or "all"),
  `ENABLE_IDEMPOTENCE=true`, `MAX_IN_FLIGHT_REQUESTS_PER_CONNECTION=1`, `RETRIES=Integer.MAX_VALUE`,
  `LINGER_MS=0`, `REQUEST_TIMEOUT_MS=ackTimeout`, `DELIVERY_TIMEOUT_MS=ackTimeout`,
  **`MAX_BLOCK_MS=ackTimeout`** (fail-fast), `CLIENT_ID="heimdall-producer"`. (`ackTimeout` = `producer.ackTimeoutMs` or 5000.)
- `PhaseResult publish(String topic, List<JsonNode> elements, String scenarioNumber, String correlationId)`:
  resolve one key per element (keyField JSONPath if set, else constant `scenarioNumber`); warn if keys
  differ; send each record **synchronously in order** with `.get(ackTimeoutMs + 2000, MILLIS)`; add headers
  `correlationId`, `scenarioNumber`, `elementIndex`, `elementCount`; any failure ⇒ `Status.FAIL` (all-or-nothing,
  stop). Snapshot = masked pretty JSON of the payload (object if 1, array if N). `close()` closes the producer.

### `Aql.java` — `final class Aql` (the Aerospike phase)
- `static List<JsonNode> query(String aql, Aerospike cfg)`: `buildCommand` → `ProcessRunner.run(cmd, timeoutMs)`;
  start failure / timeout / non-zero exit → `HeimdallException`; else `parse(stdout)`.
- `static void cleanup(Aerospike, Cleanup)`: disabled → log+return; else `truncate <ns>` (or `truncate <ns>.<set>`
  per configured set) via `ProcessRunner`; **best-effort** (failures logged, never thrown).
- `static List<String> buildCommand(String mode, Aerospike cfg, String statement)`:
  `docker-run` → `docker run --rm <toolsImage> aql -h <host> -p <port> -o <fmt> -c <statement>`;
  `docker-exec` → `docker exec <toolsContainer> aql …`; `host` → `aql …`. The statement is a **single
  argument** (no shell) so it needs no quoting. Missing required config → `HeimdallException`.
- `static List<JsonNode> parse(String stdout)`: blank → empty list; parse JSON; **walk arrays
  recursively**, keep object nodes that are NOT metadata, drop metadata objects (an object is metadata
  iff empty or every key ∈ {status, row count, number of records, number of rows, rows in set, node,
  error}, case-insensitive); invalid JSON → `HeimdallException`.

### `Validator.java` — `final class Validator`
- `static ValidationOutcome compare(JsonNode expected, JsonNode actual, Validation cfg)`:
  **prune `ignorePaths` from BOTH sides** via `prune`, then `JsonAssert.assertJsonEquals(toCompact(prunedExpected), toCompact(prunedActual), buildConfiguration(cfg))`; no throw ⇒ matched; `AssertionError` message ⇒ mismatch diff.
- `static JsonNode shapeAerospikeActual(JsonNode golden, List<JsonNode> records)`: array golden ⇒ wrap
  records as array; object golden + exactly 1 record ⇒ that object; otherwise wrap as array (so a shape
  mismatch surfaces).
- `buildConfiguration(Validation)`: strict ⇒ `Configuration.empty()`; lenient ⇒
  `withOptions(IGNORING_EXTRA_FIELDS, IGNORING_ARRAY_ORDER?, IGNORING_EXTRA_ARRAY_ITEMS?)` where array
  order is included unless `array.ignoreOrder=false` and extra-items only if `array.ignoreExtraItems=true`.
- `prune(JsonNode, List<String>)`: Jayway JsonPath `DocumentContext.delete(path)` on a `deepCopy`, using a
  `JacksonJsonNodeJsonProvider` + `JacksonMappingProvider` (sharing `Json.mapper()`) and `Option.SUPPRESS_EXCEPTIONS`;
  invalid path → warn+skip. **This is why recursive `$..` ignorePaths work** (json-unit's own ignore-paths do not — see §9).

### `CurlApiExecutor.java` — `final class CurlApiExecutor`
- `static ApiResponse execute(String curlCommand, Api cfg)`: create temp body file;
  `prepared = applyBinaryAndBaseUrl(curl, cfg)`; `wrapped = prepared + " --silent --output '<tmp>' --write-out '%{http_code}'"`;
  run `["bash","-c",wrapped]` via `ProcessRunner` with timeout (cfg or 10000); timeout/start-fail → `HeimdallException`;
  parse status as the trailing `\d{3}` of stdout, `<100` → `HeimdallException`; body = read temp file; **delete temp in finally**.
- `applyBinaryAndBaseUrl`: if `curlBinary` set and ≠ "curl", replace a leading `curl` token; then `substituteEnvVar` the configured `baseUrlEnv` (default `API_BASE`).
- `substituteEnvVar(text, name)`: regex `\$\{<name>(?::-([^}]*))?}`; resolves env → sysprop → bash-style `:-` default → "". Other `${...}` are left for the shell.

### `Reporters.java` — `final class Reporters`
- `static void write(List<ScenarioResult>, Report cfg)`: dispatch to `writeCsv`/`writeHtml`/`writeJunitXml` per the boolean toggles; `outputDir(cfg)` ensures the dir.
- **CSV** columns (exact, in order): `scenarioNumber,tableName,topicName,correlationId,msgCount,kafkaStatus,aerospikeStatus,apiStatus,overall,totalLatencyMs,failurePhase,failureReason` (per-phase status via `statusFor`). File `heimdall-report.csv`. (Commons-CSV.)
- **JUnit XML** `heimdall-report.xml`: one `<testsuite name="heimdall" tests failures errors="0" skipped time>`; one `<testcase name=ScenarioNumber classname=TableName time>`; FAIL/ERROR ⇒ `<failure message=…>`, SKIPPED ⇒ `<skipped/>`, else self-closing; 5 XML entities escaped.
- **HTML** `heimdall-report.html`: single self-contained file (no CDN). Summary counts; a `#tableFilter` text input; a `<table id="results">` with sortable `<th data-type=str|num>`; per scenario a `tr.row[data-table]` + a hidden `tr.detail` (correlationId, failure, and per phase the masked `detail`/`snapshot`/`diff` in `<pre>`). Inline CSS + vanilla JS: click row → toggle detail; filter by table; click header → sort (keeps row+detail paired). Status badges colour-coded.

### `ScenarioRunner.java` — `final class ScenarioRunner` (the §8 flow)
- Ctor `(Profile, PiiMasker, KafkaPublisher, String runId)`.
- `List<ScenarioResult> run(List<Scenario>)` loops `runOne`.
- `runOne`: set MDC (`correlationId`=scenarioNumber+"-"+runId, `scenarioNumber`, `phase`); `readInput`;
  `publisher.publish` (KAFKA); if not PASS ⇒ AEROSPIKE+API `skipped`, return; `sleep(wait.waitMsFor(table))`;
  `runAerospike` if `hasAerospikePhase`; `runApi` if `hasApiPhase`; `assemble`; clear MDC in `finally`.
- `runAerospike`: read aql file; `Aql.query`; read golden; `Validator.shapeAerospikeActual`; `Validator.compare`;
  snapshot = `"-- AQL --\n"+maskValuesIn(aql,…)+"\n\n-- Aerospike result (N record(s)) --\n"+maskBody(prettyActual,…)`;
  PASS/FAIL; exceptions (IOException, RuntimeException incl. HeimdallException) → ERROR.
- `runApi`: read curl file; `CurlApiExecutor.execute`; read golden; parse body (non-JSON ⇒ mismatch);
  `Validator.compare`; PASS iff 2xx AND match; snapshot = `"-- Request --\n…\n\n-- Response: HTTP <s> --\n"+maskBody(body,…)`;
  diff prefixes a non-2xx note; exceptions → ERROR.
- `assemble`: overall = ERROR if any phase ERROR else FAIL if any FAIL else PASS; `failurePhase`/`failureReason`
  from the first offending phase (execution order).
- `maskBody(text, sources…)` = `masker.maskValuesIn(masker.maskJsonString(text), sources)`.
- `sleep(ms)` = bounded `Thread.sleep` (the fixed wait); `elapsedMs` = nanoTime diff / 1e6.

### `PiiMasker.java` — `final class PiiMasker`
- Ctor `(boolean enabled, List<String> piiFields)` (lower-cased set); `static from(Report)`.
- `JsonNode mask(JsonNode)`: deep-copy then recursively replace values of fields whose name (CI) is PII with `MASK` (`"***MASKED***"`); input never mutated.
- `String maskJsonString(String)`: field-mask JSON text; non-JSON returned unchanged.
- `String maskValuesIn(String text, JsonNode... sources)`: harvest PII values from sources, string-replace them in `text`. Used for diffs/free text and request lines.

### `ProcessRunner.java` — `final class ProcessRunner`
- `record Result(int exitCode, String stdout, String stderr, boolean timedOut)` → `ok()`.
- `static Result run(List<String> command, long timeoutMs)`: `ProcessBuilder.start()` (start failure → `UncheckedIOException`);
  drain stdout/stderr on separate `CompletableFuture` threads (avoids pipe deadlock); `waitFor(timeout)`; on timeout
  `destroyForcibly` → `timedOut=true`. Inherits the parent environment (so `bash -c` curl sees env vars).

### `HeimdallMain.java` — `public final class HeimdallMain`
- `public static void main(String[])` → `System.exit(run())`.
- `static int run()`: read `-Dheimdall.config` (default `test-data/config.json`) and `-Dheimdall.csv`
  (default `test-data/scenarios.csv`); `Config.load(...).active()`; `new ScenarioCsvLoader(profile.testData()).load(csv)`;
  `PiiMasker.from(profile.report())`; `runId` = 8-char UUID; try-with-resources `KafkaPublisher` →
  `new ScenarioRunner(...).run(scenarios)`; `Aql.cleanup(...)` (best-effort); `Reporters.write(...)`; log summary;
  return `1` if any FAIL/ERROR and `execution.failExitCode()`, else `0`. Catch `HeimdallException`/`RuntimeException` → `2`.
- Constants `EXIT_OK=0`, `EXIT_SCENARIO_FAILURE=1`, `EXIT_FRAMEWORK_ERROR=2`.

---

## 8. Per-scenario execution flow (sequential)
```
for each CSV row, in order:
  1. correlationId = ScenarioNumber + "-" + run-uuid     (MDC: correlationId, scenarioNumber, phase)
  2. read input JSON (object ⇒ 1 msg; array ⇒ N msgs, in order)
  3. PUBLISH to TopicName: all elements, SAME KEY (⇒ same partition), SYNCHRONOUS in order, ALL-OR-NOTHING.
     Any failure ⇒ KAFKA = FAIL ⇒ AEROSPIKE + API = SKIPPED ⇒ go to assemble.
  4. WAIT fixed interval (wait.waitMsFor(TableName))      (no polling)
  5. AEROSPIKE (if both columns present): run AQL verbatim → parse records → shape vs golden → compare
  6. API (if both columns present; runs even if AEROSPIKE failed): run curl → body + status → compare (2xx AND body)
  7. assemble overall (ERROR > FAIL > PASS), failurePhase/Reason
after ALL rows: CLEANUP once (truncate ns/set, best-effort), then write all enabled reports.
```

---

## 9. Critical implementation details — MUST NOT break

1. **Recursive `ignorePaths` (`$..field`)** — json-unit 3.4.1's `whenIgnoringPaths` does **not** honour
   recursive descent, yet `config.json` ships `$..lastUpdatedTs`. `Validator` therefore prunes ignorePaths
   from both sides via **Jayway JsonPath** before comparing. Do **not** switch to json-unit's ignore-paths.
2. **`runScenarios` must launch on the Java 21 toolchain** (`javaLauncher.set(javaToolchains.launcherFor{21})`),
   else it runs on the Gradle JVM and throws `UnsupportedClassVersionError`.
3. **`max.block.ms` bounded** to `ackTimeoutMs` so an unreachable broker fails fast (~5s), not ~60s.
4. **Jackson + package-private records:** `Json` sets creator+field visibility to `ANY`. Keep it, or make
   the config records public.
5. **Single-package name choices:** the AQL class is `Aql` (config record is `Aerospike`); the Kafka producer
   config record is `ProducerCfg` (Kafka's interface is `Producer`); in `Validator`, Jayway's `Option`/`Configuration`
   are fully-qualified (json-unit imports the simple names). A record component cannot be named `wait` →
   it is `waitConfig` with `@JsonProperty("wait")`.
6. **AQL = single process argument** (no shell → no quoting/injection). **curl = `bash -c`** (shell env
   expansion); appended `-s -o <tmp> -w '%{http_code}'` win over duplicates; temp body file always deleted.
7. **PII discipline:** compare on **unmasked** data (so PII fields are validated); mask only when building
   report/log output — `maskJsonString` for JSON snapshots, `maskValuesIn` for diffs/free text.
8. **Lint:** keep `-Xlint:all,-processing,-serial,-auxiliaryclass`; the build must stay **warning-free**.
9. **Exit codes** are `HeimdallMain`'s; through Gradle/`make` any non-zero collapses to "BUILD FAILED"
   (0 = pass, non-zero = failure) — the precise 1-vs-2 is only seen running `HeimdallMain` directly.

---

## 10. Reporting (already covered in §7 Reporters) & 11. PII masking (§7 PiiMasker / §9.7)
See those sections. Reports go to `report.outputDir` (`build/heimdall-report/`) and are **always written
before exit**. PII fields (`report.piiFields`) never appear unmasked in CSV/HTML/XML/logs.

---

## 12. Verification (replaces shipped tests — D18)

1. `make build` ⇒ **BUILD SUCCESSFUL, zero warnings** (first run downloads Gradle 8.10.2 + JDK 21 + deps).
2. **No-services smoke test** — `make run` with nothing running. Expected: every scenario FAILs at KAFKA
   (broker unreachable, ~5s), AEROSPIKE+API SKIPPED, **all 3 reports written**, **no raw PII** in any report
   (grep that `ABCDE1234F`/`100200300400` are absent and `***MASKED***` is present), exit non-zero. This
   exercises the whole plumbing except live-service PASS.
3. **Full self-check** (needs real services) — point `config.json` at running Kafka/Aerospike/API, align the
   goldens, `make run`: `HD_001` + `HD_002` PASS, `HD_003` FAILs at AEROSPIKE.

If you add dev-time tests during development: `src/test/java/...` +
`testImplementation("org.junit.jupiter:junit-jupiter:5.10.3")`, and **remove them before delivery** (D18).

---

## 13. Step-by-step replication order (compiles incrementally)

1. Create the tree (§3); generate the Gradle 8.10.2 wrapper; write `settings.gradle.kts`,
   `build.gradle.kts`, `gradle.properties`, `Makefile`, `.gitignore`, `src/main/resources/logback.xml` (§5).
2. `HeimdallException`, `Json` (foundation).
3. `Model` (domain), `Config` (records + load + env subst). Build: `./gradlew compileJava`.
4. `PiiMasker`, `ProcessRunner`.
5. `ScenarioCsvLoader` (CSV + readInput).
6. `KafkaPublisher`.
7. `Aql` (query/parse/cleanup).
8. `Validator` (compare/prune/shape).
9. `CurlApiExecutor`.
10. `Reporters`.
11. `ScenarioRunner`, then `HeimdallMain`.
12. Write `test-data/` (§6). `make build` (warning-free), then the §12 smoke test.
13. Write `docker/docker-compose.yml` + `.env` (optional env; cbs images `__REPLACE_ME__`), `README.md`,
    `AGENTS.md`, `TECH_DEBT.md`. Remove any dev-time tests (D18).

---

## 14. Local Developer Setup
Path A (your services): edit `test-data/config.json` endpoints → `make run` → `make report`.
Path B (optional local env): set real images in `docker/.env` → `make up` → `make run` → `make down`.
Prerequisites: **JDK 21** (toolchain auto-provisions if the host is older, given network), **Docker** (for
the `docker-run` aql mode + optional env), **bash + curl** (API phase).

---

## 15. Status: IMPLEMENTATION COMPLETE (single module)
Built, simplified to one module/one package, verified (warning-free build + no-services smoke test).
Dev-time tests removed (D18). README + AGENTS.md + sample self-check shipped. To make scenarios PASS,
point `config.json` at running services and align the goldens.

---

## 16. Parked / not built — see `TECH_DEBT.md`
Full list P-01…P-19 + §D (the structure simplification). Highlights: real tables INVM/INCT/CUSVAA (P-17);
`__REPLACE_ME__` service images (P-02); Oracle/GoldenGate E2E (P-01); auth (P-03); polling (P-04); native
Aerospike client (P-05); parallelism (P-07); per-scenario cleanup (P-08); JSON-schema validation (P-09);
GoCD (P-14); observability exporters (P-15). Removed as pure indirection (TECH_DEBT §D): ServiceLoader SPIs,
`buildSrc`, version catalog, the no-op AuthProvider, and the unused `micrometer`/`networknt` deps.
