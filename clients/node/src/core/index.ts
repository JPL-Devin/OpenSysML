// The isomorphic core: everything that does not need a process to spawn.

export { Connection } from "./connection.js";
export type {
  ConnectionBackend,
  Encoding,
  ResponseTap,
  TransportOptions,
} from "./connection.js";
export {
  Instance,
  InstanceTree,
  Model,
  ModelSymbol,
} from "./model.js";
export type {
  AttributeFacts,
  EvalOptions,
  FeatureValue,
  ParseOptions,
  SpecializationFacts,
  TypeFacts,
} from "./model.js";
export {
  CAPABILITY_APPLY_EDITS,
  CAPABILITY_CONVERT,
  CAPABILITY_ENUM_VALUES,
  CAPABILITY_EVALUATE_SUBJECT,
  CAPABILITY_FEATURE_VALUES,
  CAPABILITY_INLINE_LANGUAGE,
  CAPABILITY_QUERY,
  CAPABILITY_STRICT_CONFORMANCE,
  CAPABILITY_SYMBOL_ATTRIBUTES,
  CAPABILITY_TYPE_FACTS,
  CAPABILITY_UNSET_VALUE,
  CAPABILITY_VERIFICATION,
  MissingCapabilityError,
  ServerInfo,
  requireCapability,
  upgradeRemedy,
} from "./capabilities.js";
export {
  ChecksumMismatchError,
  ClosedConnectionError,
  DownloadError,
  EvaluationError,
  InvalidRequestError,
  ManifestSignatureError,
  ModelFileNotFoundError,
  ModelNotFoundError,
  OpenSysMLError,
  ParseError,
  ServiceError,
  ServiceStartError,
  ServiceTimeoutError,
  SymbolNotFoundError,
  UnpinnedReleaseError,
  UnsignedReleaseError,
  UnsupportedOperationError,
} from "./errors.js";
export type { FailureCause, ModelDiagnostic } from "./errors.js";
export { fromRpcError, statusName } from "./status.js";
export type { NotFoundSubject } from "./status.js";
export { decodeValue, decodeVerdict, formatValue } from "./values.js";
export type {
  EnumValue,
  Magnitude,
  SysMLValue,
  SysMLVerdict,
  UnitFactor,
  UnitFactorization,
  VerdictSubject,
} from "./values.js";
export { baseUrl } from "./transport.js";
export { SysMLService } from "../generated/sysml_pb.js";
