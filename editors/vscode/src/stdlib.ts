import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import {
  STDLIB_CONTENT_CAPABILITY,
  STDLIB_CONTENT_METHOD,
  STDLIB_SCHEME,
  StdlibContentResult,
} from "./protocol";

/**
 * StdlibDocuments serves the `sysml-stdlib:` documents the server locates
 * standard-library declarations in, so a definition, reference or diagram
 * origin in the library opens as a read-only editor. The text comes from the
 * server, which holds the bundled library.
 */
export class StdlibDocuments implements vscode.TextDocumentContentProvider, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<vscode.Uri>();
  readonly onDidChange = this.changed.event;
  private readonly registration: vscode.Disposable;
  private client: LanguageClient | undefined;

  constructor(private readonly output: vscode.OutputChannel) {
    // Registered once for the session: the provider outlives server restarts,
    // and asks whichever client is attached when a document is opened.
    this.registration = vscode.workspace.registerTextDocumentContentProvider(STDLIB_SCHEME, this);
  }

  /** attach binds the provider to a started client that serves the request. */
  attach(client: LanguageClient | undefined): void {
    this.client = client && supportsStdlibContent(client) ? client : undefined;
    if (client && !this.client) {
      this.output.appendLine(
        `Language server does not advertise ${STDLIB_CONTENT_CAPABILITY}; standard library documents cannot be opened.`,
      );
    }
    // A document opened while no server ran is re-read from the new one.
    for (const doc of vscode.workspace.textDocuments) {
      if (doc.uri.scheme === STDLIB_SCHEME) {
        this.changed.fire(doc.uri);
      }
    }
  }

  detach(): void {
    this.client = undefined;
  }

  dispose(): void {
    this.detach();
    this.registration.dispose();
    this.changed.dispose();
  }

  async provideTextDocumentContent(uri: vscode.Uri, token: vscode.CancellationToken): Promise<string> {
    const client = this.client;
    if (!client) {
      throw new Error("The SysML v2 language server is not running, so the standard library cannot be read.");
    }
    const result = await client.sendRequest<StdlibContentResult>(
      STDLIB_CONTENT_METHOD,
      { uri: uri.toString() },
      token,
    );
    return result.text;
  }
}

function supportsStdlibContent(client: LanguageClient): boolean {
  const experimental = client.initializeResult?.capabilities.experimental as
    | Record<string, unknown>
    | undefined;
  return experimental?.[STDLIB_CONTENT_CAPABILITY] === true;
}
