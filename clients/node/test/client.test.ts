// The public API against a real service: parse, evaluate, look up, instantiate,
// and the ownership rules close() follows.

import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, test } from "node:test";
import {
  CAPABILITY_COMPLEX_VALUES,
  CAPABILITY_QUERY,
  ClosedConnectionError,
  MissingCapabilityError,
  EvaluationError,
  SymbolNotFoundError,
  SERVICE_ENV,
  connect,
  currentPrivateService,
  formatValue,
  loads,
  requireCapability,
} from "../src/node/index.js";
import { SAMPLE, serviceBinary, useServiceBinary } from "./support/service.js";

before(() => {
  useServiceBinary();
});

after(() => {
  Reflect.deleteProperty(process.env, SERVICE_ENV);
});

test("a private child is started, shared, and stopped by the last connection to close", async () => {
  const first = await connect();
  const service = currentPrivateService();
  assert.ok(service !== undefined);
  assert.equal(service.refs, 1);
  const second = await connect();
  // One child per process: both connections reach the same one.
  assert.equal(currentPrivateService(), service);
  assert.equal(service.refs, 2);

  await first.close();
  assert.ok(service.alive);
  await second.close();
  assert.ok(!service.alive);
  assert.equal(currentPrivateService(), undefined);
});

test("closing twice is harmless and a closed connection refuses calls", async () => {
  const connection = await connect();
  await connection.close();
  await connection.close();
  assert.ok(connection.isClosed);
  await assert.rejects(() => connection.loads(SAMPLE), ClosedConnectionError);
});

test("await using closes the connection at the end of the block", async () => {
  {
    await using connection = await connect();
    assert.ok(!connection.isClosed);
  }
  assert.equal(currentPrivateService(), undefined);
});

test("the handshake reports the version and the capabilities to negotiate on", async () => {
  await using connection = await connect();
  assert.ok(connection.info.answered);
  assert.notEqual(connection.info.version, "");
  assert.ok(connection.info.capabilities.size > 0);
  assert.ok(connection.info.has(CAPABILITY_QUERY));
  // A capability the service does not advertise is refused by the client, since
  // the service answers such a call rather than returning UNIMPLEMENTED.
  assert.throws(() => {
    requireCapability(connection.info, "no_such_capability", "upgrade");
  }, MissingCapabilityError);
});

test("inline source parses, and its symbols, values and objects come back", async () => {
  await using model = await loads(SAMPLE);
  assert.notEqual(model.hash, "");
  assert.ok(!model.hasErrors);

  const car = await model.symbol("Sample::Car");
  assert.equal(car.kind, "partDef");
  assert.equal(car.id, "Sample::Car");
  assert.ok((await car.children()).some((child) => child.name === "wheels"));
  assert.equal(await model.find("Sample::Nope"), undefined);
  await assert.rejects(() => model.symbol("Sample::Nope"), SymbolNotFoundError);

  const mass = await model.eval("1500.0 + 1.0");
  assert.deepEqual(mass, { kind: "real", value: 1501 });
  const count = await model.eval("2 * 3");
  assert.deepEqual(count, { kind: "int", value: 6n });

  const tree = await model.instantiate("Sample::Car");
  const wheels = tree.get("wheels");
  assert.ok(wheels?.kind === "many");
  assert.equal(wheels.values.length, 4);
  const wheel = wheels.values[0];
  assert.ok(wheel.kind === "instance");
  assert.equal(tree.byId(wheel.id)?.typeId, "Sample::Wheel");
});

const COMPLEX_MODEL = `package C {
  private import ScalarValues::*;
  private import ComplexFunctions::*;
  part def Signal {
    attribute z : Complex = rect(1.5, -2.0);
    attribute zs : Complex[2] = (rect(1.0, 2.0), rect(3.0, 4.0));
  }
}
`;

test("a complex number is one value over gRPC, Connect protobuf and Connect JSON", async () => {
  for (const options of [{ protocol: "grpc" as const }, {}, { encoding: "json" as const }]) {
    await using connection = await connect(options);
    assert.ok((await connection.serverInfo()).has(CAPABILITY_COMPLEX_VALUES));
    await using model = await connection.loads(COMPLEX_MODEL);
    assert.deepEqual(await model.eval("ComplexFunctions::rect(1.0, -1.0)"), {
      kind: "complex",
      value: { real: 1, imaginary: -1 },
    });

    const signal = await model.instantiate("C::Signal");
    const z = signal.get("z");
    assert.ok(z?.kind === "single");
    assert.deepEqual(z.value, { kind: "complex", value: { real: 1.5, imaginary: -2 } });
    assert.equal(formatValue(z.value), "1.5 - 2.0i");
    const zs = signal.get("zs");
    assert.ok(zs?.kind === "many");
    assert.deepEqual(zs.values, [
      { kind: "complex", value: { real: 1, imaginary: 2 } },
      { kind: "complex", value: { real: 3, imaginary: 4 } },
    ]);
  }
});

test("a file parses, and a syntax error is a diagnostic, not a thrown call", async () => {
  const dir = mkdtempSync(join(tmpdir(), "client-test-"));
  const path = join(dir, "sample.sysml");
  writeFileSync(path, SAMPLE);
  await using connection = await connect();
  const model = await connection.load(path);
  assert.ok(!model.hasErrors);

  const broken = await connection.loads("package Broken { part def }");
  assert.ok(broken.hasErrors);
  assert.ok(broken.diagnostics.some((diagnostic) => diagnostic.severity === "error"));
});

test("an evaluation that cannot be made is an error the caller can catch", async () => {
  await using model = await loads(SAMPLE);
  await assert.rejects(() => model.eval("1 +"), EvaluationError);
});

test("a model adopted by hash uses the service's cache, and an unknown hash fails", async () => {
  await using connection = await connect();
  const parsed = await connection.loads(SAMPLE);
  const adopted = connection.model(parsed.hash);
  assert.deepEqual(await adopted.eval("2 + 2"), { kind: "int", value: 4n });
  const missing = connection.model("0".repeat(16));
  await assert.rejects(() => missing.eval("2 + 2"));
});

test("JSON bodies are opt-in and answer the same as protobuf", async () => {
  await using json = await connect({ encoding: "json" });
  assert.equal(json.encoding, "json");
  await using model = await json.loads(SAMPLE);
  assert.deepEqual(await model.eval("2 + 2"), { kind: "int", value: 4n });

  await using proto = await connect();
  assert.equal(proto.encoding, "protobuf");
});

test("the gRPC protocol works and refuses a JSON body", async () => {
  await using connection = await connect({ protocol: "grpc" });
  await using model = await connection.loads(SAMPLE);
  assert.deepEqual(await model.eval("2 + 2"), { kind: "int", value: 4n });
  await assert.rejects(() => connect({ protocol: "grpc", encoding: "json" }));
});

test("a service this client did not start is opt-in, and close leaves it running", async () => {
  const external = spawn(serviceBinary(), ["-port", "0", "-health-port", "0", "-report-address"], {
    stdio: ["pipe", "pipe", "inherit"],
  });
  try {
    const address = await firstLine(external);
    // An explicit address, and then the same through $OPENSYSML_SERVICE.
    const connection = await connect({ address });
    assert.match(connection.info.origin, /did not start/);
    await connection.close();
    assert.equal(currentPrivateService(), undefined);
    assert.equal(external.exitCode, null);

    process.env[SERVICE_ENV] = address;
    await using fromEnv = await connect();
    await using model = await fromEnv.loads(SAMPLE);
    assert.deepEqual(await model.eval("40 + 2"), { kind: "int", value: 42n });
    // No child was started for either connection.
    assert.equal(currentPrivateService(), undefined);
  } finally {
    Reflect.deleteProperty(process.env, SERVICE_ENV);
    external.kill("SIGKILL");
  }
});

function firstLine(child: ReturnType<typeof spawn>): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    let buffered = "";
    child.stdout?.on("data", (chunk: Buffer) => {
      buffered += chunk.toString();
      const newline = buffered.indexOf("\n");
      if (newline !== -1) {
        resolve(buffered.slice(0, newline).trim());
      }
    });
    child.once("error", reject);
    child.once("exit", () => {
      reject(new Error("the service exited before reporting an address"));
    });
  });
}
