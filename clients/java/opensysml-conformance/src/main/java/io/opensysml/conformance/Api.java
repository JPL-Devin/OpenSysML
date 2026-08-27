package io.opensysml.conformance;

import com.google.protobuf.Message;
import io.opensysml.Capabilities;
import io.opensysml.CapabilityException;
import io.opensysml.Connection;
import io.opensysml.Instantiation;
import io.opensysml.Language;
import io.opensysml.Model;
import io.opensysml.ModelException;
import io.opensysml.ParseOptions;
import io.opensysml.ServiceException;
import io.opensysml.Symbol;
import io.opensysml.Value;
import io.opensysml.proto.DiagnosticsRequest;
import io.opensysml.proto.DiagnosticsResponse;
import io.opensysml.proto.EvaluateRequest;
import io.opensysml.proto.EvaluateResponse;
import io.opensysml.proto.GetSymbolRequest;
import io.opensysml.proto.InstantiateRequest;
import io.opensysml.proto.InstantiateResponse;
import io.opensysml.proto.ParseFileRequest;
import io.opensysml.proto.ParseFileResponse;
import io.opensysml.proto.ServerInfoRequest;
import io.opensysml.proto.ServerInfoResponse;
import io.opensysml.proto.SymbolResponse;
import io.opensysml.proto.SysmlProto;
import java.nio.file.Path;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;

/**
 * Makes a scenario's call through the client's public API and writes the answer back as the
 * response message the scenario is stated against. An RPC v1 does not cover is reported as such
 * rather than called over the transport, which is what the skipped count in a report counts.
 */
final class Api {

  /** The RPCs the v1 API covers, and so the ones a scenario can be run through it. */
  static final Set<String> COVERED =
      Set.of("GetServerInfo", "ParseFile", "GetSymbol", "GetDiagnostics", "Evaluate", "Instantiate");

  private final Connection connection;

  Api(Connection connection) {
    this.connection = connection;
  }

  /** What a call answered: a response, a refusal carrying a status, or nothing the API can do. */
  sealed interface Answer {
    /** The service answered. */
    record Answered(Message response) implements Answer {}

    /** The call was refused with a status. */
    record Refused(String status, String message) implements Answer {}

    /** The v1 API cannot make this call, so the scenario is skipped. */
    record Unsupported(String reason) implements Answer {}
  }

  /**
   * An empty request of the method's input type, which the scenario's protobuf JSON is merged into.
   *
   * @param method the bare method name
   * @return a builder of that method's request
   */
  static Message.Builder request(String method) {
    return switch (method) {
      case "GetServerInfo" -> ServerInfoRequest.newBuilder();
      case "ParseFile" -> ParseFileRequest.newBuilder();
      case "GetSymbol" -> GetSymbolRequest.newBuilder();
      case "GetDiagnostics" -> DiagnosticsRequest.newBuilder();
      case "Evaluate" -> EvaluateRequest.newBuilder();
      case "Instantiate" -> InstantiateRequest.newBuilder();
      default -> throw new IllegalArgumentException("no request type for " + method);
    };
  }

  /**
   * Whether the service declares an RPC at all, so a scenario naming one that does not exist is an
   * error in the suite rather than a skip.
   *
   * @param method the bare method name
   * @return whether {@code sysml.SysMLService} declares it
   */
  static boolean declared(String method) {
    return SysmlProto.getDescriptor().getServices().stream()
        .filter(service -> service.getFullName().equals("sysml.SysMLService"))
        .anyMatch(service -> service.findMethodByName(method) != null);
  }

  /**
   * Makes one call.
   *
   * @param method the bare method name
   * @param request the request the scenario named
   * @return what it answered
   */
  Answer call(String method, Message request) {
    if (!COVERED.contains(method)) {
      return new Answer.Unsupported("the v1 API does not cover " + method);
    }
    try {
      return new Answer.Answered(
          switch (method) {
            case "GetServerInfo" -> serverInfo();
            case "ParseFile" -> parse((ParseFileRequest) request);
            case "GetSymbol" -> symbol((GetSymbolRequest) request);
            case "GetDiagnostics" -> diagnostics((DiagnosticsRequest) request);
            case "Evaluate" -> evaluate((EvaluateRequest) request);
            case "Instantiate" -> instantiate((InstantiateRequest) request);
            default -> throw new IllegalStateException(method);
          });
    } catch (Unsupported e) {
      return new Answer.Unsupported(e.getMessage());
    } catch (ServiceException e) {
      return new Answer.Refused(e.status().name(), e.serviceMessage());
    } catch (CapabilityException e) {
      // The client refuses a call whose behaviour the service does not advertise, since the
      // service would ignore the field rather than answer UNIMPLEMENTED.
      return new Answer.Refused("UNIMPLEMENTED", e.getMessage());
    }
  }

  /** A call the v1 API has no way to make, thrown where the request is read. */
  private static final class Unsupported extends RuntimeException {
    private static final long serialVersionUID = 1L;

    Unsupported(String reason) {
      super(reason);
    }
  }

  private ServerInfoResponse serverInfo() {
    Capabilities capabilities = connection.capabilities();
    return ServerInfoResponse.newBuilder()
        .setVersion(capabilities.serviceVersion())
        .addAllCapabilities(new TreeSet<>(capabilities.names()))
        .build();
  }

  private ParseFileResponse parse(ParseFileRequest request) {
    ParseOptions options =
        new ParseOptions(Language.fromWireName(request.getLanguage()), request.getStrictConformance());
    Model model =
        switch (request.getSourceCase()) {
          case FILE_PATH -> connection.load(Path.of(request.getFilePath()), options);
          case CONTENT -> connection.parse(request.getContent(), options);
          case SOURCE_NOT_SET ->
              throw new Unsupported(
                  "the v1 API always names a source, so it cannot send a request naming none");
        };
    ParseFileResponse.Builder response =
        ParseFileResponse.newBuilder()
            .setModelHash(model.hash())
            .addAllDiagnostics(Rendering.diagnostics(model.parseDiagnostics()));
    model.root().ifPresent(root -> response.setRoot(Rendering.symbol(root)));
    return response.build();
  }

  private SymbolResponse symbol(GetSymbolRequest request) {
    Model model = connection.model(request.getModelHash());
    try {
      Symbol symbol = model.symbol(request.getSymbolId());
      return SymbolResponse.newBuilder().setSymbol(Rendering.symbol(symbol)).build();
    } catch (ModelException e) {
      return SymbolResponse.newBuilder().setError(e.getMessage()).build();
    }
  }

  private DiagnosticsResponse diagnostics(DiagnosticsRequest request) {
    Model model = connection.model(request.getModelHash());
    try {
      return DiagnosticsResponse.newBuilder()
          .addAllDiagnostics(Rendering.diagnostics(model.diagnostics()))
          .build();
    } catch (ModelException e) {
      return DiagnosticsResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private EvaluateResponse evaluate(EvaluateRequest request) {
    Model model = connection.model(request.getModelHash());
    boolean hasContext = !request.getContextSymbolId().isEmpty();
    boolean hasSubject = !request.getSubjectSymbolId().isEmpty();
    if (hasContext && hasSubject) {
      throw new Unsupported("the v1 API evaluates in a context or against a subject, not both");
    }
    try {
      Value value;
      if (hasSubject) {
        value = model.evalWithSubject(request.getExpression(), request.getSubjectSymbolId());
      } else if (hasContext) {
        value = model.evalInContext(request.getExpression(), request.getContextSymbolId());
      } else {
        value = model.eval(request.getExpression());
      }
      return EvaluateResponse.newBuilder().setResult(Rendering.value(value)).build();
    } catch (ModelException e) {
      return EvaluateResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }

  private InstantiateResponse instantiate(InstantiateRequest request) {
    Model model = connection.model(request.getModelHash());
    try {
      Instantiation instantiation = model.instantiate(request.getSymbolId());
      List<io.opensysml.proto.Instance> reachable =
          instantiation.reachable().stream().map(Rendering::instance).toList();
      return InstantiateResponse.newBuilder()
          .setInstance(Rendering.instance(instantiation.root()))
          .addAllInstances(reachable)
          .addAllDiagnostics(Rendering.diagnostics(instantiation.diagnostics()))
          .build();
    } catch (ModelException e) {
      return InstantiateResponse.newBuilder()
          .setError(e.getMessage())
          .addAllDiagnostics(Rendering.diagnostics(e.diagnostics()))
          .build();
    }
  }
}
