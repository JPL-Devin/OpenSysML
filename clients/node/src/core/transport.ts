// Pieces both entry points share: how an address becomes a base URL, and the
// interceptors a connection installs.

import type { Interceptor } from "@connectrpc/connect";
import type { Encoding, ResponseTap, TransportOptions } from "./connection.js";
import { OpenSysMLError } from "./errors.js";

/** Turns `host:port`, `http://host:port` or a full URL into a Connect base URL. */
export function baseUrl(address: string): string {
  const trimmed = address.trim();
  if (trimmed === "") {
    throw new OpenSysMLError("the service address is empty");
  }
  const withScheme = /^https?:\/\//.test(trimmed) ? trimmed : `http://${trimmed}`;
  let url: URL;
  try {
    url = new URL(withScheme);
  } catch (cause) {
    throw new OpenSysMLError(`${JSON.stringify(address)} is not a service address`, { cause });
  }
  if (url.port === "" && !/^https?:\/\//.test(trimmed)) {
    throw new OpenSysMLError(
      `${JSON.stringify(address)} names no port; a sysml-grpc address is host:port`,
    );
  }
  return url.origin + url.pathname.replace(/\/$/, "");
}

/** The interceptors a connection installs for its headers and its response tap. */
export function interceptors(options: TransportOptions): Interceptor[] {
  const built: Interceptor[] = [];
  const headers = options.headers;
  if (headers !== undefined) {
    built.push((next) => (request) => {
      for (const [name, value] of Object.entries(headers)) {
        request.header.set(name, value);
      }
      return next(request);
    });
  }
  const tap = options.onResponse;
  if (tap !== undefined) {
    built.push(responseTap(tap));
  }
  return built;
}

/** Protobuf unless the caller asked for JSON. */
export function encodingOf(options: TransportOptions): Encoding {
  return options.encoding ?? "protobuf";
}

function responseTap(tap: ResponseTap): Interceptor {
  return (next) => async (request) => {
    const response = await next(request);
    if (!response.stream) {
      tap({ method: request.method.name, response: response.message });
    }
    return response;
  };
}
