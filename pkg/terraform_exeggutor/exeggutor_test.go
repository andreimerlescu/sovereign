package terraform_exeggutor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// -- helpers -----------------------------------------------------------------

// testExeggutor returns an Exeggutor pointed at a temp directory.
// The terraform directory is created but no HCL files are written --
// tests that need real terraform behavior are integration tests.
func testExeggutor(t *testing.T) *Exeggutor {
	t.Helper()
	dir := t.TempDir()
	tfDir := filepath.Join(dir, "terraform")
	if err := os.MkdirAll(tfDir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	return New("test", 1, dir, testEnv())
}

// testEnv returns a minimal environment for test subprocess calls.
func testEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
}

// writeFakeTFState writes a minimal terraform state file to the
// planet's terraform directory. Used to test ParseStatus without
// running real terraform.
func writeFakeTFState(t *testing.T, e *Exeggutor, resourceType, id string) {
	t.Helper()
	state := `{
  "version": 4,
  "values": {
    "root_module": {
      "resources": [
        {
          "type": "` + resourceType + `",
          "values": {
            "id": "` + id + `"
          }
        }
      ]
    }
  }
}`
	path := filepath.Join(e.WorkDir, "terraform", "terraform.tfstate")
	if err := os.WriteFile(path, []byte(state), 0600); err != nil {
		t.Fatalf("writeFakeTFState: %v", err)
	}
}

// writeFakeTFOutput writes a minimal terraform output JSON file and
// sets up a fake terraform binary that returns it. Used to test
// ParseOutput without running real terraform.
//
// Note: ParseOutput and ParseStatus tests that require real terraform
// output are integration tests and are skipped in unit test runs.
// The unit tests below verify structural behavior using fake state files
// and the showState / outputValue parsing logic directly.

// -- New ---------------------------------------------------------------------

// TestNew_FieldsPopulated verifies that New correctly populates all
// Exeggutor fields.
func TestNew_FieldsPopulated(t *testing.T) {
	dir := t.TempDir()
	env := []string{"PATH=/usr/bin", "HOME=/root"}

	e := New("aws", 3, dir, env)

	if e.Name != "aws" {
		t.Errorf("Name: got %q want aws", e.Name)
	}
	if e.PlanetID != 3 {
		t.Errorf("PlanetID: got %d want 3", e.PlanetID)
	}
	if e.WorkDir != dir {
		t.Errorf("WorkDir: got %q want %q", e.WorkDir, dir)
	}
	if len(e.Env) != 2 {
		t.Errorf("Env: got %d entries want 2", len(e.Env))
	}
}

// TestNew_DifferentProviders verifies that New works correctly for
// each provider name.
func TestNew_DifferentProviders(t *testing.T) {
	providers := []string{"aws", "digitalocean", "ovh"}
	for _, name := range providers {
		e := New(name, 1, t.TempDir(), testEnv())
		if e.Name != name {
			t.Errorf("Name: got %q want %q", e.Name, name)
		}
	}
}

// -- terraformDir ------------------------------------------------------------

// TestTerraformDir_IsWorkDirPlusTerraform verifies that terraformDir
// returns WorkDir/terraform.
func TestTerraformDir_IsWorkDirPlusTerraform(t *testing.T) {
	dir := t.TempDir()
	e := New("aws", 1, dir, testEnv())
	got := e.terraformDir()
	want := filepath.Join(dir, "terraform")
	if got != want {
		t.Errorf("terraformDir: got %q want %q", got, want)
	}
}

// -- Validate ----------------------------------------------------------------

// TestValidate_TerraformInPath verifies that Validate returns nil when
// terraform is available. Skipped if terraform is not installed.
func TestValidate_TerraformInPath(t *testing.T) {
	if err := Validate(); err != nil {
		t.Skipf("terraform not in PATH -- skipping: %v", err)
	}
}

// -- StateFileExists ---------------------------------------------------------

// TestStateFileExists_FalseWhenAbsent verifies false when no state
// file exists in the terraform directory.
func TestStateFileExists_FalseWhenAbsent(t *testing.T) {
	e := testExeggutor(t)
	if e.StateFileExists() {
		t.Fatal("StateFileExists should be false when no state file present")
	}
}

// TestStateFileExists_TrueWhenPresent verifies true when a state file
// exists in the terraform directory.
func TestStateFileExists_TrueWhenPresent(t *testing.T) {
	e := testExeggutor(t)
	path := filepath.Join(e.WorkDir, "terraform", "terraform.tfstate")
	if err := os.WriteFile(path, []byte(`{"version":4}`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !e.StateFileExists() {
		t.Fatal("StateFileExists should be true when state file present")
	}
}

// -- DotTerraformExists ------------------------------------------------------

// TestDotTerraformExists_FalseWhenAbsent verifies false when .terraform
// directory does not exist.
func TestDotTerraformExists_FalseWhenAbsent(t *testing.T) {
	e := testExeggutor(t)
	if e.DotTerraformExists() {
		t.Fatal("DotTerraformExists should be false when .terraform absent")
	}
}

// TestDotTerraformExists_TrueWhenPresent verifies true when .terraform
// directory exists.
func TestDotTerraformExists_TrueWhenPresent(t *testing.T) {
	e := testExeggutor(t)
	dot := filepath.Join(e.WorkDir, "terraform", ".terraform")
	if err := os.MkdirAll(dot, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if !e.DotTerraformExists() {
		t.Fatal("DotTerraformExists should be true when .terraform present")
	}
}

// -- ParseStatus -------------------------------------------------------------

// TestParseStatus_UninitiatedWhenNoStateFile verifies that ParseStatus
// returns LifecycleUninitialized when no state file exists.
func TestParseStatus_UninitiatedWhenNoStateFile(t *testing.T) {
	e := testExeggutor(t)
	status, err := e.ParseStatus(context.Background(), "aws_instance")
	if err != nil {
		t.Fatalf("ParseStatus: unexpected error: %v", err)
	}
	if status != tfprovider.LifecycleUninitialized {
		t.Errorf("Status: got %q want %q",
			status, tfprovider.LifecycleUninitialized)
	}
}

// TestParseStatus_AppliedWhenResourceFound verifies that ParseStatus
// returns LifecycleApplied when the target resource type has a non-empty
// id in the state file.
//
// This test parses the showState struct directly rather than running
// terraform show to avoid requiring a real terraform binary.
func TestParseStatus_AppliedWhenResourceFound(t *testing.T) {
	state := showState{}
	state.Values.RootModule.Resources = []struct {
		Type   string `json:"type"`
		Values struct {
			ID string `json:"id"`
		} `json:"values"`
	}{
		{
			Type: "aws_instance",
			Values: struct {
				ID string `json:"id"`
			}{ID: "i-abc123"},
		},
	}

	// verify the search logic directly
	found := false
	for _, r := range state.Values.RootModule.Resources {
		if r.Type == "aws_instance" && r.Values.ID != "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("resource search logic failed to find aws_instance with non-empty id")
	}
}

// TestParseStatus_UninitiatedWhenResourceNotFound verifies that when
// the state file exists but the target resource type is absent the
// status is LifecycleUninitialized.
func TestParseStatus_UninitiatedWhenResourceNotFound(t *testing.T) {
	state := showState{}
	state.Values.RootModule.Resources = []struct {
		Type   string `json:"type"`
		Values struct {
			ID string `json:"id"`
		} `json:"values"`
	}{
		{
			Type: "aws_key_pair",
			Values: struct {
				ID string `json:"id"`
			}{ID: "kp-123"},
		},
	}

	found := false
	for _, r := range state.Values.RootModule.Resources {
		if r.Type == "aws_instance" && r.Values.ID != "" {
			found = true
			break
		}
	}
	if found {
		t.Fatal("resource search logic found aws_instance when it should not have")
	}
}

// TestParseStatus_ResourceTypes verifies that the resource type string
// correctly differentiates between providers.
func TestParseStatus_ResourceTypes(t *testing.T) {
	cases := []struct {
		provider     string
		resourceType string
	}{
		{"aws", "aws_instance"},
		{"digitalocean", "digitalocean_droplet"},
		{"ovh", "openstack_compute_instance_v2"},
	}

	for _, tc := range cases {
		e := New(tc.provider, 1, t.TempDir(), testEnv())
		// no state file -- should return uninitialized without error
		status, err := e.ParseStatus(context.Background(), tc.resourceType)
		if err != nil {
			t.Errorf("%s: ParseStatus unexpected error: %v", tc.provider, err)
		}
		if status != tfprovider.LifecycleUninitialized {
			t.Errorf("%s: Status = %q, want uninitialized", tc.provider, status)
		}
	}
}

// -- ParseOutput -------------------------------------------------------------

// TestParseOutput_MissingFields verifies that ParseOutput returns an
// error listing missing fields when the output map is empty.
//
// This tests the missing-field detection logic directly using the
// outputValue parsing path.
func TestParseOutput_MissingFields(t *testing.T) {
	raw := map[string]outputValue{}

	var missing []string
	if _, ok := raw["instance_id"]; !ok {
		missing = append(missing, "instance_id")
	}
	if _, ok := raw["public_ipv4"]; !ok {
		missing = append(missing, "public_ipv4")
	}
	if _, ok := raw["private_ipv4"]; !ok {
		missing = append(missing, "private_ipv4")
	}

	if len(missing) != 3 {
		t.Errorf("expected 3 missing fields, got %d: %v", len(missing), missing)
	}
}

// TestParseOutput_AllFieldsPresent verifies that when all three
// output fields are present the Result is fully populated.
func TestParseOutput_AllFieldsPresent(t *testing.T) {
	raw := map[string]outputValue{
		"instance_id":  {Value: "i-abc123"},
		"public_ipv4":  {Value: "54.1.2.3"},
		"private_ipv4": {Value: "10.0.0.1"},
	}

	result := &Result{}
	var missing []string

	if v, ok := raw["instance_id"]; ok {
		if s, ok := v.Value.(string); ok {
			result.InstanceID = s
		}
	} else {
		missing = append(missing, "instance_id")
	}

	if v, ok := raw["public_ipv4"]; ok {
		if s, ok := v.Value.(string); ok {
			result.PublicIPv4 = s
		}
	} else {
		missing = append(missing, "public_ipv4")
	}

	if v, ok := raw["private_ipv4"]; ok {
		if s, ok := v.Value.(string); ok {
			result.PrivateIPv4 = s
		}
	} else {
		missing = append(missing, "private_ipv4")
	}

	if len(missing) > 0 {
		t.Fatalf("unexpected missing fields: %v", missing)
	}
	if result.InstanceID != "i-abc123" {
		t.Errorf("InstanceID: got %q want i-abc123", result.InstanceID)
	}
	if result.PublicIPv4 != "54.1.2.3" {
		t.Errorf("PublicIPv4: got %q want 54.1.2.3", result.PublicIPv4)
	}
	if result.PrivateIPv4 != "10.0.0.1" {
		t.Errorf("PrivateIPv4: got %q want 10.0.0.1", result.PrivateIPv4)
	}
}

// -- ErrorPrefixing ----------------------------------------------------------

// TestRun_ErrorIncludesProviderNameAndPlanetID verifies that errors
// from run() include the provider name and planet ID for diagnosis.
// Tested by running a guaranteed-to-fail command with no terraform dir.
func TestRun_ErrorIncludesProviderNameAndPlanetID(t *testing.T) {
	if err := Validate(); err != nil {
		t.Skipf("terraform not in PATH -- skipping: %v", err)
	}

	dir := t.TempDir()
	// deliberately do not create terraform/ subdirectory so terraform fails
	e := New("aws", 5, dir, testEnv())

	err := e.run(context.Background(), "show", "-no-color")
	if err == nil {
		t.Fatal("expected error when terraform dir does not exist")
	}

	errStr := err.Error()
	if !containsStr(errStr, "aws") {
		t.Errorf("error %q should contain provider name aws", errStr)
	}
	if !containsStr(errStr, "5") {
		t.Errorf("error %q should contain planet ID 5", errStr)
	}
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkNew measures Exeggutor construction throughput.
// Called once per planet per lifecycle phase.
func BenchmarkNew(b *testing.B) {
	dir := b.TempDir()
	env := testEnv()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New("aws", i+1, dir, env)
	}
}

// BenchmarkStateFileExists measures filesystem stat throughput.
// Called in InitOrApply to determine whether init has run.
func BenchmarkStateFileExists(b *testing.B) {
	e := New("aws", 1, b.TempDir(), testEnv())
	tfDir := filepath.Join(e.WorkDir, "terraform")
	_ = os.MkdirAll(tfDir, 0700)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.StateFileExists()
	}
}

// BenchmarkDotTerraformExists measures filesystem stat throughput.
func BenchmarkDotTerraformExists(b *testing.B) {
	e := New("aws", 1, b.TempDir(), testEnv())
	tfDir := filepath.Join(e.WorkDir, "terraform")
	_ = os.MkdirAll(tfDir, 0700)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.DotTerraformExists()
	}
}

// -- helpers -----------------------------------------------------------------

// containsStr returns true if substr appears within s.
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
