package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"sort"
	"time"

	"github.com/jbeshir/mcp-servers/manifold/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	baselineBetLimit   = 200
	maxConcurrency     = 15
	moverThresholdPp   = 2.0
	recentResolvedDays = 7
	outcomeYes         = "YES"
	outcomeNo          = "NO"
)

// BaselineResult holds a computed baseline probability and an optional warning.
type BaselineResult struct {
	Baseline float64
	Warning  string
}

// computeBaseline determines the probability a market was at the given duration ago.
func computeBaseline(bets []client.Bet, currentProb float64, lookback time.Duration) BaselineResult {
	cutoff := time.Now().Add(-lookback)
	return computeBaselineAt(bets, currentProb, cutoff, lookback)
}

// computeBaselineAt determines the probability a market was at the given cutoff time.
func computeBaselineAt(
	bets []client.Bet, currentProb float64, cutoff time.Time, lookback time.Duration,
) BaselineResult {
	cutoffMs := cutoff.UnixMilli()

	// Filter out redemptions (probability didn't meaningfully change).
	var nonRedemption []client.Bet
	for _, b := range bets {
		if math.Abs(b.ProbBefore-b.ProbAfter) > 0.001 {
			nonRedemption = append(nonRedemption, b)
		}
	}

	var before, after []client.Bet
	for _, b := range nonRedemption {
		if b.CreatedTime > cutoffMs {
			after = append(after, b)
		} else {
			before = append(before, b)
		}
	}

	if len(after) == 0 {
		// No activity in the lookback window — baseline is current.
		return BaselineResult{Baseline: currentProb}
	}

	if len(before) > 0 {
		// Use the most recent pre-cutoff bet's probAfter.
		latest := before[0]
		for _, b := range before[1:] {
			if b.CreatedTime > latest.CreatedTime {
				latest = b
			}
		}
		return BaselineResult{Baseline: latest.ProbAfter}
	}

	// All fetched bets are post-cutoff — use earliest bet's probBefore (approximate).
	earliest := after[0]
	for _, b := range after[1:] {
		if b.CreatedTime < earliest.CreatedTime {
			earliest = b
		}
	}
	return BaselineResult{
		Baseline: earliest.ProbBefore,
		Warning: fmt.Sprintf(
			"baseline approximate — all %d fetched bets are within %s",
			baselineBetLimit, lookback,
		),
	}
}

// baselineResponse is the JSON output for get_24h_baseline.
type baselineResponse struct {
	ContractID   string  `json:"contractId"`
	CurrentProb  float64 `json:"currentProb"`
	BaselineProb float64 `json:"baselineProb"`
	ChangePp     float64 `json:"changePp"`
	Warning      string  `json:"warning,omitempty"`
}

func (s *Server) handleGetBaseline(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	contractID, ok := args["contractId"].(string)
	if !ok || contractID == "" {
		return mcp.NewToolResultError("contractId is required"), nil
	}

	lookback := 24 * time.Hour
	if hours, ok := args["lookbackHours"].(float64); ok && hours > 0 {
		lookback = time.Duration(hours * float64(time.Hour))
	}

	market, err := s.client.GetMarket(ctx, contractID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get market: %v", err)), nil
	}
	if market.Probability == nil {
		return mcp.NewToolResultError("market has no probability (non-binary market?)"), nil
	}

	params := url.Values{}
	params.Set("contractId", contractID)
	params.Set("limit", fmt.Sprintf("%d", baselineBetLimit))
	bets, err := s.client.ListBets(ctx, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list bets: %v", err)), nil
	}

	result := computeBaseline(bets, *market.Probability, lookback)

	resp := baselineResponse{
		ContractID:   contractID,
		CurrentProb:  *market.Probability,
		BaselineProb: result.Baseline,
		ChangePp:     (*market.Probability - result.Baseline) * 100,
		Warning:      result.Warning,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// portfolioSummary is the top-level summary in the portfolio response.
type portfolioSummary struct {
	OpenPnl           float64 `json:"openPnl"`
	RecentResolvedPnl float64 `json:"recentResolvedPnl"`
	CombinedPnl       float64 `json:"combinedPnl"`
	PositionCount     int     `json:"positionCount"`
	ExcludedNonBinary int     `json:"excludedNonBinary"`
}

// portfolioMover is a market with significant movement in the lookback window.
type portfolioMover struct {
	Question    string  `json:"question"`
	URL         string  `json:"url"`
	Outcome     string  `json:"outcome"`
	ChangePp    float64 `json:"changePp"`
	BaselinePnl float64 `json:"baselinePnl"`
	CurrentProb float64 `json:"currentProb"`
}

// portfolioPosition is a single open position.
type portfolioPosition struct {
	ContractID   string  `json:"contractId"`
	Question     string  `json:"question"`
	URL          string  `json:"url"`
	Outcome      string  `json:"outcome"`
	Shares       float64 `json:"shares"`
	CostBasis    float64 `json:"costBasis"`
	CurrentProb  float64 `json:"currentProb"`
	CurrentValue float64 `json:"currentValue"`
	Pnl          float64 `json:"pnl"`
	BaselineProb float64 `json:"baselineProb"`
	ChangePp     float64 `json:"changePp"`
	BaselinePnl  float64 `json:"baselinePnl"`
}

// portfolioResolved is a recently resolved position.
type portfolioResolved struct {
	ContractID string  `json:"contractId"`
	Question   string  `json:"question"`
	URL        string  `json:"url"`
	Outcome    string  `json:"outcome"`
	Resolution string  `json:"resolution"`
	Pnl        float64 `json:"pnl"`
	ResolvedAt int64   `json:"resolvedAt"`
}

// portfolioResponse is the full portfolio P&L response.
type portfolioResponse struct {
	Summary        portfolioSummary    `json:"summary"`
	Movers         []portfolioMover    `json:"movers"`
	Positions      []portfolioPosition `json:"positions"`
	RecentResolved []portfolioResolved `json:"recentResolved"`
}

// enrichedMarket holds fetched data for a single contract from the bulk endpoint.
type enrichedMarket struct {
	market    client.PortfolioMarket
	positions []client.ContractMetric
}

func (s *Server) handleGetPortfolioPnl(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	userID, ok := request.Params.Arguments["userId"].(string)
	if !ok || userID == "" {
		return mcp.NewToolResultError("userId is required"), nil
	}

	resolvedLookback := recentResolvedDays * 24 * time.Hour

	// Fetch all positions and market data in one paginated call.
	enriched, err := s.fetchAllUserPositions(ctx, userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to fetch portfolio: %v", err)), nil
	}

	resp := buildPortfolioResponse(enriched, resolvedLookback)

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

const bulkFetchLimit = 1000

func (s *Server) fetchAllUserPositions(
	ctx context.Context, userID string,
) (map[string]*enrichedMarket, error) {
	results := make(map[string]*enrichedMarket)
	offset := 0

	for {
		params := url.Values{}
		params.Set("userId", userID)
		params.Set("limit", fmt.Sprintf("%d", bulkFetchLimit))
		params.Set("offset", fmt.Sprintf("%d", offset))

		batch, err := s.client.GetUserMetricsWithContracts(ctx, params)
		if err != nil {
			return nil, err
		}

		// Index contracts by ID for lookup.
		contractsByID := make(map[string]client.PortfolioMarket, len(batch.Contracts))
		for _, c := range batch.Contracts {
			contractsByID[c.ID] = c
		}

		for contractID, metrics := range batch.MetricsByContract {
			market, ok := contractsByID[contractID]
			if !ok {
				continue
			}
			results[contractID] = &enrichedMarket{
				market:    market,
				positions: metrics,
			}
		}

		if len(batch.Contracts) < bulkFetchLimit {
			break
		}
		offset += bulkFetchLimit
	}

	return results, nil
}

// positionValue computes current value of shares for a given outcome and probability.
func positionValue(shares, currentProb float64, outcome string) float64 {
	if outcome == outcomeYes {
		return shares * currentProb
	}
	return shares * (1 - currentProb)
}

// positionBaselinePnl computes P&L and change in pp relative to a baseline probability.
func positionBaselinePnl(shares, currentProb, baseline float64, outcome string) (pnl, changePp float64) {
	if outcome == outcomeYes {
		return shares * (currentProb - baseline), (currentProb - baseline) * 100
	}
	return shares * (baseline - currentProb), -(currentProb - baseline) * 100
}

// processOpenPosition builds a portfolioPosition for an open market position.
func processOpenPosition(
	m *client.PortfolioMarket,
	outcome string,
	shares float64,
	profit float64,
	baselineProb float64,
) portfolioPosition {
	currentProb := *m.Prob
	currentValue := positionValue(shares, currentProb, outcome)
	baselinePnl, changePp := positionBaselinePnl(shares, currentProb, baselineProb, outcome)

	return portfolioPosition{
		ContractID:   m.ID,
		Question:     m.Question,
		URL:          m.URL(),
		Outcome:      outcome,
		Shares:       shares,
		CostBasis:    currentValue - profit,
		CurrentProb:  currentProb,
		CurrentValue: currentValue,
		Pnl:          profit,
		BaselineProb: baselineProb,
		ChangePp:     changePp,
		BaselinePnl:  baselinePnl,
	}
}

// baselineProbFromMarket derives the baseline probability from the market's ProbChanges.Day field.
func baselineProbFromMarket(m *client.PortfolioMarket) float64 {
	if m.Prob == nil {
		return 0
	}
	return *m.Prob - m.ProbChanges.Day
}

// portfolioAccumulator collects positions, movers, and resolved markets during portfolio building.
type portfolioAccumulator struct {
	positions      []portfolioPosition
	recentResolved []portfolioResolved
	movers         []portfolioMover
	openPnl        float64
	resolvedPnl    float64
	nonBinary      int
}

func (a *portfolioAccumulator) processMarket(em *enrichedMarket, resolvedLookback time.Duration) {
	m := &em.market

	if m.OutcomeType != "BINARY" || m.Prob == nil {
		a.nonBinary++
		return
	}

	baseline := baselineProbFromMarket(m)

	for _, pos := range em.positions {
		if !pos.HasYesShares && !pos.HasNoShares {
			continue
		}

		outcome := outcomeYes
		if pos.HasNoShares && !pos.HasYesShares {
			outcome = outcomeNo
		}

		shares := pos.TotalShares[outcome]
		if shares < 0.5 {
			continue
		}

		if m.IsResolved {
			a.processResolved(m, outcome, pos.Profit, resolvedLookback)
			continue
		}

		a.processOpen(m, pos, outcome, shares, baseline)
	}
}

func (a *portfolioAccumulator) processResolved(
	m *client.PortfolioMarket,
	outcome string,
	profit float64,
	resolvedLookback time.Duration,
) {
	resolvedCutoffMs := time.Now().UnixMilli() - resolvedLookback.Milliseconds()
	if m.ResolutionTime == nil || *m.ResolutionTime <= resolvedCutoffMs {
		return
	}
	resolution := ""
	if m.Resolution != nil {
		resolution = *m.Resolution
	}
	a.recentResolved = append(a.recentResolved, portfolioResolved{
		ContractID: m.ID,
		Question:   m.Question,
		URL:        m.URL(),
		Outcome:    outcome,
		Resolution: resolution,
		Pnl:        profit,
		ResolvedAt: *m.ResolutionTime,
	})
	a.resolvedPnl += profit
}

func (a *portfolioAccumulator) processOpen(
	m *client.PortfolioMarket,
	pos client.ContractMetric,
	outcome string,
	shares float64,
	baselineProb float64,
) {
	pp := processOpenPosition(m, outcome, shares, pos.Profit, baselineProb)
	a.positions = append(a.positions, pp)
	a.openPnl += pos.Profit

	if math.Abs(pp.ChangePp) >= moverThresholdPp {
		a.movers = append(a.movers, portfolioMover{
			Question:    m.Question,
			URL:         m.URL(),
			Outcome:     outcome,
			ChangePp:    pp.ChangePp,
			BaselinePnl: pp.BaselinePnl,
			CurrentProb: *m.Prob,
		})
	}

	// Handle dual positions (user holds both YES and NO).
	if pos.HasYesShares && pos.HasNoShares {
		a.processDualPosition(m, pos, outcome, pp.CurrentValue, baselineProb)
	}
}

func (a *portfolioAccumulator) processDualPosition(
	m *client.PortfolioMarket,
	pos client.ContractMetric,
	primaryOutcome string,
	primaryValue float64,
	baselineProb float64,
) {
	otherOutcome := outcomeNo
	if primaryOutcome == outcomeNo {
		otherOutcome = outcomeYes
	}
	otherShares := pos.TotalShares[otherOutcome]
	if otherShares < 0.5 {
		return
	}

	otherValue := positionValue(otherShares, *m.Prob, otherOutcome)

	// Split profit proportionally by value.
	totalValue := primaryValue + otherValue
	otherPnl := 0.0
	if totalValue > 0 {
		otherPnl = pos.Profit * (otherValue / totalValue)
	}

	otherPos := processOpenPosition(m, otherOutcome, otherShares, otherPnl, baselineProb)
	a.positions = append(a.positions, otherPos)
}

func buildPortfolioResponse(
	enriched map[string]*enrichedMarket,
	resolvedLookback time.Duration,
) portfolioResponse {
	acc := &portfolioAccumulator{}

	for _, em := range enriched {
		acc.processMarket(em, resolvedLookback)
	}

	// Sort movers by |changePp| descending.
	sort.Slice(acc.movers, func(i, j int) bool {
		return math.Abs(acc.movers[i].ChangePp) > math.Abs(acc.movers[j].ChangePp)
	})

	// Sort positions by |baselinePnl| descending.
	sort.Slice(acc.positions, func(i, j int) bool {
		return math.Abs(acc.positions[i].BaselinePnl) > math.Abs(acc.positions[j].BaselinePnl)
	})

	return portfolioResponse{
		Summary: portfolioSummary{
			OpenPnl:           acc.openPnl,
			RecentResolvedPnl: acc.resolvedPnl,
			CombinedPnl:       acc.openPnl + acc.resolvedPnl,
			PositionCount:     len(acc.positions),
			ExcludedNonBinary: acc.nonBinary,
		},
		Movers:         acc.movers,
		Positions:      acc.positions,
		RecentResolved: acc.recentResolved,
	}
}
