package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"vibinprabin/nepse-mcp/client"
	"vibinprabin/nepse-mcp/utils"
)

func RegisterGraphTools(s *server.MCPServer, c *client.NepseClient) {

	// Merged: get_nepse_index_graph + get_security_graph → get_intraday_graph
	s.AddTool(mcp.NewTool("get_intraday_graph",
		mcp.WithDescription("Intraday OHLC + trend. Omit symbol for NEPSE index. format=points adds ~20 sampled points."),
		mcp.WithString("symbol", mcp.Description("Stock symbol. Omit for NEPSE index.")),
		mcp.WithString("format", mcp.Description("'summary' (default) or 'points'")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := strings.ToUpper(request.GetString("symbol", ""))
		format := request.GetString("format", "summary")

		type DataPoint struct {
			Timestamp int64
			Value     float64
		}

		var points []DataPoint
		var label string

		if symbol == "" {
			graphData, err := c.GetNepseIndexGraph()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed: %v", err)), nil
			}
			for _, p := range graphData {
				points = append(points, DataPoint{Timestamp: p.Timestamp, Value: p.Value})
			}
			label = "NEPSE Index"
		} else {
			graphData, err := c.GetCompanyGraph(symbol)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed: %v", err)), nil
			}
			for _, p := range graphData {
				points = append(points, DataPoint{Timestamp: p.Timestamp, Value: p.Value})
			}
			label = symbol
		}

		if len(points) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No intraday data for %s.", label)), nil
		}

		// Deduplicate consecutive same-timestamp entries, keep last value
		var deduped []DataPoint
		for i, p := range points {
			if i > 0 && p.Timestamp == points[i-1].Timestamp {
				deduped[len(deduped)-1] = p
			} else {
				deduped = append(deduped, p)
			}
		}
		points = deduped

		// Filter zero values
		var nonZero []DataPoint
		for _, p := range points {
			if p.Value > 0 {
				nonZero = append(nonZero, p)
			}
		}
		if len(nonZero) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No intraday data for %s (market closed?).", label)), nil
		}
		points = nonZero

		// Compute OHLC + stats
		open := points[0].Value
		close := points[len(points)-1].Value
		high, low := points[0].Value, points[0].Value
		sum := 0.0
		for _, p := range points {
			if p.Value > high {
				high = p.Value
			}
			if p.Value < low {
				low = p.Value
			}
			sum += p.Value
		}
		vwap := sum / float64(len(points))
		change := close - open
		changePct := 0.0
		if open > 0 {
			changePct = (change / open) * 100
		}

		trend := "RANGING"
		if changePct > 0.5 {
			trend = "BULLISH"
		}
		if changePct > 1.5 {
			trend = "STRONG BULLISH"
		}
		if changePct < -0.5 {
			trend = "BEARISH"
		}
		if changePct < -1.5 {
			trend = "STRONG BEARISH"
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("**%s Intraday**\n", label))
		sb.WriteString(fmt.Sprintf("Open: %s | High: %s | Low: %s | Close: %s\n",
			utils.FormatNumber(open), utils.FormatNumber(high), utils.FormatNumber(low), utils.FormatNumber(close)))
		sb.WriteString(fmt.Sprintf("Change: %s (%s) | VWAP: %s | Trend: %s\n",
			utils.FormatNumber(change), utils.FormatPercentage(changePct), utils.FormatNumber(vwap), trend))
		sb.WriteString(fmt.Sprintf("Range: %s (%.2f%%)\n", utils.FormatNumber(high-low), ((high-low)/low)*100))

		if format == "points" && len(points) > 1 {
			sampleSize := 20
			if len(points) <= sampleSize {
				sampleSize = len(points)
			}
			step := float64(len(points)-1) / float64(sampleSize-1)

			sb.WriteString("\n| Time | Value |\n|---|---|\n")
			loc, _ := time.LoadLocation("Asia/Kathmandu")
			for i := 0; i < sampleSize; i++ {
				idx := int(math.Round(float64(i) * step))
				if idx >= len(points) {
					idx = len(points) - 1
				}
				p := points[idx]
				t := time.Unix(p.Timestamp, 0).In(loc).Format("15:04")
				sb.WriteString(fmt.Sprintf("| %s | %s |\n", t, utils.FormatNumber(p.Value)))
			}
		}

		return mcp.NewToolResultText(sb.String()), nil
	})
}
