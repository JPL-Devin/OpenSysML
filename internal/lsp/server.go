package lsp

import (
	"context"
	"io"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"

	"github.com/Open-MBEE/Systemica/internal/core/model"
)

// Server is the SysML v2 language server. It wraps a single model.Workspace and
// serves the Language Server Protocol over a jsonrpc2 stream.
type Server struct {
	baseServer
	ws     *model.Workspace
	client protocol.Client
}

// NewServer returns a Server bound to ws.
func NewServer(ws *model.Workspace) *Server {
	return &Server{ws: ws}
}

// Run wires the server to a stdio-style stream and blocks until the connection
// is done.
func (s *Server) Run(ctx context.Context, rwc io.ReadWriteCloser) error {
	stream := jsonrpc2.NewStream(rwc)
	ctx, conn, client := protocol.NewServer(ctx, s, stream, zap.NewNop())
	s.client = client
	conn.Go(ctx, s.changeHandler(protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)))
	<-conn.Done()
	return conn.Err()
}
