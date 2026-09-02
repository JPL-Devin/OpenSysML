package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * A value the service evaluated: an immutable variant of {@code sysml.Value}.
 *
 * <p>The arms are exhaustive and sealed, so a {@code switch} over them needs no default that
 * silently swallows a value kind added later — a new arm becomes a compile error instead.
 */
public sealed interface Value {

  /** An {@code Integer} value. */
  record IntegerValue(long value) implements Value {}

  /** A {@code Real} value. */
  record RealValue(double value) implements Value {}

  /** A {@code Boolean} value. */
  record BooleanValue(boolean value) implements Value {}

  /** A {@code String} value. */
  record StringValue(String value) implements Value {
    /**
     * Creates a string value.
     *
     * @param value the string, never {@code null}
     */
    public StringValue {
      Objects.requireNonNull(value, "value");
    }
  }

  /** The null value: a feature that resolved to nothing. */
  record NullValue() implements Value {}

  /**
   * A materialized feature of a value type that holds no value.
   *
   * <p>Only a service advertising the {@code unset_value} capability distinguishes it from an
   * empty object.
   */
  record UnsetValue() implements Value {}

  /**
   * A reference to a runtime instance, by the id the service assigned it.
   *
   * @param instanceId id of the referenced instance
   */
  record InstanceReference(long instanceId) implements Value {}

  /**
   * A sequence of values.
   *
   * @param elements the elements, in order
   */
  record Sequence(List<Value> elements) implements Value {
    /**
     * Creates a sequence, copying the elements.
     *
     * @param elements the elements, never {@code null}
     */
    public Sequence {
      elements = List.copyOf(elements);
    }
  }

  /**
   * A magnitude with the unit it is expressed in.
   *
   * @param quantity the quantity
   */
  record QuantityValue(Quantity quantity) implements Value {
    /**
     * Creates a quantity value.
     *
     * @param quantity the quantity, never {@code null}
     */
    public QuantityValue {
      Objects.requireNonNull(quantity, "quantity");
    }
  }

  /**
   * One literal of an enumeration definition.
   *
   * <p>Only a service advertising the {@code enum_values} capability reports a literal as itself
   * rather than as {@link NullValue}.
   *
   * @param literal the literal
   */
  record EnumerationValue(EnumLiteral literal) implements Value {
    /**
     * Creates an enumeration value.
     *
     * @param literal the literal, never {@code null}
     */
    public EnumerationValue {
      Objects.requireNonNull(literal, "literal");
    }
  }

  /**
   * This value as a {@code double}, for the numeric arms.
   *
   * @return the magnitude of an integer, real or quantity value
   * @throws IllegalStateException if this value is not numeric
   */
  default double asDouble() {
    if (this instanceof IntegerValue integer) {
      return integer.value();
    }
    if (this instanceof RealValue real) {
      return real.value();
    }
    if (this instanceof QuantityValue quantity) {
      return quantity.quantity().magnitude().doubleValue();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not a numeric value");
  }

  /**
   * This value as a {@code long}, for the integer arm alone.
   *
   * @return the integer value
   * @throws IllegalStateException if this value is not an {@link IntegerValue}
   */
  default long asLong() {
    if (this instanceof IntegerValue integer) {
      return integer.value();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not an integer value");
  }

  /**
   * This value as a {@code boolean}.
   *
   * @return the boolean value
   * @throws IllegalStateException if this value is not a {@link BooleanValue}
   */
  default boolean asBoolean() {
    if (this instanceof BooleanValue bool) {
      return bool.value();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not a boolean value");
  }

  /**
   * This value as a {@code String}.
   *
   * @return the string value
   * @throws IllegalStateException if this value is not a {@link StringValue}
   */
  default String asString() {
    if (this instanceof StringValue string) {
      return string.value();
    }
    throw new IllegalStateException(getClass().getSimpleName() + " is not a string value");
  }
}
