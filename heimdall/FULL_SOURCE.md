# FULL_SOURCE.md — Heimdall complete source (byte-faithful)

> **Purpose.** This single file contains the **exact content of every file** in the Heimdall
> repository so it can be reconstructed on another machine **from markdown only**. Use it
> together with the two companion docs you were also given:
>
> - **`AGENTS.md`** — the map of the code / how to work with it (save it to the repo root as-is).
> - **`IMPLEMENTATION_APPROACH.md`** — the spec, decisions, and per-class contracts (save to repo root as-is).
> - **`FULL_SOURCE.md`** — *this file*: every other file's full content.

> **No gaps.** The only file not embedded here is the binary `gradle/wrapper/gradle-wrapper.jar`
> (a JAR cannot be represented in text) — it is regenerated in Step 4 below. `gradlew` and
> `gradlew.bat` are also produced by that same step. Files are byte-faithful; create them exactly.

---

## Reconstruction procedure

**Step 1 — create the directory tree** from the repo root:

````bash
mkdir -p src/main/java/com/giftwrapper/heimdall \
         src/main/resources \
         gradle/wrapper \
         test-data/input \
         test-data/aql \
         test-data/api \
         test-data/expected/aerospike \
         test-data/expected/api \
         docker
````

**Step 2 — save the two companion docs** (`AGENTS.md`, `IMPLEMENTATION_APPROACH.md`) verbatim into the repo root.

**Step 3 — create every file in §'Files' below** with its exact content (the order matches the
incremental build order). Java sources go under `src/main/java/com/giftwrapper/heimdall/`.

**Step 4 — generate the Gradle wrapper** (needs internet; you need any Gradle ≥ 8.10 only to
create the wrapper, which then pins 8.10.2):

````bash
# install a bootstrap Gradle (pick one):
curl -s "https://get.sdkman.io" | bash && source "$HOME/.sdkman/bin/sdkman-init.sh" && sdk install gradle 8.10.2
# ...or download the distribution directly:
#   curl -L -o /tmp/gradle.zip https://services.gradle.org/distributions/gradle-8.10.2-bin.zip
#   unzip -q /tmp/gradle.zip -d $HOME/gradle-dist && export PATH="$HOME/gradle-dist/gradle-8.10.2/bin:$PATH"

# from the repo root, generate gradlew + gradlew.bat + gradle/wrapper/gradle-wrapper.jar:
gradle wrapper --gradle-version 8.10.2 --distribution-type bin
````

**Step 5 — build and self-verify** (first build auto-provisions JDK 21 via the foojay resolver):

````bash
make build      # expect: BUILD SUCCESSFUL, zero warnings
make run        # no services up: every scenario FAILs at KAFKA (~5s), all 3 reports written,
                # PII masked (no raw pan/accountNo in reports), process exits non-zero
make report     # open build/heimdall-report/heimdall-report.html
````

> To make scenarios PASS, point `test-data/config.json` at running Kafka/Aerospike/API and align
> the golden files (see `IMPLEMENTATION_APPROACH.md` §6/§12 and `AGENTS.md` §9).

---

## Files

### `settings.gradle.kts`
````kotlin
// Heimdall — single-module build. The foojay resolver lets the Java 21 toolchain auto-provision a
// JDK 21 when the host has an older one (see IMPLEMENTATION_APPROACH.md §12).
plugins {
    id("org.gradle.toolchains.foojay-resolver-convention") version "0.8.0"
}

rootProject.name = "heimdall"
````

### `build.gradle.kts`
````kotlin
// Heimdall — single Gradle module, single package com.giftwrapper.heimdall.
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
        // Auto-provisioned via the foojay resolver if the host JDK is older.
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
    // -auxiliaryclass is excluded: we deliberately group small types (Model, Config records) per
    // file in the single package; -serial: our RuntimeException is never serialized cross-JVM.
    options.compilerArgs.addAll(listOf("-parameters", "-Xlint:all,-processing,-serial,-auxiliaryclass"))
}

// Entry task (IMPLEMENTATION_APPROACH.md Appendix D). Working dir is the project root, so the
// default relative paths (test-data/...) resolve. Launches with the Java 21 toolchain.
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
````

### `gradle.properties`
````properties
org.gradle.caching=true
org.gradle.parallel=true
org.gradle.configuration-cache=false
````

### `gradle/wrapper/gradle-wrapper.properties`
````properties
distributionBase=GRADLE_USER_HOME
distributionPath=wrapper/dists
distributionUrl=https\://services.gradle.org/distributions/gradle-8.10.2-bin.zip
networkTimeout=10000
retries=0
retryBackOffMs=500
validateDistributionUrl=true
zipStoreBase=GRADLE_USER_HOME
zipStorePath=wrapper/dists
````

### `.gitignore`
````gitignore
# Gradle
.gradle/
build/
**/build/
!gradle/wrapper/gradle-wrapper.jar

# Heimdall run output
build/heimdall-report/

# IDE
.idea/
*.iml
.vscode/
*.ipr
*.iws
.settings/
.classpath
.project
bin/

# OS
.DS_Store

# Claude-specific files — NOT pushed. This repo is handed off to MiniMax + pi; AGENTS.md is the
# model-agnostic entry doc. These stay on disk for local Claude Code use but are never committed.
.claude/
CLAUDE.md
CLAUDE.local.md
.claude.json

# Original requirements prompt — local reference only, not part of the deliverable
first_prompt.rtf

# Logs / scratch
*.log
````

### `Makefile`
````make
# Heimdall — developer UX (IMPLEMENTATION_APPROACH.md Appendix E).
# Recipes use the `target: ; command` one-line form so no literal tabs are required.
# Override CONFIG / CSV on the command line, e.g.  make run CONFIG=my.json
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
````

### `src/main/resources/logback.xml`
````xml
<?xml version="1.0" encoding="UTF-8"?>
<!-- Heimdall logging (IMPLEMENTATION_APPROACH.md Appendix L).
     JSON console logs via the logstash encoder; correlationId/scenarioNumber/phase come from MDC
     (ScenarioRunner sets/clears them). Level overridable with -Dheimdall.log.level=DEBUG. -->
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
````

### `src/main/java/com/giftwrapper/heimdall/HeimdallException.java`
````java
package com.giftwrapper.heimdall;

/**
 * Single exception type for Heimdall. Used for config/manifest/load failures (which bubble to
 * {@code HeimdallMain} → exit code 2) and for phase execution failures (which are caught inside the
 * phase methods of {@code ScenarioRunner} and recorded as ERROR results → exit code 1).
 */
public class HeimdallException extends RuntimeException {

    public HeimdallException(String message) {
        super(message);
    }

    public HeimdallException(String message, Throwable cause) {
        super(message, cause);
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/Json.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.annotation.JsonAutoDetect.Visibility;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;

/** Shared Jackson mapper and small JSON helpers. */
final class Json {

    private static final ObjectMapper MAPPER = new ObjectMapper()
            .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);

    static {
        // Allow binding into the package-private config records (canonical constructors).
        MAPPER.setVisibility(MAPPER.getVisibilityChecker()
                .withCreatorVisibility(Visibility.ANY)
                .withFieldVisibility(Visibility.ANY));
    }

    private static final ObjectMapper PRETTY = MAPPER.copy().enable(SerializationFeature.INDENT_OUTPUT);

    private Json() {
    }

    static ObjectMapper mapper() {
        return MAPPER;
    }

    static JsonNode read(Path path) {
        try {
            return MAPPER.readTree(Files.readString(path));
        } catch (IOException e) {
            throw new UncheckedIOException("Failed to read JSON file: " + path, e);
        }
    }

    static JsonNode read(String json) {
        try {
            return MAPPER.readTree(json);
        } catch (JsonProcessingException e) {
            throw new IllegalArgumentException("Not valid JSON: " + e.getOriginalMessage(), e);
        }
    }

    static String toPretty(JsonNode node) {
        try {
            return PRETTY.writeValueAsString(node);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Failed to serialize JSON", e);
        }
    }

    static String toCompact(JsonNode node) {
        try {
            return MAPPER.writeValueAsString(node);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Failed to serialize JSON", e);
        }
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/Model.java`
````java
package com.giftwrapper.heimdall;

import java.nio.file.Path;
import java.util.List;

// Domain types for Heimdall. Kept package-private and grouped in one file (single package).

/** The three downstream phases validated per scenario, in execution order. */
enum Phase {
    KAFKA, AEROSPIKE, API
}

/** Outcome of a phase or a whole scenario. */
enum Status {
    PASS, FAIL, SKIPPED, ERROR
}

/** One CSV row resolved to file paths; a null path means that phase is skipped. */
record Scenario(
        String scenarioNumber, String tableName, String topicName,
        Path inputJson, Path aqlQuery, Path aerospikeExpected,
        Path apiEndpoint, Path apiExpected) {

    boolean hasAerospikePhase() {
        return aqlQuery != null && aerospikeExpected != null;
    }

    boolean hasApiPhase() {
        return apiEndpoint != null && apiExpected != null;
    }
}

/** Result of a single phase. {@code diff}/{@code snapshot} are already PII-masked when built. */
record PhaseResult(Phase phase, Status status, long latencyMs, String detail, String diff, String snapshot) {

    static PhaseResult skipped(Phase phase, String detail) {
        return new PhaseResult(phase, Status.SKIPPED, 0L, detail, null, null);
    }
}

/** Aggregated result for one scenario across all phases. */
record ScenarioResult(
        String scenarioNumber, String tableName, String topicName, String correlationId,
        int msgCount, Status overall, Phase failurePhase, String failureReason,
        long totalLatencyMs, List<PhaseResult> phases) {

    boolean isFailureOrError() {
        return overall == Status.FAIL || overall == Status.ERROR;
    }

    /** Status recorded for a phase, or SKIPPED if it did not run. */
    Status statusFor(Phase phase) {
        if (phases != null) {
            for (PhaseResult pr : phases) {
                if (pr.phase() == phase) {
                    return pr.status();
                }
            }
        }
        return Status.SKIPPED;
    }
}

/** Result of comparing golden vs actual JSON. */
record ValidationOutcome(boolean match, String diff) {

    static ValidationOutcome matched() {
        return new ValidationOutcome(true, "");
    }

    static ValidationOutcome mismatch(String diff) {
        return new ValidationOutcome(false, diff);
    }
}

/** Captured curl result: HTTP status + body. */
record ApiResponse(int status, String body) {

    boolean is2xx() {
        return status >= 200 && status <= 299;
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/Config.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.JsonMappingException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Loads {@code config.json}: reads the file, substitutes {@code ${ENV:default}} placeholders, binds
 * to {@link Root}, validates, and returns it. All config record types live here (§5).
 */
final class Config {

    private static final Logger log = LoggerFactory.getLogger(Config.class);

    // ${NAME} or ${NAME:default} — default is everything after the first ':' (so it may contain ':').
    private static final Pattern PLACEHOLDER = Pattern.compile("\\$\\{([A-Za-z0-9_]+)(?::([^}]*))?}");

    private Config() {
    }

    static Root load(Path path) {
        if (path == null || !Files.isRegularFile(path)) {
            throw new HeimdallException("Config file not found: " + (path == null ? "<null>" : path.toAbsolutePath()));
        }
        final String raw;
        try {
            raw = Files.readString(path);
        } catch (IOException e) {
            throw new HeimdallException("Could not read config file: " + path, e);
        }
        final Root config;
        try {
            config = Json.mapper().readValue(substituteEnv(raw), Root.class);
        } catch (JsonMappingException e) {
            throw new HeimdallException("Config JSON does not match expected structure: " + e.getOriginalMessage(), e);
        } catch (IOException e) {
            throw new HeimdallException("Config file is not valid JSON: " + path, e);
        }
        validate(config, path);
        log.info("Loaded config from {} (active profile: {})", path, config.activeProfile());
        return config;
    }

    static String substituteEnv(String text) {
        if (text == null || text.indexOf("${") < 0) {
            return text;
        }
        Matcher m = PLACEHOLDER.matcher(text);
        StringBuilder out = new StringBuilder();
        while (m.find()) {
            String name = m.group(1);
            String def = m.group(2);
            String env = System.getenv(name);
            String sys = System.getProperty(name);
            String resolved = env != null ? env : (sys != null ? sys : (def != null ? def : ""));
            if (env == null && sys == null && def == null) {
                log.warn("Config placeholder ${{}} is unset and has no default; using empty string", name);
            }
            m.appendReplacement(out, Matcher.quoteReplacement(resolved));
        }
        m.appendTail(out);
        return out.toString();
    }

    private static void validate(Root config, Path path) {
        if (config == null) {
            throw new HeimdallException("Config file is empty: " + path);
        }
        if (config.activeProfile() == null || config.activeProfile().isBlank()) {
            throw new HeimdallException("config.activeProfile is required");
        }
        if (config.profiles() == null || !config.profiles().containsKey(config.activeProfile())) {
            throw new HeimdallException("activeProfile '" + config.activeProfile() + "' not among profiles "
                    + (config.profiles() == null ? "<none>" : config.profiles().keySet()));
        }
        Profile p = config.active();
        if (p.kafka() == null || p.kafka().bootstrapServers() == null || p.kafka().bootstrapServers().isBlank()) {
            throw new HeimdallException("kafka.bootstrapServers is required in the active profile");
        }
        if (p.testData() == null || p.testData().baseDir() == null) {
            throw new HeimdallException("testData.baseDir is required in the active profile");
        }
    }
}

/** Root of config.json: named profiles + the active one. */
record Root(String activeProfile, Map<String, Profile> profiles) {
    Profile active() {
        return profiles.get(activeProfile);
    }
}

record Profile(
        Kafka kafka,
        // "wait" is illegal as a record component name (clashes with Object.wait()).
        @JsonProperty("wait") Wait waitConfig,
        Aerospike aerospike, Api api, Validation validation,
        TestData testData, Cleanup cleanup, Report report, Execution execution) {
}

record Kafka(String bootstrapServers, ProducerCfg producer) {
}

/** keyField: JSONPath used as the Kafka message key; null ⇒ key on scenarioNumber (D16). */
record ProducerCfg(String acks, int ackTimeoutMs, boolean orderedSamePartition, String keyField) {
}

record Wait(long defaultMs, Map<String, Long> perTable) {
    long waitMsFor(String tableName) {
        if (perTable != null && tableName != null) {
            Long override = perTable.get(tableName);
            if (override != null) {
                return override;
            }
        }
        return defaultMs;
    }
}

/** executor: docker-run (default) | docker-exec | host (D7). */
record Aerospike(
        String executor, String toolsImage, String toolsContainer, String host,
        int port, String aqlBinary, String outputFormat, String namespace, long timeoutMs) {
}

record Api(String curlBinary, long timeoutMs, Auth auth, String baseUrlEnv) {
}

record Auth(String type) {
}

record Validation(String mode, List<String> ignorePaths, Arr array, boolean schema) {
    boolean isStrict() {
        return "strict".equalsIgnoreCase(mode);
    }
}

record Arr(boolean ignoreOrder, boolean ignoreExtraItems) {
}

record TestData(
        String baseDir, String inputJsonDir, String aqlDir,
        String aerospikeExpectedDir, String apiRequestDir, String apiExpectedDir) {
}

record Cleanup(boolean enabled, String when, String strategy, List<String> sets) {
}

record Report(String outputDir, boolean csv, boolean html, boolean junitXml, boolean maskPii, List<String> piiFields) {
}

record Execution(String mode, int parallelism, Boolean failExitCodeOnScenarioFailure) {
    boolean failExitCode() {
        return failExitCodeOnScenarioFailure == null || failExitCodeOnScenarioFailure;
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/PiiMasker.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Locale;
import java.util.Set;

/**
 * The single shared PII masker. Field-name masking replaces any JSON value whose field name
 * (case-insensitive) is configured PII with {@value #MASK}; value masking scrubs raw PII values out
 * of arbitrary text (e.g. diff messages). Immutable; pass-through when disabled. Comparison runs on
 * unmasked data — masking is applied only when building report/log output.
 */
final class PiiMasker {

    static final String MASK = "***MASKED***";

    private final boolean enabled;
    private final Set<String> piiFieldsLower;

    PiiMasker(boolean enabled, List<String> piiFields) {
        this.enabled = enabled;
        Set<String> lower = new LinkedHashSet<>();
        if (piiFields != null) {
            for (String f : piiFields) {
                if (f != null && !f.isBlank()) {
                    lower.add(f.toLowerCase(Locale.ROOT));
                }
            }
        }
        this.piiFieldsLower = Collections.unmodifiableSet(lower);
    }

    static PiiMasker from(Report cfg) {
        return cfg == null ? new PiiMasker(false, List.of()) : new PiiMasker(cfg.maskPii(), cfg.piiFields());
    }

    /** Returns a masked deep copy (input is never mutated). */
    JsonNode mask(JsonNode node) {
        if (!enabled || node == null || node.isNull() || piiFieldsLower.isEmpty()) {
            return node;
        }
        return maskInPlace(node.deepCopy());
    }

    /** Field-name masking of JSON given as text; non-JSON text is returned unchanged. */
    String maskJsonString(String json) {
        if (!enabled || json == null || json.isBlank() || piiFieldsLower.isEmpty()) {
            return json;
        }
        try {
            return Json.toPretty(mask(Json.read(json)));
        } catch (RuntimeException notJson) {
            return json;
        }
    }

    /** Scrubs raw PII values (harvested from the source nodes) out of arbitrary text. */
    String maskValuesIn(String text, JsonNode... sources) {
        if (!enabled || text == null || text.isBlank() || piiFieldsLower.isEmpty()) {
            return text;
        }
        Set<String> secrets = new LinkedHashSet<>();
        for (JsonNode source : sources) {
            collectPiiValues(source, secrets);
        }
        String masked = text;
        for (String secret : secrets) {
            if (secret != null && !secret.isBlank()) {
                masked = masked.replace(secret, MASK);
            }
        }
        return masked;
    }

    private void collectPiiValues(JsonNode node, Set<String> out) {
        if (node == null) {
            return;
        }
        if (node instanceof ObjectNode obj) {
            obj.fields().forEachRemaining(e -> {
                if (piiFieldsLower.contains(e.getKey().toLowerCase(Locale.ROOT)) && e.getValue().isValueNode()) {
                    out.add(e.getValue().asText());
                } else {
                    collectPiiValues(e.getValue(), out);
                }
            });
        } else if (node instanceof ArrayNode arr) {
            arr.forEach(child -> collectPiiValues(child, out));
        }
    }

    private JsonNode maskInPlace(JsonNode node) {
        if (node instanceof ObjectNode obj) {
            Set<String> names = new LinkedHashSet<>();
            obj.fieldNames().forEachRemaining(names::add);
            for (String fieldName : names) {
                if (piiFieldsLower.contains(fieldName.toLowerCase(Locale.ROOT))) {
                    obj.put(fieldName, MASK);
                } else {
                    maskInPlace(obj.get(fieldName));
                }
            }
        } else if (node instanceof ArrayNode arr) {
            for (JsonNode element : arr) {
                maskInPlace(element);
            }
        }
        return node;
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/ProcessRunner.java`
````java
package com.giftwrapper.heimdall;

import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Runs an external command with a hard timeout, draining stdout/stderr on separate threads to avoid
 * pipe-buffer deadlock. Shared by the Aerospike ({@code aql}) and API ({@code curl}) phases. The
 * subprocess inherits the current environment. A failure to start raises {@link UncheckedIOException}.
 */
final class ProcessRunner {

    private static final Logger log = LoggerFactory.getLogger(ProcessRunner.class);

    private ProcessRunner() {
    }

    record Result(int exitCode, String stdout, String stderr, boolean timedOut) {
        boolean ok() {
            return !timedOut && exitCode == 0;
        }
    }

    static Result run(List<String> command, long timeoutMs) {
        log.debug("Running: {}", String.join(" ", command));
        final Process process;
        try {
            process = new ProcessBuilder(command).start();
        } catch (IOException e) {
            throw new UncheckedIOException("Could not start process '" + command.get(0) + "'", e);
        }
        CompletableFuture<String> out = readAsync(process.getInputStream());
        CompletableFuture<String> err = readAsync(process.getErrorStream());
        try {
            boolean finished = process.waitFor(timeoutMs, TimeUnit.MILLISECONDS);
            if (!finished) {
                process.destroyForcibly();
                process.waitFor(2, TimeUnit.SECONDS);
                return new Result(-1, out.join(), err.join(), true);
            }
            return new Result(process.exitValue(), out.join(), err.join(), false);
        } catch (InterruptedException e) {
            process.destroyForcibly();
            Thread.currentThread().interrupt();
            throw new IllegalStateException("Interrupted while waiting for: " + command.get(0), e);
        }
    }

    private static CompletableFuture<String> readAsync(InputStream in) {
        return CompletableFuture.supplyAsync(() -> {
            try (InputStream stream = in) {
                return new String(stream.readAllBytes(), StandardCharsets.UTF_8);
            } catch (IOException e) {
                return "";
            }
        });
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/ScenarioCsvLoader.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.io.Reader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import org.apache.commons.csv.CSVFormat;
import org.apache.commons.csv.CSVParser;
import org.apache.commons.csv.CSVRecord;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Loads the CSV manifest (§3) into {@link Scenario} records, resolving file names under the
 * configured base directories. Strict: accumulates all problems and fails once. Also reads an input
 * file into the messages to publish (array ⇒ prerequisites, in order; object ⇒ one message).
 */
final class ScenarioCsvLoader {

    private static final Logger log = LoggerFactory.getLogger(ScenarioCsvLoader.class);

    private static final String COL_SCENARIO = "ScenarioNumber";
    private static final String COL_TABLE = "TableName";
    private static final String COL_TOPIC = "TopicName";
    private static final String COL_INPUT = "input_json_file_name";
    private static final String COL_AQL = "aerospike_query_file_name";
    private static final String COL_AERO_OUT = "aerospike_output_json_file_name";
    private static final String COL_API = "api_endpoint_files";
    private static final String COL_API_OUT = "api_output_json_file_name";
    private static final List<String> REQUIRED_COLUMNS = List.of(
            COL_SCENARIO, COL_TABLE, COL_TOPIC, COL_INPUT, COL_AQL, COL_AERO_OUT, COL_API, COL_API_OUT);

    private final TestData cfg;
    private final Path baseDir;

    ScenarioCsvLoader(TestData cfg) {
        this.cfg = cfg;
        this.baseDir = Paths.get(cfg.baseDir()).toAbsolutePath().normalize();
    }

    List<Scenario> load(Path csvPath) {
        if (csvPath == null || !Files.isRegularFile(csvPath)) {
            throw new HeimdallException("CSV manifest not found: "
                    + (csvPath == null ? "<null>" : csvPath.toAbsolutePath()));
        }
        CSVFormat format = CSVFormat.DEFAULT.builder()
                .setHeader().setSkipHeaderRecord(true).setTrim(true)
                .setIgnoreEmptyLines(true).setIgnoreSurroundingSpaces(true).build();

        List<Scenario> scenarios = new ArrayList<>();
        List<String> errors = new ArrayList<>();
        Set<String> seen = new HashSet<>();

        try (Reader reader = Files.newBufferedReader(csvPath); CSVParser parser = format.parse(reader)) {
            for (String required : REQUIRED_COLUMNS) {
                if (!parser.getHeaderNames().contains(required)) {
                    errors.add("missing required column: " + required);
                }
            }
            if (!errors.isEmpty()) {
                throw new HeimdallException("CSV header invalid in " + csvPath + ":\n  - " + String.join("\n  - ", errors));
            }
            for (CSVRecord rec : parser) {
                Scenario s = parseRow(rec, rec.getRecordNumber() + 1, seen, errors);
                if (s != null) {
                    scenarios.add(s);
                }
            }
        } catch (IOException e) {
            throw new HeimdallException("Could not read CSV manifest: " + csvPath, e);
        }

        if (scenarios.isEmpty() && errors.isEmpty()) {
            throw new HeimdallException("CSV manifest has no scenario rows: " + csvPath);
        }
        if (!errors.isEmpty()) {
            throw new HeimdallException("CSV manifest " + csvPath + " has " + errors.size()
                    + " problem(s):\n  - " + String.join("\n  - ", errors));
        }
        log.info("Loaded {} scenario(s) from {}", scenarios.size(), csvPath);
        return List.copyOf(scenarios);
    }

    private Scenario parseRow(CSVRecord rec, long line, Set<String> seen, List<String> errors) {
        String num = rec.get(COL_SCENARIO);
        String table = rec.get(COL_TABLE);
        String topic = rec.get(COL_TOPIC);
        String input = rec.get(COL_INPUT);
        String aql = rec.get(COL_AQL);
        String aeroOut = rec.get(COL_AERO_OUT);
        String api = rec.get(COL_API);
        String apiOut = rec.get(COL_API_OUT);
        boolean ok = true;

        if (isBlank(num)) {
            errors.add("line " + line + ": " + COL_SCENARIO + " is required");
            ok = false;
        } else if (!seen.add(num)) {
            errors.add("line " + line + ": duplicate " + COL_SCENARIO + " '" + num + "'");
            ok = false;
        }
        if (isBlank(topic)) {
            errors.add("line " + line + ": " + COL_TOPIC + " is required");
            ok = false;
        }
        if (isBlank(input)) {
            errors.add("line " + line + ": " + COL_INPUT + " is required (it is the publish entry point)");
            ok = false;
        }
        if (isBlank(aql) != isBlank(aeroOut)) {
            errors.add("line " + line + ": Aerospike phase needs both " + COL_AQL + " and " + COL_AERO_OUT + " (or neither)");
            ok = false;
        }
        if (isBlank(api) != isBlank(apiOut)) {
            errors.add("line " + line + ": API phase needs both " + COL_API + " and " + COL_API_OUT + " (or neither)");
            ok = false;
        }
        if (!ok) {
            return null;
        }

        Path inputPath = resolve(cfg.inputJsonDir(), input);
        Path aqlPath = resolve(cfg.aqlDir(), aql);
        Path aeroOutPath = resolve(cfg.aerospikeExpectedDir(), aeroOut);
        Path apiPath = resolve(cfg.apiRequestDir(), api);
        Path apiOutPath = resolve(cfg.apiExpectedDir(), apiOut);
        requireExists(inputPath, COL_INPUT, line, errors);
        requireExists(aqlPath, COL_AQL, line, errors);
        requireExists(aeroOutPath, COL_AERO_OUT, line, errors);
        requireExists(apiPath, COL_API, line, errors);
        requireExists(apiOutPath, COL_API_OUT, line, errors);

        return new Scenario(num, table, topic, inputPath, aqlPath, aeroOutPath, apiPath, apiOutPath);
    }

    /** Returns null for a blank name (⇒ that phase is skipped). */
    private Path resolve(String subDir, String fileName) {
        if (fileName == null || fileName.isBlank()) {
            return null;
        }
        Path sub = (subDir == null || subDir.isBlank()) ? baseDir : baseDir.resolve(subDir);
        return sub.resolve(fileName.trim()).normalize();
    }

    private static void requireExists(Path p, String col, long line, List<String> errors) {
        if (p != null && !Files.isRegularFile(p)) {
            errors.add("line " + line + ": " + col + " file not found: " + p);
        }
    }

    private static boolean isBlank(String s) {
        return s == null || s.isBlank();
    }

    /** Input split into the messages to publish (object ⇒ 1; array ⇒ one per element, in order). */
    record PublishPayload(List<JsonNode> elements, boolean array) {
        int size() {
            return elements.size();
        }
    }

    static PublishPayload readInput(Path inputJson) {
        final String raw;
        try {
            raw = Files.readString(inputJson);
        } catch (IOException e) {
            throw new HeimdallException("Could not read input JSON: " + inputJson, e);
        }
        final JsonNode root;
        try {
            root = Json.read(raw);
        } catch (RuntimeException e) {
            throw new HeimdallException("Input JSON is not valid JSON: " + inputJson, e);
        }
        if (root.isArray()) {
            if (root.isEmpty()) {
                throw new HeimdallException("Input JSON array is empty (nothing to publish): " + inputJson);
            }
            List<JsonNode> elements = new ArrayList<>(root.size());
            root.forEach(elements::add);
            return new PublishPayload(List.copyOf(elements), true);
        }
        if (root.isObject()) {
            return new PublishPayload(List.of(root), false);
        }
        throw new HeimdallException("Input JSON root must be object or array, was " + root.getNodeType() + ": " + inputJson);
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/KafkaPublisher.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.jayway.jsonpath.JsonPath;
import com.jayway.jsonpath.PathNotFoundException;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Properties;
import java.util.concurrent.TimeUnit;
import org.apache.kafka.clients.producer.KafkaProducer;
import org.apache.kafka.clients.producer.Producer;
import org.apache.kafka.clients.producer.ProducerConfig;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.apache.kafka.clients.producer.RecordMetadata;
import org.apache.kafka.common.header.internals.RecordHeader;
import org.apache.kafka.common.serialization.StringSerializer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Publishes a scenario's input to {@code Kafka_topic_1} with prerequisite semantics (D16): every
 * array element becomes one message; all messages share a key (so they land on one partition); they
 * are sent synchronously in order; and publishing is all-or-nothing — any failure fails the KAFKA
 * phase. One idempotent producer is reused across scenarios.
 */
final class KafkaPublisher implements AutoCloseable {

    private static final Logger log = LoggerFactory.getLogger(KafkaPublisher.class);
    private static final String H_CORRELATION_ID = "correlationId";
    private static final String H_SCENARIO_NUMBER = "scenarioNumber";
    private static final String H_ELEMENT_INDEX = "elementIndex";
    private static final String H_ELEMENT_COUNT = "elementCount";

    private final Kafka cfg;
    private final PiiMasker masker;
    private final Producer<String, String> producer;
    private final long ackTimeoutMs;

    KafkaPublisher(Kafka cfg, PiiMasker masker) {
        this.cfg = cfg;
        this.masker = masker;
        this.ackTimeoutMs = cfg.producer() != null ? cfg.producer().ackTimeoutMs() : 5000;
        this.producer = new KafkaProducer<>(buildProducerProps(cfg));
    }

    private static Properties buildProducerProps(Kafka cfg) {
        ProducerCfg producer = cfg.producer();
        int ackTimeout = producer != null ? producer.ackTimeoutMs() : 5000;
        String acks = (producer != null && producer.acks() != null) ? producer.acks() : "all";

        Properties props = new Properties();
        props.put(ProducerConfig.BOOTSTRAP_SERVERS_CONFIG, cfg.bootstrapServers());
        props.put(ProducerConfig.KEY_SERIALIZER_CLASS_CONFIG, StringSerializer.class.getName());
        props.put(ProducerConfig.VALUE_SERIALIZER_CLASS_CONFIG, StringSerializer.class.getName());
        props.put(ProducerConfig.ACKS_CONFIG, acks);
        // Ordering for prerequisites: idempotent producer + single in-flight request.
        props.put(ProducerConfig.ENABLE_IDEMPOTENCE_CONFIG, true);
        props.put(ProducerConfig.MAX_IN_FLIGHT_REQUESTS_PER_CONNECTION, 1);
        props.put(ProducerConfig.RETRIES_CONFIG, Integer.MAX_VALUE);
        props.put(ProducerConfig.LINGER_MS_CONFIG, 0);
        props.put(ProducerConfig.REQUEST_TIMEOUT_MS_CONFIG, ackTimeout);
        props.put(ProducerConfig.DELIVERY_TIMEOUT_MS_CONFIG, ackTimeout);
        // Fail fast if the broker is unreachable (instead of max.block's 60s default).
        props.put(ProducerConfig.MAX_BLOCK_MS_CONFIG, ackTimeout);
        props.put(ProducerConfig.CLIENT_ID_CONFIG, "heimdall-producer");
        return props;
    }

    PhaseResult publish(String topic, List<JsonNode> elements, String scenarioNumber, String correlationId) {
        long start = System.nanoTime();
        String snapshot = buildSnapshot(elements);

        final List<String> keys;
        try {
            keys = resolveKeys(elements, cfg.producer() != null ? cfg.producer().keyField() : null, scenarioNumber);
        } catch (RuntimeException e) {
            return fail(start, "Could not resolve message key: " + e.getMessage(), snapshot);
        }
        if (!allSame(keys)) {
            log.warn("Scenario {}: configured keyField yields differing keys across elements; "
                    + "same-partition ordering is not guaranteed (TECH_DEBT P-18)", scenarioNumber);
        }

        int count = elements.size();
        for (int i = 0; i < count; i++) {
            ProducerRecord<String, String> record =
                    new ProducerRecord<>(topic, null, keys.get(i), Json.toCompact(elements.get(i)));
            record.headers()
                    .add(new RecordHeader(H_CORRELATION_ID, bytes(correlationId)))
                    .add(new RecordHeader(H_SCENARIO_NUMBER, bytes(scenarioNumber)))
                    .add(new RecordHeader(H_ELEMENT_INDEX, bytes(Integer.toString(i))))
                    .add(new RecordHeader(H_ELEMENT_COUNT, bytes(Integer.toString(count))));
            try {
                RecordMetadata md = producer.send(record).get(ackTimeoutMs + 2000, TimeUnit.MILLISECONDS);
                log.debug("Scenario {}: published element {}/{} to {}-{}@{}",
                        scenarioNumber, i + 1, count, md.topic(), md.partition(), md.offset());
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return fail(start, "Interrupted publishing element " + (i + 1) + "/" + count, snapshot);
            } catch (Exception e) {
                Throwable cause = (e.getCause() != null) ? e.getCause() : e;
                return fail(start, "Failed publishing element " + (i + 1) + "/" + count + " to '" + topic + "': "
                        + cause.getClass().getSimpleName() + ": " + cause.getMessage(), snapshot);
            }
        }
        String detail = "Published " + count + " message(s) to '" + topic + "' (key='" + keys.get(0)
                + "', ordered, same-partition)";
        log.info("Scenario {}: {}", scenarioNumber, detail);
        return new PhaseResult(Phase.KAFKA, Status.PASS, elapsedMs(start), detail, null, snapshot);
    }

    /** One key per element: keyField JSONPath if configured, else the constant scenarioNumber. */
    private static List<String> resolveKeys(List<JsonNode> elements, String keyField, String scenarioNumber) {
        if (keyField == null || keyField.isBlank()) {
            return elements.stream().map(e -> scenarioNumber).toList();
        }
        return elements.stream().map(e -> {
            try {
                Object value = JsonPath.read(Json.toCompact(e), keyField.trim());
                if (value == null) {
                    throw new IllegalArgumentException("keyField '" + keyField + "' resolved to null");
                }
                return String.valueOf(value);
            } catch (PathNotFoundException ex) {
                throw new IllegalArgumentException("keyField '" + keyField + "' not present in an element", ex);
            }
        }).toList();
    }

    private static boolean allSame(List<String> keys) {
        return keys.isEmpty() || keys.stream().allMatch(k -> k.equals(keys.get(0)));
    }

    private PhaseResult fail(long start, String reason, String snapshot) {
        log.error("Kafka publish failed: {}", reason);
        return new PhaseResult(Phase.KAFKA, Status.FAIL, elapsedMs(start), reason, null, snapshot);
    }

    private String buildSnapshot(List<JsonNode> elements) {
        JsonNode payload;
        if (elements.size() == 1) {
            payload = elements.get(0);
        } else {
            ArrayNode arr = Json.mapper().createArrayNode();
            elements.forEach(arr::add);
            payload = arr;
        }
        return masker.maskJsonString(Json.toPretty(payload));
    }

    private static long elapsedMs(long startNanos) {
        return (System.nanoTime() - startNanos) / 1_000_000L;
    }

    private static byte[] bytes(String s) {
        return (s == null ? "" : s).getBytes(StandardCharsets.UTF_8);
    }

    @Override
    public void close() {
        try {
            producer.close();
        } catch (RuntimeException e) {
            log.warn("Error closing Kafka producer: {}", e.getMessage());
        }
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/Aql.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.UncheckedIOException;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Locale;
import java.util.Set;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * The Aerospike phase via the external {@code aql} tool (D7). Builds the command for one of three
 * modes ({@code docker-run} default, {@code docker-exec}, {@code host}), runs it verbatim, and
 * defensively parses {@code -o json} output into record bin-maps (Appendix H). Also runs end-of-run
 * cleanup (D11). The AQL statement is passed as a single argument (no shell), so it needs no quoting.
 */
final class Aql {

    private static final Logger log = LoggerFactory.getLogger(Aql.class);

    private static final String DOCKER_RUN = "docker-run";
    private static final String DOCKER_EXEC = "docker-exec";
    private static final String HOST = "host";

    /** Keys that, when they are the only keys of an object, mark it as metadata (not a record). */
    private static final Set<String> METADATA_KEYS = Set.of(
            "status", "row count", "number of records", "number of rows", "rows in set", "node", "error");

    private Aql() {
    }

    /** Run the query and return matching record bin-maps (empty list if none). */
    static List<JsonNode> query(String aql, Aerospike cfg) {
        List<String> command = buildCommand(cfg.executor(), cfg, aql);
        final ProcessRunner.Result result;
        try {
            result = ProcessRunner.run(command, cfg.timeoutMs());
        } catch (UncheckedIOException e) {
            throw new HeimdallException("Could not start aql ('" + command.get(0) + "'): " + e.getCause().getMessage(), e);
        }
        if (result.timedOut()) {
            throw new HeimdallException("aql timed out after " + cfg.timeoutMs() + "ms");
        }
        if (result.exitCode() != 0) {
            String reason = result.stderr() != null && !result.stderr().isBlank()
                    ? result.stderr().trim() : result.stdout().trim();
            throw new HeimdallException("aql exited " + result.exitCode() + ": " + reason);
        }
        return parse(result.stdout());
    }

    /** End-of-run truncate (best-effort; failures are logged, not thrown). */
    static void cleanup(Aerospike aero, Cleanup cleanup) {
        if (cleanup == null || !cleanup.enabled()) {
            log.info("Cleanup disabled; leaving Aerospike data in place");
            return;
        }
        List<String> statements = truncateStatements(aero.namespace(), cleanup);
        if (statements.isEmpty()) {
            log.warn("Cleanup enabled but no namespace/sets resolved; nothing to truncate");
            return;
        }
        for (String stmt : statements) {
            try {
                ProcessRunner.Result r = ProcessRunner.run(buildCommand(aero.executor(), aero, stmt), aero.timeoutMs());
                if (r.ok()) {
                    log.info("Cleanup ok: {}", stmt);
                } else {
                    log.warn("Cleanup statement failed (exit {}{}): {}", r.exitCode(), r.timedOut() ? ", timed out" : "", stmt);
                }
            } catch (RuntimeException e) {
                log.warn("Cleanup statement errored: {} ({})", stmt, e.getMessage());
            }
        }
    }

    static List<String> buildCommand(String mode, Aerospike cfg, String statement) {
        String format = (cfg.outputFormat() == null || cfg.outputFormat().isBlank()) ? "json" : cfg.outputFormat();
        String aqlBinary = (cfg.aqlBinary() == null || cfg.aqlBinary().isBlank()) ? "aql" : cfg.aqlBinary();
        List<String> cmd = new ArrayList<>();
        switch (mode == null ? DOCKER_RUN : mode) {
            case DOCKER_EXEC -> {
                cmd.add("docker");
                cmd.add("exec");
                cmd.add(require(cfg.toolsContainer(), "aerospike.toolsContainer (docker-exec mode)"));
                cmd.add(aqlBinary);
            }
            case HOST -> cmd.add(aqlBinary);
            case DOCKER_RUN -> {
                cmd.add("docker");
                cmd.add("run");
                cmd.add("--rm");
                cmd.add(require(cfg.toolsImage(), "aerospike.toolsImage (docker-run mode)"));
                cmd.add(aqlBinary);
            }
            default -> throw new HeimdallException("Unknown aerospike.executor '" + mode
                    + "' (expected docker-run | docker-exec | host)");
        }
        cmd.add("-h");
        cmd.add(require(cfg.host(), "aerospike.host"));
        cmd.add("-p");
        cmd.add(Integer.toString(cfg.port()));
        cmd.add("-o");
        cmd.add(format);
        cmd.add("-c");
        cmd.add(statement);
        return cmd;
    }

    /** Defensive parse of aql -o json output: walk arrays, keep record objects, drop metadata. */
    static List<JsonNode> parse(String stdout) {
        List<JsonNode> records = new ArrayList<>();
        if (stdout == null || stdout.isBlank()) {
            return records;
        }
        final JsonNode root;
        try {
            root = Json.read(stdout);
        } catch (RuntimeException e) {
            throw new HeimdallException("aql output was not valid JSON (is outputFormat 'json'?): " + truncate(stdout), e);
        }
        collect(root, records);
        log.debug("Parsed {} record(s) from aql output", records.size());
        return records;
    }

    private static void collect(JsonNode node, List<JsonNode> out) {
        if (node == null || node.isNull()) {
            return;
        }
        if (node.isArray()) {
            for (JsonNode child : node) {
                collect(child, out);
            }
        } else if (node.isObject() && !isMetadata((ObjectNode) node)) {
            out.add(node);
        }
    }

    private static boolean isMetadata(ObjectNode obj) {
        if (obj.isEmpty()) {
            return true;
        }
        Iterator<String> names = obj.fieldNames();
        while (names.hasNext()) {
            if (!METADATA_KEYS.contains(names.next().toLowerCase(Locale.ROOT))) {
                return false;
            }
        }
        return true;
    }

    private static List<String> truncateStatements(String namespace, Cleanup cleanup) {
        List<String> statements = new ArrayList<>();
        if (namespace == null || namespace.isBlank()) {
            return statements;
        }
        List<String> sets = cleanup.sets();
        if (sets == null || sets.isEmpty()) {
            statements.add("truncate " + namespace);
        } else {
            for (String set : sets) {
                if (set != null && !set.isBlank()) {
                    statements.add("truncate " + namespace + "." + set.trim());
                }
            }
        }
        return statements;
    }

    private static String require(String value, String what) {
        if (value == null || value.isBlank()) {
            throw new HeimdallException("Missing required config: " + what);
        }
        return value;
    }

    private static String truncate(String s) {
        return s.length() <= 500 ? s : s.substring(0, 500) + "…";
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/Validator.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.jayway.jsonpath.DocumentContext;
import com.jayway.jsonpath.JsonPath;
import com.jayway.jsonpath.spi.json.JacksonJsonNodeJsonProvider;
import com.jayway.jsonpath.spi.mapper.JacksonMappingProvider;
import java.util.ArrayList;
import java.util.List;
import net.javacrumbs.jsonunit.JsonAssert;
import net.javacrumbs.jsonunit.core.Configuration;
import net.javacrumbs.jsonunit.core.Option;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * JSON comparison with json-unit (D9, Appendix I): strict = exact; lenient (default) = ignore extra
 * fields + array order (+ optional extra array items). {@code ignorePaths} (JSONPath, incl.
 * recursive {@code $..}) are pruned from both sides via Jayway JsonPath before comparing, because
 * json-unit 3.4.1's own ignore-paths does not honour recursive descent. Also reconciles the
 * Aerospike result-list shape against the golden.
 */
final class Validator {

    private static final Logger log = LoggerFactory.getLogger(Validator.class);

    private static final com.jayway.jsonpath.Configuration JSONPATH = com.jayway.jsonpath.Configuration.builder()
            .jsonProvider(new JacksonJsonNodeJsonProvider(Json.mapper()))
            .mappingProvider(new JacksonMappingProvider(Json.mapper()))
            .options(com.jayway.jsonpath.Option.SUPPRESS_EXCEPTIONS)
            .build();

    private Validator() {
    }

    static ValidationOutcome compare(JsonNode expected, JsonNode actual, Validation cfg) {
        List<String> ignorePaths = (cfg == null) ? null : cfg.ignorePaths();
        JsonNode prunedExpected = prune(expected, ignorePaths);
        JsonNode prunedActual = prune(actual, ignorePaths);
        Configuration configuration = buildConfiguration(cfg);
        try {
            JsonAssert.assertJsonEquals(Json.toCompact(prunedExpected), Json.toCompact(prunedActual), configuration);
            return ValidationOutcome.matched();
        } catch (AssertionError mismatch) {
            return ValidationOutcome.mismatch(mismatch.getMessage());
        }
    }

    /**
     * Reconcile the Aerospike record list with the golden's shape (Appendix I): array golden ⇒ wrap;
     * object golden + exactly one record ⇒ object; otherwise wrap so a shape mismatch surfaces.
     */
    static JsonNode shapeAerospikeActual(JsonNode golden, List<JsonNode> records) {
        boolean goldenIsArray = golden != null && golden.isArray();
        if (!goldenIsArray && records.size() == 1) {
            return records.get(0);
        }
        ArrayNode array = Json.mapper().createArrayNode();
        records.forEach(array::add);
        return array;
    }

    private static Configuration buildConfiguration(Validation cfg) {
        boolean strict = cfg != null && cfg.isStrict();
        if (strict) {
            return Configuration.empty();
        }
        List<Option> options = new ArrayList<>();
        options.add(Option.IGNORING_EXTRA_FIELDS);
        Arr array = cfg == null ? null : cfg.array();
        if (array == null || array.ignoreOrder()) {
            options.add(Option.IGNORING_ARRAY_ORDER);
        }
        if (array != null && array.ignoreExtraItems()) {
            options.add(Option.IGNORING_EXTRA_ARRAY_ITEMS);
        }
        Option first = options.get(0);
        Option[] rest = options.subList(1, options.size()).toArray(new Option[0]);
        return Configuration.empty().withOptions(first, rest);
    }

    /** Returns a copy of {@code node} with every matching JSONPath deleted; input is not mutated. */
    private static JsonNode prune(JsonNode node, List<String> ignorePaths) {
        if (node == null || ignorePaths == null || ignorePaths.isEmpty()) {
            return node;
        }
        DocumentContext ctx = JsonPath.using(JSONPATH).parse(node.deepCopy());
        for (String path : ignorePaths) {
            if (path == null || path.isBlank()) {
                continue;
            }
            try {
                ctx.delete(path);
            } catch (RuntimeException e) {
                log.warn("Skipping invalid ignorePath '{}': {}", path, e.getMessage());
            }
        }
        return (JsonNode) ctx.json();
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/CurlApiExecutor.java`
````java
package com.giftwrapper.heimdall;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Executes one curl command and captures body + HTTP status (D8, Appendix J). The command runs
 * verbatim via {@code bash -c} (so shell env expansion works) with flags appended to reliably
 * capture the outcome: {@code -s} (silent), {@code -o <tmp>} (body to a temp file),
 * {@code -w '%{http_code}'} (status to stdout); appended flags win over duplicates. Honours
 * {@code api.curlBinary} and substitutes {@code api.baseUrlEnv} (default {@code API_BASE}).
 */
final class CurlApiExecutor {

    private static final Logger log = LoggerFactory.getLogger(CurlApiExecutor.class);
    private static final Pattern LAST_HTTP_CODE = Pattern.compile("(\\d{3})\\s*$");

    private CurlApiExecutor() {
    }

    static ApiResponse execute(String curlCommand, Api cfg) {
        if (curlCommand == null || curlCommand.isBlank()) {
            throw new HeimdallException("API endpoint file is empty (expected one curl command)");
        }
        Path bodyFile;
        try {
            bodyFile = Files.createTempFile("heimdall-api-", ".body");
        } catch (IOException e) {
            throw new HeimdallException("Could not create temp file for API body", e);
        }
        try {
            String prepared = applyBinaryAndBaseUrl(curlCommand.strip(), cfg);
            String wrapped = prepared + " --silent --output " + singleQuote(bodyFile.toString())
                    + " --write-out '%{http_code}'";
            long timeout = (cfg != null && cfg.timeoutMs() > 0) ? cfg.timeoutMs() : 10000;

            final ProcessRunner.Result result;
            try {
                result = ProcessRunner.run(List.of("bash", "-c", wrapped), timeout);
            } catch (UncheckedIOException e) {
                throw new HeimdallException("Could not start curl via bash: " + e.getCause().getMessage(), e);
            }
            if (result.timedOut()) {
                throw new HeimdallException("curl timed out after " + timeout + "ms");
            }
            int status = parseStatus(result.stdout());
            if (status < 100) {
                String reason = result.stderr() != null && !result.stderr().isBlank()
                        ? result.stderr().trim() : "no HTTP status (exit " + result.exitCode() + ")";
                throw new HeimdallException("curl did not produce an HTTP response: " + reason);
            }
            return new ApiResponse(status, Files.readString(bodyFile));
        } catch (IOException e) {
            throw new HeimdallException("Could not read API response body", e);
        } finally {
            try {
                Files.deleteIfExists(bodyFile);
            } catch (IOException ignore) {
                log.debug("Could not delete temp body file {}", bodyFile);
            }
        }
    }

    /** Honours curlBinary (replaces a leading {@code curl} token) and substitutes the base-URL env var. */
    static String applyBinaryAndBaseUrl(String curl, Api cfg) {
        String cmd = curl;
        String binary = (cfg != null && cfg.curlBinary() != null) ? cfg.curlBinary().trim() : "curl";
        if (!binary.isBlank() && !"curl".equals(binary)) {
            if (cmd.equals("curl")) {
                cmd = binary;
            } else if (cmd.startsWith("curl ")) {
                cmd = binary + cmd.substring("curl".length());
            }
        }
        String envName = (cfg != null && cfg.baseUrlEnv() != null && !cfg.baseUrlEnv().isBlank())
                ? cfg.baseUrlEnv().trim() : "API_BASE";
        return substituteEnvVar(cmd, envName);
    }

    private static String substituteEnvVar(String text, String name) {
        Pattern p = Pattern.compile("\\$\\{" + Pattern.quote(name) + "(?::-([^}]*))?}");
        Matcher m = p.matcher(text);
        if (!m.find()) {
            return text;
        }
        String value = System.getenv(name);
        if (value == null) {
            value = System.getProperty(name);
        }
        m.reset();
        StringBuilder sb = new StringBuilder();
        while (m.find()) {
            String bashDefault = m.group(1);
            String resolved = (value != null) ? value : (bashDefault != null ? bashDefault : "");
            m.appendReplacement(sb, Matcher.quoteReplacement(resolved));
        }
        m.appendTail(sb);
        return sb.toString();
    }

    private static int parseStatus(String stdout) {
        if (stdout == null || stdout.isBlank()) {
            return -1;
        }
        Matcher m = LAST_HTTP_CODE.matcher(stdout.trim());
        return m.find() ? Integer.parseInt(m.group(1)) : -1;
    }

    private static String singleQuote(String s) {
        return "'" + s.replace("'", "'\\''") + "'";
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/Reporters.java`
````java
package com.giftwrapper.heimdall;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.io.Writer;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.List;
import java.util.Locale;
import org.apache.commons.csv.CSVFormat;
import org.apache.commons.csv.CSVPrinter;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Writes the run results to {@code build/heimdall-report/}: CSV (exact columns), a single
 * self-contained HTML (filter-by-table, sort, expandable failure bundle; no CDN), and JUnit-XML
 * (Appendix K). Each format is gated by its toggle in report config. Values are already PII-masked.
 */
final class Reporters {

    private static final Logger log = LoggerFactory.getLogger(Reporters.class);
    private static final String BASE_NAME = "heimdall-report";
    private static final int HTML_COLS = 9;

    private static final String[] CSV_HEADERS = {
            "scenarioNumber", "tableName", "topicName", "correlationId", "msgCount",
            "kafkaStatus", "aerospikeStatus", "apiStatus", "overall", "totalLatencyMs",
            "failurePhase", "failureReason"
    };

    private Reporters() {
    }

    /** Write every report format enabled in config. */
    static void write(List<ScenarioResult> results, Report cfg) {
        if (cfg == null) {
            return;
        }
        if (cfg.csv()) {
            writeCsv(results, cfg);
        }
        if (cfg.html()) {
            writeHtml(results, cfg);
        }
        if (cfg.junitXml()) {
            writeJunitXml(results, cfg);
        }
    }

    private static Path outputDir(Report cfg) {
        String dir = (cfg.outputDir() != null && !cfg.outputDir().isBlank()) ? cfg.outputDir() : "./build/heimdall-report";
        Path path = Paths.get(dir).toAbsolutePath().normalize();
        try {
            Files.createDirectories(path);
        } catch (IOException e) {
            throw new UncheckedIOException("Could not create report output dir: " + path, e);
        }
        return path;
    }

    // ---- CSV ----
    private static void writeCsv(List<ScenarioResult> results, Report cfg) {
        Path out = outputDir(cfg).resolve(BASE_NAME + ".csv");
        CSVFormat format = CSVFormat.DEFAULT.builder().setHeader(CSV_HEADERS).build();
        try (Writer writer = Files.newBufferedWriter(out); CSVPrinter printer = new CSVPrinter(writer, format)) {
            for (ScenarioResult r : results) {
                printer.printRecord(r.scenarioNumber(), r.tableName(), r.topicName(), r.correlationId(),
                        r.msgCount(), r.statusFor(Phase.KAFKA), r.statusFor(Phase.AEROSPIKE), r.statusFor(Phase.API),
                        r.overall(), r.totalLatencyMs(),
                        r.failurePhase() == null ? "" : r.failurePhase(),
                        r.failureReason() == null ? "" : r.failureReason());
            }
        } catch (IOException e) {
            throw new UncheckedIOException("Failed writing CSV report: " + out, e);
        }
        log.info("Wrote CSV report: {}", out);
    }

    // ---- JUnit XML ----
    private static void writeJunitXml(List<ScenarioResult> results, Report cfg) {
        long failures = results.stream().filter(ScenarioResult::isFailureOrError).count();
        long skipped = results.stream().filter(r -> r.overall() == Status.SKIPPED).count();
        double totalTime = results.stream().mapToLong(ScenarioResult::totalLatencyMs).sum() / 1000.0;

        StringBuilder xml = new StringBuilder(1024);
        xml.append("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n");
        xml.append(String.format(Locale.ROOT,
                "<testsuite name=\"heimdall\" tests=\"%d\" failures=\"%d\" errors=\"0\" skipped=\"%d\" time=\"%.3f\">%n",
                results.size(), failures, skipped, totalTime));
        for (ScenarioResult r : results) {
            double time = r.totalLatencyMs() / 1000.0;
            xml.append(String.format(Locale.ROOT, "  <testcase name=\"%s\" classname=\"%s\" time=\"%.3f\"",
                    esc(r.scenarioNumber()), esc(r.tableName() == null ? "" : r.tableName()), time));
            if (r.isFailureOrError()) {
                String reason = r.failureReason() == null ? r.overall().name() : r.failureReason();
                xml.append(">\n    <failure message=\"").append(esc(reason)).append("\">")
                        .append(esc("phase=" + r.failurePhase() + "; " + reason))
                        .append("</failure>\n  </testcase>\n");
            } else if (r.overall() == Status.SKIPPED) {
                xml.append(">\n    <skipped/>\n  </testcase>\n");
            } else {
                xml.append("/>\n");
            }
        }
        xml.append("</testsuite>\n");
        Path out = outputDir(cfg).resolve(BASE_NAME + ".xml");
        try {
            Files.writeString(out, xml.toString());
        } catch (IOException e) {
            throw new UncheckedIOException("Failed writing JUnit XML report: " + out, e);
        }
        log.info("Wrote JUnit XML report: {}", out);
    }

    // ---- HTML (single self-contained file) ----
    private static void writeHtml(List<ScenarioResult> results, Report cfg) {
        StringBuilder html = new StringBuilder(8192);
        html.append("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"UTF-8\">\n");
        html.append("<title>Heimdall Report</title>\n<style>\n").append(CSS).append("\n</style>\n</head>\n<body>\n");
        html.append("<h1>Heimdall — Data Propagation Report</h1>\n");
        html.append(summary(results));
        html.append("<input id=\"tableFilter\" type=\"text\" placeholder=\"Filter by table…\" />\n");
        html.append("<table id=\"results\">\n<thead>\n<tr>")
                .append(th("Scenario", "str")).append(th("Table", "str")).append(th("Topic", "str"))
                .append(th("Kafka", "str")).append(th("Aerospike", "str")).append(th("API", "str"))
                .append(th("Overall", "str")).append(th("Latency (ms)", "num")).append(th("Msgs", "num"))
                .append("</tr>\n</thead>\n<tbody>\n");
        for (ScenarioResult r : results) {
            html.append(row(r)).append(detail(r));
        }
        html.append("</tbody>\n</table>\n<script>\n").append(JS).append("\n</script>\n</body>\n</html>\n");

        Path out = outputDir(cfg).resolve(BASE_NAME + ".html");
        try {
            Files.writeString(out, html.toString());
        } catch (IOException e) {
            throw new UncheckedIOException("Failed writing HTML report: " + out, e);
        }
        log.info("Wrote HTML report: {}", out);
    }

    private static String summary(List<ScenarioResult> results) {
        long pass = results.stream().filter(r -> r.overall() == Status.PASS).count();
        long fail = results.stream().filter(r -> r.overall() == Status.FAIL).count();
        long error = results.stream().filter(r -> r.overall() == Status.ERROR).count();
        long skipped = results.stream().filter(r -> r.overall() == Status.SKIPPED).count();
        return "<div class=\"summary\"><span>Total: " + results.size() + "</span>"
                + "<span class=\"pass\">Pass: " + pass + "</span>"
                + "<span class=\"fail\">Fail: " + fail + "</span>"
                + "<span class=\"error\">Error: " + error + "</span>"
                + "<span class=\"skipped\">Skipped: " + skipped + "</span></div>\n";
    }

    private static String th(String label, String type) {
        return "<th data-type=\"" + type + "\">" + esc(label) + "</th>";
    }

    private static String row(ScenarioResult r) {
        return "<tr class=\"row\" data-table=\"" + esc(r.tableName()) + "\">"
                + td(r.scenarioNumber()) + td(r.tableName()) + td(r.topicName())
                + badge(r.statusFor(Phase.KAFKA)) + badge(r.statusFor(Phase.AEROSPIKE)) + badge(r.statusFor(Phase.API))
                + badge(r.overall()) + tdNum(r.totalLatencyMs()) + tdNum(r.msgCount()) + "</tr>\n";
    }

    private static String detail(ScenarioResult r) {
        StringBuilder d = new StringBuilder();
        d.append("<tr class=\"detail\"><td colspan=\"").append(HTML_COLS).append("\"><div class=\"detailbox\">");
        d.append("<div><b>correlationId:</b> ").append(esc(r.correlationId())).append("</div>");
        if (r.failureReason() != null && !r.failureReason().isBlank()) {
            d.append("<div class=\"fail\"><b>failure (").append(esc(String.valueOf(r.failurePhase())))
                    .append("):</b> ").append(esc(r.failureReason())).append("</div>");
        }
        if (r.phases() != null) {
            for (PhaseResult pr : r.phases()) {
                d.append("<div class=\"phase\"><h4>").append(pr.phase()).append(" — ")
                        .append(pr.status()).append(" (").append(pr.latencyMs()).append(" ms)</h4>");
                appendField(d, "detail", pr.detail());
                appendField(d, "snapshot", pr.snapshot());
                appendField(d, "diff", pr.diff());
                d.append("</div>");
            }
        }
        d.append("</div></td></tr>\n");
        return d.toString();
    }

    private static void appendField(StringBuilder d, String label, String value) {
        if (value != null && !value.isBlank()) {
            d.append("<div class=\"field\"><span>").append(label).append("</span><pre>")
                    .append(esc(value)).append("</pre></div>");
        }
    }

    private static String td(String s) {
        return "<td>" + esc(s) + "</td>";
    }

    private static String tdNum(long n) {
        return "<td data-sort=\"" + n + "\">" + n + "</td>";
    }

    private static String badge(Status status) {
        return "<td><span class=\"badge " + status.name().toLowerCase(Locale.ROOT) + "\">" + status + "</span></td>";
    }

    private static String esc(String s) {
        if (s == null) {
            return "";
        }
        return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("\"", "&quot;");
    }

    private static final String CSS = """
            body { font-family: -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif; margin: 1.5rem; color: #222; }
            h1 { font-size: 1.4rem; }
            .summary { margin: 0.5rem 0 1rem; display: flex; gap: 1rem; font-weight: 600; }
            .summary .pass { color: #1a7f37; } .summary .fail { color: #cf222e; }
            .summary .error { color: #bc4c00; } .summary .skipped { color: #6e7781; }
            #tableFilter { padding: 0.4rem 0.6rem; width: 260px; margin-bottom: 0.8rem; border: 1px solid #ccc; border-radius: 4px; }
            table { border-collapse: collapse; width: 100%; font-size: 0.9rem; }
            th, td { border: 1px solid #e1e4e8; padding: 0.4rem 0.6rem; text-align: left; }
            th { background: #f6f8fa; cursor: pointer; user-select: none; }
            th:hover { background: #eaeef2; }
            tr.row { cursor: pointer; }
            tr.row:hover { background: #f6f8fa; }
            tr.detail { display: none; }
            tr.detail.open { display: table-row; }
            .detailbox { padding: 0.5rem; background: #fbfcfd; }
            .phase { margin: 0.5rem 0; padding: 0.5rem; border-left: 3px solid #d0d7de; }
            .phase h4 { margin: 0 0 0.3rem; font-size: 0.85rem; }
            .field { margin: 0.2rem 0; }
            .field span { display: inline-block; min-width: 70px; font-weight: 600; color: #57606a; vertical-align: top; }
            .field pre { display: inline-block; margin: 0; padding: 0.4rem; background: #f6f8fa; border-radius: 4px; max-width: 90%; white-space: pre-wrap; word-break: break-word; }
            .badge { padding: 0.1rem 0.5rem; border-radius: 10px; font-size: 0.78rem; font-weight: 600; color: #fff; }
            .badge.pass { background: #1a7f37; } .badge.fail { background: #cf222e; }
            .badge.error { background: #bc4c00; } .badge.skipped { background: #6e7781; }
            """;

    private static final String JS = """
            (function () {
              var table = document.getElementById('results');
              var tbody = table.querySelector('tbody');
              tbody.addEventListener('click', function (e) {
                var tr = e.target.closest('tr.row');
                if (!tr) return;
                var detail = tr.nextElementSibling;
                if (detail && detail.classList.contains('detail')) detail.classList.toggle('open');
              });
              document.getElementById('tableFilter').addEventListener('input', function () {
                var q = this.value.trim().toLowerCase();
                tbody.querySelectorAll('tr.row').forEach(function (row) {
                  var t = (row.getAttribute('data-table') || '').toLowerCase();
                  var show = q === '' || t.indexOf(q) !== -1;
                  row.style.display = show ? '' : 'none';
                  var detail = row.nextElementSibling;
                  if (detail && detail.classList.contains('detail')) {
                    detail.classList.remove('open');
                    detail.style.display = show ? '' : 'none';
                  }
                });
              });
              var headers = table.querySelectorAll('thead th');
              headers.forEach(function (th, index) {
                th.addEventListener('click', function () {
                  var numeric = th.getAttribute('data-type') === 'num';
                  var asc = th.getAttribute('data-asc') !== 'true';
                  headers.forEach(function (h) { h.removeAttribute('data-asc'); });
                  th.setAttribute('data-asc', asc ? 'true' : 'false');
                  var pairs = [];
                  tbody.querySelectorAll('tr.row').forEach(function (row) { pairs.push([row, row.nextElementSibling]); });
                  pairs.sort(function (a, b) {
                    var av = cellValue(a[0], index, numeric);
                    var bv = cellValue(b[0], index, numeric);
                    if (av < bv) return asc ? -1 : 1;
                    if (av > bv) return asc ? 1 : -1;
                    return 0;
                  });
                  pairs.forEach(function (p) {
                    tbody.appendChild(p[0]);
                    if (p[1] && p[1].classList.contains('detail')) tbody.appendChild(p[1]);
                  });
                });
              });
              function cellValue(row, index, numeric) {
                var cell = row.cells[index];
                if (!cell) return numeric ? 0 : '';
                if (numeric) {
                  var s = cell.getAttribute('data-sort');
                  return parseFloat(s !== null ? s : cell.textContent) || 0;
                }
                return cell.textContent.trim().toLowerCase();
              }
            })();
            """;
}
````

### `src/main/java/com/giftwrapper/heimdall/ScenarioRunner.java`
````java
package com.giftwrapper.heimdall;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.List;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;

/**
 * Executes the §4 per-scenario flow sequentially: publish (prerequisite-aware) → fixed wait → AQL
 * read+compare → curl read+compare, collecting a {@link ScenarioResult} per row. Snapshots and diffs
 * are PII-masked here using the unmasked JSON as the source of values to scrub.
 */
final class ScenarioRunner {

    private static final Logger log = LoggerFactory.getLogger(ScenarioRunner.class);

    private final Profile profile;
    private final PiiMasker masker;
    private final KafkaPublisher publisher;
    private final String runId;

    ScenarioRunner(Profile profile, PiiMasker masker, KafkaPublisher publisher, String runId) {
        this.profile = profile;
        this.masker = masker;
        this.publisher = publisher;
        this.runId = runId;
    }

    List<ScenarioResult> run(List<Scenario> scenarios) {
        List<ScenarioResult> results = new ArrayList<>(scenarios.size());
        for (Scenario scenario : scenarios) {
            results.add(runOne(scenario));
        }
        return results;
    }

    ScenarioResult runOne(Scenario s) {
        String correlationId = s.scenarioNumber() + "-" + runId;
        MDC.put("correlationId", correlationId);
        MDC.put("scenarioNumber", s.scenarioNumber());
        long start = System.nanoTime();
        List<PhaseResult> phases = new ArrayList<>();
        int msgCount = 0;
        try {
            MDC.put("phase", Phase.KAFKA.name());
            ScenarioCsvLoader.PublishPayload payload;
            try {
                payload = ScenarioCsvLoader.readInput(s.inputJson());
            } catch (RuntimeException e) {
                phases.add(new PhaseResult(Phase.KAFKA, Status.ERROR, 0, "Could not read input JSON: " + e.getMessage(), null, null));
                return assemble(s, correlationId, 0, phases, elapsedMs(start));
            }
            msgCount = payload.size();
            PhaseResult kafka = publisher.publish(s.topicName(), payload.elements(), s.scenarioNumber(), correlationId);
            phases.add(kafka);
            if (kafka.status() != Status.PASS) {
                if (s.hasAerospikePhase()) {
                    phases.add(PhaseResult.skipped(Phase.AEROSPIKE, "Skipped: Kafka phase did not pass"));
                }
                if (s.hasApiPhase()) {
                    phases.add(PhaseResult.skipped(Phase.API, "Skipped: Kafka phase did not pass"));
                }
                return assemble(s, correlationId, msgCount, phases, elapsedMs(start));
            }

            sleep(profile.waitConfig().waitMsFor(s.tableName()));

            if (s.hasAerospikePhase()) {
                MDC.put("phase", Phase.AEROSPIKE.name());
                phases.add(runAerospike(s));
            }
            if (s.hasApiPhase()) {
                MDC.put("phase", Phase.API.name());
                phases.add(runApi(s));
            }
            return assemble(s, correlationId, msgCount, phases, elapsedMs(start));
        } finally {
            MDC.clear();
        }
    }

    private PhaseResult runAerospike(Scenario s) {
        long start = System.nanoTime();
        try {
            String aql = Files.readString(s.aqlQuery()).strip();
            List<JsonNode> records = Aql.query(aql, profile.aerospike());
            JsonNode golden = Json.read(s.aerospikeExpected());
            JsonNode actual = Validator.shapeAerospikeActual(golden, records);
            ValidationOutcome outcome = Validator.compare(golden, actual, profile.validation());
            String snapshot = "-- AQL --\n" + masker.maskValuesIn(aql, golden, actual)
                    + "\n\n-- Aerospike result (" + records.size() + " record(s)) --\n"
                    + maskBody(Json.toPretty(actual), golden, actual);
            if (outcome.match()) {
                return new PhaseResult(Phase.AEROSPIKE, Status.PASS, elapsedMs(start),
                        "Matched golden (" + records.size() + " record(s))", null, snapshot);
            }
            return new PhaseResult(Phase.AEROSPIKE, Status.FAIL, elapsedMs(start),
                    "Aerospike result did not match golden", masker.maskValuesIn(outcome.diff(), golden, actual), snapshot);
        } catch (IOException e) {
            return new PhaseResult(Phase.AEROSPIKE, Status.ERROR, elapsedMs(start),
                    "Could not read AQL/golden file: " + e.getMessage(), null, null);
        } catch (RuntimeException e) {
            return new PhaseResult(Phase.AEROSPIKE, Status.ERROR, elapsedMs(start),
                    "Aerospike phase error: " + e.getMessage(), null, null);
        }
    }

    private PhaseResult runApi(Scenario s) {
        long start = System.nanoTime();
        try {
            String curl = Files.readString(s.apiEndpoint()).strip();
            ApiResponse resp = CurlApiExecutor.execute(curl, profile.api());
            JsonNode golden = Json.read(s.apiExpected());
            JsonNode actualBody = null;
            try {
                actualBody = Json.read(resp.body());
            } catch (RuntimeException notJson) {
                actualBody = null;
            }
            ValidationOutcome outcome = (actualBody != null)
                    ? Validator.compare(golden, actualBody, profile.validation())
                    : ValidationOutcome.mismatch("Response body is not valid JSON");
            boolean pass = resp.is2xx() && outcome.match();
            JsonNode bodySource = (actualBody != null) ? actualBody : golden;
            String snapshot = "-- Request --\n" + masker.maskValuesIn(curl, golden, bodySource)
                    + "\n\n-- Response: HTTP " + resp.status() + " --\n" + maskBody(resp.body(), golden, bodySource);
            String detail = "HTTP " + resp.status() + (resp.is2xx() ? " (2xx)" : " (NOT 2xx)")
                    + "; body " + (outcome.match() ? "matched" : "mismatch");
            if (pass) {
                return new PhaseResult(Phase.API, Status.PASS, elapsedMs(start), detail, null, snapshot);
            }
            StringBuilder diff = new StringBuilder();
            if (!resp.is2xx()) {
                diff.append("HTTP status ").append(resp.status()).append(" is not 2xx. ");
            }
            if (!outcome.match()) {
                diff.append(masker.maskValuesIn(outcome.diff(), golden, bodySource));
            }
            return new PhaseResult(Phase.API, Status.FAIL, elapsedMs(start), detail, diff.toString(), snapshot);
        } catch (IOException e) {
            return new PhaseResult(Phase.API, Status.ERROR, elapsedMs(start),
                    "Could not read curl/golden file: " + e.getMessage(), null, null);
        } catch (RuntimeException e) {
            return new PhaseResult(Phase.API, Status.ERROR, elapsedMs(start),
                    "API phase error: " + e.getMessage(), null, null);
        }
    }

    private ScenarioResult assemble(Scenario s, String correlationId, int msgCount,
            List<PhaseResult> phases, long totalLatencyMs) {
        Status overall = Status.PASS;
        Phase failurePhase = null;
        String failureReason = null;
        for (PhaseResult pr : phases) {
            if (pr.status() == Status.ERROR) {
                overall = Status.ERROR;
                failurePhase = pr.phase();
                failureReason = pr.detail();
                break;
            }
        }
        if (overall != Status.ERROR) {
            for (PhaseResult pr : phases) {
                if (pr.status() == Status.FAIL) {
                    overall = Status.FAIL;
                    failurePhase = pr.phase();
                    failureReason = pr.detail();
                    break;
                }
            }
        }
        if (phases.isEmpty()) {
            overall = Status.ERROR;
            failureReason = "No phases executed";
        }
        log.info("Scenario {} [{}] overall={}", s.scenarioNumber(), s.tableName(), overall);
        return new ScenarioResult(s.scenarioNumber(), s.tableName(), s.topicName(), correlationId,
                msgCount, overall, failurePhase, failureReason, totalLatencyMs, List.copyOf(phases));
    }

    /** Field-name masking for JSON; value-scrubbing also covers non-JSON bodies. */
    private String maskBody(String text, JsonNode... sources) {
        return masker.maskValuesIn(masker.maskJsonString(text), sources);
    }

    /** Fixed configurable wait between publish and query (D5 — no polling). */
    private static void sleep(long millis) {
        if (millis <= 0) {
            return;
        }
        log.debug("Waiting {}ms for downstream propagation", millis);
        try {
            Thread.sleep(millis);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            log.warn("Wait interrupted after partial delay");
        }
    }

    private static long elapsedMs(long startNanos) {
        return (System.nanoTime() - startNanos) / 1_000_000L;
    }
}
````

### `src/main/java/com/giftwrapper/heimdall/HeimdallMain.java`
````java
package com.giftwrapper.heimdall;

import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.List;
import java.util.UUID;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Heimdall entry point. Reads {@code -Dheimdall.config} and {@code -Dheimdall.csv} (defaults
 * {@code test-data/config.json}, {@code test-data/scenarios.csv}), runs every scenario, writes the
 * reports (always, before exit), runs end-of-run cleanup, and exits with:
 * {@code 0} all pass · {@code 1} at least one FAIL/ERROR (toggle: execution.failExitCodeOnScenarioFailure)
 * · {@code 2} framework/config error.
 */
public final class HeimdallMain {

    private static final Logger log = LoggerFactory.getLogger(HeimdallMain.class);

    static final int EXIT_OK = 0;
    static final int EXIT_SCENARIO_FAILURE = 1;
    static final int EXIT_FRAMEWORK_ERROR = 2;

    private HeimdallMain() {
    }

    public static void main(String[] args) {
        System.exit(run());
    }

    static int run() {
        try {
            Path configPath = Paths.get(System.getProperty("heimdall.config", "test-data/config.json"));
            Path csvPath = Paths.get(System.getProperty("heimdall.csv", "test-data/scenarios.csv"));
            log.info("Heimdall starting (config={}, csv={})", configPath, csvPath);

            Profile profile = Config.load(configPath).active();
            List<Scenario> scenarios = new ScenarioCsvLoader(profile.testData()).load(csvPath);
            PiiMasker masker = PiiMasker.from(profile.report());
            String runId = UUID.randomUUID().toString().substring(0, 8);

            List<ScenarioResult> results;
            try (KafkaPublisher publisher = new KafkaPublisher(profile.kafka(), masker)) {
                results = new ScenarioRunner(profile, masker, publisher, runId).run(scenarios);
            }

            // End-of-run cleanup (D11) — best-effort, never changes the run outcome.
            try {
                Aql.cleanup(profile.aerospike(), profile.cleanup());
            } catch (RuntimeException e) {
                log.warn("Cleanup error (ignored): {}", e.getMessage());
            }

            // Reports always written before exit.
            try {
                Reporters.write(results, profile.report());
            } catch (RuntimeException e) {
                log.error("Reporting failed: {}", e.getMessage());
            }
            logSummary(results);

            boolean anyFailure = results.stream().anyMatch(ScenarioResult::isFailureOrError);
            boolean failExit = profile.execution() == null || profile.execution().failExitCode();
            return (anyFailure && failExit) ? EXIT_SCENARIO_FAILURE : EXIT_OK;

        } catch (HeimdallException e) {
            log.error("Configuration/manifest error: {}", e.getMessage());
            return EXIT_FRAMEWORK_ERROR;
        } catch (RuntimeException e) {
            log.error("Framework error", e);
            return EXIT_FRAMEWORK_ERROR;
        }
    }

    private static void logSummary(List<ScenarioResult> results) {
        long pass = results.stream().filter(r -> r.overall() == Status.PASS).count();
        long fail = results.stream().filter(r -> r.overall() == Status.FAIL).count();
        long error = results.stream().filter(r -> r.overall() == Status.ERROR).count();
        long skipped = results.stream().filter(r -> r.overall() == Status.SKIPPED).count();
        log.info("Run complete: {} scenario(s) — PASS={}, FAIL={}, ERROR={}, SKIPPED={}",
                results.size(), pass, fail, error, skipped);
    }
}
````

### `test-data/config.json`
````json
{
  "activeProfile": "local",
  "profiles": {
    "local": {
      "kafka": {
        "bootstrapServers": "${KAFKA_BOOTSTRAP:localhost:9092}",
        "producer": {
          "acks": "all",
          "ackTimeoutMs": 5000,
          "orderedSamePartition": true,
          "keyField": null
        }
      },
      "wait": {
        "defaultMs": 8000,
        "perTable": {
          "INVM": 10000
        }
      },
      "aerospike": {
        "executor": "docker-run",
        "toolsImage": "aerospike/aerospike-tools:11.0.0",
        "toolsContainer": "aerospike-tools",
        "host": "${AS_HOST:host.docker.internal}",
        "port": 3000,
        "aqlBinary": "aql",
        "outputFormat": "json",
        "namespace": "cbs",
        "timeoutMs": 15000
      },
      "api": {
        "curlBinary": "curl",
        "timeoutMs": 10000,
        "auth": { "type": "none" },
        "baseUrlEnv": "API_BASE"
      },
      "validation": {
        "mode": "lenient",
        "ignorePaths": ["$..lastUpdatedTs", "$..auditTs"],
        "array": { "ignoreOrder": true, "ignoreExtraItems": false },
        "schema": false
      },
      "testData": {
        "baseDir": "./test-data",
        "inputJsonDir": "input",
        "aqlDir": "aql",
        "aerospikeExpectedDir": "expected/aerospike",
        "apiRequestDir": "api",
        "apiExpectedDir": "expected/api"
      },
      "cleanup": {
        "enabled": true,
        "when": "end-of-run",
        "strategy": "truncate-namespace",
        "sets": []
      },
      "report": {
        "outputDir": "./build/heimdall-report",
        "csv": true,
        "html": true,
        "junitXml": true,
        "maskPii": true,
        "piiFields": ["pan", "aadhaar", "accountNo"]
      },
      "execution": {
        "mode": "sequential",
        "parallelism": 1,
        "failExitCodeOnScenarioFailure": true
      }
    }
  }
}
````

### `test-data/scenarios.csv`
````text
ScenarioNumber,TableName,TopicName,input_json_file_name,aerospike_query_file_name,aerospike_output_json_file_name,api_endpoint_files,api_output_json_file_name
HD_001,INVM,CBS-INBOUND-CHANNEL,invm_single.json,aql_invm_single.txt,expected_invm_single.json,account_details_api.txt,expected_api_invm_single.json
HD_002,INVM,CBS-INBOUND-CHANNEL,invm_prereq_array.json,aql_invm_prereq.txt,expected_invm_prereq.json,account_details_prereq_api.txt,expected_api_invm_prereq.json
HD_003,INVM,CBS-INBOUND-CHANNEL,invm_single.json,aql_invm_single.txt,expected_invm_NEGATIVE.json,,
````

### `test-data/input/invm_single.json`
````json
{
  "PK": "INVM-0001",
  "accountId": "INVM-0001",
  "accountNo": "100200300400",
  "pan": "ABCDE1234F",
  "customerName": "Placeholder Customer",
  "productType": "INVM",
  "balance": 15000.50,
  "currency": "INR",
  "status": "ACTIVE"
}
````

### `test-data/input/invm_prereq_array.json`
````json
[
  {
    "PK": "INVM-0002",
    "accountId": "INVM-0002",
    "accountNo": "100200300500",
    "pan": "PQRSX5678Y",
    "productType": "INVM",
    "balance": 5000.00,
    "currency": "INR",
    "status": "PENDING",
    "eventSeq": 1
  },
  {
    "PK": "INVM-0002",
    "accountId": "INVM-0002",
    "accountNo": "100200300500",
    "pan": "PQRSX5678Y",
    "productType": "INVM",
    "balance": 18250.75,
    "currency": "INR",
    "status": "ACTIVE",
    "eventSeq": 2
  }
]
````

### `test-data/aql/aql_invm_single.txt`
````bash
SELECT accountId, accountNo, pan, productType, balance, currency, status FROM cbs.INVM WHERE PK = 'INVM-0001'
````

### `test-data/aql/aql_invm_prereq.txt`
````bash
SELECT accountId, accountNo, pan, productType, balance, currency, status FROM cbs.INVM WHERE PK = 'INVM-0002'
````

### `test-data/api/account_details_api.txt`
````bash
curl -X GET "${API_BASE:-http://localhost:8080}/accounts/INVM-0001" -H "Accept: application/json"
````

### `test-data/api/account_details_prereq_api.txt`
````bash
curl -X GET "${API_BASE:-http://localhost:8080}/accounts/INVM-0002" -H "Accept: application/json"
````

### `test-data/expected/aerospike/expected_invm_single.json`
````json
{
  "accountId": "INVM-0001",
  "accountNo": "100200300400",
  "pan": "ABCDE1234F",
  "productType": "INVM",
  "balance": 15000.50,
  "currency": "INR",
  "status": "ACTIVE"
}
````

### `test-data/expected/aerospike/expected_invm_prereq.json`
````json
{
  "accountId": "INVM-0002",
  "accountNo": "100200300500",
  "pan": "PQRSX5678Y",
  "productType": "INVM",
  "balance": 18250.75,
  "currency": "INR",
  "status": "ACTIVE"
}
````

### `test-data/expected/aerospike/expected_invm_NEGATIVE.json`
````json
{
  "accountId": "INVM-0001",
  "accountNo": "100200300400",
  "pan": "ABCDE1234F",
  "productType": "INVM",
  "balance": 99999.99,
  "currency": "INR",
  "status": "CLOSED"
}
````

### `test-data/expected/api/expected_api_invm_single.json`
````json
{
  "accountId": "INVM-0001",
  "accountNo": "100200300400",
  "balance": 15000.50,
  "currency": "INR",
  "status": "ACTIVE"
}
````

### `test-data/expected/api/expected_api_invm_prereq.json`
````json
{
  "accountId": "INVM-0002",
  "accountNo": "100200300500",
  "balance": 18250.75,
  "currency": "INR",
  "status": "ACTIVE"
}
````

### `docker/.env`
````bash
# Heimdall optional local env images (IMPLEMENTATION_APPROACH.md Appendix F).
# Infra images are real defaults; the three CBS services are placeholders you must replace
# with your registry coordinates before `make up` will work (TECH_DEBT.md P-02).
KAFKA_IMAGE=confluentinc/cp-kafka:7.6.1
AEROSPIKE_IMAGE=aerospike/aerospike-server:7.1.0.0
AEROSPIKE_TOOLS_IMAGE=aerospike/aerospike-tools:11.0.0

DENORMALISER_IMAGE=__REPLACE_ME__
CONSUMER_IMAGE=__REPLACE_ME__
WEBSERVICES_IMAGE=__REPLACE_ME__
````

### `docker/docker-compose.yml`
````yaml
# Heimdall OPTIONAL local environment (IMPLEMENTATION_APPROACH.md Appendix F, D2).
# Heimdall is a pure client against configurable endpoints — you normally run against your own
# services and never touch this file. It exists only as a convenience to stand up a local stack.
# The three cbs-* services are __REPLACE_ME__ placeholders (TECH_DEBT.md P-02): set real image
# coordinates in docker/.env, then `make up`.
services:
  kafka:
    image: ${KAFKA_IMAGE}
    container_name: heimdall-kafka
    ports:
      - "9092:9092"
    environment:
      # KRaft mode (no ZooKeeper).
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: "broker,controller"
      KAFKA_CONTROLLER_QUORUM_VOTERS: "1@kafka:29093"
      KAFKA_LISTENERS: "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093"
      KAFKA_ADVERTISED_LISTENERS: "PLAINTEXT://localhost:9092"
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT"
      KAFKA_CONTROLLER_LISTENER_NAMES: "CONTROLLER"
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      CLUSTER_ID: "heimdall-local-cluster-0001"
    healthcheck:
      test: ["CMD-SHELL", "kafka-topics --bootstrap-server localhost:9092 --list || exit 1"]
      interval: 10s
      timeout: 10s
      retries: 12

  aerospike:
    image: ${AEROSPIKE_IMAGE}
    container_name: heimdall-aerospike
    ports:
      - "3000:3000"
    environment:
      NAMESPACE: cbs
    healthcheck:
      test: ["CMD-SHELL", "asinfo -v build || exit 1"]
      interval: 10s
      timeout: 10s
      retries: 12

  # Long-lived tools container — used only by the aerospike executor `docker-exec` mode.
  aerospike-tools:
    image: ${AEROSPIKE_TOOLS_IMAGE}
    container_name: aerospike-tools
    depends_on:
      aerospike:
        condition: service_healthy
    command: ["sleep", "infinity"]

  cbs-denormaliser:
    image: ${DENORMALISER_IMAGE}
    container_name: heimdall-cbs-denormaliser
    depends_on:
      kafka:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://localhost:8080/actuator/health || exit 1"]
      interval: 15s
      timeout: 10s
      retries: 12

  cbs-kafka-aerospike-consumer:
    image: ${CONSUMER_IMAGE}
    container_name: heimdall-cbs-consumer
    depends_on:
      kafka:
        condition: service_healthy
      aerospike:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://localhost:8080/actuator/health || exit 1"]
      interval: 15s
      timeout: 10s
      retries: 12

  cbs-web-services:
    image: ${WEBSERVICES_IMAGE}
    container_name: heimdall-cbs-web-services
    ports:
      - "8080:8080"
    depends_on:
      aerospike:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://localhost:8080/actuator/health || exit 1"]
      interval: 15s
      timeout: 10s
      retries: 12
````

### `README.md`
````markdown
# Heimdall

**End-to-end data-propagation test framework for the *Giftwrapper* banking platform.**

Heimdall publishes JSON to `Kafka_topic_1` and then asserts that the data correctly propagates all
the way through the pipeline:

```
publish → Kafka_topic_1 → cbs-denormaliser → Kafka_topic_2 → cbs-kafka-aerospike-consumer
        → Aerospike → cbs_web_services (REST API)
```

For each scenario it (1) publishes the input, (2) waits a fixed interval, (3) runs an AQL query and
compares the Aerospike record against a golden file, and (4) runs a curl request and compares the
API response against a golden file. Scenarios are defined in a **CSV manifest** — one row each.

Heimdall is a **pure client against configurable endpoints**: you point it at your own running
Kafka / Aerospike / API via `test-data/config.json` and run it. A docker-compose stack is provided
purely as an optional convenience.

> Design source of truth: `IMPLEMENTATION_APPROACH.md`. Parked decisions / known limitations:
> `TECH_DEBT.md`.

---

## Prerequisites

| Tool | Why | Notes |
|------|-----|-------|
| **JDK 21** | Build & run | The Gradle toolchain **auto-provisions** a JDK 21 if your machine has an older one (this repo was built on a JDK-17 host). No manual install needed as long as the build has network access the first time. |
| **Docker** | Aerospike `aql` (default `docker-run` mode) and the optional local env | Not needed if you set `aerospike.executor` to `host` and have `aql` installed locally. |
| **bash + curl** | API phase | Standard on macOS/Linux/CI. |

No `aql` or Aerospike Java client install is required — the default executor runs `aql` inside a
throwaway `docker run` container.

---

## Quick start

```bash
# 1. Point Heimdall at your services (edit endpoints / waits / namespace).
$EDITOR test-data/config.json

# 2. Build (first run downloads the Gradle distribution and provisions JDK 21).
make build

# 3. Run every scenario in the CSV manifest against the configured endpoints.
make run

# 4. Open the HTML report.
make report
```

Override the config or manifest:

```bash
make run CONFIG=path/to/config.json CSV=path/to/scenarios.csv
```

### Optional: bring up a local environment

The three CBS service images are placeholders (`__REPLACE_ME__`). Set real coordinates in
`docker/.env`, then:

```bash
make up      # docker compose up -d (Kafka + Aerospike + the cbs-* services)
make run
make down    # tears down and wipes volumes
```

---

## Make targets

| Target | Action |
|--------|--------|
| `make build` | `./gradlew clean build` (compiles all modules). |
| `make run` | Runs all scenarios; honours `CONFIG=` / `CSV=` overrides. |
| `make report` | Opens `build/heimdall-report/heimdall-report.html`. |
| `make up` / `make down` / `make logs` | Manage the optional docker-compose env. |
| `make clean` | Removes build output and the report. |

---

## The CSV manifest

`test-data/scenarios.csv` — one scenario per row:

```
ScenarioNumber,TableName,TopicName,input_json_file_name,aerospike_query_file_name,
aerospike_output_json_file_name,api_endpoint_files,api_output_json_file_name
```

| Column | Meaning |
|--------|---------|
| `ScenarioNumber` | Unique id; used in the report and correlation id. |
| `TableName` | Report label / HTML filter key. |
| `TopicName` | The input Kafka topic (`Kafka_topic_1`) for this scenario. |
| `input_json_file_name` | **Object ⇒ 1 message; JSON array ⇒ 1 message per element (prerequisites).** |
| `aerospike_query_file_name` | `.txt` holding one AQL query, run verbatim. |
| `aerospike_output_json_file_name` | Expected Aerospike result (golden). |
| `api_endpoint_files` | `.txt` holding **one** curl command. |
| `api_output_json_file_name` | Expected API response (golden). |

- File names are resolved under the directories in `config.json` → `testData`.
- **An empty cell skips that phase.** A phase must be specified fully (query **and** expected) or not
  at all.
- **Prerequisite arrays (D16):** when the input is a JSON array, every element is published in order,
  on the same partition, all-or-nothing — the downstream app emits the final Aerospike record only
  once all elements arrive. The query then typically returns one converged record.

### Test-data layout

```
test-data/
├─ config.json
├─ scenarios.csv
├─ input/                # input JSON (object or array)
├─ aql/                  # one AQL query per file
├─ api/                  # one curl command per file
└─ expected/
   ├─ aerospike/         # golden Aerospike records
   └─ api/               # golden API responses
```

---

## Configuration (`test-data/config.json`)

Profiles with `${ENV:default}` substitution. Key sections:

- `kafka.bootstrapServers` — `${KAFKA_BOOTSTRAP:localhost:9092}`.
- `wait.defaultMs` + `wait.perTable` — fixed wait before querying (no polling).
- `aerospike.executor` — `docker-run` (default), `docker-exec`, or `host`. On macOS the
  container-to-host address is `host.docker.internal`.
- `api.baseUrlEnv` — env var substituted into the curl command (default `API_BASE`).
- `validation.mode` — `lenient` (default) or `strict`; `validation.ignorePaths` are JSONPath
  expressions (e.g. `$..lastUpdatedTs`) ignored on both sides for dynamic fields.
- `report` — `csv` / `html` / `junitXml` toggles; `maskPii` + `piiFields` mask sensitive values
  across every output path.
- `cleanup` — end-of-run Aerospike truncate (namespace, or specific sets).

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | All scenarios passed. |
| `1` | At least one scenario FAILed/ERRORed (toggle: `execution.failExitCodeOnScenarioFailure`). |
| `2` | Framework/config error (bad config, unreadable CSV or referenced files). |

These are the exit codes of `HeimdallMain` itself. When launched via `make run` (i.e. through the
Gradle `runScenarios` task), Gradle treats any non-zero application exit as a build failure, so the
wrapper exits **0 on all-pass and non-zero on any failure** — the precise 1-vs-2 distinction is
observable only when running `HeimdallMain` directly. This is sufficient for CI stage gating; the
report and logs always distinguish FAIL from ERROR.

Reports are always written before exit.

---

## Reports

Written to `build/heimdall-report/`:

- **`heimdall-report.csv`** — one row per scenario with per-phase statuses.
- **`heimdall-report.html`** — a single self-contained file (no CDN): filter by table, sort columns,
  click a row to expand the failure bundle — published payload, AQL query + result, curl command +
  API response, and diffs.
- **`heimdall-report.xml`** — JUnit format for CI trend tooling.

PII fields (`piiFields`) are masked in every output and log line.

---

## Sample self-check

The shipped `test-data/` contains three scenarios that double as the framework's self-check:

| Scenario | Shape | Expected outcome |
|----------|-------|------------------|
| `HD_001` | single object | PASS (Kafka + Aerospike + API) |
| `HD_002` | array / prerequisites (2 elements) | PASS |
| `HD_003` | single object, **deliberately wrong** Aerospike golden | **FAIL at the AEROSPIKE phase** |

The data is **generic placeholder** content (real tables to onboard later: `INVM`, `INCT`,
`CUSVAA`). Against your real services you must align the golden files with what your pipeline
actually stores/returns; `HD_003` is intended to always fail and proves the framework reports
failures correctly.

---

## Architecture

A **single Gradle module** with **one package** `com.giftwrapper.heimdall` (~14 small, cohesive
classes). `HeimdallMain` wires the pieces directly:

```
HeimdallMain → Config(load) → ScenarioCsvLoader → ScenarioRunner
                                                     ├─ KafkaPublisher   (prerequisite fan-out)
                                                     ├─ Aql              (query / parse / cleanup, 3 modes)
                                                     ├─ Validator        (json-unit + ignorePaths)
                                                     └─ CurlApiExecutor  (body + 2xx)
                                                   → Reporters (CSV / HTML / JUnit-XML)
```

Shared helpers: `Json`, `PiiMasker`, `ProcessRunner`. The three AQL executor modes
(`docker-run`/`docker-exec`/`host`) are a switch in `Aql`; to extend behaviour (a new executor,
validator, or report format) edit the relevant class.

> **Testing note:** Heimdall *is* a test suite, so it ships with no automated tests of its own
> (D18). Its verification is this build plus the sample self-check above. See `TECH_DEBT.md` (T1/T2)
> for the "add a test suite later" trigger.
````

### `TECH_DEBT.md`
````markdown
# Heimdall — Tech Debt & Parked Decisions

This file tracks decisions we deliberately deferred and known debt to revisit later. It is the
companion to `IMPLEMENTATION_APPROACH.md` (the design source of truth). Nothing here blocks v1; each
item records **what**, **why it was parked**, and the **trigger** that should make us revisit it.

Status legend: `PARKED` (decided to defer) · `OPEN` (debt to address when convenient) · `PLACEHOLDER`
(stub shipped, needs real value).

---

## A. Testing debt

| ID | Item | Status | Why parked | Trigger to revisit |
|----|------|--------|------------|--------------------|
| T1 | **No automated test suite shipped.** The framework *is* an E2E test suite, so per decision (D18) no unit/integration tests are committed. Dev-time tests are removed before delivery. | OPEN | Avoid embedding tests inside a test framework; reduce clutter. | Long-term maintenance / multiple contributors / regressions slipping through the sample-scenario self-check. |
| T2 | If/when T1 is taken up, target coverage: CSV parsing edge cases; **array prerequisite fan-out** (order, same-partition, all-or-nothing); AQL JSON parsing (empty/single/array/quoting); validation modes (lenient/strict, ignore-paths, array-order, deep nesting); API body+2xx (non-2xx, timeout, malformed); cleanup idempotency; PII masking; config binding + `${ENV}`. | OPEN | — | Same as T1. |

> **Maintainability note:** without an automated suite, future refactors are not protected by
> regression tests. v1 mitigates via the sample scenario self-check (a known-pass + a deliberately
> failing scenario), the Validation/Review subagent gate, and `code-review`. Revisit if that proves
> insufficient.

---

## B. Parked / deferred decisions

| ID | Decision parked | Current v1 behaviour | Why parked | Trigger to revisit |
|----|-----------------|----------------------|------------|--------------------|
| P-01 | **Full E2E from Oracle + GoldenGate** | Inject at `Kafka_topic_1` only | GoldenGate/Oracle treated as trusted; faster, no Oracle infra | Need to validate the CDC/GoldenGate contract. Add an `OracleCdcInjector` (the `Injector` SPI already anticipates this). |
| P-02 | **Real service images in compose** | `__REPLACE_ME__` placeholders; you run against your own services | Images live elsewhere; Heimdall is a pure client | Want a fully self-contained `make up` env. Supply registry/name:tag in `docker/.env`. |
| P-03 | **Authentication** (Kafka SASL/SSL/Kerberos, Aerospike user/pass+TLS, API OAuth2/JWT/mTLS/API-key) | Plain/none; `AuthProvider` SPI present | "Keep it plain and simple for now" | Pointing at a secured/non-local environment. |
| P-04 | **Polling / await completion** | Fixed configurable wait (global + per-table) | Simpler; explicit | Fixed wait proves flaky/slow. Add a `CompletionStrategy` (poll-until-match) implementation. |
| P-05 | **Native Aerospike Java client query path** | AQL via `aql` CLI (`docker-run` default) | "Run the AQL verbatim" | Performance, or removing the `aql`/`docker` process dependency. Swap the `QueryExecutor` SPI. |
| P-06 | **JUnit-5-as-engine** for scenario execution | Plain sequential orchestrator loop | "Keep it simple for now" (your note) | Want native parallelism, per-scenario test isolation, richer JUnit XML. |
| P-07 | **Parallel scenario execution** | Sequential (`parallelism: 1`) | Same-key races under fixed-wait | Suites with guaranteed distinct keys; throughput needs. |
| P-08 | **Per-scenario cleanup** | End-of-run truncate of Aerospike ns/set(s) | Simpler; local env ephemeral | Long-lived/shared env where per-scenario isolation matters (needs a per-scenario delete key/AQL). |
| P-09 | **JSON Schema / API contract (OpenAPI) validation** | `validation.schema: false`; golden-file comparison only | Not required for v1 | Want structural/contract guarantees beyond example matching. |
| P-10 | **Strict validation as default** | Lenient default, configurable per profile | Start lenient, tighten later | Flip `validation.mode` to `strict` (and/or per-scenario strictness) when goldens stabilise. |
| P-11 | **Field mapping / transformation** (rename/transform fields between input and store) | Not implemented (dropped with entity descriptors) | Matching is golden-vs-actual JSON | Input and stored field naming diverge and need normalisation before compare. |
| P-12 | **Correlation-ID-based record matching** | `correlationId` used for logs/report only; matching relies on the AQL query content | AQL query already targets the record | Concurrent runs against shared data need precise per-run record matching. |
| P-13 | **Additional reporters** (Allure / ExtentReports / Elasticsearch) | CSV + single-file HTML + JUnit-XML behind `Reporter` SPI | Lightweight, GoCD-friendly | Want richer dashboards / centralised result storage. |
| P-14 | **GoCD pipeline + custom HTML tab** | Deferred; app is `make`-runnable; JUnit-XML emitted for later trends | "Worry about GoCD later" | Wiring Heimdall into the CI pipeline. |
| P-15 | **Observability exporters** (Micrometer→Prometheus/Grafana, Logback JSON→ELK/Splunk) | Timers + structured JSON logs in place; no exporters wired | Not needed locally | Centralised metrics/log aggregation required. |
| P-16 | **Heimdall-managed env lifecycle** (programmatic bring-up, e.g. Testcontainers) | Optional `docker-compose` via Makefile; Heimdall is a pure client | You run against your own local services | Want the app itself to provision/tear down infra. |
| P-17 | **Onboard real tables INVM / INCT / CUSVAA** | Generic placeholder test data | Real data/schemas not yet provided | Author real scenarios; replace `test-data/` placeholders. |
| P-18 | **Kafka message key / partitioning for prerequisites** | Same key per scenario (`keyField` JSONPath if configured, else `scenarioNumber`) to co-locate elements on one partition for ordering | The real cbs-denormaliser's partitioning/keying expectations are unknown | If the denormaliser keys on a business field, set `kafka.message.keyField` to match; verify ordering still holds at P2. |
| P-19 | **HTML report uses DataTables (inlined)** | Single self-contained HTML with a small amount of **vanilla JS** providing the same filter-by-table / sort / expandable-row behaviour; no bundled library, no CDN | Inlining DataTables+jQuery adds 100 KB+ to every report for functionality a few lines of vanilla JS already cover; keeps the file truly self-contained and matches the "keep it simple" preference | Want DataTables' richer features (pagination, multi-column search, export). Swap `HtmlReporter`'s template for an inlined DataTables bundle (the `Reporter` SPI makes this a drop-in). |

---

## C. Placeholders shipped in v1 (must be replaced before real runs)
- `docker/.env` / `docker-compose.yml` service images → `__REPLACE_ME__` (P-02).
- `test-data/` input/aql/expected files → generic placeholders (P-17).
- `config.json` endpoints → point at your real Kafka/Aerospike/API.

---

## D. Structure simplification (at user request, 2026-05-25)
The original 9-module / multi-package design was **collapsed to a single Gradle module and a single
package** (`com.giftwrapper.heimdall`, ~14 files). Removed as pure indirection: the `ServiceLoader`
SPIs + `Identified` + `SpiSelector` + `META-INF/services` (behaviour is now wired directly;
where earlier notes say "swap the SPI" — e.g. P-01, P-04, P-05, P-13 — read it as "edit/replace the
relevant class"); the 3 AQL executor classes (now a switch in `Aql`); `buildSrc` convention plugin
and the version catalog (deps inlined in `build.gradle.kts`); the 4 exception types (now one
`HeimdallException`); the no-op `AuthProvider`.

Also dropped two **unused** dependencies: `micrometer-core` (P-15 — latencies use `System.nanoTime`,
no metrics wired) and `networknt json-schema-validator` (P-09 — schema validation parked). Re-add
them if/when those parked items are taken up. No runtime behaviour changed.

---

_Last updated: 2026-05-25. Add new items as decisions are deferred during implementation._
````

