// Errors this client raises. Everything derives from OpenSysMLError, so a caller
// can catch the family without knowing the members.

/** Base class of every error this client raises. */
export class OpenSysMLError extends Error {
  constructor(message: string, options?: { cause?: unknown }) {
    super(message, options);
    this.name = new.target.name;
  }
}

/** The service could not be reached, started, or answered nothing usable. */
export class ServiceError extends OpenSysMLError {}

/** A private child service failed to start, or died while it was needed. */
export class ServiceStartError extends ServiceError {}

/** The connection was closed and cannot be used again. */
export class ClosedConnectionError extends OpenSysMLError {
  constructor() {
    super("this connection is closed; open another with connect()");
  }
}

/** A release binary could not be downloaded, or could not be installed once it was. */
export class DownloadError extends ServiceError {}

/** A download's digest contradicts the one expected of it, so it is never used. */
export class ChecksumMismatchError extends DownloadError {}

/** Nothing pins or signs a digest for the release, leaving only its origin's word. */
export class UnpinnedReleaseError extends ChecksumMismatchError {}

/**
 * No signature on the checksum manifest could be checked at all: none published,
 * unreadable, or no verifier installed. Refused exactly as an unpinned release is.
 */
export class UnsignedReleaseError extends UnpinnedReleaseError {}

/** A signature was checked and does not verify: another signer, or a changed manifest. */
export class ManifestSignatureError extends ChecksumMismatchError {}

/** A model file could not be read, or its content did not parse. */
export class ParseError extends OpenSysMLError {
  /** Diagnostics the service reported, in the order it reported them. */
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(message: string, diagnostics: readonly ModelDiagnostic[] = []) {
    super(message);
    this.diagnostics = diagnostics;
  }
}

/** One diagnostic about a model, at a source position when the service gave one. */
export interface ModelDiagnostic {
  severity: string;
  message: string;
  file?: string;
  startLine?: number;
  startColumn?: number;
  endLine?: number;
  endColumn?: number;
}

/** No symbol of that name is declared in the model. */
export class SymbolNotFoundError extends OpenSysMLError {
  readonly name_: string;

  constructor(name: string, near: readonly string[] = []) {
    const hint = near.length > 0 ? `; the model declares ${near.join(", ")}` : "";
    super(`the model declares no symbol named ${JSON.stringify(name)}${hint}`);
    this.name_ = name;
  }
}

/** An expression could not be evaluated, or a symbol could not be instantiated. */
export class EvaluationError extends OpenSysMLError {
  /** Why the service could not answer, when it classified the failure. */
  readonly reason: FailureCause;
  readonly diagnostics: readonly ModelDiagnostic[];

  constructor(
    message: string,
    reason: FailureCause = "unspecified",
    diagnostics: readonly ModelDiagnostic[] = [],
  ) {
    super(message);
    this.reason = reason;
    this.diagnostics = diagnostics;
  }
}

/** The service's classification of a failure it reported in a successful answer. */
export type FailureCause = "unspecified" | "evaluation" | "wrong_kind" | "ambiguous_subject";
