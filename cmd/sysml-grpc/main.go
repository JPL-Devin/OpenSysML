// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// sysml-grpc starts a gRPC server exposing SysML v2 parsing, resolution, and semantic services.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

// Build metadata, set by the linker: the names match the -X flags the Makefile
// and the release build pass, so a released binary reports what it was built from.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	var (
		port          = flag.Int("port", 50051, "gRPC server port")
		healthPort    = flag.Int("health-port", 8081, "Health check HTTP port")
		cacheSize     = flag.Int("cache-size", 100, "Maximum number of cached parsed files")
		logLevel      = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		showVer       = flag.Bool("version", false, "Show version and exit")
		reportAddress = flag.Bool("report-address", false,
			"Print the address to dial on stdout, as one line, once the listener is "+
				"bound; with -port 0 this is how a client learns the port the kernel chose")
		exitWithParent = flag.Bool("exit-with-parent", false,
			"Exit at end of file on stdin, so a child cannot outlive the process that "+
				"holds the write end of a pipe on it, SIGKILL included")
		transport = flag.String("transport", transportConnect,
			"Transport to serve: connect (default; gRPC, gRPC-Web and Connect on one port), "+
				"grpc (grpc-go only), or stdio (an evaluation prototype over stdin/stdout)")
		corsOrigins = flag.String("cors-allowed-origins", "", "Comma-separated exact origins allowed for browser CORS")
		tlsCert     = flag.String("tls-cert", "", "TLS certificate file for the main server")
		tlsKey      = flag.String("tls-key", "", "TLS private key file for the main server")
	)
	flag.Parse()

	switch *transport {
	case transportGRPC, transportConnect, transportStdio:
	default:
		fmt.Fprintf(os.Stderr, "sysml-grpc: unknown -transport %q; want grpc, connect or stdio\n", *transport)
		os.Exit(2)
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		fmt.Fprintln(os.Stderr, "sysml-grpc: -tls-cert and -tls-key must be supplied together")
		os.Exit(2)
	}
	origins, err := parseCORSOrigins(*corsOrigins)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sysml-grpc: %v\n", err)
		os.Exit(2)
	}

	if *transport == transportStdio {
		switch {
		case *exitWithParent:
			fmt.Fprintln(os.Stderr,
				"sysml-grpc: -exit-with-parent is not for -transport stdio, whose stdin is "+
					"the session and already ends it at end of file")
			os.Exit(2)
		case *reportAddress:
			fmt.Fprintln(os.Stderr,
				"sysml-grpc: -report-address is not for -transport stdio, which binds no "+
					"address and speaks the protocol itself on stdout")
			os.Exit(2)
		}
	}

	if *showVer {
		fmt.Printf("sysml-grpc version %s\n", Version)
		fmt.Printf("commit: %s\n", Commit)
		fmt.Printf("built: %s\n", BuildTime)
		os.Exit(0)
	}

	// Configure logging
	var lvl slog.Level
	switch *logLevel {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)

	slog.Info("Starting sysml-grpc server",
		"version", Version,
		"commit", Commit,
		"buildTime", BuildTime,
		"transport", *transport,
	)

	// Create gRPC service (cache is internal to the service)
	unavailable := unavailableCapabilitiesForTesting()
	var svc *sysmlgrpc.Service
	if len(unavailable) == 0 {
		svc, err = sysmlgrpc.NewService(*cacheSize, Version)
	} else {
		svc, err = sysmlgrpc.NewServiceWithUnavailableCapabilitiesForTesting(
			*cacheSize, Version, unavailable)
	}
	if err != nil {
		slog.Error("Invalid service configuration", "error", err)
		os.Exit(1)
	}

	// Build the standard library indexes in the background, so the first model to
	// arrive does not pay for the library and startup stays prompt.
	svc.Prewarm()

	if *transport == transportStdio {
		// The pipe is the session: there is no port to bind, nothing to poll
		// for readiness, and the client's exit closes stdin, which ends this.
		os.Exit(runStdio(svc))
	}

	// Start health check server
	var healthSrv *http.Server
	if *healthPort != 0 {
		healthSrv = &http.Server{
			Addr:              fmt.Sprintf(":%d", *healthPort),
			Handler:           healthHandler(Version),
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("Health check server listening", "addr", healthSrv.Addr)
			if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("Health check server failed", "error", err)
			}
		}()
		if *transport == transportConnect {
			slog.Warn("the separate health listener is deprecated; /health is served on the main port and -health-port 0 disables it",
				"health_port", *healthPort)
		}
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		slog.Error("Failed to listen", "port", *port, "error", err)
		os.Exit(1)
	}

	// The port is the kernel's under -port 0, so report the bound one before
	// serving: a client waiting on this line cannot miss the address it must dial.
	if *reportAddress {
		if _, err := fmt.Fprintln(os.Stdout, dialAddress(lis.Addr())); err != nil {
			slog.Error("Failed to report the listening address", "error", err)
			os.Exit(1)
		}
	}

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	if *exitWithParent {
		go exitWhenStdinCloses(shutdown)
	}

	if *transport == transportConnect {
		serveCtx, cancelServe := context.WithCancel(context.Background())
		served := make(chan error, 1)
		go func() { served <- serveConnect(serveCtx, lis, svc, Version, origins, *tlsCert, *tlsKey) }()

		select {
		case err := <-served:
			cancelServe()
			if err != nil {
				slog.Error("Connect server failed", "error", err)
				stopHealthServer(healthSrv)
				svc.Close()
				os.Exit(1)
			}
		case <-shutdown:
			slog.Info("Shutting down gracefully...")
			cancelServe()
			if err := <-served; err != nil {
				slog.Warn("Connect server shutdown error", "error", err)
			}
		}
		svc.Close()
		stopHealthServer(healthSrv)
		return
	}

	runGRPC(lis, svc, shutdown)
	stopHealthServer(healthSrv)
}

// dialAddress renders a listening address as one a client can dial: a wildcard
// host answers on the loopback address, which is where a private child is wanted.
func dialAddress(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	switch host {
	case "", "::", "0.0.0.0":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// exitWhenStdinCloses asks for shutdown once stdin reaches end of file, which is
// what the pipe on it does when the process holding the other end dies, however
// it dies. Reading also discards anything sent, so a writer cannot block.
func exitWhenStdinCloses(shutdown chan<- os.Signal) {
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		slog.Debug("stdin read ended", "error", err)
	}
	slog.Info("stdin closed, exiting with the process that started this service")
	shutdown <- syscall.SIGTERM
}

// stopHealthServer shuts the health check server down, allowing it a moment to
// finish the requests it has in hand.
func stopHealthServer(healthSrv *http.Server) {
	if healthSrv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := healthSrv.Shutdown(ctx); err != nil {
		slog.Warn("Health check server shutdown error", "error", err)
	} else {
		slog.Info("Health check server stopped")
	}
}

// healthHandler returns an HTTP handler that responds to /health with JSON status.
func healthHandler(ver string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "sysml-grpc",
			"version": ver,
		}); err != nil {
			slog.Warn("health response write failed", "error", err)
		}
	})
	return mux
}
