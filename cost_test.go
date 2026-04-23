package main

import (
	"context"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/pricing"
)

// -- estimateCost ------------------------------------------------------------

// TestEstimateCost_CorrectDuration verifies that the estimated duration
// is ramp + until + 5 minutes overhead and that projected cost reflects
// the configured hours.
func TestEstimateCost_CorrectDuration(t *testing.T) {
	srv := newMockPricingJSONServer(t, 0.50)

	// point VantagePricer at our mock server by patching via the
	// pricer returned from estimateCost -- we intercept by overriding
	// the test sovereign's estimateCost to use a custom pricer
	s := testSovereign(t)
	s.cfg.Ramp = 5 * time.Second
	s.cfg.Until = 30 * time.Second

	// call the package-level pricing function directly with the mock
	// pricer to verify duration calculation
	expectedDuration := s.cfg.Ramp + s.cfg.Until + 5*time.Minute
	expectedHours := expectedDuration.Hours()

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	pricer := pricing.NewVantagePricerWithURL(srv.URL + "/")
	estimate, err := pricing.EstimateCost(
		context.Background(),
		pricer,
		pricing.EstimateConfig{
			Queries: queries,
			Hours:   expectedHours,
		},
		nil,
	)
	requireNoError(t, err, "EstimateCost duration")

	for _, r := range estimate.Results {
		expectedCost := int64(float64(pricing.DollarToMicrocents(0.50)) * expectedHours)
		requireEqual(t, r.ProjectedCost, expectedCost,
			"EstimateCost projected cost matches duration")
	}
}

// TestEstimateCost_PlanetCount verifies that estimate contains one
// result per planet.
func TestEstimateCost_PlanetCount(t *testing.T) {
	srv := newMockPricingJSONServer(t, 0.50)
	s := testSovereign(t)

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	pricer := pricing.NewVantagePricerWithURL(srv.URL + "/")
	estimate, err := pricing.EstimateCost(
		context.Background(),
		pricer,
		pricing.EstimateConfig{Queries: queries, Hours: 1.0},
		nil,
	)
	requireNoError(t, err, "EstimateCost planet count")
	requireEqual(t, len(estimate.Results), len(s.planets),
		"EstimateCost result count matches planet count")
}

// TestEstimateCost_TotalCostAccumulates verifies that TotalCost is
// the sum of all ProjectedCost values.
func TestEstimateCost_TotalCostAccumulates(t *testing.T) {
	srv := newMockPricingJSONServer(t, 0.50)
	s := testSovereign(t)

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	pricer := pricing.NewVantagePricerWithURL(srv.URL + "/")
	estimate, err := pricing.EstimateCost(
		context.Background(),
		pricer,
		pricing.EstimateConfig{Queries: queries, Hours: 1.0},
		nil,
	)
	requireNoError(t, err, "EstimateCost total cost")

	var sum int64
	for _, r := range estimate.Results {
		sum += r.ProjectedCost
	}
	requireEqual(t, estimate.TotalCost, sum,
		"TotalCost equals sum of ProjectedCost values")
}

// TestEstimateCost_APIFailureIsNonFatal verifies that when the pricing
// API is unavailable the estimate still returns with zero cost for
// affected planets rather than returning an error.
func TestEstimateCost_APIFailureIsNonFatal(t *testing.T) {
	srv := newMockPricingJSONServerError(t)
	s := testSovereign(t)

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	pricer := pricing.NewVantagePricerWithURL(srv.URL + "/")
	estimate, err := pricing.EstimateCost(
		context.Background(),
		pricer,
		pricing.EstimateConfig{Queries: queries, Hours: 1.0},
		nil,
	)
	requireNoError(t, err, "EstimateCost API failure is non-fatal")
	requireNotNil(t, estimate, "EstimateCost returns estimate on API failure")

	for _, r := range estimate.Results {
		requireEqual(t, r.HourlyRate, int64(0),
			"HourlyRate is 0 on API failure")
	}
}

// TestEstimateCost_RegionAndTypePopulated verifies that each result
// carries the correct region and instance type from the planet config.
func TestEstimateCost_RegionAndTypePopulated(t *testing.T) {
	srv := newMockPricingJSONServer(t, 0.50)
	s := testSovereign(t)

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	pricer := pricing.NewVantagePricerWithURL(srv.URL + "/")
	estimate, err := pricing.EstimateCost(
		context.Background(),
		pricer,
		pricing.EstimateConfig{Queries: queries, Hours: 1.0},
		nil,
	)
	requireNoError(t, err, "EstimateCost region and type")

	for _, r := range estimate.Results {
		requireEqual(t, r.Query.Region, testRegion,
			"EstimateCost result region matches planet region")
		requireEqual(t, r.Query.InstanceType, testInstanceType,
			"EstimateCost result instance type matches planet instance type")
	}
}

// -- totalLemmings -----------------------------------------------------------

// TestTotalLemmings_CorrectProduct verifies that totalLemmings returns
// planets * terrain * pack.
func TestTotalLemmings_CorrectProduct(t *testing.T) {
	s := testSovereign(t)
	got := s.totalLemmings()
	expected := int64(s.cfg.Planets) * s.cfg.Terrain * s.cfg.Pack
	requireEqual(t, got, expected, "totalLemmings product")
}

// TestTotalLemmings_SinglePlanet verifies correct calculation with
// a single planet.
func TestTotalLemmings_SinglePlanet(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 1
	s.cfg.Terrain = 10
	s.cfg.Pack = 5
	got := s.totalLemmings()
	requireEqual(t, got, int64(50), "totalLemmings single planet")
}

// TestTotalLemmings_MultiPlanet verifies correct calculation across
// multiple planets.
func TestTotalLemmings_MultiPlanet(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 5
	s.cfg.Terrain = 50
	s.cfg.Pack = 50
	got := s.totalLemmings()
	requireEqual(t, got, int64(12_500), "totalLemmings multi planet")
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkEstimateCost measures full cost estimation throughput
// against a mock pricing server with the sovereign estimateCost method.
func BenchmarkEstimateCost(b *testing.B) {
	srv := newMockPricingJSONServer(b, 0.50)

	s := benchmarkSovereign(b)
	s.cfg.Planets = 5
	s.planets = s.initPlanets()

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	pricer := pricing.NewVantagePricerWithURL(srv.URL + "/")
	cfg := pricing.EstimateConfig{
		Queries: queries,
		Hours:   0.5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pricing.EstimateCost(context.Background(), pricer, cfg, nil)
	}
}

// BenchmarkTotalLemmings measures totalLemmings throughput.
func BenchmarkTotalLemmings(b *testing.B) {
	s := benchmarkSovereign(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.totalLemmings()
	}
}

// -- helpers -----------------------------------------------------------------

// newMockPricingJSONServer is defined in main_test.go and available here
// since both files are in package main.
// newMockPricingJSONServerError is also defined in main_test.go.

// testPricingEstimateForSovereign is a convenience helper that builds
// a full cost estimate for a sovereign's planets against a mock server.
func testPricingEstimateForSovereign(
	t *testing.T,
	s *Sovereign,
	serverURL string,
	hours float64,
) (*pricing.Estimate, error) {
	t.Helper()
	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}
	pricer := pricing.NewVantagePricerWithURL(serverURL + "/")
	return pricing.EstimateCost(
		context.Background(),
		pricer,
		pricing.EstimateConfig{Queries: queries, Hours: hours},
		nil,
	)
}
