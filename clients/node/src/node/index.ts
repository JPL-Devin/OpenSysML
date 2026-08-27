// Node entry point: everything the core does, plus the private-child lifecycle.

import { createConnectTransport, createGrpcTransport } from "@connectrpc/connect-node";
import type { Transport } from "@connectrpc/connect";
import { Connection } from "../core/connection.js";
import type { ConnectionBackend, TransportOptions } from "../core/connection.js";
import { OpenSysMLError } from "../core/errors.js";
import { Model } from "../core/model.js";
import type { ParseOptions } from "../core/model.js";
import { baseUrl, encodingOf, interceptors } from "../core/transport.js";
import { acquirePrivateService } from "./service.js";

export * from "../core/index.js";
export { BINARY_ENV, BinaryNotFoundError, binaryName, platformPackage, resolveBinary } from "./binary.js";
export type { Binary } from "./binary.js";
export { PrivateService, currentPrivateService } from "./service.js";

/** Names a service to connect to instead of starting one. */
export const SERVICE_ENV = "OPENSYSML_SERVICE";

/** How this process connects. Without an address it starts a private child. */
export interface ConnectOptions extends TransportOptions {
  /** HTTP version used to reach the service. HTTP/2 (h2c over plain http) by default. */
  httpVersion?: "1.1" | "2";
  /** Wire protocol. Connect by default; gRPC needs HTTP/2 and a service that serves it. */
  protocol?: "connect" | "grpc";
}

/**
 * Connects to a sysml-grpc service.
 *
 * With no address, this starts a private child of this process — one per thread,
 * shared by every connection, stopped when the last one closes. With an address,
 * or with `$OPENSYSML_SERVICE` set, it connects to a service someone else runs and
 * closing the connection leaves that service running.
 */
export async function connect(options: ConnectOptions = {}): Promise<Connection> {
  const address = options.address ?? process.env[SERVICE_ENV];
  if (address !== undefined && address !== "") {
    return connectExternal(address, options);
  }
  return connectPrivate(options);
}

/** Parses a file over a connection of its own, which the model closes. */
export async function load(path: string, options: ConnectOptions & ParseOptions = {}): Promise<Model> {
  const connection = await connect(options);
  try {
    return await Model.parse(connection, { source: { case: "filePath", value: path } }, options, true);
  } catch (error) {
    await connection.close();
    throw error;
  }
}

/** Parses inline source over a connection of its own, which the model closes. */
export async function loads(source: string, options: ConnectOptions & ParseOptions = {}): Promise<Model> {
  const connection = await connect(options);
  try {
    return await Model.parse(connection, { source: { case: "content", value: source } }, options, true);
  } catch (error) {
    await connection.close();
    throw error;
  }
}

async function connectPrivate(options: ConnectOptions): Promise<Connection> {
  const service = await acquirePrivateService();
  const backend: ConnectionBackend = {
    origin: `${service.binary.path}, started by this client`,
    release: () => service.release(),
  };
  try {
    return await Connection.open({
      transport: transportFor(baseUrl(service.address), options),
      encoding: encodingOf(options),
      backend,
      timeoutMs: options.timeoutMs,
    });
  } catch (error) {
    await service.release();
    throw error;
  }
}

async function connectExternal(address: string, options: ConnectOptions): Promise<Connection> {
  const url = baseUrl(address);
  return Connection.open({
    transport: transportFor(url, options),
    encoding: encodingOf(options),
    backend: {
      origin: `${url}, which this client did not start`,
      // A service this client did not start is never stopped by it.
      release: () => Promise.resolve(),
    },
    timeoutMs: options.timeoutMs,
  });
}

function transportFor(url: string, options: ConnectOptions): Transport {
  const binary = encodingOf(options) === "protobuf";
  if (options.protocol === "grpc") {
    if (!binary) {
      throw new OpenSysMLError("the gRPC protocol carries protobuf bodies; JSON needs the Connect protocol");
    }
    return createGrpcTransport({ baseUrl: url, interceptors: interceptors(options) });
  }
  return createConnectTransport({
    baseUrl: url,
    httpVersion: options.httpVersion ?? "2",
    useBinaryFormat: binary,
    interceptors: interceptors(options),
  });
}
