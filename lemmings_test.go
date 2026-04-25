package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/commander"
)

// -- buildLemmingsCommand ----------------------------------------------------

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

// -- sshArgs -----------------------------------------------------------------

func TestSSHArgs_ContainsRequiredFlags(t *testing.T) {
	s := testSovereign(t)
	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"

	args := s.sshArgs(planet, "echo hi")

	requireEqual(t, args[0], "ssh", "sshArgs first element is ssh")
	requireTrue(t, containsSlice(args, "-o", "StrictHostKeyChecking=no"),
		"sshArgs contains StrictHostKeyChecking=no")
	requireTrue(t, containsSlice(args, "-o", "ConnectTimeout=30"),
		"sshArgs contains ConnectTimeout=30")
	requireTrue(t, containsSlice(args, "-i", s.planetSSHKeyPath(planet)),
		"sshArgs contains identity flag")
}

func TestSSHArgs_RemoteCommandIsLast(t *testing.T) {
	s := testSovereign(t)
	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"

	args := s.sshArgs(planet, "lemmings -hit https://example.com")

	requireEqual(t, args[len(args)-1], "lemmings -hit https://example.com",
		"sshArgs remote command is last argument")
}

func TestSSHArgs_UserAtHostIsSecondToLast(t *testing.T) {
	s := testSovereign(t)
	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"
	planet.PublicIPv4 = "54.1.2.3"

	args := s.sshArgs(planet, "echo hi")

	requireEqual(t, args[len(args)-2], "rocky@54.1.2.3",
		"sshArgs user@host is second to last argument")
}

func TestSSHArgs_ExtraOptionsAppended(t *testing.T) {
	s := testSovereign(t)
	s.cfg.SSHOptions = []string{
		"UserKnownHostsFile=/tmp/known_hosts",
		"StrictHostKeyChecking=yes",
	}
	planet := testProvisionedPlanet(1)
	planet.RemoteUser = "rocky"

	args := s.sshArgs(planet, "echo hi")

	requireTrue(t, containsSlice(args, "-o", "UserKnownHostsFile=/tmp/known_hosts"),
		"sshArgs includes custom SSHOptions")
	requireTrue(t, containsSlice(args, "-o", "StrictHostKeyChecking=yes"),
		"sshArgs includes overriding StrictHostKeyChecking from SSHOptions")
}

// -- scpArgs -----------------------------------------------------------------

func TestSCPArgs_NonRecursive(t *testing.T) {
	s := testSovereign(t)
	planet := testProvisionedPlanet(1)

	args := s.scpArgs(planet, "/local/file.yml", "rocky@1.2.3.4:/remote/file.yml", false)

	requireEqual(t, args[0], "scp", "scpArgs first element is scp")
	for _, a := range args {
		requireFalse(t, a == "-r", "scpArgs non-recursive must not contain -r")
	}
}

func TestSCPArgs_Recursive(t *testing.T) {
	s := testSovereign(t)
	planet := testProvisionedPlanet(1)

	args := s.scpArgs(planet, "rocky@1.2.3.4:~/lemmings/", "/local/dir", true)

	found := false
	for _, a := range args {
		if a == "-r" {
			found = true
			break
		}
	}
	requireTrue(t, found, "scpArgs recursive must contain -r")
}

// -- runLemmings / ErrAllPlanetsLost -----------------------------------------

// TestRunLemmings_AllPlanetsLostReturnsError verifies that ErrAllPlanetsLost
// is returned when every planet fails during lemmings execution.
func TestRunLemmings_AllPlanetsLostReturnsError(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.cfg.Planets = 2
	s.cfg.Regions = []string{testRegion}
	s.planets = []*Planet{
		testProvisionedPlanet(1),
		testProvisionedPlanet(2),
	}

	// SSH fails for every call so runLemmingsOnPlanet marks every planet lost.
	cmdMock := commander.NewMockCommander()
	cmdMock.Register("ssh", commander.MockResponse{
		Err: fmt.Errorf("connection refused"),
	})
	s.cmd = cmdMock

	err := s.runLemmings()

	requireError(t, err, "runLemmings all planets lost returns error")
	requireTrue(t, errors.Is(err, ErrAllPlanetsLost),
		"runLemmings returns ErrAllPlanetsLost when all planets lost")
}

// TestRunLemmings_OnePlanetSurvives verifies that runLemmings returns nil
// when at least one planet survives execution.
func TestRunLemmings_OnePlanetSurvives(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.cfg.Planets = 2
	s.cfg.Regions = []string{testRegion}
	s.planets = []*Planet{
		testProvisionedPlanet(1),
		testProvisionedPlanet(2),
	}

	// The first SSH call succeeds; subsequent calls fail.
	// Planet 1 therefore survives and planet 2 is lost.
	s.cmd = &sequentialSSHMock{failAfter: 1}

	err := s.runLemmings()

	requireNoError(t, err, "runLemmings one survivor is not an error")
}

// sequentialSSHMock succeeds on the first failAfter ssh invocations
// and returns an error on subsequent ones. The mutex makes it safe
// for concurrent use across the goroutines launched by runLemmings.
type sequentialSSHMock struct {
	mu        sync.Mutex
	failAfter int
	calls     int
}

func (m *sequentialSSHMock) Run(_ context.Context, _ string, _ []string, name string, _ ...string) (io.Reader, io.Reader, error) {
	if name == "ssh" {
		m.mu.Lock()
		m.calls++
		fail := m.calls > m.failAfter
		m.mu.Unlock()
		if fail {
			return bytes.NewReader(nil), bytes.NewReader(nil),
				fmt.Errorf("connection refused")
		}
	}
	return bytes.NewReader(nil), bytes.NewReader(nil), nil
}

// -- completedPlanets --------------------------------------------------------

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

func TestCompletedPlanets_EmptyWhenNoneDone(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testPlanet(1),
		testLostPlanet(2),
	})
	requireEqual(t, len(s.completedPlanets()), 0,
		"completedPlanets empty when none done")
}

// -- planetReportDir ---------------------------------------------------------

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

func BenchmarkBuildLemmingsCommand(b *testing.B) {
	s := benchmarkSovereign(b)
	planet := &Planet{
		ID:           1,
		Region:       testRegion,
		InstanceType: testInstanceType,
		RemoteUser:   "rocky",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.buildLemmingsCommand(planet)
	}
}

// -- helpers -----------------------------------------------------------------

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

// containsSlice returns true if args contains flag immediately followed
// by value. Used to verify -o StrictHostKeyChecking=no style pairs in
// SSH and SCP invocations.
func containsSlice(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
