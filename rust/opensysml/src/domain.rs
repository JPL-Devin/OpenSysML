use std::collections::HashMap;
use std::sync::Arc;

use crate::{error::Error, wire, Connection};

/// Language accepted by the parser.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Language {
    /// SysML v2 notation.
    Sysml,
    /// KerML notation.
    Kerml,
}

impl Language {
    pub(crate) fn as_str(self) -> &'static str {
        match self {
            Self::Sysml => "sysml",
            Self::Kerml => "kerml",
        }
    }
}

/// Options controlling a parse operation.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ParseOptions {
    /// Language of inline content.
    pub language: Language,
    /// Require strict SysML v2 conformance.
    pub strict_conformance: bool,
}

impl Default for ParseOptions {
    fn default() -> Self {
        Self {
            language: Language::Sysml,
            strict_conformance: false,
        }
    }
}

/// An exact service capability set.
#[derive(Clone, Debug)]
pub struct Capabilities {
    wire: wire::ServerInfoResponse,
}

impl Capabilities {
    pub(crate) fn new(wire: wire::ServerInfoResponse) -> Self {
        Self { wire }
    }

    /// Whether the service advertises `capability`.
    pub fn has(&self, capability: &str) -> bool {
        self.wire.capabilities.iter().any(|item| item == capability)
    }

    /// Require a capability, returning a legible remedy when it is absent.
    pub fn require(&self, capability: &str, remedy: impl Into<String>) -> Result<(), Error> {
        if self.has(capability) {
            Ok(())
        } else {
            Err(Error::MissingCapability {
                capability: capability.to_owned(),
                remedy: remedy.into(),
            })
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::ServerInfoResponse {
        &self.wire
    }
}

/// Information advertised by a connected service.
#[derive(Clone, Debug)]
pub struct ServerInfo {
    /// Build version reported by the service.
    pub version: String,
    /// Service capabilities.
    pub capabilities: Capabilities,
    wire: wire::ServerInfoResponse,
}

impl ServerInfo {
    pub(crate) fn from_wire(wire: wire::ServerInfoResponse) -> Self {
        let capabilities = Capabilities::new(wire.clone());
        Self {
            version: wire.version.clone(),
            capabilities,
            wire,
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::ServerInfoResponse {
        &self.wire
    }
}

/// A source span attached to a diagnostic.
#[derive(Clone, Debug)]
pub struct Span {
    /// Source file name.
    pub file: String,
    /// One-based starting line.
    pub start_line: i32,
    /// One-based starting column.
    pub start_col: i32,
    /// One-based ending line.
    pub end_line: i32,
    /// One-based ending column.
    pub end_col: i32,
}

impl From<wire::Span> for Span {
    fn from(value: wire::Span) -> Self {
        Self {
            file: value.file,
            start_line: value.start_line,
            start_col: value.start_col,
            end_line: value.end_line,
            end_col: value.end_col,
        }
    }
}

/// A parser or semantic diagnostic.
#[derive(Clone, Debug)]
pub struct Diagnostic {
    /// Severity such as `error`, `warning`, or `info`.
    pub severity: String,
    /// Human-readable diagnostic message.
    pub message: String,
    /// Optional source location.
    pub span: Option<Span>,
    wire: wire::Diagnostic,
}

impl From<wire::Diagnostic> for Diagnostic {
    fn from(wire: wire::Diagnostic) -> Self {
        let span = wire.span.clone().map(Span::from);
        Self {
            severity: wire.severity.clone(),
            message: wire.message.clone(),
            span,
            wire,
        }
    }
}

impl Diagnostic {
    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::Diagnostic {
        &self.wire
    }
}

/// An integer or real quantity magnitude.
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Magnitude {
    /// An exact integer magnitude.
    Integer(i64),
    /// A floating-point magnitude.
    Real(f64),
}

/// A reduced measurement unit.
#[derive(Clone, Debug, PartialEq)]
pub struct UnitTerm {
    /// Scale numerator.
    pub scale_num: f64,
    /// Scale denominator.
    pub scale_den: f64,
    /// Base-unit factors.
    pub factors: Vec<UnitFactor>,
}

/// One base unit raised to an exponent.
#[derive(Clone, Debug, PartialEq)]
pub struct UnitFactor {
    /// Fully qualified base-unit identifier.
    pub unit_id: String,
    /// Exponent of the base unit.
    pub exponent: f64,
}

/// A numeric magnitude and its measurement unit.
#[derive(Clone, Debug, PartialEq)]
pub struct Quantity {
    /// Magnitude, preserving integer versus real.
    pub magnitude: Magnitude,
    /// Unit as written by the model.
    pub unit: String,
    /// Reduced unit term, when supplied by the service.
    pub unit_term: Option<UnitTerm>,
}

/// An enumeration literal value.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EnumLiteral {
    /// Fully qualified literal identity.
    pub literal_id: String,
    /// Fully qualified enumeration definition.
    pub enumeration_id: String,
    /// Reader-facing literal name.
    pub name: String,
}

/// A runtime value returned by the service.
#[derive(Clone, Debug, PartialEq)]
pub enum Value {
    /// Integer value.
    Integer(i64),
    /// Real value.
    Real(f64),
    /// Boolean value.
    Boolean(bool),
    /// Text value.
    Text(String),
    /// Instance reference.
    InstanceRef(i64),
    /// Sequence value.
    Sequence(Vec<Value>),
    /// Quantity value.
    Quantity(Quantity),
    /// Enumeration literal value.
    EnumLiteral(EnumLiteral),
    /// Explicit null value.
    Null,
    /// A materialized feature with no value.
    Unset,
}

pub(crate) fn value_from_wire(
    value: wire::Value,
    capabilities: &Capabilities,
) -> Result<Value, Error> {
    let Some(kind) = value.kind else {
        return Err(Error::Decode("Value has no kind".to_owned()));
    };
    match kind {
        wire::value::Kind::IntValue(v) => Ok(Value::Integer(v)),
        wire::value::Kind::RealValue(v) => Ok(Value::Real(v)),
        wire::value::Kind::BoolValue(v) => Ok(Value::Boolean(v)),
        wire::value::Kind::StringValue(v) => Ok(Value::Text(v)),
        wire::value::Kind::InstanceId(v) => Ok(Value::InstanceRef(v)),
        wire::value::Kind::Sequence(v) => Ok(Value::Sequence(
            v.elements
                .into_iter()
                .map(|item| value_from_wire(item, capabilities))
                .collect::<Result<_, _>>()?,
        )),
        wire::value::Kind::Null(_) => Ok(Value::Null),
        wire::value::Kind::Quantity(v) => {
            let magnitude = match v.magnitude {
                Some(wire::quantity::Magnitude::IntMagnitude(value)) => Magnitude::Integer(value),
                Some(wire::quantity::Magnitude::RealMagnitude(value)) => Magnitude::Real(value),
                None => return Err(Error::Decode("Quantity has no magnitude".to_owned())),
            };
            let unit_term = v.unit_term.map(|term| UnitTerm {
                scale_num: term.scale_num,
                scale_den: term.scale_den,
                factors: term
                    .factors
                    .into_iter()
                    .map(|factor| UnitFactor {
                        unit_id: factor.unit_id,
                        exponent: factor.exponent,
                    })
                    .collect(),
            });
            Ok(Value::Quantity(Quantity {
                magnitude,
                unit: v.unit,
                unit_term,
            }))
        }
        wire::value::Kind::EnumLiteral(v) => {
            capabilities.require(
                "enum_values",
                "connect to a service advertising enum_values",
            )?;
            Ok(Value::EnumLiteral(EnumLiteral {
                literal_id: v.literal_id,
                enumeration_id: v.enumeration_id,
                name: v.name,
            }))
        }
        wire::value::Kind::Unset(_) => {
            capabilities.require(
                "unset_value",
                "connect to a service advertising unset_value",
            )?;
            Ok(Value::Unset)
        }
    }
}

/// A symbol in a parsed model.
#[derive(Clone, Debug)]
pub struct Symbol {
    wire: wire::SymbolInfo,
    connection: Arc<crate::connection::ConnectionInner>,
    model_hash: String,
}

impl Symbol {
    pub(crate) fn new(
        wire: wire::SymbolInfo,
        connection: Arc<crate::connection::ConnectionInner>,
        model_hash: String,
    ) -> Self {
        Self {
            wire,
            connection,
            model_hash,
        }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::SymbolInfo {
        &self.wire
    }
    /// Fully qualified identifier.
    pub fn id(&self) -> &str {
        &self.wire.id
    }
    /// Short name.
    pub fn name(&self) -> &str {
        &self.wire.name
    }
    /// Service symbol kind.
    pub fn kind(&self) -> &str {
        &self.wire.kind
    }
    /// Child symbols, fetched lazily from the service.
    pub fn children(&self) -> Result<Vec<Symbol>, Error> {
        self.wire
            .child_ids
            .iter()
            .map(|id| {
                Connection {
                    inner: self.connection.clone(),
                }
                .get_symbol(&self.model_hash, id)
            })
            .collect()
    }
}

/// An instantiated model object.
#[derive(Clone, Debug)]
pub struct Instance {
    wire: wire::Instance,
    features: HashMap<String, FeatureValue>,
}

impl Instance {
    pub(crate) fn from_wire(
        wire: wire::Instance,
        capabilities: &Capabilities,
    ) -> Result<Self, Error> {
        let features = wire
            .feature_values
            .iter()
            .map(|(name, value)| {
                FeatureValue::from_wire(value.clone(), capabilities)
                    .map(|item| (name.clone(), item))
            })
            .collect::<Result<_, _>>()?;
        Ok(Self { wire, features })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::Instance {
        &self.wire
    }
    /// Runtime identity.
    pub fn id(&self) -> i64 {
        self.wire.id
    }
    /// Fully qualified type identifier.
    pub fn type_symbol_id(&self) -> &str {
        &self.wire.type_symbol_id
    }
    /// Feature values keyed by feature name.
    pub fn feature_values(&self) -> &HashMap<String, FeatureValue> {
        &self.features
    }
    /// One feature value, if present.
    pub fn feature(&self, name: &str) -> Option<&FeatureValue> {
        self.features.get(name)
    }
}

/// A value held by one instance feature.
#[derive(Clone, Debug)]
pub struct FeatureValue {
    wire: wire::FeatureValue,
    value: Option<Value>,
    values: Vec<Value>,
}

impl FeatureValue {
    fn from_wire(wire: wire::FeatureValue, capabilities: &Capabilities) -> Result<Self, Error> {
        capabilities.require(
            "feature_values",
            "connect to a service advertising feature_values",
        )?;
        let value = wire
            .value
            .clone()
            .map(|item| value_from_wire(item, capabilities))
            .transpose()?;
        let values = wire
            .values
            .iter()
            .cloned()
            .map(|item| value_from_wire(item, capabilities))
            .collect::<Result<_, _>>()?;
        Ok(Self {
            wire,
            value,
            values,
        })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::FeatureValue {
        &self.wire
    }
    /// Feature name.
    pub fn name(&self) -> &str {
        &self.wire.feature_name
    }
    /// Single-valued feature value.
    pub fn value(&self) -> Option<&Value> {
        self.value.as_ref()
    }
    /// Multi-valued feature values.
    pub fn values(&self) -> &[Value] {
        &self.values
    }
    /// Whether the feature was materialized.
    pub fn materialized(&self) -> bool {
        self.wire.materialized
    }
    /// In-band evaluation error for this feature.
    pub fn error(&self) -> Option<&str> {
        (!self.wire.error.is_empty()).then_some(self.wire.error.as_str())
    }
}

/// Options for evaluating an expression.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct EvalOptions {
    /// Optional context symbol identifier.
    pub context: Option<String>,
    /// Optional subject symbol identifier.
    pub subject: Option<String>,
}

/// An evaluated expression and its wire response.
#[derive(Clone, Debug)]
pub struct Evaluation {
    /// Decoded expression result.
    pub result: Value,
    wire: wire::EvaluateResponse,
}

impl Evaluation {
    pub(crate) fn new(result: Value, wire: wire::EvaluateResponse) -> Self {
        Self { result, wire }
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::EvaluateResponse {
        &self.wire
    }
}

/// A parsed model.
#[derive(Clone, Debug)]
pub struct Model {
    wire: wire::ParseFileResponse,
    root: Symbol,
    diagnostics: Vec<Diagnostic>,
    connection: Connection,
}

impl Model {
    pub(crate) fn from_wire(
        wire: wire::ParseFileResponse,
        connection: Connection,
    ) -> Result<Self, Error> {
        let root_wire = wire
            .root
            .clone()
            .ok_or_else(|| Error::Decode("parse response has no root".to_owned()))?;
        let root = Symbol::new(root_wire, connection.inner.clone(), wire.model_hash.clone());
        let diagnostics = wire
            .diagnostics
            .iter()
            .cloned()
            .map(Diagnostic::from)
            .collect();
        Ok(Self {
            wire,
            root,
            diagnostics,
            connection,
        })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::ParseFileResponse {
        &self.wire
    }
    /// Content hash used by subsequent service requests.
    pub fn hash(&self) -> &str {
        &self.wire.model_hash
    }
    /// Diagnostics in service order.
    pub fn diagnostics(&self) -> &[Diagnostic] {
        &self.diagnostics
    }
    /// Root namespace symbol.
    pub fn root(&self) -> &Symbol {
        &self.root
    }
    /// Evaluate an expression and return its domain value.
    pub fn eval(&self, expr: &str) -> Result<Value, Error> {
        Ok(self.evaluate(expr, &EvalOptions::default())?.result)
    }
    /// Evaluate an expression with optional context and subject.
    pub fn evaluate(&self, expr: &str, options: &EvalOptions) -> Result<Evaluation, Error> {
        self.connection.evaluate(self.hash(), expr, options)
    }
    /// Look up a fully qualified symbol.
    pub fn symbol(&self, fqn: &str) -> Result<Symbol, Error> {
        self.connection.get_symbol(self.hash(), fqn)
    }
    /// Instantiate a part or usage.
    pub fn instantiate(&self, fqn: &str) -> Result<Instantiation, Error> {
        self.connection.instantiate(self.hash(), fqn)
    }
}

/// An instantiation response and its primary instance.
#[derive(Clone, Debug)]
pub struct Instantiation {
    /// Primary instantiated object.
    pub instance: Instance,
    instances: Vec<Instance>,
    wire: wire::InstantiateResponse,
}

impl Instantiation {
    pub(crate) fn from_wire(
        wire: wire::InstantiateResponse,
        capabilities: &Capabilities,
    ) -> Result<Self, Error> {
        let instance_wire = wire
            .instance
            .clone()
            .ok_or_else(|| Error::Decode("instantiate response has no instance".to_owned()))?;
        let instance = Instance::from_wire(instance_wire, capabilities)?;
        let instances = wire
            .instances
            .iter()
            .cloned()
            .map(|item| Instance::from_wire(item, capabilities))
            .collect::<Result<_, _>>()?;
        Ok(Self {
            instance,
            instances,
            wire,
        })
    }

    /// The response this was built from; for conformance tooling and debugging.
    pub fn wire(&self) -> &wire::InstantiateResponse {
        &self.wire
    }
    /// All reachable instances included by the service.
    pub fn instances(&self) -> &[Instance] {
        &self.instances
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn capabilities(names: &[&str]) -> Capabilities {
        Capabilities::new(wire::ServerInfoResponse {
            version: "test".to_owned(),
            capabilities: names.iter().map(|name| (*name).to_owned()).collect(),
        })
    }

    #[test]
    fn value_arms_are_decoded_without_loss() {
        let caps = capabilities(&["enum_values", "unset_value"]);
        let sequence = wire::Value {
            kind: Some(wire::value::Kind::Sequence(wire::ValueSequence {
                elements: vec![
                    wire::Value {
                        kind: Some(wire::value::Kind::IntValue(7)),
                    },
                    wire::Value {
                        kind: Some(wire::value::Kind::RealValue(2.5)),
                    },
                ],
            })),
        };
        let result = value_from_wire(sequence, &caps);
        assert_eq!(
            result.ok(),
            Some(Value::Sequence(vec![Value::Integer(7), Value::Real(2.5)]))
        );
    }

    #[test]
    fn unsupported_value_capability_is_explicit() {
        let result = value_from_wire(
            wire::Value {
                kind: Some(wire::value::Kind::Unset(true)),
            },
            &capabilities(&[]),
        );
        assert!(
            matches!(result, Err(Error::MissingCapability { capability, .. }) if capability == "unset_value")
        );
    }
}
