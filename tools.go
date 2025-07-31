package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/acrmp/mcp"
	"github.com/axiomhq/axiom-go/axiom"
	"golang.org/x/time/rate"
)

var MCP_USER_AGENT = fmt.Sprintf("mcp-server-axiom/%s", Version)

func createTools(cfg config) ([]mcp.ToolDefinition, error) {
	client, err := axiom.NewClient(
		axiom.SetToken(cfg.token),
		axiom.SetURL(cfg.url),
		axiom.SetOrganizationID(cfg.orgID),
		axiom.SetUserAgent(MCP_USER_AGENT),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Axiom client: %w", err)
	}

	return []mcp.ToolDefinition{
		{
			Metadata: mcp.Tool{
				Name:        "queryApl",
				Description: ptr(queryDescription),
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: mcp.ToolInputSchemaProperties{
						"query": map[string]any{
							"type":        "string",
							"description": "The APL query to run",
						},
					},
				},
			},
			Execute:   newQueryHandler(client),
			RateLimit: rate.NewLimiter(rate.Limit(cfg.queryRateLimit), cfg.queryRateBurst),
		},
		{
			Metadata: mcp.Tool{
				Name:        "listDatasets",
				Description: ptr("List all available Axiom datasets"),
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: mcp.ToolInputSchemaProperties{},
				},
			},
			Execute:   newListDatasetsHandler(client),
			RateLimit: rate.NewLimiter(rate.Limit(cfg.datasetsRateLimit), cfg.datasetsRateBurst),
		},
		{
			Metadata: mcp.Tool{
				Name:        "getDatasetSchema",
				Description: ptr("Get the schema of an Axiom dataset by fetching a single event. This is useful for understanding the structure of the data before writing a query."),
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: mcp.ToolInputSchemaProperties{
						"datasetName": map[string]any{
							"type":        "string",
							"description": "The name of the dataset to get the schema for.",
						},
					},
				},
			},
			Execute:   newGetDatasetSchemaHandler(client),
			RateLimit: rate.NewLimiter(rate.Limit(cfg.queryRateLimit), cfg.queryRateBurst),
		},
		{
			Metadata: mcp.Tool{
				Name:        "getSavedQueries",
				Description: ptr("Retrieve saved/starred queries from Axiom - shows APL queries that users have bookmarked for reuse"),
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: mcp.ToolInputSchemaProperties{},
				},
			},
			Execute:   newGetSavedQueriesHandler(client),
			RateLimit: rate.NewLimiter(rate.Limit(1), 1),
		},
		{
			Metadata: mcp.Tool{
				Name:        "getMonitors",
				Description: ptr("Retrieve monitors from Axiom"),
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: mcp.ToolInputSchemaProperties{},
				},
			},
			Execute:   newGetMonitorsHandler(client),
			RateLimit: rate.NewLimiter(rate.Limit(cfg.monitorsRateLimit), cfg.monitorsRateBurst),
		},
		{
			Metadata: mcp.Tool{
				Name:        "getMonitorsHistory",
				Description: ptr("Get recent check history of monitors. Use getMonitors to list all available monitors first."),
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: mcp.ToolInputSchemaProperties{
						"monitorIds": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Array of monitor IDs to get history for",
						},
					},
				},
			},
			Execute:   newGetMonitorsHistoryHandler(cfg, axiom.DefaultHTTPClient()),
			RateLimit: rate.NewLimiter(rate.Limit(cfg.monitorsRateLimit), cfg.monitorsRateBurst),
		},
	}, nil
}

// ptr returns a pointer to the provided string
func ptr(s string) *string {
	return &s
}

const queryDescription = `
# Instructions
1. Query Axiom datasets using Axiom Processing Language (APL). The query must be a valid APL query string.
2. ALWAYS get the schema of the dataset before running queries rather than guessing.
   You can do this by getting a single event and projecting all fields.
3. Keep in mind that there's a maximum row limit of 65000 rows per query.
4. Prefer aggregations over non aggregating queries when possible to reduce the amount of data returned.
5. Be selective in what you project in each query (unless otherwise needed, like for discovering the schema).
   It's expensive to project all fields.
6. ALWAYS restrict the time range of the query to the smallest possible range that
   meets your needs. This will reduce the amount of data scanned and improve query performance.
7. NEVER guess the schema of the dataset. If you don't where something is, use search first to find in which fields
   it appears.

# Examples
Basic:
- Filter: ['logs'] | where ['severity'] == "error" or ['duration'] > 500ms
- Time range: ['logs'] | where ['_time'] > ago(2h) and ['_time'] < now()
- Project rename: ['logs'] | project-rename responseTime=['duration'], path=['url']

Aggregations:
- Count by: ['logs'] | summarize count() by bin(['_time'], 5m), ['status']
- Multiple aggs: ['logs'] | summarize count(), avg(['duration']), max(['duration']), p95=percentile(['duration'], 95) by ['endpoint']
- Dimensional: ['logs'] | summarize dimensional_analysis(['isError'], pack_array(['endpoint'], ['status']))
- Histograms: ['logs'] | summarize histogram(['responseTime'], 100) by ['endpoint']
- Distinct: ['logs'] | summarize dcount(['userId']) by bin_auto(['_time'])

Search & Parse:
- Search all: search "error" or "exception"
- Parse logs: ['logs'] | parse-kv ['message'] as (duration:long, error:string) with (pair_delimiter=",")
- Regex extract: ['logs'] | extend errorCode = extract("error code ([0-9]+)", 1, ['message'])
- Contains ops: ['logs'] | where ['message'] contains_cs "ERROR" or ['message'] startswith "FATAL"

Data Shaping:
- Extend & Calculate: ['logs'] | extend duration_s = ['duration']/1000, success = ['status'] < 400
- Dynamic: ['logs'] | extend props = parse_json(['properties']) | where ['props.level'] == "error"
- Pack/Unpack: ['logs'] | extend fields = pack("status", ['status'], "duration", ['duration'])
- Arrays: ['logs'] | where ['url'] in ("login", "logout", "home") | where array_length(['tags']) > 0

Advanced:
- Make series: ['metrics'] | make-series avg(['cpu']) default=0 on ['_time'] step 1m by ['host']
- Join: ['errors'] | join kind=inner (['users'] | project ['userId'], ['email']) on ['userId']
- Union: union ['logs-app*'] | where ['severity'] == "error"
- Fork: ['logs'] | fork (where ['status'] >= 500 | as errors) (where ['status'] < 300 | as success)
- Case: ['logs'] | extend level = case(['status'] >= 500, "error", ['status'] >= 400, "warn", "info")

Time Operations:
- Bin & Range: ['logs'] | where ['_time'] between(datetime(2024-01-01)..now())
- Multiple time bins: ['logs'] | summarize count() by bin(['_time'], 1h), bin(['_time'], 1d)
- Time shifts: ['logs'] | extend prev_hour = ['_time'] - 1h

String Operations:
- String funcs: ['logs'] | extend domain = tolower(extract("://([^/]+)", 1, ['url']))
- Concat: ['logs'] | extend full_msg = strcat(['level'], ": ", ['message'])
- Replace: ['logs'] | extend clean_msg = replace_regex("(password=)[^&]*", "\\1***", ['message'])

Common Patterns:
- Error analysis: ['logs'] | where ['severity'] == "error" | summarize error_count=count() by ['error_code'], ['service']
- Status codes: ['logs'] | summarize requests=count() by ['status'], bin_auto(['_time']) | where ['status'] >= 500
- Latency tracking: ['logs'] | summarize p50=percentile(['duration'], 50), p90=percentile(['duration'], 90) by ['endpoint']
- User activity: ['logs'] | summarize user_actions=count() by ['userId'], ['action'], bin(['_time'], 1h)`

// ErrEmptyQuery indicates that an empty query was provided
var ErrEmptyQuery = errors.New("query must not be empty")

// newQueryHandler creates a handler for executing APL queries
func newQueryHandler(client *axiom.Client) func(mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
	return func(params mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
		query, ok := params.Arguments["query"].(string)
		if !ok {
			return mcp.CallToolResult{}, errors.New("invalid query parameter type")
		}

		if len(query) == 0 {
			return mcp.CallToolResult{}, ErrEmptyQuery
		}

		ctx := context.Background()
		result, err := client.Query(ctx, query)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("query execution failed: %w", err)
		}

		jsonData, err := json.Marshal(result)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
		}

		return mcp.CallToolResult{
			Content: []any{
				mcp.TextContent{
					Text: string(jsonData),
					Type: "text",
				},
			},
		}, nil
	}
}

// newListDatasetsHandler creates a handler for listing available datasets
func newListDatasetsHandler(client *axiom.Client) func(mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
	return func(params mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
		ctx := context.Background()
		datasets, err := client.Datasets.List(ctx)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to list datasets: %w", err)
		}

		jsonData, err := json.Marshal(datasets)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
		}

		return mcp.CallToolResult{
			Content: []any{
				mcp.TextContent{
					Text: string(jsonData),
					Type: "text",
				},
			},
		}, nil
	}
}

// newGetDatasetSchemaHandler creates a handler for getting complete dataset schema
func newGetDatasetSchemaHandler(client *axiom.Client) func(mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
	return func(params mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
		datasetName, ok := params.Arguments["datasetName"].(string)
		if !ok || datasetName == "" {
			return mcp.CallToolResult{}, fmt.Errorf("datasetName parameter is required")
		}

		ctx := context.Background()

		query := fmt.Sprintf("['%s'] | getschema", datasetName)
		result, err := client.Query(ctx, query)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to get dataset schema: %w", err)
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
		}

		return mcp.CallToolResult{
			Content: []any{
				mcp.TextContent{
					Text: string(jsonData),
					Type: "text",
				},
			},
		}, nil
	}
}

// SavedQuery represents a starred query from the v2/apl-starred-queries endpoint
type SavedQuery struct {
	Dataset string `json:"dataset"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Query   struct {
		Apl string `json:"apl"`
	} `json:"query"`
	Who string `json:"who"`
	ID  string `json:"id"`
}

// newGetSavedQueriesHandler creates a handler for retrieving saved queries
func newGetSavedQueriesHandler(client *axiom.Client) func(mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
	return func(params mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
		ctx := context.Background()

		var res []SavedQuery
		if err := client.Call(ctx, http.MethodGet, "/v2/apl-starred-queries?limit=100&who=all", nil, &res); err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to execute request: %w", err)
		}

		jsonData, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
		}

		return mcp.CallToolResult{
			Content: []any{
				mcp.TextContent{
					Text: string(jsonData),
					Type: "text",
				},
			},
		}, nil
	}
}

// newGetMonitorsHandler creates a handler for retrieving monitors
func newGetMonitorsHandler(client *axiom.Client) func(mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
	return func(params mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
		ctx := context.Background()

		res, err := client.Monitors.List(ctx)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to list monitors: %w", err)
		}

		jsonData, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
		}

		return mcp.CallToolResult{
			Content: []any{
				mcp.TextContent{
					Text: string(jsonData),
					Type: "text",
				},
			},
		}, nil
	}
}

// newGetMonitorsHistoryHandler creates a handler for retrieving monitor history
func newGetMonitorsHistoryHandler(cfg config, httpClient *http.Client) func(mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
	return func(params mcp.CallToolRequestParams) (mcp.CallToolResult, error) {
		ctx := context.Background()

		// Parse monitor IDs from parameters
		monitorIdsRaw, ok := params.Arguments["monitorIds"]
		if !ok {
			return mcp.CallToolResult{}, fmt.Errorf("monitorIds parameter is required")
		}

		monitorIdsSlice, ok := monitorIdsRaw.([]any)
		if !ok {
			return mcp.CallToolResult{}, fmt.Errorf("monitorIds must be an array")
		}

		// Convert to string slice and filter/trim
		var monitorIds []string
		for _, id := range monitorIdsSlice {
			if idStr, ok := id.(string); ok {
				trimmed := strings.TrimSpace(idStr)
				if trimmed != "" {
					monitorIds = append(monitorIds, trimmed)
				}
			}
		}

		if len(monitorIds) == 0 {
			return mcp.CallToolResult{}, fmt.Errorf("at least one valid monitor ID is required")
		}

		// Convert API URL to App URL for internal endpoint
		baseURL := strings.Replace(cfg.url, "://api.", "://app.", 1)
		fullURL := fmt.Sprintf("%s/api/internal/monitors/history?monitorIds=%s", baseURL,
			strings.Join(monitorIds, ","))

		req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+cfg.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-AXIOM-ORG-ID", cfg.orgID)

		resp, err := httpClient.Do(req)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to execute request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)

			// permission errors
			if resp.StatusCode == 403 {
				return mcp.CallToolResult{
					Content: []any{
						mcp.TextContent{
							Text: fmt.Sprintf("⚠️ Permission denied: Your API token does not have access to monitor history.\n\nNote: Monitor history uses an internal API endpoint that may require special permissions.\n\nMonitor IDs requested: %v\n\nError details: %s", monitorIds, string(body)),
							Type: "text",
						},
					},
				}, nil
			}

			return mcp.CallToolResult{}, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return mcp.CallToolResult{}, fmt.Errorf("failed to read response body: %w", err)
		}

		var parsed any
		if err := json.Unmarshal(body, &parsed); err != nil {
			return mcp.CallToolResult{
				Content: []any{
					mcp.TextContent{Text: string(body), Type: "text"},
				},
			}, nil
		}

		pretty, _ := json.MarshalIndent(parsed, "", "  ")
		return mcp.CallToolResult{
			Content: []any{
				mcp.TextContent{Text: string(pretty), Type: "text"},
			},
		}, nil
	}
}
