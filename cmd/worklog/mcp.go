package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/mcpsrv"
	"github.com/snezhinskiy/worklog/internal/store"
)

// runMCP serves the worklog MCP server over stdio. It is dispatched from main
// when the first positional arg is "mcp" so the user can wire `worklog mcp`
// into Claude Desktop / mcp.json without juggling two binaries.
func runMCP(args []string) {
	fs := flag.NewFlagSet("worklog mcp", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db (sqlite)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db:", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := mcpsrv.New(db)
	if err := srv.Run(ctx, &mcpsdk.StdioTransport{}); err != nil && !isCleanShutdown(ctx, err) {
		fmt.Fprintln(os.Stderr, "mcp:", err)
		os.Exit(1)
	}
}

// isCleanShutdown reports whether err represents the client closing stdin or
// a signal cancelling ctx — both routine exits, not failures worth reporting.
func isCleanShutdown(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	// The SDK wraps the EOF in a string ("server is closing: EOF"), so the
	// errors.Is check above misses it. Fall back to substring match.
	return strings.HasSuffix(err.Error(), "EOF")
}
