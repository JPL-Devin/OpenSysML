package io.opensysml;

import java.io.IOException;
import java.io.InvalidObjectException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.io.Serial;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;

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

  private transient List<Diagnostic> diagnostics;

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

  @Serial
  private void writeObject(ObjectOutputStream stream) throws IOException {
    stream.defaultWriteObject();
    stream.writeInt(diagnostics.size());
    for (Diagnostic diagnostic : diagnostics) {
      stream.writeObject(diagnostic.severity());
      stream.writeObject(diagnostic.message());
      stream.writeBoolean(diagnostic.span().isPresent());
      if (diagnostic.span().isPresent()) {
        Diagnostic.Span span = diagnostic.span().orElseThrow();
        stream.writeObject(span.file());
        stream.writeInt(span.startLine());
        stream.writeInt(span.startColumn());
        stream.writeInt(span.endLine());
        stream.writeInt(span.endColumn());
      }
    }
  }

  @Serial
  private void readObject(ObjectInputStream stream) throws IOException, ClassNotFoundException {
    stream.defaultReadObject();
    int count = stream.readInt();
    if (count < 0) {
      throw new InvalidObjectException("negative diagnostic count");
    }
    List<Diagnostic> restored = new ArrayList<>(count);
    for (int index = 0; index < count; index++) {
      Object severity = stream.readObject();
      Object message = stream.readObject();
      if (!(severity instanceof Diagnostic.Severity diagnosticSeverity)
          || !(message instanceof String diagnosticMessage)) {
        throw new InvalidObjectException("invalid diagnostic");
      }
      Optional<Diagnostic.Span> span = Optional.empty();
      if (stream.readBoolean()) {
        Object file = stream.readObject();
        if (!(file instanceof String spanFile)) {
          throw new InvalidObjectException("invalid diagnostic span");
        }
        span =
            Optional.of(
                new Diagnostic.Span(
                    spanFile,
                    stream.readInt(),
                    stream.readInt(),
                    stream.readInt(),
                    stream.readInt()));
      }
      restored.add(new Diagnostic(diagnosticSeverity, diagnosticMessage, span));
    }
    diagnostics = List.copyOf(restored);
  }
}
