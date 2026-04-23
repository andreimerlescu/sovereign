package main

import (
	"testing"
	"time"
)

// -- buildLemmingsCommand ----------------------------------------------------

// TestBuildLemmingsCommand_NoPlan verifies the command string when
// no plan file is configured.
func TestBuildLemmingsCommand_NoPlan(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Hit = "https://example.com"
	s.cfg.Terrain = 10
	s.cfg.Pack = 5
	s.cfg.Until = 30 * time.Second
	s.cfg.Ramp = 5 * time.Second
	s.cfg.Plan = ""

	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"

	cmd := s.buildLemmingsCommand(planet)

	requireTrue(t, len(cmd) > 0, "buildLemmingsCommand non-empty")
	requireTrue(t, containsStr(cmd, "lemmings"), "buildLemmingsCommand has lemmings")
	requireTrue(t, containsStr(cmd, "-hit https://example.com"),
		"buildLemmingsCommand has hit")
	requireTrue(t, containsStr(cmd, "-terrain 10"),
		"buildLemmingsCommand has terrain")
	requireTrue(t, containsStr(cmd, "-pack 5"),
		"buildLemmingsCommand has pack")
	requireTrue(t, containsStr(cmd, "-tty=false"),
		"buildLemmingsCommand has tty=false")
	requireTrue(t, containsStr(cmd, "-save-to ~/lemmings"),
		"buildLemmingsCommand has save-to")
	requireFalse(t, containsStr(cmd, "-plan"),
		"buildLemmingsCommand no plan when not configured")
}

// TestBuildLemmingsCommand_WithPlan verifies the command string when
// a plan file is configured.
func TestBuildLemmingsCommand_WithPlan(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Plan = "/local/path/lemmings.plan.yml"
	s.cfg.RemoteUser = "rocky"

	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"

	cmd := s.buildLemmingsCommand(planet)

	requireTrue(t, containsStr(cmd, "-plan /home/rocky/lemmings.plan.yml"),
		"buildLemmingsCommand has plan path with remote user")
}

// TestBuildLemmingsCommand_PlanPathUsesRemoteUser verifies that the
// plan remote path uses the planet's RemoteUser, not a hardcoded user.
func TestBuildLemmingsCommand_PlanPathUsesRemoteUser(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Plan = "/local/path/lemmings.plan.yml"

	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "ubuntu"

	cmd := s.buildLemmingsCommand(planet)

	requireTrue(t, containsStr(cmd, "-plan /home/ubuntu/lemmings.plan.yml"),
		"buildLemmingsCommand plan path uses remote user")
	requireFalse(t, containsStr(cmd, "/home/rocky/"),
		"buildLemmingsCommand plan path does not hardcode rocky")
}

// TestBuildLemmingsCommand_UntilAndRamp verifies that until and ramp
// durations are rendered as strings in the command.
func TestBuildLemmingsCommand_UntilAndRamp(t *testing.T) {
	s := testSovereign(t)
	s.cfg.Until = 45 * time.Second
	s.cfg.Ramp = 2 * time.Minute
	s.cfg.Plan = ""

	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"

	cmd := s.buildLemmingsCommand(planet)

	requireTrue(t, containsStr(cmd, s.cfg.Until.String()),
		"buildLemmingsCommand until duration in command")
	requireTrue(t, containsStr(cmd, s.cfg.Ramp.String()),
		"buildLemmingsCommand ramp duration in command")
}

// -- completedPlanets --------------------------------------------------------

// TestCompletedPlanets_OnlyDoneWithReportPath verifies that only
// planets in PlanetStatusDone with a non-empty ReportPath are returned.
func TestCompletedPlanets_OnlyDoneWithReportPath(t *testing.T) {
	done1 := testDonePlanet(t, 1)
	done2 := testDonePlanet(t, 2)
	lost := testLostPlanet(3)
	pending := testPlanet(4)
	doneNoPath := testDonePlanet(t, 5)
	doneNoPath.ReportPath = ""

	s := testSovereignWithPlanets(t, []*Planet{done1, done2, lost, pending, doneNoPath})
	completed := s.completedPlanets()

	requireEqual(t, len(completed), 2, "completedPlanets count")
	requireEqual(t, completed[0].ID, 1, "completedPlanets first planet")
	requireEqual(t, completed[1].ID, 2, "completedPlanets second planet")
}

// TestCompletedPlanets_EmptyWhenNoneDone verifies that an empty
// slice is returned when no planets are done.
func TestCompletedPlanets_EmptyWhenNoneDone(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testPlanet(1),
		testLostPlanet(2),
	})
	requireEqual(t, len(s.completedPlanets()), 0,
		"completedPlanets empty when none done")
}

// -- planetReportDir ---------------------------------------------------------

// TestPlanetReportDir_ContainsPlanetID verifies that the report
// directory path contains the planet ID.
func TestPlanetReportDir_ContainsPlanetID(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(7)
	dir := s.planetReportDir(planet)
	requireTrue(t, containsStr(dir, "planet-7"),
		"planetReportDir contains planet ID")
	requireTrue(t, containsStr(dir, "reports"),
		"planetReportDir contains reports")
}

// -- planetSSHKeyPath --------------------------------------------------------

// TestPlanetSSHKeyPath_ContainsPem verifies that the SSH key path
// ends with a .pem file.
func TestPlanetSSHKeyPath_ContainsPem(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(1)
	path := s.planetSSHKeyPath(planet)
	requireTrue(t, containsStr(path, ".pem"),
		"planetSSHKeyPath contains .pem")
	requireTrue(t, containsStr(path, "sovereign-planet"),
		"planetSSHKeyPath contains sovereign-planet")
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkBuildLemmingsCommand measures command string construction.
func BenchmarkBuildLemmingsCommand(b *testing.B) {
	s := benchmarkSovereign(b)
	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.buildLemmingsCommand(planet)
	}
}

// -- helpers -----------------------------------------------------------------

// containsStr returns true if substr is found within s.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
