package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// Pricing in sovereign is represented as int64 microcents throughout.
// This eliminates float64 rounding errors in currency arithmetic.
//
// Unit reference:
//   1 dollar      = 100 cents       = 100,000 microcents
//   1 cent        = 1,000 microcents
//   $0.50/hr      = 50,000 microcents
//   $1.234567/hr  = 123,456 microcents (sub-cent precision preserved)
//
// The only place float64 currency exists in sovereign is at the
// API boundary in fetchHourlyRate where the instances.vantage.sh
// response is immediately converted to microcents via dollarToMicrocents
// and never stored as float64 anywhere in the system.

const (
    // microCentsPerDollar is the number of microcents in one US dollar.
    microCentsPerDollar int64 = 100_000

    // microCentsPerCent is the number of microcents in one US cent.
    microCentsPerCent int64 = 1_000

    // pricingAPITimeout is the maximum time allowed for a single
    // request to the instances.vantage.sh pricing API.
    pricingAPITimeout = 10 * time.Second

    // pricingAPIBaseURL is the instances.vantage.sh endpoint used
    // for AWS on-demand pricing lookups.
    pricingAPIBaseURL = "https://instances.vantage.sh/aws/pricing/"
)

// CostEstimate holds the projected cost of a sovereign run before
// any infrastructure is provisioned. All monetary values are int64
// microcents. Use microcentsToDisplay for human-readable output.
type CostEstimate struct {
    Planets   []PlanetEstimate
    Duration  time.Duration
    TotalCost int64 // microcents
}

// PlanetEstimate holds the projected cost for a single planet.
// All monetary values are int64 microcents.
type PlanetEstimate struct {
    ID            int
    Region        string
    InstanceType  string
    HourlyRate    int64 // microcents per hour
    ProjectedCost int64 // microcents for estimated duration
}

// vantageResponse is the subset of the instances.vantage.sh API
// response that sovereign needs for cost estimation. The OnDemand
// field is a float64 dollar amount from the API and is converted
// to microcents immediately -- it is never stored as float64.
type vantageResponse struct {
    InstanceType string  `json:"instance_type"`
    Region       string  `json:"region"`
    OnDemand     float64 `json:"on_demand"`
    Spot         float64 `json:"spot"`
}

// dollarToMicrocents converts a float64 dollar amount received from
// the pricing API into an int64 microcent value. This is the only
// place in sovereign where float64 currency arithmetic occurs.
//
// Examples:
//   0.50    -> 50,000
//   1.00    -> 100,000
//   0.0034  -> 340
//   0.12345 -> 12,345
func dollarToMicrocents(dollars float64) int64 {
    return int64(dollars * float64(microCentsPerDollar))
}

// microcentsToDisplay renders a microcent value as a human-readable
// dollar string with two decimal places.
//
// Examples:
//   50,000  -> "$0.50"
//   100,000 -> "$1.00"
//   123,456 -> "$1.23"
//   340     -> "$0.00"  (sub-cent values round down at display time)
func microcentsToDisplay(mc int64) string {
    dollars := mc / microCentsPerDollar
    cents := (mc % microCentsPerDollar) / microCentsPerCent
    return fmt.Sprintf("$%d.%02d", dollars, cents)
}

// microcentsToDetailedDisplay renders a microcent value with full
// precision for use in cost breakdowns where sub-cent accuracy matters.
//
// Examples:
//   50,000  -> "$0.50000"
//   123,456 -> "$1.23456"
func microcentsToDetailedDisplay(mc int64) string {
    dollars := mc / microCentsPerDollar
    remainder := mc % microCentsPerDollar
    return fmt.Sprintf("$%d.%05d", dollars, remainder)
}

// estimateCost calculates the projected cost of the sovereign run
// by querying instances.vantage.sh for current AWS on-demand pricing
// for each planet's region and instance type.
//
// Duration is estimated as ramp + until + 5 minutes provisioning
// and collection overhead per planet.
//
// If pricing cannot be retrieved for a planet the hourly rate is
// recorded as 0 microcents and a warning is printed. Cost estimation
// failure is non-fatal -- the run proceeds with a zero cost estimate
// for the affected planet.
func (s *Sovereign) estimateCost() (*CostEstimate, error) {
    estimate := &CostEstimate{
        Planets:  make([]PlanetEstimate, len(s.planets)),
        Duration: s.cfg.Ramp + s.cfg.Until + 5*time.Minute,
    }

    hours := estimate.Duration.Hours()

    for i, planet := range s.planets {
        rate, err := fetchHourlyRate(s.ctx, planet.Region, planet.InstanceType)
        if err != nil {
            fmt.Printf(
                "  sovereign: warning: could not fetch pricing for %s in %s: %v\n",
                planet.InstanceType, planet.Region, err,
            )
            rate = 0
        }

        projected := int64(float64(rate) * hours)
        estimate.Planets[i] = PlanetEstimate{
            ID:            planet.ID,
            Region:        planet.Region,
            InstanceType:  planet.InstanceType,
            HourlyRate:    rate,
            ProjectedCost: projected,
        }
        estimate.TotalCost += projected
    }

    return estimate, nil
}

// fetchHourlyRate queries the instances.vantage.sh API for the
// on-demand hourly rate of a given instance type in a given AWS region.
// Returns the rate as int64 microcents per hour.
//
// The on-demand rate is used rather than spot pricing because spot
// availability is not guaranteed at provisioning time. Sovereign
// uses on-demand for conservative cost estimation.
//
// Returns an error when:
//   -- instance type is empty
//   -- the HTTP request fails or times out
//   -- the API returns a non-200 status
//   -- the instance type is not found in the response
//
// The caller treats all errors as non-fatal and records 0 microcents
// for the affected planet.
func fetchHourlyRate(ctx context.Context, region, instanceType string) (int64, error) {
    if instanceType == "" {
        return 0, fmt.Errorf("instance type not specified")
    }

    url := fmt.Sprintf(
        "%s?region=%s&os=linux&instance_type=%s",
        pricingAPIBaseURL, region, instanceType,
    )

    reqCtx, cancel := context.WithTimeout(ctx, pricingAPITimeout)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
    if err != nil {
        return 0, fmt.Errorf("build pricing request: %w", err)
    }
    req.Header.Set("Accept", "application/json")
    req.Header.Set("User-Agent", "sovereign/"+version)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return 0, fmt.Errorf("fetch pricing: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return 0, fmt.Errorf("pricing API returned %d for %s in %s",
            resp.StatusCode, instanceType, region)
    }

    var instances []vantageResponse
    if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
        return 0, fmt.Errorf("decode pricing response: %w", err)
    }

    for _, inst := range instances {
        if inst.InstanceType == instanceType {
            return dollarToMicrocents(inst.OnDemand), nil
        }
    }

    return 0, fmt.Errorf("instance type %s not found in pricing response for region %s",
        instanceType, region)
}

// totalLemmings returns the total number of lemmings that will run
// across all planets combined. Used in cost summary output.
func (s *Sovereign) totalLemmings() int64 {
    return int64(s.cfg.Planets) * s.cfg.Terrain * s.cfg.Pack
}​​​​​​​​​​​​​​​​
