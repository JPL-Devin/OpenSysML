/** Finds the SVG element Mermaid drew a rendering node as. */
export function nodeElement(svg: SVGElement, id: string): SVGElement | undefined {
  // Mermaid's bottom participant box has neither id nor data-id, so it remains inert.
  for (const candidate of svg.querySelectorAll<SVGElement>(
    'g[data-et="participant"][data-id], g.node, g.cluster, g.statediagram-state',
  )) {
    const renderingID = candidate.dataset.id ?? bareID(candidate.id);
    if (renderingID === id) {
      return candidate;
    }
  }
  return undefined;
}

// bareID is the id a Mermaid element was drawn for, without the diagram prefix
// and the ordinal suffix Mermaid adds.
export function bareID(raw: string): string {
  const marked = /^(?:.*-)?(?:flowchart|state|statediagram(?:-state)?)-(.+)$/.exec(raw);
  return (marked ? marked[1] : raw).replace(/-\d+$/, "");
}
