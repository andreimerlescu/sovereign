package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// defaultVantageBaseURL is the instances.vantage.sh pricing endpoint.
	defaultVantageBaseURL = "https://instances.vantage.sh/aws/pricing/"

	// pricingAPITimeout is the maximum time allowed for a single
	// request to the pricing API.
	pricingAPITimeout = 10 * time.Second
)

// vantageResponse is the subset of the instances.vantage.sh API
// response that the VantagePricer needs. OnDemand is a float64 dollar
// amount that is converted to microcents immediately on receipt and
// never stored as float64.
type vantageResponse struct {
	InstanceType string  `json:"instance_type"`
	Region       string  `json:"region"`
	OnDemand     float64 `json:"on_demand"`
}

// VantagePricer implements Pricer using the instances.vantage.sh API.
// It prices AWS instances by region and instance type using on-demand
// rates. On-demand rates are used rather than spot pricing because spot
// availability is not guaranteed at provisioning time.
//
// VantagePricer is safe for concurrent use.
type VantagePricer struct {
	baseURL    string
	httpClient *http.Client
}

// NewVantagePricer constructs a VantagePricer using the default
// instances.vantage.sh endpoint and a default HTTP client.
func NewVantagePricer() *VantagePricer {
	return &VantagePricer{
		baseURL:    defaultVantageBaseURL,
		httpClient: &http.Client{},
	}
}

// NewVantagePricerWithURL constructs a VantagePricer with a custom
// base URL. Used in tests to point at a mock server.
func NewVantagePricerWithURL(baseURL string) *VantagePricer {
	return &VantagePricer{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// FetchHourlyRate queries instances.vantage.sh for the on-demand hourly
// rate of the instance type and region in the provided query.
//
// Returns an error when:
//   - InstanceType is empty
//   - the HTTP request fails or times out
//   - the API returns a non-200 status
//   - the instance type is not found in the response
//
// The OS field in PriceQuery defaults to linux when empty.
// The Provider field is not used -- vantage.sh is AWS-specific.
func (p *VantagePricer) FetchHourlyRate(ctx context.Context, query PriceQuery) (int64, error) {
	if query.InstanceType == "" {
		return 0, fmt.Errorf("vantage: instance type not specified")
	}

	os := query.OS
	if os == "" {
		os = "linux"
	}

	url := fmt.Sprintf(
		"%s?region=%s&os=%s&instance_type=%s",
		p.baseURL, query.Region, os, query.InstanceType,
	)

	reqCtx, cancel := context.WithTimeout(ctx, pricingAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("vantage: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sovereign/pricing")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("vantage: fetch pricing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("vantage: API returned %d for %s in %s",
			resp.StatusCode, query.InstanceType, query.Region)
	}

	var instances []vantageResponse
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return 0, fmt.Errorf("vantage: decode response: %w", err)
	}

	for _, inst := range instances {
		if inst.InstanceType == query.InstanceType {
			return DollarToMicrocents(inst.OnDemand), nil
		}
	}

	return 0, fmt.Errorf("vantage: instance type %s not found in response for region %s",
		query.InstanceType, query.Region)
}
