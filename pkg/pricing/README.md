# pricing

> Cloud compute pricing for sovereign.
> A Pricer interface with pluggable pricing sources and integer
> microcent arithmetic throughout.

---

## What This Package Is

This package provides cost estimation for sovereign runs. Before
any infrastructure is provisioned, sovereign queries a pricing
source for the hourly rate of each planet's compute resource and
presents a projected cost to the operator.

The design is source-agnostic. The Pricer interface accepts any
pricing backend. VantagePricer is the default implementation using
the public instances.vantage.sh API. Future implementations can
price DigitalOcean Droplets, OVH flavors, or any other compute
resource by implementing the same interface.

All monetary values in this package are int64 microcents. Float64
never escapes the Pricer implementation boundary. This eliminates
float64 rounding errors in currency arithmetic.

---

## Files

    pricing.go        Pricer interface, PriceQuery, PriceResult,
                      EstimateConfig, Estimate, EstimateCost, FormatQuery
    microcents.go     DollarToMicrocents, MicrocentsToDisplay,
                      MicrocentsToDetailedDisplay, arithmetic constants
    vantage.go        VantagePricer -- instances.vantage.sh implementation
    pricing_test.go   full test, benchmark, and fuzz coverage

---

## Dependencies

    standard library
    net/http           HTTP client for VantagePricer
    encoding/json      API response decoding

This package never imports sovereign main or any other sovereign
package. It has no dependency on the events or ledger packages.

---

## Microcent Arithmetic

Unit reference:

    1 dollar      = 100,000 microcents
    1 cent        = 1,000 microcents
    $0.50/hr      = 50,000 microcents
    $1.234567/hr  = 123,456 microcents (sub-cent precision preserved)

The only place float64 currency exists in this package is inside
DollarToMicrocents where the API response is immediately converted.
The result is never stored as float64 anywhere.

Constants:

    MicroCentsPerDollar int64 = 100,000
    MicroCentsPerCent   int64 = 1,000

Display functions:

    MicrocentsToDisplay(mc int64) string
        -- two decimal places
        -- $0.50, $1.23, $10.00
        -- sub-cent values round down

    MicrocentsToDetailedDisplay(mc int64) string
        -- five decimal places
        -- $0.50000, $1.23456
        -- used in per-planet cost breakdown output

---

## The Pricer Interface

    type Pricer interface {
        FetchHourlyRate(ctx context.Context, query PriceQuery) (int64, error)
    }

FetchHourlyRate returns the on-demand hourly rate as int64 microcents.
Returns an error when pricing is unavailable. The caller treats all
errors as non-fatal and records zero microcents for the affected query.

---

## PriceQuery

    type PriceQuery struct {
        Provider     string   // aws, digitalocean, ovh
        Region       string   // us-east-1, nyc1, GRA11
        InstanceType string   // t3.medium, s-2vcpu-4gb, b2-7
        OS           string   // linux, windows -- defaults to linux
    }

Pricers use whichever fields are relevant to their pricing source
and ignore the rest. VantagePricer uses Region, InstanceType, and OS.
It ignores Provider since instances.vantage.sh is AWS-specific.

---

## EstimateCost

EstimateCost queries a Pricer for each query in EstimateConfig and
returns an Estimate with per-query results and a total projected cost:

    estimate, err := pricing.EstimateCost(
        ctx,
        pricing.NewVantagePricer(),
        pricing.EstimateConfig{
            Queries: queries,
            Hours:   (ramp + until + 5*time.Minute).Hours(),
        },
        fmt.Printf,
    )

The printf argument is called for non-fatal pricing warnings. Pass
fmt.Printf for production use or nil to suppress warnings.

EstimateCost never returns an error. Pricing failures are non-fatal
-- they record zero microcents for the affected query and log a
warning via the printf function.

---

## VantagePricer

The default Pricer implementation. Queries instances.vantage.sh for
AWS on-demand pricing by region and instance type.

    pricer := pricing.NewVantagePricer()

For tests, use NewVantagePricerWithURL to point at a mock server:

    pricer := pricing.NewVantagePricerWithURL(mockServer.URL + "/")

VantagePricer uses on-demand rates rather than spot pricing because
spot availability is not guaranteed at provisioning time. On-demand
gives a conservative upper-bound cost estimate.

Request timeout is 10 seconds per query. The parent context is
also respected -- cancelling the context cancels the request.

---

## Implementing a New Pricer

To add a DigitalOcean pricer:

    type DigitalOceanPricer struct{ token string }

    func (p *DigitalOceanPricer) FetchHourlyRate(
        ctx context.Context,
        query pricing.PriceQuery,
    ) (int64, error) {
        // query DigitalOcean pricing API
        // convert dollar response to microcents via DollarToMicrocents
        // return int64 microcents
    }

Then pass it to EstimateCost:

    estimate, err := pricing.EstimateCost(ctx, &DigitalOceanPricer{},
        cfg, fmt.Printf)

The Pricer contract requires that float64 never escapes the
implementation. Use DollarToMicrocents at the API boundary.

---

## Usage in sovereign

sovereign's estimateCost method constructs PriceQuery values from
the planet slice and calls EstimateCost:

    queries := make([]pricing.PriceQuery, len(s.planets))
    for i, planet := range s.planets {
        queries[i] = pricing.PriceQuery{
            Provider:     s.cfg.Provider,
            Region:       planet.Region,
            InstanceType: planet.InstanceType,
        }
    }

    hours := (s.cfg.Ramp + s.cfg.Until + 5*time.Minute).Hours()
    estimate, err := pricing.EstimateCost(
        s.ctx,
        pricing.NewVantagePricer(),
        pricing.EstimateConfig{Queries: queries, Hours: hours},
        fmt.Printf,
    )

printCostEstimate in sovereign.go uses the display functions:

    pricing.MicrocentsToDetailedDisplay(r.HourlyRate)
    pricing.MicrocentsToDisplay(estimate.TotalCost)

---

## Import Path

    import "github.com/andreimerlescu/sovereign/pkg/pricing"

---

## Testing

    go test -race ./pkg/pricing/...
    go test -bench=. -benchmem ./pkg/pricing/...
    go test -fuzz=FuzzDollarToMicrocents -fuzztime=10s ./pkg/pricing/...
    go test -fuzz=FuzzMicrocentsToDisplay -fuzztime=10s ./pkg/pricing/...​​​​​​​​​​​​​​​​
