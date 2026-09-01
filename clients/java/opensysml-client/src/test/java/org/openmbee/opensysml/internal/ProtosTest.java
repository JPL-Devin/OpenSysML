package org.openmbee.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.openmbee.opensysml.Instance;
import org.openmbee.opensysml.TransportException;
import org.openmbee.opensysml.Value;
import org.openmbee.opensysml.proto.AttributeInfo;
import org.openmbee.opensysml.proto.FeatureValue;
import org.openmbee.opensysml.proto.SymbolInfo;
import org.openmbee.opensysml.proto.ValueSequence;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.Test;

/** Reading the wire into the client's own types, including what a future service may send. */
class ProtosTest {

  @Test
  void aValueOfNoKindIsAbsentRatherThanGuessed() {
    assertEquals(Optional.empty(), Protos.value(org.openmbee.opensysml.proto.Value.getDefaultInstance()));
  }

  @Test
  void aSequenceElementOfAKindTheClientCannotReadIsRefusedRatherThanDropped() {
    // A newer service sending a value kind this release has no arm for must not silently shorten
    // the sequence it belongs to.
    org.openmbee.opensysml.proto.Value sequence =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addElements(org.openmbee.opensysml.proto.Value.getDefaultInstance())
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(3)))
            .build();
    TransportException failed = assertThrows(TransportException.class, () -> Protos.value(sequence));
    assertTrue(failed.getMessage().contains("does not know"), failed.getMessage());
  }

  @Test
  void aFeatureValueOfAKindTheClientCannotReadIsRefusedRatherThanDropped() {
    org.openmbee.opensysml.proto.Instance instance =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(1)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "wheels",
                FeatureValue.newBuilder()
                    .setFeatureName("wheels")
                    .addValues(org.openmbee.opensysml.proto.Value.newBuilder().setInstanceId(2))
                    .addValues(org.openmbee.opensysml.proto.Value.getDefaultInstance())
                    .setMaterialized(true)
                    .build())
            .build();
    assertThrows(TransportException.class, () -> Protos.instance(instance));
  }

  @Test
  void aSingularValueOfAKindTheClientCannotReadIsRefusedRatherThanReadAsNoValue() {
    org.openmbee.opensysml.proto.Instance instance =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(1)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "mass",
                FeatureValue.newBuilder()
                    .setFeatureName("mass")
                    .setValue(org.openmbee.opensysml.proto.Value.getDefaultInstance())
                    .setMaterialized(true)
                    .build())
            .build();
    assertThrows(TransportException.class, () -> Protos.instance(instance));

    SymbolInfo symbol =
        SymbolInfo.newBuilder()
            .setId("Demo::Vehicle")
            .setName("Vehicle")
            .setKind("partDef")
            .addAttributes(
                AttributeInfo.newBuilder()
                    .setName("mass")
                    .setValue(org.openmbee.opensysml.proto.Value.getDefaultInstance()))
            .build();
    assertThrows(TransportException.class, () -> Protos.symbol(symbol));
  }

  @Test
  void anAttributeThatWasSentNoValueHoldsNone() {
    SymbolInfo symbol =
        SymbolInfo.newBuilder()
            .setId("Demo::Vehicle")
            .setName("Vehicle")
            .setKind("partDef")
            .addAttributes(AttributeInfo.newBuilder().setName("mass").setType("Real"))
            .build();
    assertEquals(Optional.empty(), Protos.symbol(symbol).attributes().get(0).value());
  }

  @Test
  void aSequenceOfReadableElementsKeepsItsOrderAndLength() {
    org.openmbee.opensysml.proto.Value sequence =
        org.openmbee.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setUnset(true))
                    .addElements(org.openmbee.opensysml.proto.Value.newBuilder().setStringValue("x")))
            .build();
    assertEquals(
        Optional.of(
            new Value.Sequence(
                List.of(new Value.IntegerValue(1), new Value.UnsetValue(), new Value.StringValue("x")))),
        Protos.value(sequence));
  }

  @Test
  void anInstanceReadsItsFeatureValuesWholeAndImmutably() {
    org.openmbee.opensysml.proto.Instance instance =
        org.openmbee.opensysml.proto.Instance.newBuilder()
            .setId(7)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "mass",
                FeatureValue.newBuilder()
                    .setFeatureName("mass")
                    .setValue(org.openmbee.opensysml.proto.Value.newBuilder().setRealValue(1200.0))
                    .setMaterialized(true)
                    .build())
            .build();
    Instance read = Protos.instance(instance);
    assertEquals(7, read.id());
    Instance.FeatureValue mass = read.featureValues().get("mass");
    assertEquals(Optional.of(new Value.RealValue(1200.0)), mass.value());
    assertEquals(List.of(), mass.values());
    assertTrue(mass.materialized());
    assertEquals(Optional.empty(), mass.error());
  }
}
