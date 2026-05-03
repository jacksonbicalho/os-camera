package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"camera/internal/ffprobe"
)

type probeInput struct {
	URL string `json:"url" jsonschema:"RTSP or media stream URL to probe"`
}

func main() {
	prober := ffprobe.NewProber(&ffprobe.OSExecutor{})

	s := mcp.NewServer(&mcp.Implementation{Name: "ffprobe", Version: "v1.0.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "probe_stream",
		Description: "Run ffprobe on an RTSP or media stream URL and return stream/format metadata as JSON.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in probeInput) (*mcp.CallToolResult, any, error) {
		out, err := prober.Probe(ctx, in.URL)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
		}, nil, nil
	})

	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
