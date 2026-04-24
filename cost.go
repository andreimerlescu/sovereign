package main

import (
	"fmt"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/pricing"
)

// CostEstimate holds the projected cost of a sovereign run before
// any infrastructure is provisioned. All monetary values are int64
// microcents.
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

// estimateCost calculates the projected cost of the sovereign run
// by querying the configured pricing source for each planet's region
// and instance type.
//
// Duration is estimated as ramp + until + 5 minutes provisioning
// and collection overhead per planet.
//
// If pricing cannot be retrieved for a planet the hourly rate is
// recorded as 0 microcents and a warning is printed. Cost estimation
// failure is non-fatal -- the run proceeds with a zero cost estimate
// for the affected planet.
func (s *Sovereign) estimateCost() (*CostEstimate, error) {
	duration := s.cfg.Ramp + s.cfg.Until + 5*time.Minute
	hours := duration.Hours()

	queries := make([]pricing.PriceQuery, len(s.planets))
	for i, planet := range s.planets {
		queries[i] = pricing.PriceQuery{
			Provider:     s.cfg.Provider,
			Region:       planet.Region,
			InstanceType: planet.InstanceType,
		}
	}

	cfg := pricing.EstimateConfig{
		Queries: queries,
		Hours:   hours,
	}

	pricer := pricing.NewVantagePricer()
	printf := func(format string, args ...any) {
		fmt.Printf(format, args...)
	}
	estimate, err := pricing.EstimateCost(s.ctx, pricer, cfg, printf)
	if err != nil {
		return nil, fmt.Errorf("cost: estimate: %w", err)
	}

	result := &CostEstimate{
		Duration:  duration,
		TotalCost: estimate.TotalCost,
		Planets:   make([]PlanetEstimate, len(estimate.Results)),
	}

	for i, r := range estimate.Results {
		result.Planets[i] = PlanetEstimate{
			ID:            s.planets[i].ID,
			Region:        r.Query.Region,
			InstanceType:  r.Query.InstanceType,
			HourlyRate:    r.HourlyRate,
			ProjectedCost: r.ProjectedCost,
		}
	}

	return result, nil
}

// totalLemmings returns the total number of lemmings that will run
// across all planets combined.
func (s *Sovereign) totalLemmings() int64 {
	return int64(s.cfg.Planets) * s.cfg.Terrain * s.cfg.Pack
}
