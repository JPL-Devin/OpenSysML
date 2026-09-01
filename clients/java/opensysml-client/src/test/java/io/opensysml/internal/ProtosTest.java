package io.opensysml.internal;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import io.opensysml.Instance;
import io.opensysml.TransportException;
import io.opensysml.Value;
import io.opensysml.proto.FeatureValue;
import io.opensysml.proto.ValueSequence;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.Test;

/** Reading the wire into the client's own types, including what a future service may send. */
class ProtosTest {

  @Test
  void aValueOfNoKindIsAbsentRatherThanGuessed() {
    assertEquals(Optional.empty(), Protos.value(io.opensysml.proto.Value.getDefaultInstance()));
  }

  @Test
  void aSequenceElementOfAKindTheClientCannotReadIsRefusedRatherThanDropped() {
    // A newer service sending a value kind this release has no arm for must not silently shorten
    // the sequence it belongs to.
    io.opensysml.proto.Value sequence =
        io.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(io.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addElements(io.opensysml.proto.Value.getDefaultInstance())
                    .addElements(io.opensysml.proto.Value.newBuilder().setIntValue(3)))
            .build();
    TransportException failed = assertThrows(TransportException.class, () -> Protos.value(sequence));
    assertTrue(failed.getMessage().contains("does not know"), failed.getMessage());
  }

  @Test
  void aFeatureValueOfAKindTheClientCannotReadIsRefusedRatherThanDropped() {
    io.opensysml.proto.Instance instance =
        io.opensysml.proto.Instance.newBuilder()
            .setId(1)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "wheels",
                FeatureValue.newBuilder()
                    .setFeatureName("wheels")
                    .addValues(io.opensysml.proto.Value.newBuilder().setInstanceId(2))
                    .addValues(io.opensysml.proto.Value.getDefaultInstance())
                    .setMaterialized(true)
                    .build())
            .build();
    assertThrows(TransportException.class, () -> Protos.instance(instance));
  }

  @Test
  void aSequenceOfReadableElementsKeepsItsOrderAndLength() {
    io.opensysml.proto.Value sequence =
        io.opensysml.proto.Value.newBuilder()
            .setSequence(
                ValueSequence.newBuilder()
                    .addElements(io.opensysml.proto.Value.newBuilder().setIntValue(1))
                    .addElements(io.opensysml.proto.Value.newBuilder().setUnset(true))
                    .addElements(io.opensysml.proto.Value.newBuilder().setStringValue("x")))
            .build();
    assertEquals(
        Optional.of(
            new Value.Sequence(
                List.of(new Value.IntegerValue(1), new Value.UnsetValue(), new Value.StringValue("x")))),
        Protos.value(sequence));
  }

  @Test
  void anInstanceReadsItsFeatureValuesWholeAndImmutably() {
    io.opensysml.proto.Instance instance =
        io.opensysml.proto.Instance.newBuilder()
            .setId(7)
            .setTypeSymbolId("Demo::Vehicle")
            .putFeatureValues(
                "mass",
                FeatureValue.newBuilder()
                    .setFeatureName("mass")
                    .setValue(io.opensysml.proto.Value.newBuilder().setRealValue(1200.0))
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
