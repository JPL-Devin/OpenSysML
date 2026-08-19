// The custom methods the OpenSysML language server adds for diagrams, and the
// payloads they carry. They mirror internal/lsp/render.go.

export const RENDER_METHOD = "opensysml/render";
export const VIEWS_METHOD = "opensysml/views";
export const RENDER_CHANGED_METHOD = "opensysml/renderChanged";

/** The capability the server advertises when it serves the render methods. */
export const RENDER_CAPABILITY = "openSysmlRender";

export interface Position {
  line: number;
  character: number;
}

export interface Range {
  start: Position;
  end: Position;
}

/**
 * Where an element was declared: `range` is the whole declaration, `selectionRange`
 * the declared identifier alone, which is where clicking a node goes.
 */
export interface RenderOrigin {
  uri: string;
  range: Range;
  selectionRange?: Range;
}

export interface RenderNode {
  id: string;
  kind: string;
  name: string;
  detail: string;
  parent?: string;
  origin?: RenderOrigin;
}

export interface RenderEdge {
  from: string;
  to: string;
  label: string;
  kind: string;
  origin?: RenderOrigin;
}

export interface RenderRow {
  cells: string[];
  origin?: RenderOrigin;
}

export interface RenderParams {
  textDocument: { uri: string };
  view?: string;
  form?: string;
}

export interface RenderResult {
  view: string;
  kind: string;
  stated: string;
  form: string;
  artifact: string;
  nodes: RenderNode[];
  edges: RenderEdge[];
  rows?: RenderRow[];
  columns?: string[];
  notices: string[];
  version: number;
}

export interface ViewsParams {
  textDocument: { uri: string };
}

export interface ViewInfo {
  name: string;
  kind: string;
  supported: boolean;
  reason?: string;
}

export interface ViewsResult {
  views: ViewInfo[];
}

export interface RenderChangedParams {
  textDocument: { uri: string };
  version: number;
}

/** One entry of the panel's view picker. */
export interface PickerEntry {
  /** What is sent as the render request's `view`. */
  value: string;
  label: string;
  /** False for a view whose rendering kind the server does not produce. */
  supported: boolean;
  reason?: string;
}

/** A message the extension sends the webview. */
export type ToWebview =
  | { type: "render"; result: RenderResult; selected: string }
  | { type: "views"; views: PickerEntry[]; selected: string }
  | { type: "error"; message: string }
  | { type: "highlight"; id: string | undefined };

/** A message the webview sends the extension. */
export type FromWebview =
  | { type: "ready" }
  | { type: "reveal"; id: string }
  | { type: "pick"; view: string }
  | { type: "failed"; message: string };
