package com.cbs.utils.correlation;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Instant;
import java.util.Map;
import java.util.Objects;
import java.util.TreeMap;

/**
 * Generates a deterministic, opaque correlationId from a set of named business attributes.
 *
 * <p>The same attributes always produce the same id on every JVM, service and library version, so
 * the value can be attached as a Jaeger tag, an OpenTelemetry baggage entry and a log MDC field to
 * correlate everything that happened to one logical message across asynchronous services.
 *
 * <p><b>Determinism guarantees</b>
 * <ul>
 *   <li>Attribute <b>order is irrelevant</b> — entries are sorted by name before hashing.</li>
 *   <li>Values are canonicalized <b>by logical value, not Java type</b>: {@code 12L}, {@code 12},
 *       {@code "12"} and {@code new BigDecimal("12.0")} all encode to {@code "12"}, so a field
 *       deserialized as a number in one service and a string in another still agrees.</li>
 *   <li>{@code null}, the empty string and an <b>absent</b> field are three distinct encodings.</li>
 *   <li>Length-prefixed framing removes composite-key ambiguity: {@code acct="12",branch="3"}
 *       never collides with {@code acct="1",branch="23"}.</li>
 * </ul>
 *
 * <p>Output is the first 128 bits of a SHA-256 digest as 32 lowercase hex chars. Hashing also keeps
 * PII (account numbers, etc.) out of traces and logs. A leading version byte lets the algorithm
 * evolve deliberately (bump {@link #VERSION}, regenerate golden vectors, realign all services).
 *
 * <p><b>Thread-safety / performance:</b> instances are not thread-safe — create one per call (cheap).
 * The digest is held per-thread. Cost is sub-microsecond and negligible next to Kafka
 * deserialization, so it will not be a bottleneck on a consume path.
 */
public final class CorrelationId {

    /** Algorithm version. Bump on ANY encoding change; it invalidates all previously computed ids. */
    static final byte VERSION = 1;

    private static final char[] HEX = "0123456789abcdef".toCharArray();

    private static final ThreadLocal<MessageDigest> SHA256 = ThreadLocal.withInitial(() -> {
        try {
            return MessageDigest.getInstance("SHA-256");
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is required but unavailable", e);
        }
    });

    /** Sorted by name => output is independent of insertion order. null value => null bytes. */
    private final TreeMap<String, byte[]> fields = new TreeMap<>();

    private CorrelationId() {}

    public static CorrelationId builder() {
        return new CorrelationId();
    }

    /**
     * Adds a named attribute. Value may be {@code null}. Supported value types: {@link CharSequence},
     * integral {@link Number}s, {@link BigDecimal}, {@link Boolean}, {@link Instant}.
     *
     * @throws IllegalArgumentException on an empty/duplicate name or an unsupported value type
     */
    public CorrelationId with(String name, Object value) {
        Objects.requireNonNull(name, "attribute name");
        if (name.isEmpty()) {
            throw new IllegalArgumentException("attribute name must not be empty");
        }
        if (fields.containsKey(name)) {
            throw new IllegalArgumentException("duplicate attribute name: " + name);
        }
        fields.put(name, canonicalBytes(value));
        return this;
    }

    /** Convenience: build directly from a map of attributes (entry order irrelevant). */
    public static String of(Map<String, ?> attributes) {
        Objects.requireNonNull(attributes, "attributes");
        CorrelationId b = builder();
        for (Map.Entry<String, ?> e : attributes.entrySet()) {
            b.with(e.getKey(), e.getValue());
        }
        return b.build();
    }

    /**
     * Computes the 32-char lowercase-hex correlationId.
     *
     * @throws IllegalStateException if no attributes were added
     */
    public String build() {
        if (fields.isEmpty()) {
            throw new IllegalStateException("at least one attribute is required");
        }

        // First pass: compute exact size so we allocate the backing buffer once (no resizing).
        int size = 1; // version byte
        for (Map.Entry<String, byte[]> e : fields.entrySet()) {
            byte[] name = e.getKey().getBytes(StandardCharsets.UTF_8);
            byte[] val = e.getValue();
            size += 4 + name.length;          // length-prefixed name
            size += 1;                        // null/present tag
            if (val != null) {
                size += 4 + val.length;       // length-prefixed value
            }
        }

        ByteBuffer buf = ByteBuffer.allocate(size); // ByteBuffer is big-endian => stable across CPUs
        buf.put(VERSION);
        for (Map.Entry<String, byte[]> e : fields.entrySet()) {
            byte[] name = e.getKey().getBytes(StandardCharsets.UTF_8);
            byte[] val = e.getValue();
            buf.putInt(name.length);
            buf.put(name);
            if (val == null) {
                buf.put((byte) 0);            // null
            } else {
                buf.put((byte) 1);            // present (empty string => present, length 0)
                buf.putInt(val.length);
                buf.put(val);
            }
        }

        MessageDigest md = SHA256.get();
        md.reset();
        byte[] digest = md.digest(buf.array());
        return hex(digest, 16); // first 128 bits
    }

    /** Current algorithm version, e.g. for emitting alongside the id or asserting in tests. */
    public static int algorithmVersion() {
        return VERSION;
    }

    // --- canonicalization ---

    private static byte[] canonicalBytes(Object value) {
        if (value == null) {
            return null;
        }
        String canonical;
        if (value instanceof CharSequence) {
            canonical = value.toString();
        } else if (value instanceof BigDecimal) {
            canonical = canonicalDecimal((BigDecimal) value);
        } else if (value instanceof Byte || value instanceof Short
                || value instanceof Integer || value instanceof Long
                || value instanceof BigInteger) {
            canonical = value.toString();
        } else if (value instanceof Boolean) {
            canonical = value.toString();
        } else if (value instanceof Instant) {
            canonical = value.toString(); // ISO-8601 UTC, e.g. 2026-06-08T10:15:30Z
        } else {
            // Reject types whose toString() is not guaranteed identical across services
            // (Double/Float/Date/enums/custom objects). Force the caller to pass a canonical String.
            throw new IllegalArgumentException(
                "unsupported attribute type " + value.getClass().getName()
                + "; pass a canonical String");
        }
        return canonical.getBytes(StandardCharsets.UTF_8);
    }

    private static String canonicalDecimal(BigDecimal d) {
        if (d.signum() == 0) {
            return "0"; // sidestep BigDecimal.ZERO.stripTrailingZeros() representation quirks
        }
        return d.stripTrailingZeros().toPlainString();
    }

    private static String hex(byte[] b, int len) {
        char[] out = new char[len * 2];
        for (int i = 0; i < len; i++) {
            int v = b[i] & 0xFF;
            out[i * 2] = HEX[v >>> 4];
            out[i * 2 + 1] = HEX[v & 0x0F];
        }
        return new String(out);
    }
}
