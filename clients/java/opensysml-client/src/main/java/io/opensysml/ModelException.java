package io.opensysml;

import java.util.List;
import java.util.Objects;

/**
 * A failure the service reported inside an answer it did give: an expression that would not
 * evaluate, a symbol that is not there, a source that would not parse.
 *
 * <p>The call itself succeeded, so this is never a {@link ServiceException}: the difference matters
 * to anything that treats a refused call as a service problem and a rejected model as a user
 * problem.
 */
public class ModelException extends OpenSysMLException {

  private static final long serialVersionUID = 1L;

  private final List<Diagnostic> diagnostics;

  /**
   * Creates a model exception.
   *
   * @param message the failure, as the service worded it
   * @param diagnostics diagnostics the answer carried
   */
  public ModelException(String message, List<Diagnostic> diagnostics) {
    super(message);
    this.diagnostics = List.copyOf(Objects.requireNonNull(diagnostics, "diagnostics"));
  }

  /**
   * The diagnostics the answer carried.
   *
   * @return the diagnostics, possibly empty
   */
  public List<Diagnostic> diagnostics() {
    return diagnostics;
  }
}
