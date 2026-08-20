package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// pilotLine matches the GNU-format diagnostics the validators print on stderr.
var pilotLine = regexp.MustCompile(`^(.+\.(?:sysml|kerml)):(\d+):(\d+): (error|warning|info|ignore): (.*)$`)

// pilotVersion reports which pilot release the validator was built against,
// read from the wrapper's pinned Maven properties.
func pilotVersion(validator string) (string, error) {
	pom, err := os.ReadFile(filepath.Join(filepath.Dir(validator), "pom.xml"))
	if err != nil {
		return "", fmt.Errorf("read the validator's pom.xml: %w", err)
	}
	tag := mavenProperty(string(pom), "sysml.release.tag")
	version := mavenProperty(string(pom), "sysml.artifact.version")
	if tag == "" || version == "" {
		return "", fmt.Errorf("%s does not pin sysml.release.tag/sysml.artifact.version", validator)
	}
	return fmt.Sprintf("%s (jupyter-sysml-kernel %s)", tag, version), nil
}

func mavenProperty(pom, name string) string {
	open, close := "<"+name+">", "</"+name+">"
	start := strings.Index(pom, open)
	end := strings.Index(pom, close)
	if start < 0 || end < start {
		return ""
	}
	return strings.TrimSpace(pom[start+len(open) : end])
}

// pilotDiagnostics runs the reference validator over the batch and returns its
// diagnostics per file.
//
// The wrapper feeds every input to one accumulating SysMLInteractive session and
// reports each diagnostic under the input's base name, which constrains a batch
// twice: the inputs must be ordered so a file's cross-file imports are processed
// before it, and base names within one invocation must be unique for the output
// to be attributable. Files are therefore ordered by their import dependencies
// and split into invocations with distinct base names.
func pilotDiagnostics(validator, repo, dir string, files []string, timeout time.Duration) (map[string][]diagnostic, error) {
	out := make(map[string][]diagnostic, len(files))
	for _, batch := range batchByBaseName(orderByImports(repo, dir, files)) {
		byBase := make(map[string]string, len(batch))
		args := make([]string, 0, len(batch))
		for _, rel := range batch {
			byBase[path.Base(rel)] = rel
			args = append(args, filepath.Join(repo, dir, rel))
		}
		if err := runPilot(validator, args, byBase, out, timeout); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func kermlDiagnostics(validator, repo, dir string, files []string, timeout time.Duration) (map[string][]diagnostic, error) {
	root, err := filepath.Abs(filepath.Join(repo, dir))
	if err != nil {
		return nil, fmt.Errorf("resolve KerML corpus root: %w", err)
	}

	byPath := make(map[string]string, len(files))
	args := []string{"--root", root}
	for _, rel := range files {
		byPath[rel] = rel
		args = append(args, filepath.Join(root, filepath.FromSlash(rel)))
	}

	out := make(map[string][]diagnostic, len(files))
	if err := runPilot(validator, args, byPath, out, timeout); err != nil {
		return nil, err
	}
	return out, nil
}

func runPilot(validator string, args []string, byBase map[string]string, out map[string][]diagnostic, timeout time.Duration) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, validator, args...)
	cmd.Stdout = nil // "Reading <library file>..." progress noise
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", validator, err)
	}

	var unattributed []string
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		match := pilotLine.FindStringSubmatch(line)
		if match == nil {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "log4j:") {
				unattributed = append(unattributed, line)
			}
			continue
		}
		rel, ok := byBase[match[1]]
		if !ok {
			unattributed = append(unattributed, line)
			continue
		}
		lineNo, err := strconv.Atoi(match[2])
		if err != nil {
			return fmt.Errorf("pilot reported a non-numeric line in %q", line)
		}
		out[rel] = append(out[rel], diagnostic{
			File:     rel,
			Line:     lineNo,
			Severity: match[4],
			Category: categorizePilot(match[5]),
			Message:  match[5],
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read the validator's output: %w", err)
	}

	// Exit 1 only means the batch had errors; anything else is the validator
	// itself failing, and its diagnostics cannot be trusted.
	err = cmd.Wait()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("%s failed (%w); stderr:\n%s", validator, err, strings.Join(unattributed, "\n"))
	}
	for _, line := range unattributed {
		// Never dropped silently: an unattributable diagnostic would otherwise
		// look like agreement.
		fmt.Fprintf(os.Stderr, "pilot output not attributable to a corpus file: %s\n", line)
	}
	return nil
}

// batchByBaseName splits files, in order, into groups whose base names are
// unique: the nth file sharing a base name goes into the nth group.
func batchByBaseName(files []string) [][]string {
	var batches [][]string
	seen := make(map[string]int, len(files))
	for _, rel := range files {
		base := path.Base(rel)
		index := seen[base]
		seen[base] = index + 1
		for len(batches) <= index {
			batches = append(batches, nil)
		}
		batches[index] = append(batches[index], rel)
	}
	return batches
}
