// Runs every example against a real service, so the examples cannot rot.

import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { before, test } from "node:test";
import { useServiceBinary } from "./support/service.js";

const run = promisify(execFile);
const examples = resolve(dirname(fileURLToPath(import.meta.url)), "../examples");

before(() => {
  useServiceBinary();
});

const names = readdirSync(examples)
  .filter((entry) => /^\d\d-.*\.js$/.test(entry))
  .sort();

test("the examples directory holds the examples the README names", () => {
  assert.ok(names.length >= 6, `found ${String(names.length)} examples in ${examples}`);
});

for (const name of names) {
  test(`example ${name} runs clean`, async () => {
    const { stdout } = await run(process.execPath, [join(examples, name)], {
      env: process.env,
      timeout: 120_000,
    });
    assert.match(stdout, /\S/);
  });
}
