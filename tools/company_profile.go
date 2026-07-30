package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"vibinprabin/nepse-mcp/company"
	"vibinprabin/nepse-mcp/utils"
)

func RegisterCompanyProfileTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_company_profile",
		mcp.WithDescription("Full company profile: facts (LTP, EPS, P/E, BV, PBV, 52W range, market cap, shares), fundamentals (26 sector-specific metrics: ROA, ROE, NPL, CD ratio, Base Rate, Capital Fund, DPS, EPS, NetProfit, NII etc), fundamental trends (5-period EPS/NII/NetProfit), and corporate actions (bonus, cash dividend, right, FPO, merger) with dates and percents. One call per stock."),
		mcp.WithString("symbol", mcp.Required(), mcp.Description("Stock symbol (e.g., NABIL)")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		symbol := strings.ToUpper(strings.TrimSpace(request.GetString("symbol", "")))
		if symbol == "" {
			return mcp.NewToolResultError("Symbol is required"), nil
		}

		cc := company.NewClient()
		data, err := cc.GetCompanyData(symbol)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch profile for %s: %v", symbol, err)), nil
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "# %s — %s\n", data.Key, data.Subtitle)
		sb.WriteString("\n")

		sb.WriteString("## Overview\n")
		sb.WriteString("| Metric | Value |\n|---|---|\n")
		fm := data.FactsMap()
		for _, key := range []string{"ltp", "eps", "p/e", "book value", "pbv", "market cap", "paid-up capital", "listed shares", "public shares", "promoter shares", "52 week high low", "high low price", "all time high", "all time low", "float market cap"} {
			if v, ok := fm[key]; ok && v != "" && v != "-" {
				sb.WriteString(fmt.Sprintf("| %s | %s |\n", titleCase(key), v))
			}
		}
		sb.WriteString("\n")

		fundByPeriod := data.FundamentalsByPeriod()
		var periods []string
		for p := range fundByPeriod {
			periods = append(periods, p)
		}
		sort.Slice(periods, func(i, j int) bool {
			if periods[i] == "current" || periods[i] == "Current market snapshot" {
				return true
			}
			if periods[j] == "current" || periods[j] == "Current market snapshot" {
				return false
			}
			return periods[i] > periods[j]
		})

		for _, period := range periods {
			items := fundByPeriod[period]
			if period == "current" || period == "Current market snapshot" {
				continue
			}
			fmt.Fprintf(&sb, "## Fundamentals — %s\n", period)
			sb.WriteString("| Metric | Value |\n|---|---|\n")
			for _, fi := range items {
				val := formatFundValue(fi)
				sb.WriteString(fmt.Sprintf("| %s | %s |\n", fi.Label, val))
			}
			sb.WriteString("\n")
		}

		if data.FundamentalTrends != nil {
			ft := data.FundamentalTrends
			sb.WriteString("## Trends (5-quarter)\n")
			fmt.Fprintf(&sb, "_Source: %s | Updated: %s_\n\n", ft.Source, ft.UpdatedAt)

			periodLabels := make([]string, len(ft.Periods))
			for i, p := range ft.Periods {
				periodLabels[i] = p.Label
			}

			for _, m := range ft.Metrics {
				if !m.Available || len(m.Values) == 0 {
					continue
				}
				fmt.Fprintf(&sb, "**%s**: ", m.Label)
				for i, v := range m.Values {
					if i > 0 {
						sb.WriteString(" → ")
					}
					sb.WriteString(utils.FormatNumber(v.Float()))
				}
				sb.WriteString("\n")
			}

			if ft.Note != "" {
				fmt.Fprintf(&sb, "_%s_\n", ft.Note)
			}
			sb.WriteString("\n")
		}

		actions := data.CorporateActions()
		if len(actions) > 0 {
			sb.WriteString("## Corporate Actions\n")
			sb.WriteString("| Date | Type | Detail |\n|---|---|---|\n")
			for i := len(actions) - 1; i >= 0; i-- {
				a := actions[i]
				pct := ""
				if a.ActionDetails != nil && a.ActionDetails.Percent.Float() > 0 {
					pct = fmt.Sprintf("%.0f%%", a.ActionDetails.Percent.Float())
				}
				typ := actionTypeLabel(a.Type, pct)
				desc := a.Description
				if desc == "" && a.ActionDetails != nil {
					if a.ActionDetails.Percent.Float() > 0 {
						desc = fmt.Sprintf("%.0f%%", a.ActionDetails.Percent.Float())
					}
				}
				if len(desc) > 160 {
					desc = desc[:160] + "…"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", a.Date, typ, desc))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("_Source: LaganiLab LLNMP | Trends: get_price_history for price data_\n")
		return mcp.NewToolResultText(sb.String()), nil
	})
}

func formatFundValue(fi company.FundamentalItem) string {
	if fi.Value == nil {
		return "-"
	}
	switch fi.Format {
	case "number":
		switch v := fi.Value.(type) {
		case float64:
			return utils.FormatNumber(v)
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	case "currency":
		switch v := fi.Value.(type) {
		case float64:
			return utils.FormatCurrency(v)
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	default:
		return fmt.Sprintf("%v", fi.Value)
	}
}

func actionTypeLabel(t, pct string) string {
	switch t {
	case "bonus":
		return "BONUS " + pct
	case "cash_dividend":
		return "DIV " + pct
	case "right":
		return "RIGHT " + pct
	case "fpo":
		return "FPO"
	case "listing":
		return "LISTING"
	case "merger":
		return "MERGER"
	case "acquisition_share_addition":
		return "ACQUISITION"
	case "company_action":
		return "CORP ACTION"
	default:
		return strings.ToUpper(t)
	}
}

func titleCase(s string) string {
	return strings.Title(strings.ToLower(s))
}
