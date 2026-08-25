// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// No shipped client speaks stdio, so the binary's wiring of -transport stdio is
// what would rot unnoticed.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestStdioTransportAnswersAFramedCall(t *testing.T) {
	cmd := exec.Command(serviceBinary(t), "-transport", "stdio")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	logs := &strings.Builder{}
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the service: %v", err)
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"GetServerInfo","params":{}}`
	frame := fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(body), body)
	if _, err := stdin.Write([]byte(frame)); err != nil {
		t.Fatalf("writing the call: %v", err)
	}

	answer := readAnswer(t, stdout)
	// The pipe closing is how a supervisor ends a session, and the process is
	// expected to exit on it rather than hang.
	if err := stdin.Close(); err != nil {
		t.Fatalf("closing stdin: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("the service exited with %v, logs:\n%s", err, logs)
	}

	var res struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(answer, &res); err != nil {
		t.Fatalf("decoding %q: %v", answer, err)
	}
	if res.Error != nil {
		t.Fatalf("GetServerInfo failed: %s", res.Error.Message)
	}
	if res.JSONRPC != "2.0" {
		t.Errorf("answer jsonrpc = %q, want 2.0", res.JSONRPC)
	}
	if !strings.Contains(string(res.Result), "version") {
		t.Errorf("result = %s, want the server's version in it", res.Result)
	}
}

// readAnswer reads one Content-Length framed body off the server's stdout.
func readAnswer(t *testing.T, r io.Reader) []byte {
	t.Helper()
	reader := textproto.NewReader(bufio.NewReader(r))
	header, err := reader.ReadMIMEHeader()
	if err != nil {
		t.Fatalf("reading the answer's headers: %v", err)
	}
	length, err := strconv.Atoi(header.Get("Content-Length"))
	if err != nil {
		t.Fatalf("Content-Length %q: %v", header.Get("Content-Length"), err)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader.R, body); err != nil {
		t.Fatalf("reading the answer's body: %v", err)
	}
	return body
}
