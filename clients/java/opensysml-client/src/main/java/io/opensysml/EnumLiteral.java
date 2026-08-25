package io.opensysml;

import java.util.Objects;

/**
 * One literal of an enumeration definition, identified by its declaration rather than by a number
 * or a name.
 *
 * @param literalId FQN of the literal's declaration ({@code "D::Color::red"}), its identity
 * @param enumerationId FQN of the enumeration definition declaring it ({@code "D::Color"})
 * @param name the literal as a reader writes it ({@code "Color::red"})
 */
public record EnumLiteral(String literalId, String enumerationId, String name) {

  /**
   * Creates an enumeration literal.
   *
   * @param literalId FQN of the literal's declaration, never {@code null}
   * @param enumerationId FQN of the declaring enumeration, never {@code null}
   * @param name the literal as written, never {@code null}
   */
  public EnumLiteral {
    Objects.requireNonNull(literalId, "literalId");
    Objects.requireNonNull(enumerationId, "enumerationId");
    Objects.requireNonNull(name, "name");
  }
}
