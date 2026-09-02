// Values, quantities and verdicts as discriminated unions: a caller switches on
// `kind` and the compiler checks the switch is exhaustive.

import type {
  EnumLiteral,
  Quantity,
  UnitTerm,
  Value,
  Verdict,
} from "../generated/sysml_pb.js";
import { FailureReason } from "../generated/sysml_pb.js";
import type { FailureCause } from "./errors.js";

/** A quantity's magnitude: an integer or a real, never both. */
export type Magnitude = { kind: "int"; value: bigint } | { kind: "real"; value: number };

/** One unit raised to an exponent, as the service factorises a derived unit. */
export interface UnitFactor {
  unitId: string;
  exponent: number;
}

/** A unit as a scale factor over base-unit powers, when the service reports one. */
export interface UnitFactorization {
  scaleNum: number;
  scaleDen: number;
  factors: UnitFactor[];
}

/** A complex number in rectangular form: one value, never a sequence of two reals. */
export interface ComplexValue {
  real: number;
  imaginary: number;
}

/** An enumeration literal, identified by the enumeration that declares it. */
export interface EnumValue {
  name: string;
  literalId: string;
  enumerationId: string;
}

/**
 * A value the service computed. `absent` is the case a service sent no value at
 * all for, which is distinct from `unset` — a feature that exists and has none.
 */
export type SysMLValue =
  | { kind: "int"; value: bigint }
  | { kind: "real"; value: number }
  | { kind: "complex"; value: ComplexValue }
  | { kind: "boolean"; value: boolean }
  | { kind: "string"; value: string }
  | { kind: "instance"; id: bigint }
  | { kind: "sequence"; elements: SysMLValue[] }
  | { kind: "quantity"; magnitude: Magnitude; unit: string; unitTerm?: UnitFactorization }
  | { kind: "enum"; value: EnumValue }
  | { kind: "null"; reason: string }
  | { kind: "unset" }
  | { kind: "absent" };

/** What a verification answered about. Kept for the verification RPCs of a later version. */
export interface VerdictSubject {
  /** "constraint", "requirement" or "satisfy". */
  kind: string;
  /** FQN of the verified element; empty for an anonymous satisfy assertion. */
  elementId: string;
  /** The element as a reader names it. */
  element: string;
  /** The instance verified against, when there was one. */
  instanceId?: bigint;
  instanceTypeId?: string;
}

/**
 * One verification's answer. `undecided` is the service reporting it could not
 * answer, which a `holds: false` alone does not distinguish.
 */
export type SysMLVerdict =
  | { kind: "holds"; subject: VerdictSubject }
  | { kind: "fails"; subject: VerdictSubject; condition: string }
  | { kind: "undecided"; subject: VerdictSubject; error: string; cause: FailureCause };

/** Decodes a `sysml.Value` into the union. */
export function decodeValue(value: Value | undefined): SysMLValue {
  if (value === undefined) {
    return { kind: "absent" };
  }
  const kind = value.kind;
  switch (kind.case) {
    case "intValue":
      return { kind: "int", value: kind.value };
    case "realValue":
      return { kind: "real", value: kind.value };
    case "complex":
      return { kind: "complex", value: { real: kind.value.real, imaginary: kind.value.imaginary } };
    case "boolValue":
      return { kind: "boolean", value: kind.value };
    case "stringValue":
      return { kind: "string", value: kind.value };
    case "instanceId":
      return { kind: "instance", id: kind.value };
    case "sequence":
      return { kind: "sequence", elements: kind.value.elements.map(decodeValue) };
    case "quantity":
      return decodeQuantity(kind.value);
    case "enumLiteral":
      return { kind: "enum", value: decodeEnumLiteral(kind.value) };
    case "null":
      return { kind: "null", reason: kind.value };
    case "unset":
      return { kind: "unset" };
    case undefined:
      return { kind: "absent" };
  }
}

/** Decodes a `sysml.Verdict` into the union. */
export function decodeVerdict(verdict: Verdict): SysMLVerdict {
  const subject: VerdictSubject = {
    kind: verdict.kind,
    elementId: verdict.elementId,
    element: verdict.element,
    ...(verdict.instanceId === 0n ? {} : { instanceId: verdict.instanceId }),
    ...(verdict.instanceTypeId === "" ? {} : { instanceTypeId: verdict.instanceTypeId }),
  };
  if (verdict.error !== "") {
    return {
      kind: "undecided",
      subject,
      error: verdict.error,
      cause: failureCause(verdict.failureReason),
    };
  }
  return verdict.holds
    ? { kind: "holds", subject }
    : { kind: "fails", subject, condition: verdict.condition };
}

/** Names the enum the service reports for a failure it could not answer through. */
export function failureCause(reason: FailureReason): FailureCause {
  switch (reason) {
    case FailureReason.EVALUATION:
      return "evaluation";
    case FailureReason.WRONG_KIND:
      return "wrong_kind";
    case FailureReason.AMBIGUOUS_SUBJECT:
      return "ambiguous_subject";
    case FailureReason.UNSPECIFIED:
      return "unspecified";
  }
}

/** Renders a value the way the REPL prints one, for logs and error messages. */
export function formatValue(value: SysMLValue): string {
  switch (value.kind) {
    case "int":
      return value.value.toString();
    case "real":
      return formatReal(value.value);
    case "complex":
      return formatComplex(value.value);
    case "boolean":
      return value.value ? "true" : "false";
    case "string":
      return JSON.stringify(value.value);
    case "instance":
      return `<instance ${value.id.toString()}>`;
    case "sequence":
      return `(${value.elements.map(formatValue).join(", ")})`;
    case "quantity": {
      const magnitude =
        value.magnitude.kind === "int"
          ? value.magnitude.value.toString()
          : formatReal(value.magnitude.value);
      return value.unit === "" ? magnitude : `${magnitude}[${value.unit}]`;
    }
    case "enum":
      return value.value.name;
    case "null":
      return value.reason === "" ? "null" : `null (${value.reason})`;
    case "unset":
      return "unset";
    case "absent":
      return "absent";
  }
}

function formatReal(value: number): string {
  return Number.isInteger(value) ? value.toFixed(1) : value.toString();
}

/** `1.5 - 2.0i`, as the REPL prints a Complex; the sign is the imaginary part's. */
function formatComplex(value: ComplexValue): string {
  const sign = value.imaginary < 0 || Object.is(value.imaginary, -0) ? "-" : "+";
  return `${formatReal(value.real)} ${sign} ${formatReal(Math.abs(value.imaginary))}i`;
}

function decodeQuantity(quantity: Quantity): SysMLValue {
  const magnitude: Magnitude =
    quantity.magnitude.case === "intMagnitude"
      ? { kind: "int", value: quantity.magnitude.value }
      : { kind: "real", value: quantity.magnitude.value ?? 0 };
  const unitTerm = quantity.unitTerm === undefined ? undefined : decodeUnitTerm(quantity.unitTerm);
  return {
    kind: "quantity",
    magnitude,
    unit: quantity.unit,
    ...(unitTerm === undefined ? {} : { unitTerm }),
  };
}

function decodeUnitTerm(term: UnitTerm): UnitFactorization {
  return {
    scaleNum: term.scaleNum,
    scaleDen: term.scaleDen,
    factors: term.factors.map((factor) => ({
      unitId: factor.unitId,
      exponent: factor.exponent,
    })),
  };
}

function decodeEnumLiteral(literal: EnumLiteral): EnumValue {
  return {
    name: literal.name,
    literalId: literal.literalId,
    enumerationId: literal.enumerationId,
  };
}
