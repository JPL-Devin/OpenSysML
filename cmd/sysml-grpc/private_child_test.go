// How a client starts a service of its own: a port from the kernel, reported on
// stdout, and a lifetime bounded by the pipe on stdin.
package main

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// child is a service started the way a private client starts one: it holds the
// write end of the pipe on its stdin, and reads the address from its stdout.
type child struct {
	t     *testing.T
	cmd   *exec.Cmd
	stdin io.WriteCloser
	out   *bufio.Reader
	exit  chan error
}

// startChild starts the built binary with a stdin pipe and args. The pipes are
// this test's own rather than exec's, so waiting on the exit does not close them
// under a read: the exit is what several of these tests are about.
func startChild(t *testing.T, args ...string) *child {
	t.Helper()
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	cmd := exec.Command(serviceBinary(t), args...)
	cmd.Stdin, cmd.Stdout = inRead, outWrite
	c := &child{t: t, cmd: cmd, stdin: inWrite, out: bufio.NewReader(outRead), exit: make(chan error, 1)}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the service: %v", err)
	}
	// The child holds the ends it was given; this process keeps only the ends it
	// drives, so an end of file means the child's, not a copy left open here.
	_ = inRead.Close()
	_ = outWrite.Close()
	go func() { c.exit <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = inWrite.Close()
		_ = cmd.Process.Kill()
		<-c.exit
		_ = outRead.Close()
	})
	return c
}

// reportedAddress reads the one line the service prints once it is bound.
func (c *child) reportedAddress(timeout time.Duration) string {
	c.t.Helper()
	type read struct {
		line string
		err  error
	}
	lines := make(chan read, 1)
	go func() {
		line, err := c.out.ReadString('\n')
		lines <- read{strings.TrimSpace(line), err}
	}()
	select {
	case got := <-lines:
		if got.err != nil {
			c.t.Fatalf("reading the reported address: %v", got.err)
		}
		return got.line
	case <-time.After(timeout):
		c.t.Fatalf("no address reported within %s", timeout)
		return ""
	}
}

// waitExit waits for the process to end, failing the test if it outlives the
// timeout: a service still running is the orphan this flag exists to prevent.
func (c *child) waitExit(timeout time.Duration) error {
	c.t.Helper()
	select {
	case err := <-c.exit:
		c.exit <- err // the cleanup drains it, so exactly one Wait ever runs
		return err
	case <-time.After(timeout):
		c.t.Fatalf("the service was still running %s after its stdin closed", timeout)
		return nil
	}
}

// A client that asks for port 0 is told the port it got, and the service is
// serving at that address by the time the line is readable.
func TestReportedAddressIsTheKernelsChoiceAndServesRPCs(t *testing.T) {
	c := startChild(t, "-port", "0", "-health-port", "0", "-report-address")
	addr := c.reportedAddress(30 * time.Second)

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("reported address %q: %v", addr, err)
	}
	if host != "127.0.0.1" {
		t.Errorf("reported host = %q, want the loopback address a client can dial", host)
	}
	if port == "0" || port == "50051" {
		t.Errorf("reported port = %q, want one the kernel assigned", port)
	}

	client := dial(t, addr)
	info, err := client.GetServerInfo(callContext(t), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo on the reported address: %v", err)
	}
	if info.GetVersion() == "" {
		t.Error("the service answered with no version")
	}
}

// The guarantee the Python client relies on: the service dies with the process
// holding the write end of the pipe, whatever becomes of that process.
func TestClosingStdinEndsTheService(t *testing.T) {
	c := startChild(t, "-port", "0", "-health-port", "0", "-report-address", "-exit-with-parent")
	addr := c.reportedAddress(30 * time.Second)

	if err := c.stdin.Close(); err != nil {
		t.Fatalf("closing the service's stdin: %v", err)
	}
	if err := c.waitExit(30 * time.Second); err != nil {
		t.Errorf("exiting at end of file on stdin: %v, want a clean exit", err)
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = conn.Close()
		t.Errorf("%s still accepts connections after the service exited", addr)
	}
}

// Without the flag the service keeps serving, so the flag is what couples the
// lifetimes rather than something the pipe does on its own.
func TestWithoutTheFlagAClosedStdinLeavesTheServiceServing(t *testing.T) {
	c := startChild(t, "-port", "0", "-health-port", "0", "-report-address")
	addr := c.reportedAddress(30 * time.Second)

	if err := c.stdin.Close(); err != nil {
		t.Fatalf("closing the service's stdin: %v", err)
	}
	select {
	case err := <-c.exit:
		c.exit <- err
		t.Fatalf("the service exited at end of file on stdin without -exit-with-parent: %v", err)
	case <-time.After(2 * time.Second):
	}

	client := dial(t, addr)
	if _, err := client.GetServerInfo(callContext(t), &pb.ServerInfoRequest{}); err != nil {
		t.Errorf("GetServerInfo after stdin closed: %v", err)
	}
}

// Neither flag means anything for a transport whose stdin is the session, so
// asking is refused rather than silently ignored.
func TestTheLifecycleFlagsAreRefusedForStdio(t *testing.T) {
	for _, flag := range []string{"-report-address", "-exit-with-parent"} {
		t.Run(flag, func(t *testing.T) {
			out, err := exec.Command(serviceBinary(t), "-transport", "stdio", flag).CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("running with %s: %v, want a nonzero exit", flag, err)
			}
			if exit.ExitCode() != 2 {
				t.Errorf("exit code = %d, want 2 for a usage error", exit.ExitCode())
			}
			if !strings.Contains(string(out), flag+" is not for -transport stdio") {
				t.Errorf("output = %q, want it to name %s and stdio", out, flag)
			}
		})
	}
}
