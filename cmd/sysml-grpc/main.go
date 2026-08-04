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

	pb "github.com/Open-MBEE/Systemica/api/proto"
	sysmlgrpc "github.com/Open-MBEE/Systemica/internal/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	var (
		port       = flag.Int("port", 50051, "gRPC server port")
		healthPort = flag.Int("health-port", 8081, "Health check HTTP port")
		cacheSize  = flag.Int("cache-size", 100, "Maximum number of cached parsed files")
		logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		showVer    = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("sysml-grpc version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built: %s\n", buildDate)
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
		"version", version,
		"commit", commit,
		"buildDate", buildDate,
	)

	// Create gRPC service (cache is internal to the service)
	svc := sysmlgrpc.NewService(*cacheSize)

	// Start health check server
	healthSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *healthPort),
		Handler: healthHandler(version),
	}
	go func() {
		slog.Info("Health check server listening", "addr", healthSrv.Addr)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health check server failed", "error", err)
		}
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		slog.Error("Failed to listen", "port", *port, "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(),
		),
	)
	pb.RegisterSysMLServiceServer(grpcServer, svc)
	reflection.Register(grpcServer) // Enable server reflection for grpcurl/grpcui

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("gRPC server listening", "addr", lis.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdown
	slog.Info("Shutting down gracefully...")

	// Stop health check server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := healthSrv.Shutdown(ctx); err != nil {
		slog.Warn("Health check server shutdown error", "error", err)
	} else {
		slog.Info("Health check server stopped")
	}

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
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "sysml-grpc",
			"version": ver,
		})
	})
	return mux
}
