// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Transports this binary can serve. The default is the grpc-go server this
// service has always been; connect is an evaluation prototype documented in
// docs/internals/design/transport-evaluation.md.
const (
	transportGRPC    = "grpc"
	transportConnect = "connect"
	transportStdio   = "stdio"
)

// connectHandler builds the routes a Connect server answers: every RPC of
// SysMLService under /sysml.SysMLService/, gRPC server reflection, and the
// health endpoint that otherwise needs a port of its own.
func connectHandler(svc *sysmlgrpc.Service, ver string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(
		sysmlgrpc.NewConnectAdapter(svc),
		connect.WithInterceptors(connectLoggingInterceptor()),
	))

	// grpcurl and grpcui ask the reflection service for the schema, which
	// connect-go serves from a package of its own rather than in the handler.
	reflector := grpcreflect.NewStaticReflector(protoconnect.SysMLServiceName)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	mux.Handle("/health", healthHandler(ver))
	return mux
}

// serveConnect serves gRPC, gRPC-Web and the Connect protocol on one port until
// the context is cancelled. h2c carries HTTP/2 in cleartext, which is how an
// existing gRPC client reaches an address that offers no TLS.
func serveConnect(ctx context.Context, lis net.Listener, svc *sysmlgrpc.Service, ver string) error {
	srv := &http.Server{
		Handler:           h2c.NewHandler(connectHandler(svc, ver), &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("Connect server listening",
			"addr", lis.Addr(), "protocols", "grpc, grpc-web, connect")
		errs <- srv.Serve(lis)
	}()

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("connect server shutdown: %w", err)
	}
	slog.Info("Connect server stopped")
	return nil
}

// connectLoggingInterceptor logs each call with its procedure and duration, as
// the grpc-go path's interceptor does.
func connectLoggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			res, err := next(ctx, req)
			dur := time.Since(start)
			if err != nil {
				slog.Error("Connect call failed",
					"procedure", req.Spec().Procedure,
					"protocol", req.Peer().Protocol,
					"duration_ms", dur.Milliseconds(),
					"error", err,
				)
			} else {
				slog.Debug("Connect call",
					"procedure", req.Spec().Procedure,
					"protocol", req.Peer().Protocol,
					"duration_ms", dur.Milliseconds(),
				)
			}
			return res, err
		}
	}
}
