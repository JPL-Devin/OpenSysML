package io.opensysml.internal;

import io.opensysml.Diagnostic;
import io.opensysml.EnumLiteral;
import io.opensysml.Instance;
import io.opensysml.Instantiation;
import io.opensysml.Quantity;
import io.opensysml.Symbol;
import io.opensysml.TransportException;
import io.opensysml.Value;
import io.opensysml.proto.AttributeInfo;
import io.opensysml.proto.FeatureValue;
import io.opensysml.proto.InstantiateResponse;
import io.opensysml.proto.MultiplicityInfo;
import io.opensysml.proto.Span;
import io.opensysml.proto.Specialization;
import io.opensysml.proto.SymbolInfo;
import io.opensysml.proto.TypeInfo;
import io.opensysml.proto.UnitFactor;
import io.opensysml.proto.UnitTerm;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * Reads the generated messages into the client's own immutable types, so no generated class and no
 * builder reaches a caller.
 */
public final class Protos {

  private Protos() {}

  /**
   * A value, absent when the message names no kind.
   *
   * @param value the generated value
   * @return the value, or empty when no arm is set
   */
  public static Optional<Value> value(io.opensysml.proto.Value value) {
    return switch (value.getKindCase()) {
      case INT_VALUE -> Optional.of(new Value.IntegerValue(value.getIntValue()));
      case REAL_VALUE -> Optional.of(new Value.RealValue(value.getRealValue()));
      case BOOL_VALUE -> Optional.of(new Value.BooleanValue(value.getBoolValue()));
      case STRING_VALUE -> Optional.of(new Value.StringValue(value.getStringValue()));
      case INSTANCE_ID -> Optional.of(new Value.InstanceReference(value.getInstanceId()));
      case SEQUENCE -> Optional.of(sequence(value));
      case NULL -> Optional.of(new Value.NullValue());
      case QUANTITY -> Optional.of(new Value.QuantityValue(quantity(value.getQuantity())));
      case ENUM_LITERAL -> Optional.of(new Value.EnumerationValue(literal(value.getEnumLiteral())));
      case UNSET -> Optional.of(new Value.UnsetValue());
      case KIND_NOT_SET -> Optional.empty();
    };
  }

  private static Value sequence(io.opensysml.proto.Value value) {
    List<Value> elements = new ArrayList<>();
    for (io.opensysml.proto.Value element : value.getSequence().getElementsList()) {
      elements.add(readable(element));
    }
    return new Value.Sequence(elements);
  }

  /**
   * A value of a collection the client must read whole: dropping an element it cannot read would
   * hand back a shorter sequence as if the service had sent one.
   */
  private static Value readable(io.opensysml.proto.Value value) {
    return value(value)
        .orElseThrow(
            () ->
                new TransportException(
                    "the service answered a value of a kind this client does not know", null));
  }

  /**
   * A quantity.
   *
   * @param quantity the generated quantity
   * @return the immutable quantity
   */
  public static Quantity quantity(io.opensysml.proto.Quantity quantity) {
    Number magnitude =
        quantity.getMagnitudeCase() == io.opensysml.proto.Quantity.MagnitudeCase.INT_MAGNITUDE
            ? Long.valueOf(quantity.getIntMagnitude())
            : Double.valueOf(quantity.getRealMagnitude());
    Optional<Quantity.UnitTerm> reduction =
        quantity.hasUnitTerm() ? Optional.of(unitTerm(quantity.getUnitTerm())) : Optional.empty();
    return new Quantity(magnitude, present(quantity.getUnit()), reduction);
  }

  private static Quantity.UnitTerm unitTerm(UnitTerm term) {
    List<Quantity.UnitFactor> factors = new ArrayList<>();
    for (UnitFactor factor : term.getFactorsList()) {
      factors.add(new Quantity.UnitFactor(factor.getUnitId(), factor.getExponent()));
    }
    return new Quantity.UnitTerm(term.getScaleNum(), term.getScaleDen(), factors);
  }

  /**
   * An enumeration literal.
   *
   * @param literal the generated literal
   * @return the immutable literal
   */
  public static EnumLiteral literal(io.opensysml.proto.EnumLiteral literal) {
    return new EnumLiteral(
        literal.getLiteralId(), literal.getEnumerationId(), literal.getName());
  }

  /**
   * Diagnostics.
   *
   * @param diagnostics the generated diagnostics
   * @return immutable diagnostics, in order
   */
  public static List<Diagnostic> diagnostics(
      List<io.opensysml.proto.Diagnostic> diagnostics) {
    List<Diagnostic> read = new ArrayList<>(diagnostics.size());
    for (io.opensysml.proto.Diagnostic diagnostic : diagnostics) {
      read.add(
          new Diagnostic(
              Diagnostic.Severity.fromWireName(diagnostic.getSeverity()),
              diagnostic.getMessage(),
              diagnostic.hasSpan() ? Optional.of(span(diagnostic.getSpan())) : Optional.empty()));
    }
    return List.copyOf(read);
  }

  private static Diagnostic.Span span(Span span) {
    return new Diagnostic.Span(
        span.getFile(),
        span.getStartLine(),
        span.getStartCol(),
        span.getEndLine(),
        span.getEndCol());
  }

  /**
   * A symbol.
   *
   * @param symbol the generated symbol
   * @return the immutable symbol
   */
  public static Symbol symbol(SymbolInfo symbol) {
    List<Symbol.Attribute> attributes = new ArrayList<>(symbol.getAttributesCount());
    for (AttributeInfo attribute : symbol.getAttributesList()) {
      attributes.add(
          new Symbol.Attribute(
              attribute.getName(),
              attribute.getType(),
              attribute.hasValue() ? value(attribute.getValue()) : Optional.empty(),
              present(attribute.getUnit())));
    }
    List<Symbol.Specialization> specializations = new ArrayList<>(symbol.getSpecializationsCount());
    for (Specialization specialization : symbol.getSpecializationsList()) {
      specializations.add(
          new Symbol.Specialization(
              specialization.getKind(),
              specialization.getDeclared(),
              present(specialization.getTargetId()),
              present(specialization.getTargetKind())));
    }
    return new Symbol(
        symbol.getId(),
        symbol.getName(),
        symbol.getKind(),
        symbol.getMetadataMap(),
        symbol.getChildIdsList(),
        attributes,
        symbol.hasTypeInfo() ? Optional.of(typeFacts(symbol.getTypeInfo())) : Optional.empty(),
        symbol.hasMultiplicity()
            ? Optional.of(multiplicity(symbol.getMultiplicity()))
            : Optional.empty(),
        specializations,
        symbol.getWithheldLibraryAttributes());
  }

  private static Symbol.TypeFacts typeFacts(TypeInfo typeInfo) {
    return new Symbol.TypeFacts(
        present(typeInfo.getDeclared()),
        present(typeInfo.getResolvedId()),
        present(typeInfo.getResolvedKind()),
        present(typeInfo.getPrimitive()),
        present(typeInfo.getPrimitiveSource()),
        typeInfo.getQuantity(),
        present(typeInfo.getUnit()));
  }

  private static Symbol.Multiplicity multiplicity(MultiplicityInfo multiplicity) {
    return new Symbol.Multiplicity(
        present(multiplicity.getLower()), present(multiplicity.getUpper()));
  }

  /**
   * An instance.
   *
   * @param instance the generated instance
   * @return the immutable instance
   */
  public static Instance instance(io.opensysml.proto.Instance instance) {
    Map<String, Instance.FeatureValue> featureValues = new LinkedHashMap<>();
    for (Map.Entry<String, FeatureValue> entry : instance.getFeatureValuesMap().entrySet()) {
      FeatureValue featureValue = entry.getValue();
      List<Value> values = new ArrayList<>(featureValue.getValuesCount());
      for (io.opensysml.proto.Value each : featureValue.getValuesList()) {
        values.add(readable(each));
      }
      featureValues.put(
          entry.getKey(),
          new Instance.FeatureValue(
              featureValue.getFeatureName(),
              featureValue.hasValue() ? value(featureValue.getValue()) : Optional.empty(),
              values,
              featureValue.getMaterialized(),
              present(featureValue.getError())));
    }
    return new Instance(instance.getId(), instance.getTypeSymbolId(), featureValues);
  }

  /**
   * What an instantiation built.
   *
   * @param response the generated answer
   * @return the immutable instantiation
   */
  public static Instantiation instantiation(InstantiateResponse response) {
    List<Instance> reachable = new ArrayList<>(response.getInstancesCount());
    for (io.opensysml.proto.Instance instance : response.getInstancesList()) {
      reachable.add(instance(instance));
    }
    return new Instantiation(
        instance(response.getInstance()), reachable, diagnostics(response.getDiagnosticsList()));
  }

  /**
   * A string field, absent when it holds its default.
   *
   * @param field the field value
   * @return the string, or empty when it is empty
   */
  public static Optional<String> present(String field) {
    return field.isEmpty() ? Optional.empty() : Optional.of(field);
  }
}
