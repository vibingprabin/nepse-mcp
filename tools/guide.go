package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const guideMovers = `# Movers — capital-gains pipeline

Names with the biggest upside are usually the ones already moving: volume expansion
on a small float, persistent accumulation, a catalyst. Deep-discount names can be
cheap because nothing moves them — the opposite of a capital-gains candidate.

## Screen — where the money is
- get_movers_screen(limit) — one call: top turnover/volume/gainers cross-referenced with float %, volume, 52W position, ranked
- get_top_list(type='turnover') and (type='volume') — who is trading
- get_top_list(type='gainers') — who is rising
- get_live_market_data(symbols, limit) — the raw tape; filter it by hand when your angle isn't in the top lists

## Filter — what separates a move from a trap
- Float / public share %: a small float squeezes; check get_security_details / get_company_profile
- Volume vs its own normal: recent volume far above the 50D average = attention
- Price position: fresh breakout above a base, or off a washout low
- Sector rotation: which sectors are receiving money this month (get_market_summary)

## Analyze — does the move have legs
- get_price_history(start_date, end_date, limit≈90, include_analysis=true) — 20-session gain, range, avg vol, volatility
- analyze_broker_sentiment(period_days=7) and (14) and (30) — persistent accumulators vs churn
- get_broker_floorsheet(view='inferred', top_n) — who holds at what cost; recent buyers near the price = fuel
- Breakevens above the price are prior buyers who may sell into a rally — informational, not a verdict. A name off its highs has underwater holders by definition; a name near its highs has everyone in profit, which is its own risk. Fresh momentum off a base is the setup; extension near highs is the risk.
- get_news(company=symbol) — the catalyst, or the absence of one
- get_market_sentiment(history_days) — the regime; momentum needs liquidity

## Rank
Potential rises with: float tightness, volume expansion, flow conviction (one-sided
accumulation, not churn), and a live catalyst. The exit constraint: position vs
average volume — the trade needs to be exitable.`

const guideOverview = `# NEPSE MCP Guide

Sources: NEPSE official API (market, prices, lists, depth, floorsheet, graphs) + LaganiLab broker model v2 (broker flow, inferred positions, fear & greed; coverage from 2014-05-05).

Tools (call with exact param names):
- Market: get_market_summary(include_sectors), get_live_market_data(symbols, limit), get_market_sentiment(history_days, index_key)
- Securities: search_securities(query, sector, limit), get_security_details(symbol), get_company_profile(symbol), compare_securities(symbols)
- Price: get_price_history(symbol, start_date, end_date, limit, sort, include_analysis), get_market_depth(symbol), get_floor_sheet(symbol, limit), get_intraday_graph(symbol, format)
- Lists: get_top_list(type, limit)
- Movers: get_movers_screen(limit) — capital-gains screen: top lists cross-referenced with float %, volume, 52W position
- Broker: get_broker_floorsheet(symbol, view, top_n, from_date, to_date — dates optional, default trailing 30d), analyze_broker_sentiment(symbol, period_days, show_brokers)
- Ansu Invest: get_stock_valuation(symbol), get_valuation_screener(sector, limit), get_research_articles(symbol, limit, include_images), get_research_article(slug, include_images)
- News: get_news(source, category, company, query, limit, page), get_news_article(url, source, include_images) — sources: sharesansar, merolagani, stockssessions, pulse (company disclosures)

Topics: overview | pipeline | movers | speculation | value | sentiment | surface | workflows | broker | market | ansu | news | reliability`

const guidePipeline = `# BUYABILITY — the research domain (default orientation)

You are an independent research agent: a synthesis machine, not a value
analyst, not a momentum bot. The question is always: is this stock BUYABLE,
and from what style? The pipeline below is a DEFAULT path, not a cage — skip
nodes, revisit them, or recombine them as the question demands. Combine
evidence axes until they agree or you can name the conflict. The market does
not hand out multifold gains on many names at once; your edge is finding the
few where the pieces align — and being willing to commit when they do.

## Evidence axes (six lenses)
- Structure — get_price_history(limit≈90, include_analysis) + get_security_details:
  base vs breakout vs extension; 52W position, float %, listing age, volume
  pattern. A parabolic run (>+50%/20d) or fresh listing (<12wk) is HIGH-RISK:
  label it, size down or skip — but the label describes risk, not a verdict.
- Hands — analyze_flow_change + analyze_broker_sentiment(7d/14d/30d) +
  get_broker_floorsheet(view='inferred'): is a NEW hand buying, persistent or
  churn, has anything CHANGED vs the prior window. The load-bearing axis for
  momentum and rotation cases.
- Catalyst — get_news(source='pulse', company=) + get_news(company=) +
  get_news(query=, category='announcement') + get_research_articles: the
  small thing that could turn a trinket into a symphony — a Q4 filing, a
  rights/bonus action, a board decision, a sector policy. Pulse is the
  company-filings feed: check it first, its tables carry the numbers.
- Regime — get_market_sentiment + the market's own 20-session return: the
  highest-weight axis. Every case is regime-conditional.
- History — what happened when similar setups occurred: pull longer price
  histories, prior runs of the same name, sector analogs, and compare. A
  setup you have seen resolve before is a setup you can trade.
- Valuation — get_stock_valuation / get_valuation_screener LAST, as a
  counterweight: it describes the market's expectation, never decides. The
  market is neither blind nor deaf — a cheap label with no accumulating hand
  is a trap signal, and an expensive label with a live setup is not a veto.

## Thinking habits
- Hypothesis first: "this moves if X" — hunt evidence for and against it.
- State before you call: each call closes a named gap (know / need / which
  call / expected). Calls that close no gap are noise.
- Two or more independent axes agreeing is a case; disagreeing is a conflict
  to resolve by name, deciding which evidence is load-bearing.
- Verify before you claim: keep an evidence ledger per candidate — every
  load-bearing claim carries its source and date.
- Score the alternatives before committing: buyable, not-now, and the
  contrarian read against the same evidence; force the bear case.
- Label the style: momentum / value / event / contrarian / rotation. A name
  can be BUYABLE in one style and dead in another. Valuation-negative is
  never a veto — size the risk instead of dropping the idea.
- History before concluding: check what happened when similar setups occurred.
- Stop when new calls add no new fact and change no belief; verify each claim
  once, then commit.

## Verdict (decisive)
BUYABLE — entry zone, target, stop, size against average volume, horizon,
and the falsifier (what observable breaks the story).
NOT-NOW — the ONE missing condition and the observable that would satisfy it.
WATCH — only when the missing evidence is genuinely unobservable today.
No hedged verdicts. Depth is welcome: more calls, alternatives, and history
are better than a rushed read. Voice is assertive: state the verdict, then
the evidence — no hedged preambles, no "this is just my opinion". Decisiveness
is not the absence of doubt; it is doubt examined and dismissed with reasons.

## Rules (few, high-leverage)
- Quote the numbers or don't make the claim; every load-bearing claim carries
  its source and date.
- Every BUYABLE verdict carries its falsifier.
- No verdict from SMA/EMA/MACD/fear-greed labels — those describe mood, they
  don't decide.
- Valuation tools are terminal counterweights: they describe the market's
  expectation, they never confirm or reject a trade by themselves. The market
  is neither blind nor deaf — a cheap label with no accumulating hand is a
  trap signal, not a buy signal, and an expensive label with a live setup is
  not a sell signal either.
- Reason from the numbers in front of you, not from remembered names or
  stories. No case is inherited; every case is built from the tape you pull.`

const guideSpeculation = `# SPECULATION — buy / trade / momentum questions

Run the PIPELINE (topic='pipeline'): screen, tape, hands, trinket, your read,
valuation caveat last. The trade case is made by price action, volume, flow,
and catalyst — get_price_history(limit≈90, include_analysis), analyze_flow_change,
analyze_broker_sentiment(7d/14d/30d), get_broker_floorsheet(view), get_news.
A parabolic run or fresh listing is higher risk — size accordingly, never size
up into extension. Quote the numbers; state what would break the story.`

const guideValue = `# VALUE — the terminal layer for valuation questions

This guide is for the LAST layer only. Run the PIPELINE (topic='pipeline')
first — screen, tape, hands, trinket — and answer the valuation question here,
as a counterweight, never as the case. Do not start a query with
get_valuation_screener; it ranks by a model's intrinsic guess, not by what the
hands are doing.

Data for the valuation layer:
- get_stock_valuation(symbol) — intrinsic vs LTP, verdict, valuation date
- get_valuation_screener(sector, limit) — the landscape
- get_company_profile(symbol) — the business's actuals
- get_research_articles(symbol) → get_research_article(slug) — the report behind the number

The market is neither blind nor deaf: if a name is cheap and nobody is
accumulating, that is a trap signal, not a buy signal. A discount matters only
relative to how fresh the model is (check the valuation date) and to why the gap exists — temporary (sentiment, a one-off) or structural
(deteriorating business, leverage). Liquidity decides whether you can actually
act on it.
The case stands or falls on structure + hands + catalyst; the valuation is a
caveat to your own read, never a shortcut past it.`

const guideSentimentTrack = `# SENTIMENT — market direction / health

Pull the fear/greed score and its components, breadth (advancers/decliners), turnover, sector moves, and where the money is flowing — get_market_sentiment(history_days), get_market_summary(include_sectors), get_top_list(type, limit). Describe the regime and the flows.`

const guideSurface = `# SURFACE — quick quote

Return the current price snapshot. No deep dive.`

const guideWorkflows = `# Workflows

Call patterns by intent (load the track guide once per intent):
- Morning brief (sentiment): get_market_sentiment(history_days) + get_market_summary(include_sectors) + get_top_list(type='gainers'|'losers'|'turnover', limit)
- Speculative trade: get_price_history(start_date, end_date, limit≈90, include_analysis=true) + analyze_broker_sentiment(period_days=7/14/30) + get_broker_floorsheet(view) + get_market_sentiment + get_news(company=symbol)
- Value hunt: get_valuation_screener(sector, limit) → get_stock_valuation(symbol) + get_company_profile(symbol) + get_price_history(limit, include_analysis) + get_research_articles(symbol) → get_research_article(slug) + get_broker_floorsheet(view='inferred') + get_news(company=symbol)
- Quick quote (surface): one tool call.`

const guideBroker = `# Broker Flow (LaganiLab adjusted-broker-position-v2)

Call: get_broker_floorsheet(symbol, view, top_n, from_date, to_date) — ONE call returns all views; switch the view param, don't re-call. Dates optional: default trailing 30d to today; ignored for view='inferred' (which is all-time as of to_date).

Views (view param):
- summary (default) — meta + top 5 each side. Cheapest full picture.
- holding/released/buyer/seller — activity within from→to window
- inferred (alias: positions) — ALL-TIME holdings as of to_date: AdjQty, Breakeven, MktValue, UnrealPL, DivRecvd, Conf. Same table for any from_date; only to_date matters.

Inferred fields:
- Breakeven: adjusted cost/share ('-' = untraceable). Price below breakeven = underwater holder.
- Conf <40 = directional only. DivRecvd/Bonus = corporate-action-adjusted P&L.

Meta: confidence_label + warnings (mergers, pre-2014 gap, unmatched sells) — respect before trusting quantities.

Signals (analyze_broker_sentiment(symbol, period_days, show_brokers)):
- Purity: cohort one-sidedness. >60% conviction, ~50% churn
- Gini x5 views: inequality across ALL brokers; rising across timeframes = concentration building
- HHI/Eff#: top-share dominance; Eff#<4 = fragile/monolithic
- Acc = Dist always (every buy has a sell) — read magnitude, not ratio

Workflow:
- period_days=7/14/30: pattern at all ranges = persistent; short-range only = fleeting
- 6mo & 1yr floorsheet (from_date, to_date): dormancy check — inferred holder absent from recent activity = dormant old money; present = active hand
- Uncertain? Slice weekly from→to ranges; watch accumulators/distributors evolve week by week`

const guideSentiment = `# Fear & Greed (get_market_sentiment)

Score 0-100 from 7 components: momentum, volatility (high=fear), turnover, drawdown, risk_appetite, downside_pressure (100=heavy selling), clv_pressure (closes near lows=fear).

Labels: 0-24 Extreme Fear, 25-44 Fear, 45-55 Neutral, 56-75 Greed, 76-100 Extreme Greed.

Use:
- Extreme Fear + rising AccRatio on quality = accumulation opportunity
- Extreme Greed + rising HHI concentration = late stage, tighten risk
- Price new high + falling score = weakening internals
- downside_pressure 100 on selloff = capitulation check; watch next-day breadth
- index_key (banking_subindex, hydropower_index, ...) for sector sentiment

The score is descriptive: it labels the crowd's mood, it never decides a trade.
Regime for sizing comes from the market's own 20-session return, not the label.`

const guideMarket = `# Market Tools

- get_market_summary(include_sectors): status, index, turnover, mktcap, supply/demand; adds sub-indices
- get_live_market_data(symbols, limit): LTP feed — market hours only
- get_top_list(type, limit): gainers | losers | turnover | volume | transactions
- get_price_history(symbol, start_date, end_date, limit, include_analysis): OHLCV; analysis adds 20-session gain (parabolic gate), period range, avg vol, volatility — structure facts, not oscillators
- get_market_depth(symbol) / get_floor_sheet(symbol, limit): order book / trade log — market hours only
- get_intraday_graph(symbol, format): OHLC+trend; format='points' for ~20 samples

Hours: Sun-Thu 11:00-15:00 NPT. Off-hours, most endpoints return last session's data.`

const guideAnsu = `# Ansu Invest (valuation & research)

Intrinsic value + verdict per stock; expert research reports.

- get_stock_valuation symbol=X — one stock: verdict, intrinsic value, LTP, P/E, P/B, EPS, % upside
- get_valuation_screener sector= — every covered stock ranked most-undervalued first
- get_research_articles symbol= — latest expert reports (title, slug, summary)
- get_research_article slug= — full report text + charts

Images: articles carry a hero photo and embedded valuation charts. include_images=true
returns them as MCP image content — only enable it if your model has vision; text-only
models omit it (saves tokens). Slugs come from get_research_articles.

An intrinsic value is only as good as what was fed in: loan growth, credit quality,
cost of equity, terminal growth. Those assumptions live in the research article
behind the number. Read it, sanity-check its assumptions against actuals
(get_company_profile), and say whether they hold or look stale. A verdict quoted
without reading what's behind it is a black box. Weight the number by recency too:
a months-old model makes today's discount partly an artifact, and the screener
ranks by that model.

Flow: screener → single-stock verdict → the article behind it → fundamentals check
→ verdict.`

const guideNews = `# News (ShareSansar + Merolagani)

Headline scanner + on-demand full text. Two-stage pull keeps tokens minimal: scan many headlines (~30 tokens each), open only what matters.

- get_news source=sharesansar|merolagani|both|stockssessions|pulse category=latest|... company= query= limit= page= — compact numbered list: title, date, language, source, url. both = parallel fetch + merge + dedupe, sorted newest-first.
- get_news_article url= (source= optional) — full text of one article. Pass the url from get_news (full URL or bare path).

StocksSessions (source='stockssessions'): Nepali headline aggregator (beemapost/arthasarokar/bizmandu/nepalipaisa links). Articles open via get_news_article (generic reader). API rate limit 60/min — lists cache 10min.

Pulse (source='pulse'): company disclosures — Q4/annual reports, rights/bonus actions — with FULL English analysis inline (ProseMirror: paragraphs + financial tables with YoY numbers). Filter with company=ticker/name. This is the company-filings feed: the trinket node's first stop. get_news_article(url of a /pulse/ slug) returns the extracted body with tables.

Filters (all optional, AND-combined where supported):
- company=TICKER — server-side symbol filter on both sources.
- query=free text — headline keyword search, Merolagani only (ShareSansar ignores it).

Categories:
- ShareSansar (English): latest, announcement, exclusive, company-analysis, ipo-fpo, dividend, allotment, listing, technical, weekly, video
- Merolagani (Nepali, Devanagari): latest, corporate, company-news, stock-market, hydropower, insurance, international, interview, it-auto, market-analysis, opinion, technical, tourism, economy, federal-economy, monetary-policy, current-affairs, others, budget-2077-78, budget-2078-79, budget-2081-82, budget-2082-83, budget-2083-84, budget-fy-2076-077, covid-19-updates, election, from-other-sources, local-election-2079, video, video-category

Notes:
- Merolagani list is a JSON API (fast, supports company + keyword search); ShareSansar is HTML with sequential cursor pagination — page N costs N fetches, so page=1 is the common case.
- Lists cache 10min; articles 6h (immutable).
- News is a catalyst scanner: use get_news with query= or category='announcement' (SS) / 'corporate' (ML) to catch IPOs, rights, mergers, provision reversals before they're priced.
- Article images: include_images=true on get_news_article ONLY if your model has vision; text-only models omit it.`

const guideReliability = `# Reliability

- Broker data lags: floorsheets processed after close; holidays serve last trading day
- type=all broker calls take ~2-10s for long ranges (full-history replay) — keep ranges tight
- Aggregated broker endpoint failure → auto per-view fallback, marked "degraded(partial views)"
- Pre-2014 holdings are inferred, lower confidence — respect ⚠ warnings in meta
- depth/floor-sheet/live-feed need market hours; WASM-auth token self-heals on 401/403
- Broker names map to NEPSE member codes 1-101; unknown render as "Broker N"`

func RegisterGuideTool(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("get_usage_guide",
		mcp.WithDescription("Server docs. Topics: overview(default)|pipeline|movers|speculation|value|sentiment|surface|workflows|broker|market|ansu|news|reliability. Stock questions: load 'pipeline' (the buyability research domain) once per query; 'overview' once."),
		mcp.WithString("topic", mcp.Description("overview|pipeline|movers|speculation|value|sentiment|surface|workflows|broker|market|ansu|news|reliability")),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		topic := strings.ToLower(strings.TrimSpace(request.GetString("topic", "overview")))
		switch topic {
		case "", "overview":
			return mcp.NewToolResultText(guideOverview), nil
		case "pipeline", "flow", "tree", "capital-gains":
			return mcp.NewToolResultText(guidePipeline), nil
		case "movers", "screener":
			return mcp.NewToolResultText(guideMovers), nil
		case "speculation", "momentum", "trade":
			return mcp.NewToolResultText(guideSpeculation), nil
		case "value", "valuation", "research", "undervalued":
			return mcp.NewToolResultText(guideValue), nil
		case "sentiment", "market-direction":
			return mcp.NewToolResultText(guideSentimentTrack), nil
		case "feargreed", "fear", "greed":
			return mcp.NewToolResultText(guideSentiment), nil
		case "surface", "quote", "snapshot":
			return mcp.NewToolResultText(guideSurface), nil
		case "workflows", "workflow", "recipes":
			return mcp.NewToolResultText(guideWorkflows), nil
		case "broker", "brokers", "floorsheet":
			return mcp.NewToolResultText(guideBroker), nil
		case "market":
			return mcp.NewToolResultText(guideMarket), nil
		case "ansu", "articles":
			return mcp.NewToolResultText(guideAnsu), nil
		case "news", "headlines":
			return mcp.NewToolResultText(guideNews), nil
		case "reliability", "limits", "errors":
			return mcp.NewToolResultText(guideReliability), nil
		default:
			return mcp.NewToolResultText(fmt.Sprintf("Unknown topic %q. Valid: overview, movers, speculation, value, sentiment, surface, workflows, broker, market, ansu, news, reliability.\n\n%s", topic, guideOverview)), nil
		}
	})
}
