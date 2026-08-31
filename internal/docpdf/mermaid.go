package docpdf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// renderDiagrams renders each Mermaid block to an SVG in dir with the pinned
// mermaid-cli, returning the image file names in block order. A document
// without diagrams needs no diagram tool at all.
func renderDiagrams(dir string, blocks []block) ([]string, error) {
	var sources []string
	for _, blk := range blocks {
		if blk.Kind == blockMermaid {
			sources = append(sources, blk.Source)
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	mmdc, err := mermaidTool.locate("")
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, len(sources))
	for i, source := range sources {
		input := fmt.Sprintf("diagram-%d.mmd", i+1)
		output := fmt.Sprintf("diagram-%d.svg", i+1)
		if err := os.WriteFile(filepath.Join(dir, input), []byte(source+"\n"), 0o600); err != nil {
			return nil, err
		}
		args := []string{"--input", input, "--output", output, "--quiet"}
		if config := strings.TrimSpace(os.Getenv(MermaidPuppeteerEnv)); config != "" {
			args = append(args, "--puppeteerConfigFile", config)
		}
		if err := runTool(dir, mmdc, args...); err != nil {
			return nil, err
		}
		if _, err := os.Stat(filepath.Join(dir, output)); err != nil {
			return nil, &Error{Kind: ErrorToolFailed, Tool: mermaidTool.name, Detail: "wrote no SVG for " + input}
		}
		images = append(images, output)
	}
	return images, nil
}

// markdownWithImages rewrites the document's Markdown with each Mermaid fence
// replaced by a reference to its rendered image, for converters that read
// Markdown themselves.
func markdownWithImages(markdown string, images []string) string {
	lines := strings.Split(markdown, "\n")
	var out []string
	image := 0
	for i := 0; i < len(lines); i++ {
		if lines[i] == "```mermaid" && image < len(images) {
			for i++; i < len(lines) && lines[i] != "```"; i++ {
			}
			out = append(out, "![diagram]("+images[image]+")")
			image++
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}
