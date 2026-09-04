// The unions a consumer switches on, and how a protobuf Value becomes one.

import assert from "node:assert/strict";
import { test } from "node:test";
import { create } from "@bufbuild/protobuf";
import {
  ComplexSchema,
  EnumLiteralSchema,
  FailureReason,
  QuantitySchema,
  UnitFactorSchema,
  UnitTermSchema,
  ValueSchema,
  ValueSequenceSchema,
  VerdictSchema,
} from "../src/generated/sysml_pb.js";
import { decodeValue, decodeVerdict, failureCause, formatValue } from "../src/core/values.js";

test("a value the service never sent is absent, and an unset feature is unset", () => {
  assert.deepEqual(decodeValue(undefined), { kind: "absent" });
  assert.deepEqual(decodeValue(create(ValueSchema, {})), { kind: "absent" });
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "unset", value: true } })), {
    kind: "unset",
  });
});

test("integers keep their width and reals stay numbers", () => {
  const big = 9007199254740993n;
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "intValue", value: big } })), {
    kind: "int",
    value: big,
  });
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "realValue", value: 0.5 } })), {
    kind: "real",
    value: 0.5,
  });
});

test("a complex number is one value with both parts, never two reals", () => {
  const complex = (real: number, imaginary: number) =>
    create(ValueSchema, { kind: { case: "complex", value: create(ComplexSchema, { real, imaginary }) } });
  assert.deepEqual(decodeValue(complex(1.5, -2)), {
    kind: "complex",
    value: { real: 1.5, imaginary: -2 },
  });
  assert.equal(formatValue(decodeValue(complex(1.5, -2))), "1.5 - 2.0i");
  assert.equal(formatValue(decodeValue(complex(1, 2))), "1.0 + 2.0i");
  assert.equal(formatValue(decodeValue(complex(0, 0))), "0.0 + 0.0i");
  assert.equal(formatValue(decodeValue(complex(-0.25, 1e300))), "-0.25 + 1e+300i");
  // An empty message is zero, as every proto3 default is.
  assert.deepEqual(decodeValue(create(ValueSchema, { kind: { case: "complex", value: create(ComplexSchema, {}) } })), {
    kind: "complex",
    value: { real: 0, imaginary: 0 },
  });

  const nested = create(ValueSchema, {
    kind: {
      case: "sequence",
      value: create(ValueSequenceSchema, { elements: [complex(1, 2), complex(3, -4)] }),
    },
  });
  assert.deepEqual(decodeValue(nested), {
    kind: "sequence",
    elements: [
      { kind: "complex", value: { real: 1, imaginary: 2 } },
      { kind: "complex", value: { real: 3, imaginary: -4 } },
    ],
  });
  assert.equal(formatValue(decodeValue(nested)), "(1.0 + 2.0i, 3.0 - 4.0i)");
});

test("a sequence decodes its elements", () => {
  const value = create(ValueSchema, {
    kind: {
      case: "sequence",
      value: create(ValueSequenceSchema, {
        elements: [
          create(ValueSchema, { kind: { case: "intValue", value: 1n } }),
          create(ValueSchema, { kind: { case: "stringValue", value: "two" } }),
        ],
      }),
    },
  });
  const decoded = decodeValue(value);
  assert.equal(decoded.kind, "sequence");
  assert.deepEqual(decoded.elements, [
    { kind: "int", value: 1n },
    { kind: "string", value: "two" },
  ]);
  assert.equal(formatValue(decoded), '(1, "two")');
});

test("a quantity carries its magnitude, unit and reduction", () => {
  const value = create(ValueSchema, {
    kind: {
      case: "quantity",
      value: create(QuantitySchema, {
        magnitude: { case: "realMagnitude", value: 1500 },
        unit: "kg",
        unitTerm: create(UnitTermSchema, {
          scaleNum: 1000,
          scaleDen: 1,
          factors: [create(UnitFactorSchema, { unitId: "SI::g", exponent: 1 })],
        }),
      }),
    },
  });
  const decoded = decodeValue(value);
  assert.equal(decoded.kind, "quantity");
  assert.deepEqual(decoded.magnitude, { kind: "real", value: 1500 });
  assert.equal(decoded.unit, "kg");
  assert.deepEqual(decoded.unitTerm, {
    scaleNum: 1000,
    scaleDen: 1,
    factors: [{ unitId: "SI::g", exponent: 1 }],
  });
  assert.equal(formatValue(decoded), "1500.0[kg]");
});

test("an enum literal keeps the enumeration that declares it", () => {
  const value = create(ValueSchema, {
    kind: {
      case: "enumLiteral",
      value: create(EnumLiteralSchema, {
        name: "Colour::red",
        literalId: "D::Colour::red",
        enumerationId: "D::Colour",
      }),
    },
  });
  assert.deepEqual(decodeValue(value), {
    kind: "enum",
    value: { name: "Colour::red", literalId: "D::Colour::red", enumerationId: "D::Colour" },
  });
});

test("a verdict holds, fails or is undecided, and always names its subject", () => {
  const held = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "Sample::Check",
      element: "Sample::Check",
      instanceId: 7n,
      holds: true,
    }),
  );
  assert.equal(held.kind, "holds");
  assert.deepEqual(held.subject, {
    kind: "constraint",
    elementId: "Sample::Check",
    element: "Sample::Check",
    instanceId: 7n,
  });

  const failed = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "Sample::Check",
      element: "Sample::Check",
      holds: false,
      condition: "mass < 1000",
    }),
  );
  assert.equal(failed.kind, "fails");
  assert.equal(failed.condition, "mass < 1000");

  // holds: false with an error is no answer at all, not a failed verdict.
  const undecided = decodeVerdict(
    create(VerdictSchema, {
      kind: "constraint",
      elementId: "Sample::Check",
      element: "Sample::Check",
      holds: false,
      error: "radius is unbound",
      failureReason: FailureReason.EVALUATION,
    }),
  );
  assert.equal(undecided.kind, "undecided");
  assert.equal(undecided.cause, "evaluation");
});

test("every failure reason has a name", () => {
  assert.equal(failureCause(FailureReason.UNSPECIFIED), "unspecified");
  assert.equal(failureCause(FailureReason.EVALUATION), "evaluation");
  assert.equal(failureCause(FailureReason.WRONG_KIND), "wrong_kind");
  assert.equal(failureCause(FailureReason.AMBIGUOUS_SUBJECT), "ambiguous_subject");
});
