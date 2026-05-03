package mcpsrv

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// Version is reported in the MCP server handshake.
const Version = "0.1.0"

// New builds an MCP server wired to the given store. Tools are registered up
// front; the caller drives the lifecycle by passing the server to Run on a
// transport (typically StdioTransport).
func New(db domain.Store) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "worklog",
		Title:   "Worklog",
		Version: Version,
	}, nil)
	addProjectTools(s, db)
	addTaskTools(s, db)
	addLogTools(s, db)
	addActivityTools(s, db)
	addReportTools(s, db)
	return s
}
