package com.cbs.utils.correlation;

import org.junit.jupiter.api.Test;

import java.math.BigDecimal;
import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class CorrelationIdTest {

    @Test
    void deterministic_sameInputSameOutput() {
        String a = CorrelationId.builder().with("acct", "12").with("branch", "3").build();
        String b = CorrelationId.builder().with("acct", "12").with("branch", "3").build();
        assertEquals(a, b);
    }

    @Test
    void orderIndependent() {
        String a = CorrelationId.builder().with("acct", "12").with("branch", "3").build();
        String b = CorrelationId.builder().with("branch", "3").with("acct", "12").build();
        assertEquals(a, b);
    }

    @Test
    void mapInputMatchesBuilder() {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("branch", "3");
        m.put("acct", "12");
        assertEquals(
            CorrelationId.builder().with("acct", "12").with("branch", "3").build(),
            CorrelationId.of(m));
    }

    @Test
    void compositeKeyNotAmbiguous() {
        // acct="12",branch="3"  must NOT collide with  acct="1",branch="23"
        String a = CorrelationId.builder().with("acct", "12").with("branch", "3").build();
        String b = CorrelationId.builder().with("acct", "1").with("branch", "23").build();
        assertNotEquals(a, b);
    }

    @Test
    void nullEmptyAbsentAllDiffer() {
        String nullVal  = CorrelationId.builder().with("x", (Object) null).with("y", "1").build();
        String emptyVal = CorrelationId.builder().with("x", "").with("y", "1").build();
        String absent   = CorrelationId.builder().with("y", "1").build();
        assertNotEquals(nullVal, emptyVal);
        assertNotEquals(nullVal, absent);
        assertNotEquals(emptyVal, absent);
    }

    @Test
    void canonicalValueIsTypeAgnostic() {
        String asString  = CorrelationId.builder().with("acct", "12").build();
        String asLong    = CorrelationId.builder().with("acct", 12L).build();
        String asInt     = CorrelationId.builder().with("acct", 12).build();
        String asDecimal = CorrelationId.builder().with("acct", new BigDecimal("12.0")).build();
        assertEquals(asString, asLong);
        assertEquals(asString, asInt);
        assertEquals(asString, asDecimal);
    }

    @Test
    void decimalZeroIsCanonical() {
        String z1 = CorrelationId.builder().with("amt", new BigDecimal("0.00")).build();
        String z2 = CorrelationId.builder().with("amt", BigDecimal.ZERO).build();
        String z3 = CorrelationId.builder().with("amt", "0").build();
        assertEquals(z1, z2);
        assertEquals(z1, z3);
    }

    @Test
    void differentValuesProduceDifferentIds() {
        assertNotEquals(
            CorrelationId.builder().with("acct", "12").build(),
            CorrelationId.builder().with("acct", "13").build());
    }

    @Test
    void outputIs32LowercaseHex() {
        String id = CorrelationId.builder().with("acct", "12").build();
        assertEquals(32, id.length());
        assertTrue(id.matches("[0-9a-f]{32}"), id);
    }

    @Test
    void duplicateNameRejected() {
        assertThrows(IllegalArgumentException.class,
            () -> CorrelationId.builder().with("acct", "1").with("acct", "2"));
    }

    @Test
    void emptyNameRejected() {
        assertThrows(IllegalArgumentException.class,
            () -> CorrelationId.builder().with("", "1"));
    }

    @Test
    void unsupportedTypeRejected() {
        assertThrows(IllegalArgumentException.class,
            () -> CorrelationId.builder().with("x", new Object()).build());
    }

    @Test
    void atLeastOneAttributeRequired() {
        assertThrows(IllegalStateException.class, () -> CorrelationId.builder().build());
    }

    /**
     * GOLDEN VECTOR — frozen output for a fixed input. If this assertion ever fails, the algorithm
     * (encoding, normalization, hash or version byte) changed and EVERY service must realign to the
     * new cbs-utils version. This is the guardrail that guarantees the id is identical everywhere.
     */
    @Test
    void goldenVector_pinsAlgorithm() {
        String id = CorrelationId.builder()
            .with("tableName", "INVM")
            .with("primaryKey", "1001")
            .with("scn", 99887766L)
            .build();
        assertEquals("2dafe546f98a1591816771460ea4604c", id);
        assertEquals(1, CorrelationId.algorithmVersion());
    }
}
