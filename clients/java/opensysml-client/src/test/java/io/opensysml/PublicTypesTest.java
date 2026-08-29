package io.opensysml;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InvalidObjectException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Test;

/** The public value types: immutable, comparable by value, and free of generated types. */
class PublicTypesTest {

  @Test
  void aSequenceCopiesWhatItWasGiven() {
    List<Value> elements = new ArrayList<>(List.of(new Value.IntegerValue(1)));
    Value.Sequence sequence = new Value.Sequence(elements);
    elements.add(new Value.IntegerValue(2));
    List<Value> copied = sequence.elements();
    assertEquals(1, copied.size());
    assertThrows(UnsupportedOperationException.class, () -> copied.add(new Value.NullValue()));
  }

  @Test
  void valuesCompareByValue() {
    assertEquals(new Value.RealValue(1.5), new Value.RealValue(1.5));
    assertEquals(
        new Value.EnumerationValue(new EnumLiteral("D::Color::red", "D::Color", "Color::red")),
        new Value.EnumerationValue(new EnumLiteral("D::Color::red", "D::Color", "Color::red")));
    assertNotEquals(new Value.IntegerValue(1), new Value.RealValue(1.0));
  }

  @Test
  void anUnsetValueIsNotTheModelsNull() {
    assertNotEquals(new Value.UnsetValue(), new Value.NullValue());
  }

  @Test
  void aQuantityKeepsIntegerAndRealApart() {
    Quantity integral = new Quantity(5L, Optional.of("kg"), Optional.empty());
    Quantity real = new Quantity(5.0, Optional.of("kg"), Optional.empty());
    assertTrue(integral.isIntegral());
    assertFalse(real.isIntegral());
    assertNotEquals(integral, real);
  }

  @Test
  void anInstanceCopiesItsFeatureValues() {
    Map<String, Instance.FeatureValue> featureValues = new java.util.LinkedHashMap<>();
    featureValues.put(
        "mass",
        new Instance.FeatureValue(
            "mass",
            Optional.of(new Value.RealValue(1500.0)),
            List.of(),
            true,
            Optional.empty()));
    Instance instance = new Instance(1, "Demo::Vehicle", featureValues);
    featureValues.clear();
    Map<String, Instance.FeatureValue> copied = instance.featureValues();
    assertEquals(1, copied.size());
    assertThrows(UnsupportedOperationException.class, copied::clear);
  }

  @Test
  void anInstantiationResolvesReferences() {
    Instance root = new Instance(1, "Demo::Vehicle", Map.of());
    Instance engine = new Instance(2, "Demo::Engine", Map.of());
    Instantiation instantiation = new Instantiation(root, List.of(root, engine), List.of());
    assertEquals(
        Optional.of(engine), instantiation.resolve(new Value.InstanceReference(2)));
    assertEquals(Optional.empty(), instantiation.resolve(new Value.InstanceReference(3)));
  }

  @Test
  void diagnosticSeverityReadsTheWireName() {
    assertEquals(Diagnostic.Severity.ERROR, Diagnostic.Severity.fromWireName("error"));
    assertEquals(Diagnostic.Severity.WARNING, Diagnostic.Severity.fromWireName("warning"));
    assertEquals(Diagnostic.Severity.INFO, Diagnostic.Severity.fromWireName("info"));
    assertEquals(Diagnostic.Severity.UNKNOWN, Diagnostic.Severity.fromWireName("hint"));
    assertEquals(Diagnostic.Severity.UNKNOWN, Diagnostic.Severity.fromWireName(""));
  }

  @Test
  void aModelExceptionPreservesDiagnosticsWhenSerialized() throws Exception {
    List<Diagnostic> diagnostics =
        List.of(
            new Diagnostic(
                Diagnostic.Severity.ERROR,
                "invalid model",
                Optional.of(new Diagnostic.Span("model.sysml", 2, 3, 2, 8))),
            new Diagnostic(Diagnostic.Severity.WARNING, "unlocated", Optional.empty()));
    ModelException original = new ModelException("rejected", diagnostics);

    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (ObjectOutputStream output = new ObjectOutputStream(bytes)) {
      output.writeObject(original);
    }
    try (ObjectInputStream input =
        new ObjectInputStream(new ByteArrayInputStream(bytes.toByteArray()))) {
      ModelException restored = (ModelException) input.readObject();
      assertEquals(original.getMessage(), restored.getMessage());
      List<Diagnostic> restoredDiagnostics = restored.diagnostics();
      Diagnostic first = diagnostics.get(0);
      assertEquals(diagnostics, restoredDiagnostics);
      assertThrows(UnsupportedOperationException.class, () -> restoredDiagnostics.add(first));
    }
  }

  @Test
  void aModelExceptionRejectsExcessiveSerializedDiagnostics() throws Exception {
    ByteArrayOutputStream bytes = new ByteArrayOutputStream();
    try (ObjectOutputStream output = new ExcessiveDiagnosticCountStream(bytes)) {
      output.writeObject(new ModelException("rejected", List.of()));
    }

    try (ObjectInputStream input =
        new ObjectInputStream(new ByteArrayInputStream(bytes.toByteArray()))) {
      InvalidObjectException failure =
          assertThrows(InvalidObjectException.class, input::readObject);
      assertEquals("too many diagnostics", failure.getMessage());
    }
  }

  private static final class ExcessiveDiagnosticCountStream extends ObjectOutputStream {
    private boolean replaceNextInt;

    ExcessiveDiagnosticCountStream(ByteArrayOutputStream output) throws IOException {
      super(output);
    }

    @Override
    public void defaultWriteObject() throws IOException {
      super.defaultWriteObject();
      replaceNextInt = true;
    }

    @Override
    public void writeInt(int value) throws IOException {
      if (replaceNextInt) {
        super.writeInt(Integer.MAX_VALUE);
        replaceNextInt = false;
      } else {
        super.writeInt(value);
      }
    }
  }

  @Test
  void capabilitiesNegotiateOnNames() {
    Capabilities capabilities =
        new Capabilities("dev", java.util.Set.of(Capabilities.QUERY, Capabilities.TYPE_FACTS));
    assertTrue(capabilities.has(Capabilities.QUERY));
    assertFalse(capabilities.has(Capabilities.CONVERT));
    capabilities.require(Capabilities.TYPE_FACTS);
    CapabilityException refused =
        assertThrows(CapabilityException.class, () -> capabilities.require(Capabilities.CONVERT));
    assertEquals(Capabilities.CONVERT, refused.capability());
    java.util.Set<String> names = capabilities.names();
    assertThrows(UnsupportedOperationException.class, () -> names.add("invented"));
  }

  @Test
  void optionsRejectWhatWouldFailLater() {
    ConnectionOptions.Builder builder = ConnectionOptions.builder();
    assertThrows(IllegalArgumentException.class, () -> builder.service("localhost", 0));
    assertThrows(IllegalArgumentException.class, () -> builder.expectedBinarySha256("not-a-digest"));
    assertThrows(
        IllegalArgumentException.class, () -> builder.requestTimeout(java.time.Duration.ZERO));
    ConnectionOptions options = ConnectionOptions.defaults();
    assertEquals(Encoding.PROTOBUF, options.encoding());
    assertTrue(options.autoStart());
    assertFalse(options.isolatedService());
    assertEquals(Optional.empty(), options.host());
  }

  @Test
  void refusingToStartWithoutAServiceIsReportedAtOpen() {
    ConnectionOptions options = ConnectionOptions.builder().autoStart(false).build();
    if (System.getenv(ConnectionOptions.SERVICE_ENV) != null) {
      return; // an external service is named in this environment, so opening would succeed
    }
    ServiceStartException refused =
        assertThrows(ServiceStartException.class, () -> Connection.open(options));
    assertTrue(refused.getMessage().contains("autoStart"));
  }
}
