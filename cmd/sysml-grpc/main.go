// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// sysml-grpc starts a gRPC server exposing SysML v2 parsing, resolution, and semantic services.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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
		port       = flag.Int("port", 50051, "gRPC server port")
		healthPort = flag.Int("health-port", 8081, "Health check HTTP port")
		cacheSize  = flag.Int("cache-size", 100, "Maximum number of cached parsed files")
		logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		showVer    = flag.Bool("version", false, "Show version and exit")
		transport  = flag.String("transport", transportConnect,
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
	svc, err := sysmlgrpc.NewService(*cacheSize, Version)
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

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

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

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(),
		),
	)
	pb.RegisterSysMLServiceServer(grpcServer, svc)
	reflection.Register(grpcServer) // Enable server reflection for grpcurl/grpcui

	go func() {
		slog.Info("gRPC server listening", "addr", lis.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdown
	slog.Info("Shutting down gracefully...")
	svc.Close()

	stopHealthServer(healthSrv)

	// Stop gRPC server
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		slog.Info("gRPC server stopped")
	case <-time.After(30 * time.Second):
		slog.Warn("Forcing gRPC server shutdown after timeout")
		grpcServer.Stop()
	}
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

// loggingInterceptor logs each gRPC call with method name and duration.
func loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		dur := time.Since(start)

		if err != nil {
			slog.Error("gRPC call failed",
				"method", info.FullMethod,
				"duration_ms", dur.Milliseconds(),
				"error", err,
			)
		} else {
			slog.Debug("gRPC call",
				"method", info.FullMethod,
				"duration_ms", dur.Milliseconds(),
			)
		}
		return resp, err
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
