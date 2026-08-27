import { randomBytes } from "node:crypto";
import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";
import {
  FromWebview,
  PickerEntry,
  RENDER_CAPABILITY,
  RENDER_CHANGED_METHOD,
  RENDER_METHOD,
  RenderChangedParams,
  RenderNode,
  RenderResult,
  ToWebview,
  VIEWS_METHOD,
  ViewsResult,
} from "./protocol";

/** The context key the Open Diagram command is enabled by. */
const SUPPORTED_KEY = "opensysml.renderSupported";

/** The type a restored panel is revived under. */
const PANEL_TYPE = "opensysml.diagram";

const PSEUDO_VIEW_LABELS: Record<string, string> = {
  tree: "Model tree",
  interconnection: "Interconnections",
  state: "State machines",
  action: "Action flows",
  table: "Element table",
  sequence: "Message sequence",
};

// This is the historical set for servers that predate the pseudoViews field; do not grow it.
const HISTORICAL_PSEUDO_VIEWS = ["#tree", "#interconnection", "#state", "#action", "#table"];

// Pseudo-views are always offered because a document being written usually declares no view.
function pseudoViewEntries(specs: string[] | undefined): PickerEntry[] {
  return (specs ?? HISTORICAL_PSEUDO_VIEWS).map((value) => {
    const kind = value.startsWith("#") ? value.slice(1) : value;
    return {
      value,
      label: `${PSEUDO_VIEW_LABELS[kind] ?? kind} (no view declared)`,
      supported: true,
    };
  });
}

/**
 * DiagramPanels owns the diagram webviews: one per document, drawn from the
 * server's rendering of it and redrawn when the server says it went stale.
 */
export class DiagramPanels implements vscode.Disposable {
  private readonly panels = new Map<string, DiagramPanel>();
  private readonly disposables: vscode.Disposable[] = [];
  private command: vscode.Disposable | undefined;
  private notification: vscode.Disposable | undefined;
  private client: LanguageClient | undefined;

  constructor(
    private readonly extensionUri: vscode.Uri,
    private readonly output: vscode.OutputChannel,
  ) {
    this.disposables.push(
      vscode.window.onDidChangeTextEditorSelection((event) => {
        this.panels.get(event.textEditor.document.uri.toString())?.highlightAt(event.selections[0].active);
      }),
      vscode.window.registerWebviewPanelSerializer(PANEL_TYPE, {
        deserializeWebviewPanel: async (panel, state: { uri?: string; view?: string } | undefined) => {
          if (!state?.uri) {
            panel.dispose();
            return;
          }
          this.adopt(vscode.Uri.parse(state.uri), panel, state.view ?? "");
        },
      }),
    );
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, false);
  }

  /**
   * attach binds the panels to a started client. The command is registered only
   * when the server advertised the render capability, so an older `sysml-lsp`
   * keeps working without a diagram panel instead of erroring.
   */
  attach(client: LanguageClient | undefined): void {
    this.detach();
    if (!client || !supportsRender(client)) {
      if (client) {
        this.output.appendLine(
          `Language server does not advertise ${RENDER_CAPABILITY}; the diagram panel stays unavailable.`,
        );
      }
      for (const panel of this.panels.values()) {
        panel.fail("The language server does not serve diagrams.");
      }
      return;
    }
    this.client = client;
    this.command = vscode.commands.registerCommand("opensysml.openDiagram", () => this.open());
    this.notification = client.onNotification(RENDER_CHANGED_METHOD, (params: RenderChangedParams) => {
      this.panels.get(vscode.Uri.parse(params.textDocument.uri).toString())?.refresh();
    });
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, true);
    for (const panel of this.panels.values()) {
      panel.refresh();
    }
  }

  /** detach drops what a client owned, so a restart does not leave it behind. */
  detach(): void {
    this.client = undefined;
    this.notification?.dispose();
    this.notification = undefined;
    this.command?.dispose();
    this.command = undefined;
    void vscode.commands.executeCommand("setContext", SUPPORTED_KEY, false);
  }

  dispose(): void {
    this.detach();
    for (const disposable of this.disposables) {
      disposable.dispose();
    }
    for (const panel of [...this.panels.values()]) {
      panel.dispose();
    }
  }

  /** open shows the panel for the active document, beside it. */
  private open(): void {
    const editor = vscode.window.activeTextEditor;
    if (!editor || !isModel(editor.document)) {
      void vscode.window.showInformationMessage("Open a .sysml or .kerml file to draw a diagram of it.");
      return;
    }
    const key = editor.document.uri.toString();
    const existing = this.panels.get(key);
    if (existing) {
      existing.reveal();
      return;
    }
    const panel = vscode.window.createWebviewPanel(
      PANEL_TYPE,
      `Diagram: ${basename(editor.document.uri)}`,
      { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
      { enableScripts: true, localResourceRoots: [vscode.Uri.joinPath(this.extensionUri, "dist")] },
    );
    this.adopt(editor.document.uri, panel, "");
  }

  // adopt takes ownership of a panel, whether it was just created or restored.
  private adopt(docURI: vscode.Uri, panel: vscode.WebviewPanel, selected: string): void {
    const key = docURI.toString();
    this.panels.get(key)?.dispose();
    const diagram = new DiagramPanel(docURI, panel, selected, this.extensionUri, this.output, () => this.client);
    this.panels.set(key, diagram);
    panel.onDidDispose(() => {
      if (this.panels.get(key) === diagram) {
        this.panels.delete(key);
      }
    });
  }
}

/** DiagramPanel is one document's diagram. */
class DiagramPanel {
  private readonly disposables: vscode.Disposable[] = [];
  private selected: string;
  private nodes: RenderNode[] = [];
  private pending = false;
  private again = false;
  private disposed = false;

  constructor(
    private readonly docURI: vscode.Uri,
    private readonly panel: vscode.WebviewPanel,
    selected: string,
    extensionUri: vscode.Uri,
    private readonly output: vscode.OutputChannel,
    private readonly client: () => LanguageClient | undefined,
  ) {
    this.selected = selected;
    this.panel.webview.html = html(this.panel.webview, extensionUri, docURI, selected);
    this.disposables.push(
      this.panel.webview.onDidReceiveMessage((message: FromWebview) => this.receive(message)),
      // A hidden panel is not drawn and not rendered for: the webview is torn
      // down while hidden, so it is refreshed when it comes back.
      this.panel.onDidChangeViewState(() => {
        if (this.panel.visible) {
          this.refresh();
        }
      }),
    );
    this.panel.onDidDispose(() => this.dispose());
  }

  reveal(): void {
    this.panel.reveal(vscode.ViewColumn.Beside, true);
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    for (const disposable of this.disposables) {
      disposable.dispose();
    }
    this.panel.dispose();
  }

  /** fail leaves the last diagram on screen and states why it is out of date. */
  fail(message: string): void {
    this.post({ type: "error", message });
  }

  /**
   * refresh pulls a fresh rendering. Nothing is requested for a hidden panel,
   * which is what the push-notify/pull-artifact protocol is for. A request in
   * flight is not doubled; it is repeated once it settles, since a pick of
   * another view has no notification of its own to redraw it.
   */
  refresh(): void {
    if (this.disposed || !this.panel.visible) {
      return;
    }
    if (this.pending) {
      this.again = true;
      return;
    }
    const client = this.client();
    if (!client) {
      this.fail("The language server is not running.");
      return;
    }
    this.pending = true;
    void this.render(client).finally(() => {
      this.pending = false;
      if (this.again) {
        this.again = false;
        this.refresh();
      }
    });
  }

  private async render(client: LanguageClient): Promise<void> {
    const textDocument = { uri: this.docURI.toString() };
    let listing: ViewsResult | undefined;
    let views: PickerEntry[] = [];
    try {
      listing = await client.sendRequest<ViewsResult>(VIEWS_METHOD, { textDocument });
      views = (listing?.views ?? []).map((info) => ({
        value: info.name,
        label: `${info.name} — ${info.kind}`,
        supported: info.supported,
        reason: info.reason,
      }));
    } catch (err) {
      // The listing is the picker's content, not the diagram: a failure there
      // must not cost the render.
      this.output.appendLine(`Listing the views of ${this.docURI.fsPath} failed: ${errorMessage(err)}`);
      views = [];
    }
    // A document declaring no drawable view is rendered as its model tree, so
    // the panel shows the model being written rather than nothing.
    if (this.selected === "" && !views.some((entry) => entry.supported)) {
      this.selected = "#tree";
    }
    this.post({
      type: "views",
      views: [...views, ...pseudoViewEntries(listing?.pseudoViews)],
      selected: this.selected,
    });
    try {
      // No form is asked for: the server writes the machine form of the kind it
      // rendered, which is Mermaid for a diagram and Markdown for a table.
      const result = await client.sendRequest<RenderResult>(RENDER_METHOD, {
        textDocument,
        view: this.selected === "" ? undefined : this.selected,
      });
      this.nodes = result.nodes ?? [];
      this.post({ type: "render", result, selected: this.selected });
      this.highlightActive();
    } catch (err) {
      this.fail(errorMessage(err));
    }
  }

  /** highlightAt marks the node whose declaration contains the cursor. */
  highlightAt(at: vscode.Position): void {
    this.post({ type: "highlight", id: this.nodeAt(at)?.id });
  }

  private highlightActive(): void {
    const editor = vscode.window.visibleTextEditors.find(
      (candidate) => candidate.document.uri.toString() === this.docURI.toString(),
    );
    if (editor) {
      this.highlightAt(editor.selection.active);
    }
  }

  // nodeAt is the innermost located node whose declaration contains at.
  private nodeAt(at: vscode.Position): RenderNode | undefined {
    let found: RenderNode | undefined;
    let foundRange: vscode.Range | undefined;
    for (const node of this.nodes) {
      if (!node.origin || vscode.Uri.parse(node.origin.uri).toString() !== this.docURI.toString()) {
        continue;
      }
      const range = toRange(node.origin.range);
      if (!range.contains(at)) {
        continue;
      }
      if (!foundRange || foundRange.contains(range)) {
        found = node;
        foundRange = range;
      }
    }
    return found;
  }

  private receive(message: FromWebview): void {
    switch (message.type) {
      case "ready":
        this.refresh();
        return;
      case "pick":
        this.selected = message.view;
        this.refresh();
        return;
      case "reveal":
        void this.revealSource(message.id);
        return;
      case "failed":
        this.fail(message.message);
        return;
    }
  }

  // revealSource opens the declaration a node was built from.
  private async revealSource(id: string): Promise<void> {
    const origin = this.nodes.find((node) => node.id === id)?.origin;
    if (!origin) {
      return;
    }
    // The identifier alone when the server located it: selecting the whole
    // declaration would select the element's entire body.
    const range = toRange(origin.selectionRange ?? origin.range);
    const document = await vscode.workspace.openTextDocument(vscode.Uri.parse(origin.uri));
    const editor = await vscode.window.showTextDocument(document, {
      viewColumn: vscode.ViewColumn.One,
      preserveFocus: false,
    });
    editor.selection = new vscode.Selection(range.start, range.end);
    editor.revealRange(range, vscode.TextEditorRevealType.InCenterIfOutsideViewport);
  }

  private post(message: ToWebview): void {
    if (!this.disposed) {
      void this.panel.webview.postMessage(message);
    }
  }
}

/** supportsRender reports whether the server advertised the render capability. */
function supportsRender(client: LanguageClient): boolean {
  const experimental = client.initializeResult?.capabilities?.experimental as
    | Record<string, unknown>
    | undefined;
  return experimental?.[RENDER_CAPABILITY] === true;
}

function isModel(document: vscode.TextDocument): boolean {
  return document.languageId === "sysml" || document.languageId === "kerml";
}

function basename(uri: vscode.Uri): string {
  const parts = uri.path.split("/");
  return parts[parts.length - 1] || uri.toString();
}

function toRange(range: { start: { line: number; character: number }; end: { line: number; character: number } }): vscode.Range {
  return new vscode.Range(
    new vscode.Position(range.start.line, range.start.character),
    new vscode.Position(range.end.line, range.end.character),
  );
}

function errorMessage(err: unknown): string {
  if (err && typeof err === "object" && "message" in err) {
    return String((err as { message: unknown }).message);
  }
  return String(err);
}

/**
 * html is the panel's document. Scripts are the bundled webview script alone,
 * allowed by nonce, and nothing is loaded from the network: the diagram is drawn
 * by the Mermaid bundled into the extension.
 */
function html(
  webview: vscode.Webview,
  extensionUri: vscode.Uri,
  docURI: vscode.Uri,
  selected: string,
): string {
  const script = webview.asWebviewUri(vscode.Uri.joinPath(extensionUri, "dist", "webview.js"));
  const nonce = randomNonce();
  const csp = [
    "default-src 'none'",
    `img-src ${webview.cspSource} data:`,
    `style-src ${webview.cspSource} 'unsafe-inline'`,
    `font-src ${webview.cspSource} data:`,
    `script-src 'nonce-${nonce}'`,
  ].join("; ");
  const state = attribute(JSON.stringify({ uri: docURI.toString(), view: selected }));
  return `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="${csp}" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>SysML diagram</title>
    <style>
      body { margin: 0; padding: 0.5rem; font-family: var(--vscode-font-family); color: var(--vscode-foreground); }
      #bar { display: flex; align-items: center; gap: 0.5rem; padding-bottom: 0.5rem; }
      #view { flex: 1 1 auto; max-width: 30rem; }
      #kind { opacity: 0.8; font-size: 0.9em; }
      #status { color: var(--vscode-errorForeground); min-height: 1.2em; font-size: 0.9em; white-space: pre-wrap; }
      #diagram { overflow: auto; }
      #diagram.stale { opacity: 0.45; }
      #diagram svg { max-width: 100%; height: auto; }
      #diagram g.opensysml-node { cursor: pointer; }
      /* Mermaid injects its own stylesheet into the SVG, so the highlight has to
         win over it. */
      #diagram .opensysml-selected > rect, #diagram .opensysml-selected > polygon,
      #diagram .opensysml-selected > circle, #diagram .opensysml-selected > path {
        stroke: var(--vscode-focusBorder) !important; stroke-width: 3px !important;
      }
      details { margin-top: 0.75rem; font-size: 0.9em; }
      pre { white-space: pre-wrap; }
    </style>
  </head>
  <body data-state='${state}'>
    <div id="bar">
      <label for="view">View</label>
      <select id="view"></select>
      <span id="kind"></span>
    </div>
    <div id="status"></div>
    <div id="diagram"></div>
    <details id="notices" hidden>
      <summary></summary>
      <ul id="notice-list"></ul>
    </details>
    <details id="undrawable" hidden>
      <summary></summary>
      <ul id="undrawable-list"></ul>
    </details>
    <script nonce="${nonce}" src="${script}"></script>
  </body>
</html>`;
}

// attribute escapes a value written into an HTML attribute. A document path or a
// quoted view name may hold any of these, and one of them would end the value.
function attribute(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/'/g, "&#39;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

// randomNonce is a per-page nonce, so only the script this page shipped runs.
function randomNonce(): string {
  return randomBytes(16).toString("hex");
}
