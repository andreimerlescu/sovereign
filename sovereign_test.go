package main

import (
	"testing"
	"time"
)

// -- initPlanets -------------------------------------------------------------

// TestInitPlanets_CorrectCount verifies that initPlanets returns
// exactly cfg.Planets planets.
func TestInitPlanets_CorrectCount(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 5
	s.cfg.Regions = []string{"us-east-1"}
	planets := s.initPlanets()
	requireEqual(t, len(planets), 5, "initPlanets correct count")
}

// TestInitPlanets_IDsStartAtOne verifies that planet IDs begin at 1.
func TestInitPlanets_IDsStartAtOne(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 3
	s.cfg.Regions = []string{"us-east-1"}
	planets := s.initPlanets()
	requireEqual(t, planets[0].ID, 1, "initPlanets first ID is 1")
	requireEqual(t, planets[1].ID, 2, "initPlanets second ID is 2")
	requireEqual(t, planets[2].ID, 3, "initPlanets third ID is 3")
}

// TestInitPlanets_RegionsDistributedRoundRobin verifies that regions
// are assigned round-robin across planets.
func TestInitPlanets_RegionsDistributedRoundRobin(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 5
	s.cfg.Regions = []string{"us-east-1", "us-west-1", "eu-west-1"}
	planets := s.initPlanets()

	requireEqual(t, planets[0].Region, "us-east-1", "initPlanets round robin 0")
	requireEqual(t, planets[1].Region, "us-west-1", "initPlanets round robin 1")
	requireEqual(t, planets[2].Region, "eu-west-1", "initPlanets round robin 2")
	requireEqual(t, planets[3].Region, "us-east-1", "initPlanets round robin 3")
	requireEqual(t, planets[4].Region, "us-west-1", "initPlanets round robin 4")
}

// TestInitPlanets_UsesDefaultRegionsWhenNoneConfigured verifies that
// defaultAWSRegions is used when cfg.Regions is empty.
func TestInitPlanets_UsesDefaultRegionsWhenNoneConfigured(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 1
	s.cfg.Regions = []string{}
	planets := s.initPlanets()
	requireEqual(t, planets[0].Region, defaultAWSRegions[0],
		"initPlanets uses default regions")
}

// TestInitPlanets_AllPendingStatus verifies that all planets start
// in PlanetStatusPending.
func TestInitPlanets_AllPendingStatus(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 3
	s.cfg.Regions = []string{"us-east-1"}
	planets := s.initPlanets()
	for _, p := range planets {
		requireEqual(t, p.Status, PlanetStatusPending,
			"initPlanets all pending")
	}
}

// TestInitPlanets_RemoteUserPopulated verifies that each planet
// gets the RemoteUser from cfg.
func TestInitPlanets_RemoteUserPopulated(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 2
	s.cfg.Regions = []string{"us-east-1"}
	s.cfg.RemoteUser = "rocky"
	planets := s.initPlanets()
	for _, p := range planets {
		requireEqual(t, p.RemoteUser, "rocky",
			"initPlanets RemoteUser populated")
	}
}

// TestInitPlanets_InstanceTypePopulated verifies that each planet
// gets the InstanceType from cfg.
func TestInitPlanets_InstanceTypePopulated(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Planets = 2
	s.cfg.Regions = []string{"us-east-1"}
	s.cfg.InstanceType = "c6a.large"
	planets := s.initPlanets()
	for _, p := range planets {
		requireEqual(t, p.InstanceType, "c6a.large",
			"initPlanets InstanceType populated")
	}
}

// -- newProvider -------------------------------------------------------------

// TestNewProvider_AWS verifies that the aws provider string returns
// a non-nil provider without error.
func TestNewProvider_AWS(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY", "test-key")
	t.Setenv("AWS_ACCESS_SECRET_KEY", "test-secret")

	cfg := testConfig()
	cfg.Provider = "aws"
	p, err := newProvider(cfg)
	requireNoError(t, err, "newProvider aws")
	requireTrue(t, p != nil, "newProvider aws non-nil")
	requireEqual(t, p.Name(), "aws", "newProvider aws name")
}

// TestNewProvider_UnknownProvider verifies that an unknown provider
// string returns an error.
func TestNewProvider_UnknownProvider(t *testing.T) {
	cfg := testConfig()
	cfg.Provider = "unknowncloud"
	_, err := newProvider(cfg)
	requireError(t, err, "newProvider unknown provider")
}

// TestNewProvider_DigitalOcean verifies that the digitalocean provider
// returns a non-nil stub provider.
func TestNewProvider_DigitalOcean(t *testing.T) {
	cfg := testConfig()
	cfg.Provider = "digitalocean"
	p, err := newProvider(cfg)
	requireNoError(t, err, "newProvider digitalocean")
	requireTrue(t, p != nil, "newProvider digitalocean non-nil")
	requireEqual(t, p.Name(), "digitalocean", "newProvider digitalocean name")
}

// TestNewProvider_GCP verifies that the gcp provider returns a
// non-nil stub provider.
func TestNewProvider_GCP(t *testing.T) {
	cfg := testConfig()
	cfg.Provider = "gcp"
	p, err := newProvider(cfg)
	requireNoError(t, err, "newProvider gcp")
	requireTrue(t, p != nil, "newProvider gcp non-nil")
	requireEqual(t, p.Name(), "gcp", "newProvider gcp name")
}

// TestNewProvider_OCI verifies that the oci provider returns a
// non-nil stub provider.
func TestNewProvider_OCI(t *testing.T) {
	cfg := testConfig()
	cfg.Provider = "oci"
	p, err := newProvider(cfg)
	requireNoError(t, err, "newProvider oci")
	requireTrue(t, p != nil, "newProvider oci non-nil")
	requireEqual(t, p.Name(), "oci", "newProvider oci name")
}

// TestNewProvider_OVH verifies that the ovh provider returns a
// non-nil stub provider.
func TestNewProvider_OVH(t *testing.T) {
	cfg := testConfig()
	cfg.Provider = "ovh"
	p, err := newProvider(cfg)
	requireNoError(t, err, "newProvider ovh")
	requireTrue(t, p != nil, "newProvider ovh non-nil")
	requireEqual(t, p.Name(), "ovh", "newProvider ovh name")
}

// -- lostPlanets / pendingPlanets / allPlanetsTerminal -----------------------

// TestLostPlanets_ReturnsOnlyLost verifies that only lost planets
// are returned.
func TestLostPlanets_ReturnsOnlyLost(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testPlanet(1),
		testLostPlanet(2),
		testDonePlanet(t, 3),
		testLostPlanet(4),
	})
	lost := s.lostPlanets()
	requireEqual(t, len(lost), 2, "lostPlanets count")
	requireEqual(t, lost[0].ID, 2, "lostPlanets first ID")
	requireEqual(t, lost[1].ID, 4, "lostPlanets second ID")
}

// TestPendingPlanets_ReturnsNonTerminal verifies that planets not in
// done or lost status are returned.
func TestPendingPlanets_ReturnsNonTerminal(t *testing.T) {
	running := testProvisionedPlanet(2)
	s := testSovereignWithPlanets(t, []*Planet{
		testPlanet(1),
		running,
		testDonePlanet(t, 3),
		testLostPlanet(4),
	})
	pending := s.pendingPlanets()
	requireEqual(t, len(pending), 2, "pendingPlanets count")
}

// TestAllPlanetsTerminal_TrueWhenAllDoneOrLost verifies that the
// function returns true when every planet is terminal.
func TestAllPlanetsTerminal_TrueWhenAllDoneOrLost(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testDonePlanet(t, 1),
		testLostPlanet(2),
		testDonePlanet(t, 3),
	})
	requireTrue(t, s.allPlanetsTerminal(), "allPlanetsTerminal true")
}

// TestAllPlanetsTerminal_FalseWhenAnyPending verifies that the
// function returns false when any planet is still running.
func TestAllPlanetsTerminal_FalseWhenAnyPending(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testDonePlanet(t, 1),
		testProvisionedPlanet(2),
	})
	requireFalse(t, s.allPlanetsTerminal(), "allPlanetsTerminal false")
}

// TestAllPlanetsTerminal_TrueForEmptyPlanets verifies that the
// function returns true when there are no planets.
func TestAllPlanetsTerminal_TrueForEmptyPlanets(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{})
	requireTrue(t, s.allPlanetsTerminal(),
		"allPlanetsTerminal true for empty")
}

// -- printCostEstimate -------------------------------------------------------

// TestPrintCostEstimate_NoPanic verifies that printCostEstimate does
// not panic with a valid estimate.
func TestPrintCostEstimate_NoPanic(t *testing.T) {
	s := testSovereign(t)
	estimate := &CostEstimate{
		Duration:  30 * time.Second,
		TotalCost: 50_000,
		Planets: []PlanetEstimate{
			{
				ID:            1,
				Region:        testRegion,
				InstanceType:  testInstanceType,
				HourlyRate:    50_000,
				ProjectedCost: 25_000,
			},
		},
	}
	// must not panic
	s.printCostEstimate(estimate)
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkInitPlanets measures planet initialization throughput.
func BenchmarkInitPlanets(b *testing.B) {
	s := benchmarkSovereign(b)
	s.cfg.Planets = 100
	s.cfg.Regions = defaultAWSRegions
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.initPlanets()
	}
}
