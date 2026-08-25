use thiserror::Error;

/// Canonical status names used by gRPC and Connect.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Status {
    /// The operation completed successfully.
    Ok,
    /// The request was cancelled.
    Cancelled,
    /// An unknown service error occurred.
    Unknown,
    /// The caller supplied an invalid argument.
    InvalidArgument,
    /// The deadline expired before completion.
    DeadlineExceeded,
    /// The requested resource was not found.
    NotFound,
    /// The requested operation is not available.
    Unimplemented,
    /// The service is unavailable.
    Unavailable,
    /// The request is unauthenticated.
    Unauthenticated,
    /// The caller is not authorized.
    PermissionDenied,
    /// The resource already exists.
    AlreadyExists,
    /// The operation was refused because the resource is exhausted.
    ResourceExhausted,
    /// The service refused the operation because of preconditions.
    FailedPrecondition,
    /// The operation was aborted.
    Aborted,
    /// The caller attempted an operation outside its range.
    OutOfRange,
    /// The operation is not implemented.
    Internal,
    /// The service cannot provide the requested data.
    DataLoss,
}

impl Status {
    pub(crate) fn from_connect_code(code: &str) -> Self {
        match code.to_ascii_lowercase().as_str() {
            "ok" => Self::Ok,
            "cancelled" => Self::Cancelled,
            "invalid_argument" => Self::InvalidArgument,
            "deadline_exceeded" => Self::DeadlineExceeded,
            "not_found" => Self::NotFound,
            "unimplemented" => Self::Unimplemented,
            "unavailable" => Self::Unavailable,
            "unauthenticated" => Self::Unauthenticated,
            "permission_denied" => Self::PermissionDenied,
            "already_exists" => Self::AlreadyExists,
            "resource_exhausted" => Self::ResourceExhausted,
            "failed_precondition" => Self::FailedPrecondition,
            "aborted" => Self::Aborted,
            "out_of_range" => Self::OutOfRange,
            "internal" => Self::Internal,
            "data_loss" => Self::DataLoss,
            _ => Self::Unknown,
        }
    }
}

/// Errors returned by the blocking OpenSysML client.
#[derive(Debug, Error)]
pub enum Error {
    /// The service refused an RPC at the transport layer.
    #[error("service returned {status:?}: {message}")]
    Service {
        /// Canonical gRPC status.
        status: Status,
        /// Message supplied by the service.
        message: String,
    },
    /// The service does not advertise an operation's required capability.
    #[error("missing capability {capability:?}: {remedy}")]
    MissingCapability {
        /// Capability that was required.
        capability: String,
        /// Suggested way to obtain the capability.
        remedy: String,
    },
    /// A private service could not be started or stopped cleanly.
    #[error("service start failed: {0}")]
    ServiceStart(String),
    /// No sysml-grpc binary could be resolved.
    #[error("sysml-grpc binary not found; looked in: {looked_in:?}")]
    BinaryNotFound {
        /// Locations searched.
        looked_in: Vec<String>,
    },
    /// The HTTP transport failed before a response was decoded.
    #[error("transport error: {0}")]
    Transport(String),
    /// A successful HTTP response was not valid protobuf.
    #[error("protobuf decode error: {0}")]
    Decode(String),
    /// The service answered successfully but reported an in-band model error.
    #[error("model error: {0}")]
    Model(String),
    /// Local filesystem or process I/O failed.
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),
}
