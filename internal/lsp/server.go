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
	ws           *model.Workspace
	client       protocol.Client
	conn         jsonrpc2.Conn
	shutdownDone bool
}

// NewServer returns a Server bound to ws.
func NewServer(ws *model.Workspace) *Server {
	return &Server{ws: ws}
}

// Run wires the server to a stdio-style stream and blocks until the connection
// is done.
//
// The connection is built here rather than with protocol.NewServer because that
// helper already starts a read loop; a second conn.Go would run two concurrent
// readers over the same stream and corrupt the framing.
func (s *Server) Run(ctx context.Context, rwc io.ReadWriteCloser) error {
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(rwc))
	client := protocol.ClientDispatcher(conn, zap.NewNop())
	ctx = protocol.WithClient(ctx, client)
	s.client = client
	s.conn = conn
	conn.Go(ctx, s.changeHandler(protocol.ServerHandler(s, jsonrpc2.MethodNotFoundHandler)))
	<-conn.Done()
	return conn.Err()
}
