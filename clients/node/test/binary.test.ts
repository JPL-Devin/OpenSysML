// Finding the service binary, and refusing to invent one.

import assert from "node:assert/strict";
import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, test } from "node:test";
import { BINARY_ENV, BinaryNotFoundError, binaryName, platformPackage, resolveBinary } from "../src/node/index.js";

const saved = { binary: process.env[BINARY_ENV], path: process.env["PATH"], home: process.env["HOME"] };

afterEach(() => {
  restore(BINARY_ENV, saved.binary);
  restore("PATH", saved.path);
  restore("HOME", saved.home);
});

test("the platform package and file name follow npm's os/cpu names", () => {
  assert.equal(platformPackage("linux", "x64"), "@opensysml/sysml-grpc-linux-x64");
  assert.equal(platformPackage("darwin", "arm64"), "@opensysml/sysml-grpc-darwin-arm64");
  assert.equal(platformPackage("win32", "x64"), "@opensysml/sysml-grpc-win32-x64");
  assert.equal(binaryName("linux"), "sysml-grpc");
  assert.equal(binaryName("win32"), "sysml-grpc.exe");
});

test("$OPENSYSML_BINARY wins, and a path that is not executable is an error", () => {
  const dir = mkdtempSync(join(tmpdir(), "binary-test-"));
  const executable = join(dir, "sysml-grpc");
  writeFileSync(executable, "#!/bin/sh\n");
  chmodSync(executable, 0o755);
  process.env[BINARY_ENV] = executable;
  assert.equal(resolveBinary().path, executable);
  assert.match(resolveBinary().source, /OPENSYSML_BINARY/);

  const plain = join(dir, "not-executable");
  writeFileSync(plain, "");
  chmodSync(plain, 0o644);
  process.env[BINARY_ENV] = plain;
  assert.throws(() => resolveBinary(), BinaryNotFoundError);
});

test("a binary on $PATH is used, and with nothing anywhere the error says so", (t) => {
  if (process.platform === "win32") {
    t.skip("chmod and $PATH semantics differ on Windows");
    return;
  }
  const dir = mkdtempSync(join(tmpdir(), "binary-path-"));
  const empty = mkdtempSync(join(tmpdir(), "binary-home-"));
  const executable = join(dir, binaryName());
  writeFileSync(executable, "#!/bin/sh\n");
  chmodSync(executable, 0o755);
  restore(BINARY_ENV, undefined);
  process.env["PATH"] = dir;
  process.env["HOME"] = empty;
  assert.equal(resolveBinary().path, executable);

  process.env["PATH"] = empty;
  let message = "";
  assert.throws(
    () => resolveBinary(),
    (error: unknown) => {
      assert.ok(error instanceof BinaryNotFoundError);
      message = error.message;
      return true;
    },
  );
  // The message must name every place looked, and promise no download.
  assert.match(message, /OPENSYSML_BINARY/);
  assert.match(message, /@opensysml\/sysml-grpc-/);
  assert.match(message, /never downloads/);
});

function restore(name: string, value: string | undefined): void {
  if (value === undefined) {
    Reflect.deleteProperty(process.env, name);
  } else {
    process.env[name] = value;
  }
}
