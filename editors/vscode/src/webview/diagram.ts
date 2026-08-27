// The diagram panel's script: it draws the Mermaid artifact the server produced,
// reports a node click back, and highlights the node the cursor is in. Mermaid is
// bundled into this script, so nothing is loaded from the network.
import mermaid from "mermaid";

import type { FromWebview, PickerEntry, RenderResult, ToWebview } from "../protocol";
import { nodeElement } from "./nodes";

interface WebviewApi {
  postMessage(message: FromWebview): void;
  setState(state: unknown): void;
  getState(): unknown;
}

declare function acquireVsCodeApi(): WebviewApi;

const vscode = acquireVsCodeApi();
const body = document.body;
const picker = document.getElementById("view") as HTMLSelectElement;
const kindLabel = document.getElementById("kind") as HTMLElement;
const status = document.getElementById("status") as HTMLElement;
const diagram = document.getElementById("diagram") as HTMLElement;
const notices = document.getElementById("notices") as HTMLDetailsElement;
const noticeList = document.getElementById("notice-list") as HTMLElement;

const documentURI = (JSON.parse(body.dataset.state ?? "{}") as { uri?: string }).uri ?? "";
const saved = (vscode.getState() ?? {}) as { view?: string; last?: RenderResult };
let selected = saved.view ?? "";
let last: RenderResult | undefined = saved.last;
let selectedNode: string | undefined;
let drawn = 0;

mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: darkTheme() ? "dark" : "default" });

// The panel is torn down while it is hidden, so the rendering it last drew is
// put back — dimmed until the server answers — rather than showing nothing.
if (last) {
  const restore = drawn + 1;
  void draw(last).then(() => {
    if (restore === drawn) {
      diagram.classList.add("stale");
    }
  });
}

picker.addEventListener("change", () => {
  selected = picker.value;
  remember();
  vscode.postMessage({ type: "pick", view: selected });
});

window.addEventListener("message", (event: MessageEvent<ToWebview>) => {
  const message = event.data;
  switch (message.type) {
    case "views":
      fillPicker(message.views, message.selected);
      return;
    case "render":
      void draw(message.result);
      return;
    case "error":
      showError(message.message);
      return;
    case "highlight":
      highlight(message.id);
      return;
  }
});

vscode.postMessage({ type: "ready" });

// fillPicker lists the document's views and the pseudo-views, marking the ones
// whose rendering kind the server does not produce with why.
function fillPicker(views: PickerEntry[], pick: string): void {
  selected = pick;
  remember();
  picker.replaceChildren();
  for (const entry of views) {
    const option = document.createElement("option");
    option.value = entry.value;
    option.textContent = entry.supported ? entry.label : `${entry.label} (not drawable)`;
    option.disabled = !entry.supported;
    if (entry.reason) {
      option.title = entry.reason;
    }
    picker.append(option);
  }
  picker.value = pick;
}

// draw replaces the diagram with the rendering. A failure to draw leaves the last
// diagram up, dimmed, so a mid-keystroke parse error does not blank the panel.
async function draw(result: RenderResult): Promise<void> {
  // A drawing another one started after is abandoned rather than drawn over it:
  // the restored rendering and the server's answer race on load.
  const generation = ++drawn;
  try {
    if (result.form === "mermaid") {
      const { svg } = await mermaid.render(`opensysml-diagram-${generation}`, result.artifact);
      if (generation !== drawn) {
        return;
      }
      diagram.innerHTML = svg;
      markNodes(result);
      // The drawing replaced the marked node, so what the cursor is in is marked
      // again: a redraw arrives without the cursor having moved.
      highlight(selectedNode);
    } else {
      // A table is written as Markdown rather than drawn, so it is shown as the
      // artifact it is.
      const pre = document.createElement("pre");
      pre.textContent = result.artifact;
      diagram.replaceChildren(pre);
    }
    diagram.classList.remove("stale");
    status.textContent = "";
    last = result;
    remember();
    showNotices(result);
    kindLabel.textContent = describe(result);
  } catch (err) {
    if (generation !== drawn) {
      return;
    }
    const message = err instanceof Error ? err.message : String(err);
    showError(message);
    vscode.postMessage({ type: "failed", message });
  }
}

// describe is what the rendering is, and how its kind was decided when the server
// had to decide it.
function describe(result: RenderResult): string {
  const name = result.view || "no view";
  return result.stated ? `${name} — ${result.kind} (${result.stated})` : `${name} — ${result.kind}`;
}

function showNotices(result: RenderResult): void {
  const list = result.notices ?? [];
  noticeList.replaceChildren();
  notices.hidden = list.length === 0;
  if (list.length === 0) {
    return;
  }
  (notices.firstElementChild as HTMLElement).textContent =
    list.length === 1 ? "1 notice" : `${list.length} notices`;
  for (const notice of list) {
    const item = document.createElement("li");
    item.textContent = notice;
    noticeList.append(item);
  }
}

// showError dims what is on screen rather than clearing it: the diagram shown is
// the last one the model was drawable at, and saying so is more useful than a
// blank panel.
function showError(message: string): void {
  status.textContent = message;
  if (diagram.childElementCount > 0) {
    diagram.classList.add("stale");
  }
}

// markNodes makes each node of the rendering clickable, so clicking it opens the
// declaration it was built from.
function markNodes(result: RenderResult): void {
  const svg = diagram.querySelector("svg");
  if (!svg) {
    return;
  }
  for (const node of result.nodes ?? []) {
    const element = nodeElement(svg, node.id);
    // A node with no locatable declaration — a standard library symbol — has
    // nowhere to go, so it stays inert.
    if (!element || !node.origin) {
      continue;
    }
    element.classList.add("opensysml-node");
    element.dataset.opensysmlId = node.id;
    element.addEventListener("click", () => vscode.postMessage({ type: "reveal", id: node.id }));
  }
}

// highlight marks the node the cursor is in, and only that one. The id is kept so
// a redraw can mark it again.
function highlight(id: string | undefined): void {
  selectedNode = id;
  for (const marked of diagram.querySelectorAll(".opensysml-selected")) {
    marked.classList.remove("opensysml-selected");
  }
  if (!id) {
    return;
  }
  const element = diagram.querySelector(`[data-opensysml-id="${cssEscape(id)}"]`);
  element?.classList.add("opensysml-selected");
}

// cssEscape quotes an id for an attribute selector, since CSS.escape is not in
// every webview host.
function cssEscape(value: string): string {
  return value.replace(/["\\]/g, "\\$&");
}

// remember keeps what the panel is showing, so a window reload restores it.
function remember(): void {
  vscode.setState({ uri: documentURI, view: selected, last });
}

function darkTheme(): boolean {
  return (
    body.classList.contains("vscode-dark") || body.classList.contains("vscode-high-contrast")
  );
}
