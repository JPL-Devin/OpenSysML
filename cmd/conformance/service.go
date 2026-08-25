package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// service is a sysml-grpc the runner started, and the connection to it.
type service struct {
	binary  string
	process *exec.Cmd
	conn    *grpc.ClientConn
	log     *os.File
	// exited closes when the process has been reaped, so an early crash is seen
	// rather than waited out. waitErr is read only after it closes.
	exited  chan struct{}
	waitErr error
}

// buildService builds cmd/sysml-grpc, so a run tests the working tree rather
// than whatever binary happens to be installed.
func buildService(repoRoot, outDir string) (string, error) {
	binary := filepath.Join(outDir, "sysml-grpc")
	// #nosec G204 -- the command is fixed; only the output path varies.
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/sysml-grpc")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("building cmd/sysml-grpc: %w", err)
	}
	return binary, nil
}

// startService starts the service on a free port and waits until it answers.
func startService(ctx context.Context, binary, logPath string, cacheSize int) (*service, error) {
	port, healthPort, err := freePorts()
	if err != nil {
		return nil, err
	}
	log, err := os.Create(logPath) // #nosec G304 -- logPath is the runner's own temporary directory.
	if err != nil {
		return nil, err
	}
	// #nosec G204 -- binary is what the runner just built, or the one named on the command line.
	process := exec.CommandContext(ctx, binary,
		"-port", fmt.Sprint(port),
		"-health-port", fmt.Sprint(healthPort),
		"-cache-size", fmt.Sprint(cacheSize),
		"-log-level", "error",
	)
	process.Stdout = log
	process.Stderr = log
	if err := process.Start(); err != nil {
		_ = log.Close()
		return nil, fmt.Errorf("starting %s: %w", binary, err)
	}

	svc := &service{binary: binary, process: process, log: log, exited: make(chan struct{})}
	go func() {
		svc.waitErr = process.Wait()
		close(svc.exited)
	}()
	target := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials())) //nolint:staticcheck // grpc.NewClient postdates the pinned grpc version
	if err != nil {
		svc.stop()
		return nil, err
	}
	svc.conn = conn
	if err := svc.waitReady(ctx, target, logPath); err != nil {
		svc.stop()
		return nil, err
	}
	return svc, nil
}

// waitReady calls GetServerInfo until the service answers it.
func (s *service) waitReady(ctx context.Context, target, logPath string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-s.exited:
			return fmt.Errorf("the service exited before answering (%v); its log is %s", s.waitErr, logPath)
		default:
		}
		call, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, lastErr = s.call(call, "GetServerInfo", nil)
		cancel()
		if lastErr == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("the service on %s did not answer GetServerInfo (%v); its log is %s", target, lastErr, logPath)
}

// call invokes one method by name, with a request built for its input type. A
// nil request sends an empty message.
func (s *service) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	descriptor, err := methodByName(method)
	if err != nil {
		return nil, err
	}
	if request == nil {
		request = dynamicpb.NewMessage(descriptor.Input())
	}
	response := dynamicpb.NewMessage(descriptor.Output())
	path := fmt.Sprintf("/%s/%s", serviceName, descriptor.Name())
	if err := s.conn.Invoke(ctx, path, request.Interface(), response.Interface()); err != nil {
		return nil, err
	}
	return response, nil
}

// stop closes the connection and terminates the service.
func (s *service) stop() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.process != nil && s.process.Process != nil {
		_ = s.process.Process.Kill()
		<-s.exited
	}
	if s.log != nil {
		_ = s.log.Close()
	}
}

// freePorts reserves two distinct ports. Both listeners stay open until both
// ports are chosen, so the second cannot be the port the first just released.
func freePorts() (int, int, error) {
	var listeners []net.Listener
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()
	var ports []int
	for range 2 {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return 0, 0, err
		}
		listeners = append(listeners, listener)
		ports = append(ports, listener.Addr().(*net.TCPAddr).Port)
	}
	return ports[0], ports[1], nil
}
