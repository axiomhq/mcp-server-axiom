# mcp-server-axiom

[![smithery badge](https://smithery.ai/badge/axiom-mcp)](https://smithery.ai/server/axiom-mcp)

A [Model Context Protocol](https://modelcontextprotocol.io/) server implementation for [Axiom](https://axiom.co) that enables AI agents to query your data using Axiom Processing Language (APL).

## Status

Works with Claude desktop app. Implements two MCP [tools](https://modelcontextprotocol.io/docs/concepts/tools):

- queryApl: Execute APL queries against Axiom datasets
- listDatasets: List available Axiom datasets

No support for MCP [resources](https://modelcontextprotocol.io/docs/concepts/resources) or [prompts](https://modelcontextprotocol.io/docs/concepts/prompts) yet.

## Installation

### Installing via Smithery

To install Axiom MCP Server for Claude Desktop automatically via [Smithery](https://smithery.ai/server/axiom-mcp):

```bash
npx -y @smithery/cli install axiom-mcp --client claude
```

### Releases

Download the latest built binary from the [releases page](https://github.com/axiomhq/axiom-mcp/releases).

### Source

```bash
go install github.com/axiomhq/axiom-mcp@latest
```

## Configuration

Configure using one of these methods:

### Config File Example (config.txt):
```txt
token xaat-your-token
url https://api.axiom.co
query-rate 1
query-burst 1
datasets-rate 1
datasets-burst 1
```

### Command Line Flags:
```bash
axiom-mcp \
  -token xaat-your-token \
  -url https://api.axiom.co \
  -query-rate 1 \
  -query-burst 1 \
  -datasets-rate 1 \
  -datasets-burst 1
```

### Environment Variables:
```bash
export AXIOM_TOKEN=xaat-your-token
export AXIOM_URL=https://api.axiom.co
export AXIOM_ORG_ID=your-org-id
export AXIOM_QUERY_RATE=1
export AXIOM_QUERY_BURST=1
export AXIOM_DATASETS_RATE=1
export AXIOM_DATASETS_BURST=1
```

## Usage

1. Create a config file:
```bash
echo "token xaat-your-token" > config.txt
```

2. Configure the Claude app to use the MCP server:

```bash
code ~/Library/Application\ Support/Claude/claude_desktop_config.json
```

```json
{
  "mcpServers": {
    "axiom": {
      "command": "/path/to/your/axiom-mcp-binary",
      "args" : ["--config", "/path/to/your/config.txt"],
      "env": { // Alternatively, you can set the environment variables here
        "AXIOM_TOKEN": "xaat-your-token",
        "AXIOM_URL": "https://api.axiom.co",
        "AXIOM_ORG_ID": "your-org-id"
      }
    }
  }
}
```

## License

MIT License - see LICENSE file
