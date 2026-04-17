cost.go

package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// CostEstimate holds the projected cost of a sovereign run
// before any infrastructure is provisioned.
type CostEstimate struct {
    Planets  []PlanetEstimate
    Duration time.Duration
    TotalCost float64
}

// PlanetEstimate holds the projected cost for a single planet.
type PlanetEstimate struct {
    ID           int
    Region       string
    InstanceType string
    HourlyRate   float64
    ProjectedCost float64
}

// vantageInstance is the subset of the instances.vantage.sh API
// response that sovereign needs for cost estimation.
type vantageInstance struct {
    InstanceType string  `json:"instance_type"`
    Region       string  `json:"region"`
    OnDemand     float64 `json:"on_demand"`
    Spot         float64 `json:"spot"`
}

// estimateCost calculates the projected cost of the sovereign run
// by querying instances.vantage.sh for current AWS pricing for
// each planet's region and instance type.
//
// Duration is estimated as ramp + until + a 5 minute overhead
// for provisioning and collection per planet.
//
// If pricing cannot be retrieved for a planet, the hourly rate
// is recorded as 0.0 and a warning is printed. The run is not
// aborted -- cost estimation failure is non-fatal.
func (s *Sovereign) estimateCost() (*CostEstimate, error) {
    estimate := &CostEstimate{
        Planets:  make([]PlanetEstimate, len(s.planets)),
        Duration: s.cfg.Ramp + s.cfg.Until + 5*time.Minute,
    }

    hours := estimate.Duration.Hours()

    for i, planet := range s.planets {
        rate, err := fetchHourlyRate(s.ctx, planet.Region, planet.InstanceType)
        if err != nil {
            fmt.Printf("  sovereign: warning: could not fetch pricing for %s in %s: %v\n",
                planet.InstanceType, planet.Region, err)
            rate = 0.0
        }

        projected := rate * hours
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
//
// Returns the on-demand rate. Spot pricing is not used for estimation
// because spot availability is not guaranteed at provisioning time.
//
// The API endpoint is:
//
//     https://instances.vantage.sh/aws/pricing/
//
// Requests time out after 10 seconds. A non-200 response or a missing
// instance type returns an error -- the caller treats this as non-fatal
// and records 0.0 for the affected planet.
func fetchHourlyRate(ctx context.Context, region, instanceType string) (float64, error) {
    if instanceType == "" {
        return 0.0, fmt.Errorf("instance type not specified")
    }

    url := fmt.Sprintf(
        "https://instances.vantage.sh/aws/pricing/?region=%s&os=linux&instance_type=%s",
        region, instanceType,
    )

    reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
    if err != nil {
        return 0.0, fmt.Errorf("build request: %w", err)
    }
    req.Header.Set("Accept", "application/json")
    req.Header.Set("User-Agent", "sovereign/"+version)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return 0.0, fmt.Errorf("fetch pricing: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return 0.0, fmt.Errorf("pricing API returned %d for %s in %s",
            resp.StatusCode, instanceType, region)
    }

    var instances []vantageInstance
    if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
        return 0.0, fmt.Errorf("decode pricing response: %w", err)
    }

    for _, inst := range instances {
        if inst.InstanceType == instanceType {
            return inst.OnDemand, nil
        }
    }

    return 0.0, fmt.Errorf("instance type %s not found in pricing response for region %s",
        instanceType, region)
}​​​​​​​​​​​​​​​​
