package main

import (
    "context"
    "testing"
    "time"
)

// -- dollarToMicrocents -------------------------------------------------------

// TestDollarToMicrocents_HalfDollar verifies that $0.50 converts to
// exactly 50,000 microcents -- the canonical test value used throughout
// the sovereign test suite.
func TestDollarToMicrocents_HalfDollar(t *testing.T) {
    got := dollarToMicrocents(0.50)
    requireEqual(t, got, int64(50_000), "dollarToMicrocents(0.50)")
}

// TestDollarToMicrocents_OneDollar verifies that $1.00 converts to
// exactly 100,000 microcents.
func TestDollarToMicrocents_OneDollar(t *testing.T) {
    got := dollarToMicrocents(1.00)
    requireEqual(t, got, int64(100_000), "dollarToMicrocents(1.00)")
}

// TestDollarToMicrocents_Zero verifies that $0.00 converts to 0 microcents.
func TestDollarToMicrocents_Zero(t *testing.T) {
    got := dollarToMicrocents(0.0)
    requireEqual(t, got, int64(0), "dollarToMicrocents(0.0)")
}

// TestDollarToMicrocents_SubCent verifies that sub-cent values are
// preserved with microcent precision.
func TestDollarToMicrocents_SubCent(t *testing.T) {
    cases := []struct {
        input    float64
        expected int64
    }{
        {0.0034, 340},
        {0.00001, 1},
        {0.12345, 12_345},
    }
    for _, tc := range cases {
        got := dollarToMicrocents(tc.input)
        requireEqual(t, got, tc.expected,
            "dollarToMicrocents sub-cent precision")
    }
}

// TestDollarToMicrocents_LargeDollar verifies that dollar amounts
// above $1.00 convert correctly.
func TestDollarToMicrocents_LargeDollar(t *testing.T) {
    cases := []struct {
        input    float64
        expected int64
    }{
        {1.234, 123_400},
        {10.00, 1_000_000},
        {99.99, 9_999_000},
    }
    for _, tc := range cases {
        got := dollarToMicrocents(tc.input)
        requireEqual(t, got, tc.expected,
            "dollarToMicrocents large dollar value")
    }
}

// TestDollarToMicrocents_NeverFloat verifies that integer arithmetic
// on microcents does not suffer from float64 rounding. The canonical
// float64 trap: 0.1 + 0.2 != 0.3 in float64 but must be exact in
// microcents after conversion.
func TestDollarToMicrocents_NeverFloat(t *testing.T) {
    a := dollarToMicrocents(0.10)
    b := dollarToMicrocents(0.20)
    sum := a + b
    expected := dollarToMicrocents(0.30)
    // integer addition is exact -- this must hold
    requireEqual(t, sum, expected,
        "microcent integer arithmetic must be exact")
}

// -- microcentsToDisplay -----------------------------------------------------

// TestMicrocentsToDisplay_HalfDollar verifies that 50,000 microcents
// renders as "$0.50".
func TestMicrocentsToDisplay_HalfDollar(t *testing.T) {
    got := microcentsToDisplay(50_000)
    requireEqual(t, got, "$0.50", "microcentsToDisplay(50_000)")
}

// TestMicrocentsToDisplay_OneDollar verifies that 100,000 microcents
// renders as "$1.00".
func TestMicrocentsToDisplay_OneDollar(t *testing.T) {
    got := microcentsToDisplay(100_000)
    requireEqual(t, got, "$1.00", "microcentsToDisplay(100_000)")
}

// TestMicrocentsToDisplay_Zero verifies that 0 microcents renders
// as "$0.00".
func TestMicrocentsToDisplay_Zero(t *testing.T) {
    got := microcentsToDisplay(0)
    requireEqual(t, got, "$0.00", "microcentsToDisplay(0)")
}

// TestMicrocentsToDisplay_SubCentRoundsDown verifies that sub-cent
// microcent values round down at display time -- 340 microcents
// is less than one cent so it displays as "$0.00".
func TestMicrocentsToDisplay_SubCentRoundsDown(t *testing.T) {
    got := microcentsToDisplay(340)
    requireEqual(t, got, "$0.00", "microcentsToDisplay(340)")
}

// TestMicrocentsToDisplay_LargeDollar verifies rendering of values
// above $1.00.
func TestMicrocentsToDisplay_LargeDollar(t *testing.T) {
    cases := []struct {
        input    int64
        expected string
    }{
        {200_000, "$2.00"},
        {150_000, "$1.50"},
        {999_000, "$9.99"},
        {1_000_000, "$10.00"},
    }
    for _, tc := range cases {
        got := microcentsToDisplay(tc.input)
        requireEqual(t, got, tc.expected,
            "microcentsToDisplay large value")
    }
}

// TestMicrocentsToDisplay_PaddedCents verifies that cents below 10
// are zero-padded to two digits.
func TestMicrocentsToDisplay_PaddedCents(t *testing.T) {
    got := microcentsToDisplay(105_000)
    requireEqual(t, got, "$1.05", "microcentsToDisplay padded cents")
}

// -- microcentsToDetailedDisplay ---------------------------------------------

// TestMicrocentsToDetailedDisplay_HalfDollar verifies that 50,000
// microcents renders as "$0.50000" with full precision.
func TestMicrocentsToDetailedDisplay_HalfDollar(t *testing.T) {
    got := microcentsToDetailedDisplay(50_000)
    requireEqual(t, got, "$0.50000",
        "microcentsToDetailedDisplay(50_000)")
}

// TestMicrocentsToDetailedDisplay_SubCent verifies that sub-cent
// precision is preserved in the detailed display format.
func TestMicrocentsToDetailedDisplay_SubCent(t *testing.T) {
    got := microcentsToDetailedDisplay(123_456)
    requireEqual(t, got, "$1.23456",
        "microcentsToDetailedDisplay(123_456)")
}

// TestMicrocentsToDetailedDisplay_Zero verifies that zero renders
// with full precision padding.
func TestMicrocentsToDetailedDisplay_Zero(t *testing.T) {
    got := microcentsToDetailedDisplay(0)
    requireEqual(t, got, "$0.00000",
        "microcentsToDetailedDisplay(0)")
}

// -- fetchHourlyRate ---------------------------------------------------------

// TestFetchHourlyRate_Success verifies that fetchHourlyRate correctly
// parses the pricing API response and returns the on-demand rate
// converted to microcents.
func TestFetchHourlyRate_Success(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    got, err := fetchHourlyRate(context.Background(), testRegion, testInstanceType)
    requireNoError(t, err, "fetchHourlyRate success")
    requireEqual(t, got, mockHourlyRateMicrocents,
        "fetchHourlyRate returned rate")
}

// TestFetchHourlyRate_EmptyInstanceType verifies that an empty
// instance type returns an error without making an HTTP request.
func TestFetchHourlyRate_EmptyInstanceType(t *testing.T) {
    _, err := fetchHourlyRate(context.Background(), testRegion, "")
    requireError(t, err, "fetchHourlyRate empty instance type")
}

// TestFetchHourlyRate_APIError verifies that a non-200 response from
// the pricing API returns an error.
func TestFetchHourlyRate_APIError(t *testing.T) {
    srv := newMockPricingServerError(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    _, err := fetchHourlyRate(context.Background(), testRegion, testInstanceType)
    requireError(t, err, "fetchHourlyRate API error")
}

// TestFetchHourlyRate_InstanceNotFound verifies that a response
// containing no matching instance type returns an error.
func TestFetchHourlyRate_InstanceNotFound(t *testing.T) {
    srv := newMockPricingServerEmpty(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    _, err := fetchHourlyRate(context.Background(), testRegion, testInstanceType)
    requireError(t, err, "fetchHourlyRate instance not found")
}

// TestFetchHourlyRate_ContextCancelled verifies that a cancelled
// context causes fetchHourlyRate to return an error promptly.
func TestFetchHourlyRate_ContextCancelled(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    _, err := fetchHourlyRate(ctx, testRegion, testInstanceType)
    requireError(t, err, "fetchHourlyRate cancelled context")
}

// TestFetchHourlyRate_ReturnsMicrocents verifies that the returned
// value is in microcents not dollars. A $0.50 response must return
// 50,000 not 0 or 50 or 0.50.
func TestFetchHourlyRate_ReturnsMicrocents(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    got, err := fetchHourlyRate(context.Background(), testRegion, testInstanceType)
    requireNoError(t, err, "fetchHourlyRate returns microcents")
    requireTrue(t, got >= 1_000,
        "fetchHourlyRate: returned value should be microcents not dollars or cents")
}

// -- estimateCost ------------------------------------------------------------

// TestEstimateCost_CorrectDuration verifies that the estimated duration
// is ramp + until + 5 minutes overhead.
func TestEstimateCost_CorrectDuration(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost duration")

    expected := s.cfg.Ramp + s.cfg.Until + 5*time.Minute
    requireEqual(t, estimate.Duration, expected,
        "estimateCost duration")
}

// TestEstimateCost_PlanetCount verifies that estimate contains one
// entry per planet.
func TestEstimateCost_PlanetCount(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost planet count")
    requireEqual(t, len(estimate.Planets), len(s.planets),
        "estimateCost planet count")
}

// TestEstimateCost_HourlyRateInMicrocents verifies that each planet
// estimate carries the hourly rate in microcents matching the mock
// server response.
func TestEstimateCost_HourlyRateInMicrocents(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost hourly rate")

    for _, pe := range estimate.Planets {
        requireEqual(t, pe.HourlyRate, mockHourlyRateMicrocents,
            "estimateCost planet hourly rate")
    }
}

// TestEstimateCost_TotalCostAccumulates verifies that TotalCost is
// the sum of all planet ProjectedCost values.
func TestEstimateCost_TotalCostAccumulates(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost total cost")

    var sum int64
    for _, pe := range estimate.Planets {
        sum += pe.ProjectedCost
    }
    requireEqual(t, estimate.TotalCost, sum,
        "estimateCost TotalCost equals sum of planet costs")
}

// TestEstimateCost_ProjectedCostNonZero verifies that each planet's
// projected cost is greater than zero when pricing is available.
func TestEstimateCost_ProjectedCostNonZero(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost projected cost")

    for _, pe := range estimate.Planets {
        requireTrue(t, pe.ProjectedCost > 0,
            "estimateCost planet projected cost should be non-zero")
    }
}

// TestEstimateCost_APIFailureIsNonFatal verifies that when the pricing
// API is unavailable the estimate still returns with zero cost for
// the affected planets rather than returning an error.
func TestEstimateCost_APIFailureIsNonFatal(t *testing.T) {
    srv := newMockPricingServerError(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost API failure is non-fatal")
    requireNotNil(t, estimate, "estimateCost returns estimate on API failure")

    for _, pe := range estimate.Planets {
        requireEqual(t, pe.HourlyRate, int64(0),
            "estimateCost planet rate is 0 on API failure")
    }
}

// TestEstimateCost_RegionAndTypePopulated verifies that each planet
// estimate carries the correct region and instance type from the
// sovereign config.
func TestEstimateCost_RegionAndTypePopulated(t *testing.T) {
    srv := newMockPricingServer(t)
    original := pricingAPIBaseURL
    pricingAPIBaseURL = srv.URL + "/"
    t.Cleanup(func() { pricingAPIBaseURL = original })

    s := testSovereign(t)
    estimate, err := s.estimateCost()
    requireNoError(t, err, "estimateCost region and type")

    for _, pe := range estimate.Planets {
        requireEqual(t, pe.Region, testRegion,
            "estimateCost planet region")
        requireEqual(t, pe.InstanceType, testInstanceType,
            "estimateCost planet instance type")
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

// BenchmarkDollarToMicrocents measures conversion throughput.
// Called once per planet per pricing API response -- should be
// sub-nanosecond.
func BenchmarkDollarToMicrocents(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        dollarToMicrocents(0.50)
    }
}

// BenchmarkMicrocentsToDisplay measures display rendering throughput.
// Called once per planet in printCostEstimate -- should be fast.
func BenchmarkMicrocentsToDisplay(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        microcentsToDisplay(50_000)
    }
}

// BenchmarkMicrocentsToDetailedDisplay measures detailed display
// rendering throughput.
func BenchmarkMicrocentsToDetailedDisplay(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        microcentsToDetailedDisplay(123_456)
    }
}

// BenchmarkEstimateCost measures full cost estimation throughput
// against the mock pricing server with 5 planets.
func BenchmarkEstimateCost(b *testing.B) {
	srv := newMockPricingServer(b)
	original := pricingAPIBaseURL
	pricingAPIBaseURL = srv.URL + "/"
	b.Cleanup(func() { pricingAPIBaseURL = original })

	cfg := testConfig()
	cfg.Planets = 5
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	stateDir := b.TempDir()
	ledger, err := NewLedger(stateDir, time.Now())
	if err != nil {
		b.Fatalf("BenchmarkEstimateCost: NewLedger: %v", err)
	}

	s := &Sovereign{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		ledger: ledger,
	}
	s.planets = s.initPlanets()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.estimateCost()
	}
}

// -- Fuzz --------------------------------------------------------------------

// FuzzDollarToMicrocents verifies that dollarToMicrocents never panics
// on arbitrary float64 input and always returns a non-negative value
// for non-negative input.
func FuzzDollarToMicrocents(f *testing.F) {
    f.Add(0.0)
    f.Add(0.50)
    f.Add(1.00)
    f.Add(0.0034)
    f.Add(99.99)

    f.Fuzz(func(t *testing.T, dollars float64) {
        if dollars < 0 {
            return // negative prices are not defined
        }
        got := dollarToMicrocents(dollars)
        if got < 0 {
            t.Errorf("dollarToMicrocents(%v) returned negative microcents: %d",
                dollars, got)
        }
    })
}

// FuzzMicrocentsToDisplay verifies that microcentsToDisplay never panics
// on arbitrary int64 input and always returns a non-empty string
// beginning with "$".
func FuzzMicrocentsToDisplay(f *testing.F) {
    f.Add(int64(0))
    f.Add(int64(50_000))
    f.Add(int64(100_000))
    f.Add(int64(999_999))
    f.Add(int64(1_000_000))

    f.Fuzz(func(t *testing.T, mc int64) {
        if mc < 0 {
            return // negative costs are not defined
        }
        got := microcentsToDisplay(mc)
        if len(got) == 0 {
            t.Errorf("microcentsToDisplay(%d) returned empty string", mc)
        }
        if got[0] != '$' {
            t.Errorf("microcentsToDisplay(%d) = %q -- must begin with '$'", mc, got)
        }
    })
}​​​​​​​​​​​​​​​​
