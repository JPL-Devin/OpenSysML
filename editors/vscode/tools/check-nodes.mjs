import { mkdir, mkdtemp, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { tmpdir } from "node:os";
import { build } from "esbuild";
import { JSDOM } from "jsdom";

const outputDir = process.env.NODE_CHECK_OUTPUT
  ? resolve(process.env.NODE_CHECK_OUTPUT)
  : await mkdtemp(resolve(tmpdir(), "opensysml-node-check-"));
const nodesPath = resolve(new URL("../src/webview/nodes.ts", import.meta.url).pathname);
const compiled = await build({
  bundle: true,
  entryPoints: [nodesPath],
  format: "esm",
  platform: "node",
  write: false,
});
const { nodeElement } = await import(
  `data:text/javascript;base64,${Buffer.from(compiled.outputFiles[0].text).toString("base64")}`,
);

const dom = new JSDOM("<!doctype html><html><body></body></html>", {
  pretendToBeVisual: true,
});
const { window } = dom;
if (!window.SVGElement.prototype.getBBox) {
  window.SVGElement.prototype.getBBox = function () {
    const text = this.textContent ?? "";
    return { height: 16, width: text.length * 8, x: 0, y: 0 };
  };
}
for (const name of [
  "window",
  "document",
  "navigator",
  "DOMParser",
  "XMLSerializer",
  "Node",
  "Element",
  "HTMLElement",
  "SVGElement",
  "SVGSVGElement",
  "CSSStyleSheet",
  "requestAnimationFrame",
  "cancelAnimationFrame",
]) {
  Object.defineProperty(globalThis, name, {
    configurable: true,
    value: window[name],
    writable: true,
  });
}
globalThis.getComputedStyle = window.getComputedStyle.bind(window);

const { default: mermaid } = await import("mermaid");
mermaid.initialize({
  deterministicIds: true,
  securityLevel: "strict",
  startOnLoad: false,
});

const diagrams = [
  {
    id: "sequence-node-check",
    source: `sequenceDiagram
  participant n0 as part producer
  participant n1 as part server
  participant n2 as part consumer
  n2->>n1: subscribe_message
  n0->>n1: publish_message
  n1->>n2: deliver_message
`,
    nodes: ["n0", "n1", "n2"],
  },
  {
    id: "flowchart-node-check",
    source: `flowchart TD
  n0[part producer]
  n1[part server]
  n0-->n1
`,
    nodes: ["n0", "n1"],
  },
];

for (const diagram of diagrams) {
  const { svg } = await mermaid.render(diagram.id, diagram.source);
  const output = resolve(outputDir, `${diagram.id}.svg`);
  await mkdir(dirname(output), { recursive: true });
  await writeFile(output, svg, "utf8");

  const container = document.createElement("div");
  container.innerHTML = svg;
  const svgElement = container.querySelector("svg");
  if (!svgElement) {
    throw new Error(`${diagram.id}: Mermaid returned no SVG element`);
  }
  for (const id of diagram.nodes) {
    const element = nodeElement(svgElement, id);
    if (!element) {
      throw new Error(`${diagram.id}: nodeElement did not find ${id}`);
    }
    console.log(
      `${diagram.id}: ${id} -> <${element.tagName.toLowerCase()} id="${element.id}" data-id="${element.dataset.id ?? ""}" class="${element.getAttribute("class") ?? ""}">`,
    );
  }
  if (nodeElement(svgElement, "no-such-node")) {
    throw new Error(`${diagram.id}: nodeElement matched a nonexistent node`);
  }
  console.log(`${diagram.id}: ${output}`);
}
