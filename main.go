// An MCP server implementation for Axiom that enables AI agents
// to query data using Axiom Processing Language (APL).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/acrmp/mcp"
	"github.com/peterbourgon/ff/v3"
)

func main() {
	cfg, err := setupConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	tools, err := createTools(cfg)
	if err != nil {
		log.Fatalf("create tools: %v", err)
	}

	info := mcp.Implementation{
		Name:    "axiom-mcp",
		Version: Version,
	}

	s := mcp.NewServer(info, tools)
	s.Serve()
}

// Version information
var (
	Version   = "dev"     // Set by goreleaser
	CommitSHA = "unknown" // Set by goreleaser
	BuildTime = "unknown" // Set by goreleaser
)

// config holds the server configuration parameters
type config struct {
	// Axiom connection settings
	token string // API token for authentication
	url   string // Optional custom API URL
	orgID string // Organization ID

	// Rate limiting configuration
	queryRateLimit    float64 // Maximum queries per second
	queryRateBurst    int     // Maximum burst capacity for queries
	datasetsRateLimit float64 // Maximum dataset list operations per second
	datasetsRateBurst int     // Maximum burst capacity for dataset operations
}

// setupConfig initializes and parses the configuration
func setupConfig() (config, error) {
	fs := flag.NewFlagSet("axiom-mcp", flag.ExitOnError)

	var cfg config
	fs.StringVar(&cfg.token, "token", "", "Axiom API token")
	fs.StringVar(&cfg.url, "url", "", "Axiom API URL (optional)")
	fs.StringVar(&cfg.orgID, "org-id", "", "Axiom organization ID")
	fs.Float64Var(&cfg.queryRateLimit, "query-rate", 1, "Queries per second limit")
	fs.IntVar(&cfg.queryRateBurst, "query-burst", 1, "Query burst capacity")
	fs.Float64Var(&cfg.datasetsRateLimit, "datasets-rate", 1, "Dataset list operations per second")
	fs.IntVar(&cfg.datasetsRateBurst, "datasets-burst", 1, "Dataset list burst capacity")

	var configFile string
	fs.StringVar(&configFile, "config", "", "config file path")

	err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("AXIOM"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	)
	if err != nil {
		return cfg, fmt.Errorf("failed to parse configuration: %w", err)
	}

	if cfg.token == "" {
		return cfg, errors.New("Axiom token must be provided via -token flag, AXIOM_TOKEN env var, or config file")
	}

	return cfg, nil
}
