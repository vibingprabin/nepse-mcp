package tools

import (
	"strings"
	"testing"
)

func TestGuideOverviewListsTools(t *testing.T) {
	for _, s := range []string{
		"get_broker_floorsheet", "analyze_broker_sentiment", "get_price_history",
		"get_stock_valuation", "get_valuation_screener", "get_news", "get_market_sentiment",
		"get_movers_screen",
	} {
		if !strings.Contains(guideOverview, s) {
			t.Errorf("guideOverview missing %q", s)
		}
	}
}

func TestGuideNoSlop(t *testing.T) {
	// Guides carry data and call patterns, not behavior mandates or output templates.
	// The model forms its own thesis; these phrases are banned from the guide layer.
	banned := []string{
		"Frame:", "Verdict shape", "Standing rules", "Discipline",
		"you must", "MANDATORY", "REJECT", "name the frame", "CAPITAL-GAINS question",
	}
	for name, g := range map[string]string{
		"guideOverview":       guideOverview,
		"guideMovers":         guideMovers,
		"guidePipeline":       guidePipeline,
		"guideSpeculation":    guideSpeculation,
		"guideValue":        guideValue,
		"guideSentimentTrack": guideSentimentTrack,
		"guideSurface":      guideSurface,
		"guideWorkflows":    guideWorkflows,
		"guideBroker":       guideBroker,
		"guideMarket":       guideMarket,
		"guideAnsu":         guideAnsu,
		"guideNews":         guideNews,
		"guideReliability":  guideReliability,
	} {
		for _, b := range banned {
			if strings.Contains(g, b) {
				t.Errorf("%s still contains %q", name, b)
			}
		}
	}
}

func TestGuideTrackDataLeads(t *testing.T) {
	if !strings.Contains(guideSpeculation, "get_price_history") {
		t.Error("speculation guide should point at the data to pull")
	}
	if !strings.Contains(guideSpeculation, "catalyst") {
		t.Error("speculation guide should mention catalyst as part of the trade case")
	}
	if !strings.Contains(guideValue, "get_stock_valuation") {
		t.Error("value guide should point at the data to pull")
	}
	if !strings.Contains(guideValue, "why the gap exists") {
		t.Error("value guide should raise why-the-gap as a question")
	}
	if !strings.Contains(guideSentimentTrack, "get_market_sentiment") {
		t.Error("sentiment guide should point at fear/greed data")
	}
}

func TestGuidePipelineTree(t *testing.T) {
	for _, s := range []string{
		"analyze_flow_change", "BUYABILITY", "Evidence axes", "Structure", "Hands",
		"Catalyst", "Regime", "History", "Valuation", "BUYABLE", "NOT-NOW",
		"falsifier", "Hypothesis first",
	} {
		if !strings.Contains(guidePipeline, s) {
			t.Errorf("guidePipeline missing %q", s)
		}
	}
}

func TestGuideValueCaveatFirst(t *testing.T) {
	// Valuation must be framed as a counterweight, never as the case.
	if !strings.Contains(guideValue, "neither blind nor deaf") {
		t.Error("value guide must carry the market-isn't-blind caveat")
	}
	if !strings.Contains(guidePipeline, "market is neither blind nor deaf") {
		t.Error("pipeline must frame valuation as a counterweight")
	}
	if !strings.Contains(guidePipeline, "get_stock_valuation / get_valuation_screener LAST") {
		t.Error("valuation must be the terminal layer in the pipeline")
	}
	// Buyability: valuation must never be a veto.
	if !strings.Contains(guidePipeline, "never a veto") {
		t.Error("pipeline must permit buyable verdicts in valuation-negative names")
	}
}

func TestGuideBrokerNeutral(t *testing.T) {
	if strings.Contains(guideBroker, "overhead supply") {
		t.Error("guideBroker must not frame underwater holders as overhead supply")
	}
}

func TestGuideWorkflowsReconciled(t *testing.T) {
	if !strings.Contains(guideWorkflows, "Sentiment") && !strings.Contains(guideWorkflows, "sentiment") {
		t.Error("guideWorkflows missing sentiment pattern")
	}
	if strings.Contains(guideWorkflows, "layers") {
		t.Error("guideWorkflows still references the removed layers topic")
	}
}

func TestGuideMoversPipeline(t *testing.T) {
	for _, s := range []string{
		"get_top_list", "get_live_market_data", "analyze_broker_sentiment",
		"get_broker_floorsheet", "get_price_history", "get_news", "Float",
		"volume expansion",
	} {
		if !strings.Contains(guideMovers, s) {
			t.Errorf("guideMovers missing %q (pipeline step)", s)
		}
	}
	if strings.Contains(guideMovers, "overhead supply") {
		t.Error("guideMovers must not frame underwater holders as overhead supply")
	}
}
