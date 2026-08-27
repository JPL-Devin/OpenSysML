// Finding the sysml-grpc binary. This client never downloads one: the binary
// arrives as an optional per-platform npm package, whose integrity npm checks,
// or the caller names a build of their own.

import { accessSync, constants } from "node:fs";
import { createRequire } from "node:module";
import { homedir } from "node:os";
import { delimiter, join } from "node:path";
import { OpenSysMLError } from "../core/errors.js";

/** Names a build of sysml-grpc to use, in place of any package or cache. */
export const BINARY_ENV = "OPENSYSML_BINARY";

/** Where a binary was found, and how. */
export interface Binary {
  path: string;
  /** Human-readable provenance, used in error messages and ServerInfo.origin. */
  source: string;
}

/** No usable binary was found, and this client will not fetch one. */
export class BinaryNotFoundError extends OpenSysMLError {}

/** The npm package that carries the service binary for this platform. */
export function platformPackage(
  platform: string = process.platform,
  arch: string = process.arch,
): string {
  return `@opensysml/sysml-grpc-${platform}-${arch}`;
}

/** The binary's file name on this platform. */
export function binaryName(platform: string = process.platform): string {
  return platform === "win32" ? "sysml-grpc.exe" : "sysml-grpc";
}

/**
 * Finds a sysml-grpc binary: $OPENSYSML_BINARY, then the platform package, then
 * the cache the Python client fills, then $PATH. Never downloads.
 */
export function resolveBinary(): Binary {
  const looked: string[] = [];

  const named = process.env[BINARY_ENV];
  if (named !== undefined && named !== "") {
    if (!isExecutable(named)) {
      throw new BinaryNotFoundError(
        `$${BINARY_ENV} names ${named}, which is not an executable file`,
      );
    }
    return { path: named, source: `${named} (from $${BINARY_ENV})` };
  }
  looked.push(`$${BINARY_ENV}`);

  const packaged = fromPlatformPackage();
  if (packaged !== undefined) {
    return packaged;
  }
  looked.push(`the ${platformPackage()} package`);

  const cached = join(homedir(), ".opensysml", "bin", binaryName());
  if (isExecutable(cached)) {
    return { path: cached, source: `${cached} (cached)` };
  }
  looked.push(cached);

  const onPath = fromPath();
  if (onPath !== undefined) {
    return onPath;
  }
  looked.push("$PATH");

  throw new BinaryNotFoundError(
    `no sysml-grpc binary: looked at ${looked.join(", ")}.\n` +
      `  fix: install the service for this platform (npm install ${platformPackage()}), or\n` +
      `       build it (make build-grpc) and set $${BINARY_ENV} to the result, or\n` +
      `       start a service yourself and pass its address to connect().\n` +
      "  This client never downloads a binary.",
  );
}

function fromPlatformPackage(): Binary | undefined {
  const name = platformPackage();
  const require_ = createRequire(import.meta.url);
  let manifest: string;
  try {
    manifest = require_.resolve(`${name}/package.json`);
  } catch {
    // An optional dependency npm skipped for this platform, or none published for it.
    return undefined;
  }
  const path = join(manifest, "..", "bin", binaryName());
  if (!isExecutable(path)) {
    throw new BinaryNotFoundError(
      `the ${name} package is installed but holds no executable at ${path}; reinstall it`,
    );
  }
  return { path, source: `${path} (from ${name})` };
}

function fromPath(): Binary | undefined {
  const directories = (process.env["PATH"] ?? "").split(delimiter).filter((entry) => entry !== "");
  for (const directory of directories) {
    const candidate = join(directory, binaryName());
    if (isExecutable(candidate)) {
      return { path: candidate, source: `${candidate} (on $PATH)` };
    }
  }
  return undefined;
}

function isExecutable(path: string): boolean {
  try {
    accessSync(path, constants.X_OK);
    return true;
  } catch {
    return false;
  }
}
