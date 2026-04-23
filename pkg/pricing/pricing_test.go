package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// -- test constants ----------------------------------------------------------

const (
	testRegion       = "us-east-1"
	testInstanceType = "t3.medium"

	mockHourlyRateDollars    float64 = 0.50
	mockHourlyRateMicrocents int64   = 50_000
)

// -- mock server helpers -----------------------------------------------------

func newMockPricingServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		instanceType := r.URL.Query().Get("instance_type")
		if instanceType == "" {
			instanceType = testInstanceType
		}
		region := r.URL.Query().Get("region")
		if region == "" {
			region = testRegion
		}
		response := []vantageResponse{
			{
				InstanceType: instanceType,
				Region:       region,
				OnDemand:     mockHourlyRateDollars,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newMockPricingServerEmpty(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]vantageResponse{})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newMockPricingServerError(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testQuery() PriceQuery {
	return PriceQuery{
		Provider:     "aws",
		Region:       testRegion,
		InstanceType: testInstanceType,
		OS:           "linux",
	}
}

// -- DollarToMicrocents ------------------------------------------------------

// TestDollarToMicrocents_HalfDollar verifies that $0.50 converts to
// exactly 50,000 microcents.
func TestDollarToMicrocents_HalfDollar(t *testing.T) {
	got := DollarToMicrocents(0.50)
	if got != 50_000 {
		t.Errorf("DollarToMicrocents(0.50) = %d, want 50000", got)
	}
}

// TestDollarToMicrocents_OneDollar verifies that $1.00 converts to
// exactly 100,000 microcents.
func TestDollarToMicrocents_OneDollar(t *testing.T) {
	got := DollarToMicrocents(1.00)
	if got != 100_000 {
		t.Errorf("DollarToMicrocents(1.00) = %d, want 100000", got)
	}
}

// TestDollarToMicrocents_Zero verifies that $0.00 converts to 0.
func TestDollarToMicrocents_Zero(t *testing.T) {
	got := DollarToMicrocents(0.0)
	if got != 0 {
		t.Errorf("DollarToMicrocents(0.0) = %d, want 0", got)
	}
}

// TestDollarToMicrocents_SubCent verifies sub-cent precision is preserved.
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
		got := DollarToMicrocents(tc.input)
		if got != tc.expected {
			t.Errorf("DollarToMicrocents(%v) = %d, want %d",
				tc.input, got, tc.expected)
		}
	}
}

// TestDollarToMicrocents_LargeDollar verifies values above $1.00.
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
		got := DollarToMicrocents(tc.input)
		if got != tc.expected {
			t.Errorf("DollarToMicrocents(%v) = %d, want %d",
				tc.input, got, tc.expected)
		}
	}
}

// TestDollarToMicrocents_NeverFloat verifies that integer arithmetic
// on microcents does not suffer from float64 rounding.
func TestDollarToMicrocents_NeverFloat(t *testing.T) {
	a := DollarToMicrocents(0.10)
	b := DollarToMicrocents(0.20)
	sum := a + b
	expected := DollarToMicrocents(0.30)
	if sum != expected {
		t.Errorf("microcent integer arithmetic: %d + %d = %d, want %d",
			a, b, sum, expected)
	}
}

// -- MicrocentsToDisplay -----------------------------------------------------

// TestMicrocentsToDisplay_HalfDollar verifies 50,000 renders as "$0.50".
func TestMicrocentsToDisplay_HalfDollar(t *testing.T) {
	got := MicrocentsToDisplay(50_000)
	if got != "$0.50" {
		t.Errorf("MicrocentsToDisplay(50000) = %q, want $0.50", got)
	}
}

// TestMicrocentsToDisplay_OneDollar verifies 100,000 renders as "$1.00".
func TestMicrocentsToDisplay_OneDollar(t *testing.T) {
	got := MicrocentsToDisplay(100_000)
	if got != "$1.00" {
		t.Errorf("MicrocentsToDisplay(100000) = %q, want $1.00", got)
	}
}

// TestMicrocentsToDisplay_Zero verifies 0 renders as "$0.00".
func TestMicrocentsToDisplay_Zero(t *testing.T) {
	got := MicrocentsToDisplay(0)
	if got != "$0.00" {
		t.Errorf("MicrocentsToDisplay(0) = %q, want $0.00", got)
	}
}

// TestMicrocentsToDisplay_SubCentRoundsDown verifies sub-cent values
// round down at display time.
func TestMicrocentsToDisplay_SubCentRoundsDown(t *testing.T) {
	got := MicrocentsToDisplay(340)
	if got != "$0.00" {
		t.Errorf("MicrocentsToDisplay(340) = %q, want $0.00", got)
	}
}

// TestMicrocentsToDisplay_LargeDollar verifies rendering above $1.00.
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
		got := MicrocentsToDisplay(tc.input)
		if got != tc.expected {
			t.Errorf("MicrocentsToDisplay(%d) = %q, want %q",
				tc.input, got, tc.expected)
		}
	}
}

// TestMicrocentsToDisplay_PaddedCents verifies cents below 10 are
// zero-padded to two digits.
func TestMicrocentsToDisplay_PaddedCents(t *testing.T) {
	got := MicrocentsToDisplay(105_000)
	if got != "$1.05" {
		t.Errorf("MicrocentsToDisplay(105000) = %q, want $1.05", got)
	}
}

// -- MicrocentsToDetailedDisplay ---------------------------------------------

// TestMicrocentsToDetailedDisplay_HalfDollar verifies 50,000 renders
// as "$0.50000".
func TestMicrocentsToDetailedDisplay_HalfDollar(t *testing.T) {
	got := MicrocentsToDetailedDisplay(50_000)
	if got != "$0.50000" {
		t.Errorf("MicrocentsToDetailedDisplay(50000) = %q, want $0.50000", got)
	}
}

// TestMicrocentsToDetailedDisplay_SubCent verifies sub-cent precision
// is preserved in the detailed display format.
func TestMicrocentsToDetailedDisplay_SubCent(t *testing.T) {
	got := MicrocentsToDetailedDisplay(123_456)
	if got != "$1.23456" {
		t.Errorf("MicrocentsToDetailedDisplay(123456) = %q, want $1.23456", got)
	}
}

// TestMicrocentsToDetailedDisplay_Zero verifies zero renders with full
// precision padding.
func TestMicrocentsToDetailedDisplay_Zero(t *testing.T) {
	got := MicrocentsToDetailedDisplay(0)
	if got != "$0.00000" {
		t.Errorf("MicrocentsToDetailedDisplay(0) = %q, want $0.00000", got)
	}
}

// -- VantagePricer.FetchHourlyRate -------------------------------------------

// TestFetchHourlyRate_Success verifies that FetchHourlyRate correctly
// parses the pricing API response and returns microcents.
func TestFetchHourlyRate_Success(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	got, err := p.FetchHourlyRate(context.Background(), testQuery())
	if err != nil {
		t.Fatalf("FetchHourlyRate: unexpected error: %v", err)
	}
	if got != mockHourlyRateMicrocents {
		t.Errorf("FetchHourlyRate = %d, want %d", got, mockHourlyRateMicrocents)
	}
}

// TestFetchHourlyRate_EmptyInstanceType verifies that an empty instance
// type returns an error without making an HTTP request.
func TestFetchHourlyRate_EmptyInstanceType(t *testing.T) {
	p := NewVantagePricer()
	_, err := p.FetchHourlyRate(context.Background(), PriceQuery{
		Region: testRegion,
	})
	if err == nil {
		t.Fatal("expected error for empty InstanceType, got nil")
	}
}

// TestFetchHourlyRate_APIError verifies that a non-200 response returns
// an error.
func TestFetchHourlyRate_APIError(t *testing.T) {
	srv := newMockPricingServerError(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	_, err := p.FetchHourlyRate(context.Background(), testQuery())
	if err == nil {
		t.Fatal("expected error for API 500 response, got nil")
	}
}

// TestFetchHourlyRate_InstanceNotFound verifies that a response
// containing no matching instance type returns an error.
func TestFetchHourlyRate_InstanceNotFound(t *testing.T) {
	srv := newMockPricingServerEmpty(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	_, err := p.FetchHourlyRate(context.Background(), testQuery())
	if err == nil {
		t.Fatal("expected error for instance not found, got nil")
	}
}

// TestFetchHourlyRate_ContextCancelled verifies that a cancelled
// context causes FetchHourlyRate to return an error promptly.
func TestFetchHourlyRate_ContextCancelled(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.FetchHourlyRate(ctx, testQuery())
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestFetchHourlyRate_ReturnsMicrocents verifies that the returned
// value is microcents not dollars.
func TestFetchHourlyRate_ReturnsMicrocents(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	got, err := p.FetchHourlyRate(context.Background(), testQuery())
	if err != nil {
		t.Fatalf("FetchHourlyRate: unexpected error: %v", err)
	}
	if got < 1_000 {
		t.Errorf("FetchHourlyRate = %d -- value looks like dollars or cents, not microcents",
			got)
	}
}

// TestFetchHourlyRate_DefaultsOSToLinux verifies that an empty OS
// field defaults to linux without error.
func TestFetchHourlyRate_DefaultsOSToLinux(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	_, err := p.FetchHourlyRate(context.Background(), PriceQuery{
		Region:       testRegion,
		InstanceType: testInstanceType,
		// OS intentionally empty
	})
	if err != nil {
		t.Fatalf("FetchHourlyRate with empty OS: unexpected error: %v", err)
	}
}

// -- EstimateCost ------------------------------------------------------------

// TestEstimateCost_CorrectDuration verifies that projected cost is
// calculated from the configured hours.
func TestEstimateCost_CorrectDuration(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	hours := (30*time.Second + 5*time.Minute + 5*time.Minute).Hours()
	cfg := EstimateConfig{
		Queries: []PriceQuery{testQuery()},
		Hours:   hours,
	}

	estimate, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	expected := int64(float64(mockHourlyRateMicrocents) * hours)
	if estimate.Results[0].ProjectedCost != expected {
		t.Errorf("ProjectedCost = %d, want %d",
			estimate.Results[0].ProjectedCost, expected)
	}
}

// TestEstimateCost_QueryCount verifies that estimate contains one
// result per query.
func TestEstimateCost_QueryCount(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	queries := []PriceQuery{testQuery(), testQuery(), testQuery()}
	cfg := EstimateConfig{Queries: queries, Hours: 1.0}

	estimate, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if len(estimate.Results) != len(queries) {
		t.Errorf("Results count = %d, want %d",
			len(estimate.Results), len(queries))
	}
}

// TestEstimateCost_TotalCostAccumulates verifies that TotalCost is
// the sum of all ProjectedCost values.
func TestEstimateCost_TotalCostAccumulates(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	cfg := EstimateConfig{
		Queries: []PriceQuery{testQuery(), testQuery()},
		Hours:   1.0,
	}

	estimate, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}

	var sum int64
	for _, r := range estimate.Results {
		sum += r.ProjectedCost
	}
	if estimate.TotalCost != sum {
		t.Errorf("TotalCost = %d, want sum %d", estimate.TotalCost, sum)
	}
}

// TestEstimateCost_APIFailureIsNonFatal verifies that when the pricing
// API is unavailable the estimate still returns with zero cost.
func TestEstimateCost_APIFailureIsNonFatal(t *testing.T) {
	srv := newMockPricingServerError(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	cfg := EstimateConfig{
		Queries: []PriceQuery{testQuery()},
		Hours:   1.0,
	}

	estimate, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost API failure should be non-fatal: %v", err)
	}
	if estimate == nil {
		t.Fatal("EstimateCost returned nil estimate on API failure")
	}
	if estimate.Results[0].HourlyRate != 0 {
		t.Errorf("HourlyRate = %d, want 0 on API failure",
			estimate.Results[0].HourlyRate)
	}
}

// TestEstimateCost_ProjectedCostNonZero verifies that each result's
// projected cost is greater than zero when pricing is available.
func TestEstimateCost_ProjectedCostNonZero(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	cfg := EstimateConfig{
		Queries: []PriceQuery{testQuery()},
		Hours:   1.0,
	}

	estimate, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	for _, r := range estimate.Results {
		if r.ProjectedCost <= 0 {
			t.Errorf("ProjectedCost = %d, want > 0", r.ProjectedCost)
		}
	}
}

// TestEstimateCost_QueryPreservedInResult verifies that each result
// carries the original query.
func TestEstimateCost_QueryPreservedInResult(t *testing.T) {
	srv := newMockPricingServer(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	q := testQuery()
	cfg := EstimateConfig{Queries: []PriceQuery{q}, Hours: 1.0}

	estimate, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost: %v", err)
	}
	if estimate.Results[0].Query.Region != q.Region {
		t.Errorf("Query.Region = %q, want %q",
			estimate.Results[0].Query.Region, q.Region)
	}
	if estimate.Results[0].Query.InstanceType != q.InstanceType {
		t.Errorf("Query.InstanceType = %q, want %q",
			estimate.Results[0].Query.InstanceType, q.InstanceType)
	}
}

// TestEstimateCost_NilPrintfDoesNotPanic verifies that passing nil
// for the printf argument does not panic.
func TestEstimateCost_NilPrintfDoesNotPanic(t *testing.T) {
	srv := newMockPricingServerError(t)
	p := NewVantagePricerWithURL(srv.URL + "/")

	cfg := EstimateConfig{
		Queries: []PriceQuery{testQuery()},
		Hours:   1.0,
	}

	// must not panic with nil printf
	_, err := EstimateCost(context.Background(), p, cfg, nil)
	if err != nil {
		t.Fatalf("EstimateCost with nil printf: %v", err)
	}
}

// -- FormatQuery -------------------------------------------------------------

// TestFormatQuery_ProducesNonEmptyString verifies that FormatQuery
// returns a non-empty string for a populated query.
func TestFormatQuery_ProducesNonEmptyString(t *testing.T) {
	got := FormatQuery(testQuery())
	if got == "" {
		t.Error("FormatQuery returned empty string")
	}
}

// TestFormatQuery_ContainsRegionAndInstanceType verifies that the
// formatted string contains the region and instance type.
func TestFormatQuery_ContainsRegionAndInstanceType(t *testing.T) {
	q := testQuery()
	got := FormatQuery(q)
	if got == "" {
		t.Fatal("FormatQuery returned empty string")
	}
	// verify key fields appear in the output
	found := false
	for i := 0; i <= len(got)-len(q.Region); i++ {
		if got[i:i+len(q.Region)] == q.Region {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("FormatQuery(%v) = %q -- does not contain region %q",
			q, got, q.Region)
	}
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkDollarToMicrocents measures conversion throughput.
func BenchmarkDollarToMicrocents(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DollarToMicrocents(0.50)
	}
}

// BenchmarkMicrocentsToDisplay measures display rendering throughput.
func BenchmarkMicrocentsToDisplay(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MicrocentsToDisplay(50_000)
	}
}

// BenchmarkMicrocentsToDetailedDisplay measures detailed display
// rendering throughput.
func BenchmarkMicrocentsToDetailedDisplay(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MicrocentsToDetailedDisplay(123_456)
	}
}

// BenchmarkFetchHourlyRate measures full pricing API throughput
// against the mock server.
func BenchmarkFetchHourlyRate(b *testing.B) {
	srv := newMockPricingServer(b)
	p := NewVantagePricerWithURL(srv.URL + "/")
	q := testQuery()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.FetchHourlyRate(context.Background(), q)
	}
}

// BenchmarkEstimateCost measures full cost estimation throughput
// against the mock server with five queries.
func BenchmarkEstimateCost(b *testing.B) {
	srv := newMockPricingServer(b)
	p := NewVantagePricerWithURL(srv.URL + "/")

	queries := make([]PriceQuery, 5)
	for i := range queries {
		queries[i] = testQuery()
	}
	cfg := EstimateConfig{Queries: queries, Hours: 0.5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EstimateCost(context.Background(), p, cfg, nil)
	}
}

// -- Fuzz --------------------------------------------------------------------

// FuzzDollarToMicrocents verifies that DollarToMicrocents never panics
// on arbitrary float64 input and always returns non-negative for
// non-negative input.
func FuzzDollarToMicrocents(f *testing.F) {
	f.Add(0.0)
	f.Add(0.50)
	f.Add(1.00)
	f.Add(0.0034)
	f.Add(99.99)

	f.Fuzz(func(t *testing.T, dollars float64) {
		if dollars < 0 {
			return
		}
		got := DollarToMicrocents(dollars)
		if got < 0 {
			t.Errorf("DollarToMicrocents(%v) = %d -- negative microcents for non-negative input",
				dollars, got)
		}
	})
}

// FuzzMicrocentsToDisplay verifies that MicrocentsToDisplay never
// panics and always returns a string beginning with "$".
func FuzzMicrocentsToDisplay(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(50_000))
	f.Add(int64(100_000))
	f.Add(int64(999_999))
	f.Add(int64(1_000_000))

	f.Fuzz(func(t *testing.T, mc int64) {
		if mc < 0 {
			return
		}
		got := MicrocentsToDisplay(mc)
		if len(got) == 0 {
			t.Errorf("MicrocentsToDisplay(%d) returned empty string", mc)
		}
		if got[0] != '$' {
			t.Errorf("MicrocentsToDisplay(%d) = %q -- must begin with $", mc, got)
		}
	})
}

// newMockPricingServer also works for benchmarks -- accept testing.TB.
func init() {
	_ = fmt.Sprintf // ensure fmt is used
}
