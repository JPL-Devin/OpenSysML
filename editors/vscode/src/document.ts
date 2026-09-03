import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import {
  DOCUMENTS_METHOD,
  DocumentsResult,
  RENDER_DOCUMENT_CAPABILITY,
  RENDER_DOCUMENT_METHOD,
  RenderDocumentResult,
} from "./protocol";

/** The context key the Render Document command is enabled by. */
const SUPPORTED_KEY = "opensysml.renderDocumentSupported";

/**
 * DocumentRendering owns the Render Document command: a quick pick of the
 * workspace's document definitions, rendered by the server to Markdown and
 * opened in a preview beside the editor.
 */
export class DocumentRendering implements vscode.Disposable {
  private command: vscode.Disposable | undefined;
  private client: LanguageClient | undefined;

  constructor(private readonly output: vscode.OutputChannel) {
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, false);
  }

  /**
   * attach binds the command to a started client. It is registered only when
   * the server advertised the capability, so an older `sysml-lsp` keeps
   * working without the command instead of erroring.
   */
  attach(client: LanguageClient | undefined): void {
    this.detach();
    if (!client || !supportsRenderDocument(client)) {
      if (client) {
        this.output.appendLine(
          `Language server does not advertise ${RENDER_DOCUMENT_CAPABILITY}; the Render Document command stays unavailable.`,
        );
      }
      return;
    }
    this.client = client;
    this.command = vscode.commands.registerCommand("opensysml.renderDocument", () => this.render());
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, true);
  }

  /** detach drops what a client owned, so a restart does not leave it behind. */
  detach(): void {
    this.client = undefined;
    this.command?.dispose();
    this.command = undefined;
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, false);
  }

  dispose(): void {
    this.detach();
  }

  private async render(): Promise<void> {
    const client = this.client;
    if (!client) {
      return;
    }
    let name: string;
    try {
      const listing = await client.sendRequest<DocumentsResult>(DOCUMENTS_METHOD, {});
      const documents = listing?.documents ?? [];
      if (documents.length === 0) {
        void vscode.window.showInformationMessage(
          "The workspace declares no documents. One is a part def specializing DocumentQueries::Document.",
        );
        return;
      }
      const picked = await vscode.window.showQuickPick(
        documents.map((doc) => ({
          label: doc.name,
          description: basename(doc.uri),
        })),
        { placeHolder: "Document to render" },
      );
      if (!picked) {
        return;
      }
      name = picked.label;
    } catch (err) {
      void vscode.window.showErrorMessage(`Listing documents failed: ${errorMessage(err)}`);
      return;
    }
    try {
      const result = await client.sendRequest<RenderDocumentResult>(RENDER_DOCUMENT_METHOD, { name });
      const document = await vscode.workspace.openTextDocument({
        language: "markdown",
        content: result.markdown,
      });
      await vscode.window.showTextDocument(document, { viewColumn: vscode.ViewColumn.Beside });
      await vscode.commands.executeCommand("markdown.showPreview", document.uri);
    } catch (err) {
      void vscode.window.showErrorMessage(`Rendering ${name} failed: ${errorMessage(err)}`);
    }
  }
}

/** supportsRenderDocument reports whether the server advertised the capability. */
function supportsRenderDocument(client: LanguageClient): boolean {
  const experimental = client.initializeResult?.capabilities?.experimental as
    | Record<string, unknown>
    | undefined;
  return experimental?.[RENDER_DOCUMENT_CAPABILITY] === true;
}

function basename(uri: string): string {
  return uri.split("/").at(-1) || uri;
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    return String((err as { message: unknown }).message);
  }
  return String(err);
}
