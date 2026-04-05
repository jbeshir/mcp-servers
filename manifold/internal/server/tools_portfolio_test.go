package server

import (
	"math"
	"testing"
	"time"

	"github.com/jbeshir/mcp-servers/manifold/internal/client"
)

func TestComputeBaseline_NoActivity(t *testing.T) {
	// No bets at all — baseline should equal current probability.
	result := computeBaselineAt(nil, 0.65, time.UnixMilli(1000), time.Hour)
	if result.Baseline != 0.65 {
		t.Errorf("expected baseline 0.65, got %f", result.Baseline)
	}
	if result.Warning != "" {
		t.Errorf("expected no warning, got %q", result.Warning)
	}
}

func TestComputeBaseline_NoRecentActivity(t *testing.T) {
	// All bets are before cutoff — baseline should equal current probability.
	bets := []client.Bet{
		{CreatedTime: 500, ProbBefore: 0.40, ProbAfter: 0.50},
		{CreatedTime: 800, ProbBefore: 0.50, ProbAfter: 0.60},
	}
	result := computeBaselineAt(bets, 0.60, time.UnixMilli(1000), time.Hour)
	if result.Baseline != 0.60 {
		t.Errorf("expected baseline 0.60, got %f", result.Baseline)
	}
	if result.Warning != "" {
		t.Errorf("expected no warning, got %q", result.Warning)
	}
}

func TestComputeBaseline_ExactWithPreCutoffBets(t *testing.T) {
	// Bets span the cutoff — use latest pre-cutoff bet's probAfter.
	bets := []client.Bet{
		{CreatedTime: 800, ProbBefore: 0.40, ProbAfter: 0.50},
		{CreatedTime: 900, ProbBefore: 0.50, ProbAfter: 0.55},
		{CreatedTime: 1100, ProbBefore: 0.55, ProbAfter: 0.70},
		{CreatedTime: 1200, ProbBefore: 0.70, ProbAfter: 0.65},
	}
	result := computeBaselineAt(bets, 0.65, time.UnixMilli(1000), time.Hour)
	if result.Baseline != 0.55 {
		t.Errorf("expected baseline 0.55, got %f", result.Baseline)
	}
	if result.Warning != "" {
		t.Errorf("expected no warning, got %q", result.Warning)
	}
}

func TestComputeBaseline_ApproximateAllPostCutoff(t *testing.T) {
	// All bets are after cutoff — use earliest bet's probBefore + warning.
	bets := []client.Bet{
		{CreatedTime: 1100, ProbBefore: 0.40, ProbAfter: 0.50},
		{CreatedTime: 1200, ProbBefore: 0.50, ProbAfter: 0.65},
	}
	result := computeBaselineAt(bets, 0.65, time.UnixMilli(1000), time.Hour)
	if result.Baseline != 0.40 {
		t.Errorf("expected baseline 0.40, got %f", result.Baseline)
	}
	if result.Warning == "" {
		t.Error("expected a warning for approximate baseline")
	}
}

func TestComputeBaseline_RedemptionsFiltered(t *testing.T) {
	// Redemption bets (prob unchanged) should be ignored.
	bets := []client.Bet{
		{CreatedTime: 800, ProbBefore: 0.50, ProbAfter: 0.55},          // real, pre-cutoff
		{CreatedTime: 1100, ProbBefore: 0.55, ProbAfter: 0.55},         // redemption, post-cutoff
		{CreatedTime: 1100, ProbBefore: 0.550001, ProbAfter: 0.550001}, // redemption (within tolerance)
		{CreatedTime: 1200, ProbBefore: 0.55, ProbAfter: 0.65},         // real, post-cutoff
	}
	result := computeBaselineAt(bets, 0.65, time.UnixMilli(1000), time.Hour)
	// Pre-cutoff real bet exists, so exact baseline = 0.55.
	if result.Baseline != 0.55 {
		t.Errorf("expected baseline 0.55, got %f", result.Baseline)
	}
	if result.Warning != "" {
		t.Errorf("expected no warning, got %q", result.Warning)
	}
}

func TestBaselineProbFromMarket(t *testing.T) {
	prob := 0.65
	m := &client.PortfolioMarket{
		Prob:        &prob,
		ProbChanges: client.ProbChanges{Day: 0.05},
	}
	if got := baselineProbFromMarket(m); math.Abs(got-0.60) > 0.0001 {
		t.Errorf("expected baseline ~0.60, got %f", got)
	}
}

func TestBaselineProbFromMarket_ZeroProbChanges(t *testing.T) {
	prob := 0.65
	m := &client.PortfolioMarket{Prob: &prob}
	if got := baselineProbFromMarket(m); got != 0.65 {
		t.Errorf("expected baseline 0.65 (current), got %f", got)
	}
}

func TestBuildPortfolioResponse_BasicOpenPosition(t *testing.T) {
	prob := 0.65
	enriched := map[string]*enrichedMarket{
		"abc": {
			market: client.PortfolioMarket{
				ID:          "abc",
				Question:    "Will X happen?",
				Slug:        "will-x-happen",
				OutcomeType: "BINARY",
				Prob:        &prob,
				ProbChanges: client.ProbChanges{Day: 0.05},
			},
			positions: []client.ContractMetric{
				{
					ContractID:   "abc",
					HasYesShares: true,
					TotalShares:  map[string]float64{outcomeYes: 100.0},
					Profit:       20.0,
				},
			},
		},
	}

	resp := buildPortfolioResponse(enriched, 7*24*time.Hour)
	if resp.Summary.PositionCount != 1 {
		t.Errorf("expected 1 position, got %d", resp.Summary.PositionCount)
	}
	if len(resp.Positions) != 1 {
		t.Fatalf("expected 1 position in list, got %d", len(resp.Positions))
	}

	pos := resp.Positions[0]
	if pos.Outcome != outcomeYes {
		t.Errorf("expected YES outcome, got %s", pos.Outcome)
	}
	// currentValue = 100 * 0.65 = 65.
	if pos.CurrentValue != 65.0 {
		t.Errorf("expected current value 65.0, got %f", pos.CurrentValue)
	}
	// costBasis = currentValue - profit = 65 - 20 = 45.
	if pos.CostBasis != 45.0 {
		t.Errorf("expected cost basis 45.0, got %f", pos.CostBasis)
	}
	if pos.Pnl != 20.0 {
		t.Errorf("expected P&L 20.0, got %f", pos.Pnl)
	}
	// baseline = 0.65 - 0.05 = 0.60, changePp = 5.0.
	if math.Abs(pos.BaselineProb-0.60) > 0.0001 {
		t.Errorf("expected baseline ~0.60, got %f", pos.BaselineProb)
	}
	if math.Abs(pos.ChangePp-5.0) > 0.01 {
		t.Errorf("expected changePp ~5.0, got %f", pos.ChangePp)
	}
}

func TestBuildPortfolioResponse_NonBinaryExcluded(t *testing.T) {
	enriched := map[string]*enrichedMarket{
		"multi": {
			market: client.PortfolioMarket{
				ID:          "multi",
				OutcomeType: "MULTIPLE_CHOICE",
			},
			positions: []client.ContractMetric{
				{ContractID: "multi", HasYesShares: true, TotalShares: map[string]float64{outcomeYes: 50}},
			},
		},
	}

	resp := buildPortfolioResponse(enriched, 3*24*time.Hour)
	if resp.Summary.ExcludedNonBinary != 1 {
		t.Errorf("expected 1 excluded non-binary, got %d", resp.Summary.ExcludedNonBinary)
	}
	if resp.Summary.PositionCount != 0 {
		t.Errorf("expected 0 positions, got %d", resp.Summary.PositionCount)
	}
}

func TestBuildPortfolioResponse_RecentResolved(t *testing.T) {
	prob := 1.0
	resolution := outcomeYes
	resolvedAt := time.Now().UnixMilli() - 1000
	enriched := map[string]*enrichedMarket{
		"resolved": {
			market: client.PortfolioMarket{
				ID:             "resolved",
				Question:       "Did Y happen?",
				Slug:           "did-y-happen",
				OutcomeType:    "BINARY",
				Prob:           &prob,
				IsResolved:     true,
				Resolution:     &resolution,
				ResolutionTime: &resolvedAt,
			},
			positions: []client.ContractMetric{
				{
					ContractID:   "resolved",
					HasYesShares: true,
					TotalShares:  map[string]float64{outcomeYes: 100.0},
					Profit:       55.0,
				},
			},
		},
	}

	resp := buildPortfolioResponse(enriched, 7*24*time.Hour)
	if len(resp.RecentResolved) != 1 {
		t.Fatalf("expected 1 recent resolved, got %d", len(resp.RecentResolved))
	}
	if resp.RecentResolved[0].Pnl != 55.0 {
		t.Errorf("expected resolved P&L 55.0, got %f", resp.RecentResolved[0].Pnl)
	}
	if resp.Summary.RecentResolvedPnl != 55.0 {
		t.Errorf("expected resolved total 55.0, got %f", resp.Summary.RecentResolvedPnl)
	}
}
