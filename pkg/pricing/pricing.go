package pricing

import (
	"context"
	"fmt"
)

// Pricer is the interface every pricing source must implement.
// A Pricer takes a PriceQuery describing a compute resource and
// returns the hourly rate as int64 microcents.
//
// All monetary values in this package are int64 microcents.
// Float64 never escapes the Pricer implementation boundary.
//
// Unit reference:
//
//	1 dollar  = 100,000 microcents
//	1 cent    = 1,000 microcents
//	$0.50/hr  = 50,000 microcents
type Pricer interface {
	// FetchHourlyRate returns the on-demand hourly rate for the
	// described compute resource as int64 microcents.
	//
	// Returns an error when the resource cannot be priced. The
	// caller treats all errors as non-fatal and records zero
	// microcents for the affected query.
	FetchHourlyRate(ctx context.Context, query PriceQuery) (int64, error)
}

// PriceQuery describes a compute resource to be priced.
// Pricers use whichever fields are relevant to their pricing source
// and ignore the rest.
type PriceQuery struct {
	// Provider identifies the cloud provider.
	// Examples: aws, digitalocean, ovh
	Provider string

	// Region is the cloud region or datacenter code.
	// Examples: us-east-1, nyc1, GRA11
	Region string

	// InstanceType is the instance size or flavor name.
	// Examples: t3.medium, s-2vcpu-4gb, b2-7
	InstanceType string

	// OS is the operating system for pricing lookup.
	// Examples: linux, windows
	// Defaults to linux when empty.
	OS string
}

// PriceResult holds the pricing result for a single PriceQuery.
// HourlyRate is in int64 microcents. Zero indicates pricing was
// unavailable for this query.
type PriceResult struct {
	Query         PriceQuery
	HourlyRate    int64 // microcents per hour
	ProjectedCost int64 // microcents for the estimated duration
}

// EstimateConfig carries the parameters for a cost estimation run.
type EstimateConfig struct {
	// Queries is the list of compute resources to price.
	Queries []PriceQuery

	// Hours is the estimated run duration in hours.
	// Used to compute ProjectedCost per result.
	Hours float64
}

// Estimate holds the results of a cost estimation run.
type Estimate struct {
	Results   []PriceResult
	TotalCost int64 // microcents -- sum of all ProjectedCost values
}

// EstimateCost queries the provided Pricer for each query in cfg and
// returns an Estimate with per-query results and a total projected cost.
//
// Pricing failures are non-fatal. A failed query records zero microcents
// and a warning is printed to the provided printf function. The run
// continues with remaining queries.
//
// The printf argument accepts a format string and args matching
// fmt.Printf. Pass fmt.Printf for production use or a test-controlled
// function in tests.
func EstimateCost(
	ctx context.Context,
	pricer Pricer,
	cfg EstimateConfig,
	printf func(string, ...any),
) (*Estimate, error) {
	if printf == nil {
		printf = func(string, ...any) {}
	}

	estimate := &Estimate{
		Results: make([]PriceResult, len(cfg.Queries)),
	}

	for i, query := range cfg.Queries {
		rate, err := pricer.FetchHourlyRate(ctx, query)
		if err != nil {
			printf("  sovereign: warning: could not fetch pricing for %s in %s: %v\n",
				query.InstanceType, query.Region, err)
			rate = 0
		}

		projected := int64(float64(rate) * cfg.Hours)
		estimate.Results[i] = PriceResult{
			Query:         query,
			HourlyRate:    rate,
			ProjectedCost: projected,
		}
		estimate.TotalCost += projected
	}

	return estimate, nil
}

// FormatQuery returns a human-readable description of a PriceQuery
// for use in log output.
func FormatQuery(q PriceQuery) string {
	return fmt.Sprintf("%s %s %s", q.Provider, q.Region, q.InstanceType)
}
