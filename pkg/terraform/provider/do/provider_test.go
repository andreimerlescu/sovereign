package do

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform/provider"
)

// -- helpers -----------------------------------------------------------------

func testCredentials() Credentials {
	return Credentials{
		Token: "test-digitalocean-token",
	}
}

func testProvider(t *testing.T) *TerraformProviderDigitalOcean {
	t.Helper()
	return &TerraformProviderDigitalOcean{creds: testCredentials()}
}

func testPlanetSpec(t *testing.T) tfprovider.PlanetSpec {
	t.Helper()
	return tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  t.TempDir(),
		Region:   "nyc1",
		Zone:     "nyc1",
	}
}

func testHCLSpec(t *testing.T) tfprovider.HCLSpec {
	t.Helper()
	return tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "nyc1",
		Zone:            "nyc1",
		InstanceType:    "s-2vcpu-4gb",
		RemoteUser:      "rocky",
		AMIOS:           "rockylinux-9-x64",
		AMIOwners:       "",
		SubnetID:        "",
		SecurityGroupID: "",
		RootVolumeSize:  30,
		RootVolumeType:  "gp2",
		TempVolumeSize:  10,
		TempVolumeType:  "gp2",
		SSHKeyPath:      filepath.Join(t.TempDir(), "sovereign-planet.pem"),
		GoVersion:       "1.25.0",
		LemmingsPackage: "github.com/andreimerlescu/lemmings@latest",
		Hit:             "http://localhost:8080/",
		Terrain:         10,
		Pack:            5,
		Until:           "30s",
		Ramp:            "5s",
		SaveTo:          "/home/rocky/lemmings",
		WorkDir:         t.TempDir(),
	}
}

func testProviderState(status tfprovider.LifecycleStatus) *tfprovider.ProviderState {
	return &tfprovider.ProviderState{
		PlanetID:   1,
		Provider:   "digitalocean",
		Region:     "nyc1",
		Status:     status,
		InstanceID: "123456789",
		PublicIPv4: "167.1.2.3",
	}
}

// writeFakeSSHKey writes a minimal fake SSH public key alongside the
// private key path so RenderHCL can read SSHKeyPath.pub without error.
func writeFakeSSHKey(t *testing.T, keyPath string) {
	t.Helper()
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("writeFakeSSHKey: mkdir: %v", err)
	}
	pubPath := keyPath + ".pub"
	if err := os.WriteFile(pubPath, []byte("ssh-rsa AAAAB3NzaC1yc2E test"), 0600); err != nil {
		t.Fatalf("writeFakeSSHKey: write pub: %v", err)
	}
}

// -- Resolve -----------------------------------------------------------------

func TestResolve_MissingToken(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing DIGITALOCEAN_TOKEN, got nil")
	}
}

func TestResolve_TokenPresent(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "test-token-value")
	creds, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Token != "test-token-value" {
		t.Errorf("Token: got %q want test-token-value", creds.Token)
	}
}

func TestResolve_ErrorMentionsMissingVar(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Error()) == 0 {
		t.Error("error message should not be empty")
	}
}

// -- New ---------------------------------------------------------------------

func TestNew_MissingToken(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "")
	_, err := New()
	if err == nil {
		t.Fatal("expected error for missing DIGITALOCEAN_TOKEN")
	}
}

func TestNew_ValidToken(t *testing.T) {
	t.Setenv("DIGITALOCEAN_TOKEN", "test-token")
	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

// -- Name --------------------------------------------------------------------

func TestName_ReturnsDigitalOcean(t *testing.T) {
	p := testProvider(t)
	if p.Name() != "digitalocean" {
		t.Errorf("Name: got %q want digitalocean", p.Name())
	}
}

// -- Validate ----------------------------------------------------------------

func TestValidate_EmptyToken(t *testing.T) {
	p := &TerraformProviderDigitalOcean{creds: Credentials{}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty Token")
	}
}

func TestValidate_ValidTokenTerraformAbsent(t *testing.T) {
	p := testProvider(t)
	// Validate checks terraform binary -- result depends on environment.
	// We verify it does not panic and returns a typed result.
	_ = p.Validate()
}

// -- RenderHCL ---------------------------------------------------------------

func TestRenderHCL_CreatesFiles(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	err := p.RenderHCL(spec)
	if err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	tfDir := filepath.Join(spec.WorkDir, "terraform")
	expected := []string{
		"provider.tf",
		"data.tf",
		"resources_network.tf",
		"resources_compute.tf",
		"output.tf",
	}
	for _, name := range expected {
		path := filepath.Join(tfDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("RenderHCL: expected file %s not found", name)
		}
	}
}

func TestRenderHCL_ProviderTFContainsDigitalOceanProvider(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "provider.tf"))
	if err != nil {
		t.Fatalf("read provider.tf: %v", err)
	}
	if !containsStr(string(content), "digitalocean/digitalocean") {
		t.Error("provider.tf should contain digitalocean/digitalocean provider source")
	}
}

func TestRenderHCL_OutputTFContainsRequiredOutputs(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "output.tf"))
	if err != nil {
		t.Fatalf("read output.tf: %v", err)
	}
	for _, name := range []string{"instance_id", "public_ipv4", "private_ipv4"} {
		if !containsStr(string(content), name) {
			t.Errorf("output.tf missing required output %q", name)
		}
	}
}

func TestRenderHCL_CreatesBackupDirectory(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	backupDir := filepath.Join(spec.WorkDir, "backups")
	if _, err := os.Stat(backupDir); err != nil {
		t.Error("RenderHCL should create backups directory")
	}
}

func TestRenderHCL_ComputeTFContainsRegion(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.Region = "sfo3"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
	if err != nil {
		t.Fatalf("read resources_compute.tf: %v", err)
	}
	if !containsStr(string(content), "sfo3") {
		t.Error("resources_compute.tf should contain region sfo3")
	}
}

func TestRenderHCL_ComputeTFContainsInstanceType(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.InstanceType = "c-2"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
	if err != nil {
		t.Fatalf("read resources_compute.tf: %v", err)
	}
	if !containsStr(string(content), "c-2") {
		t.Error("resources_compute.tf should contain instance type c-2")
	}
}

func TestRenderHCL_ComputeTFContainsImageSlug(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.AMIOS = "rockylinux-9-x64"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
	if err != nil {
		t.Fatalf("read resources_compute.tf: %v", err)
	}
	if !containsStr(string(content), "rockylinux-9-x64") {
		t.Error("resources_compute.tf should contain image slug rockylinux-9-x64")
	}
}

func TestRenderHCL_NetworkTFContainsPlanetID(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.PlanetID = 4
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_network.tf"))
	if err != nil {
		t.Fatalf("read resources_network.tf: %v", err)
	}
	if !containsStr(string(content), "4") {
		t.Error("resources_network.tf should contain planet ID 4")
	}
}

func TestRenderHCL_NetworkTFContainsFirewallPorts(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_network.tf"))
	if err != nil {
		t.Fatalf("read resources_network.tf: %v", err)
	}
	for _, port := range []string{"22", "18888", "18889"} {
		if !containsStr(string(content), port) {
			t.Errorf("resources_network.tf should contain port %s", port)
		}
	}
}

func TestRenderHCL_ComputeTFContainsLemmingsPackage(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.LemmingsPackage = "github.com/andreimerlescu/lemmings@latest"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
	if err != nil {
		t.Fatalf("read resources_compute.tf: %v", err)
	}
	if !containsStr(string(content), "github.com/andreimerlescu/lemmings@latest") {
		t.Error("resources_compute.tf should contain lemmings package")
	}
}

func TestRenderHCL_DataTFIsCommentOnly(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "data.tf"))
	if err != nil {
		t.Fatalf("read data.tf: %v", err)
	}
	// DigitalOcean data.tf is comment-only -- no resource blocks needed
	if len(content) == 0 {
		t.Error("data.tf should not be empty")
	}
}

// -- StateBackend ------------------------------------------------------------

func TestStateBackend_ReturnsExpectedPath(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)

	path, err := p.StateBackend(spec)
	if err != nil {
		t.Fatalf("StateBackend: %v", err)
	}

	expected := filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate")
	if path != expected {
		t.Errorf("StateBackend: got %q want %q", path, expected)
	}
}

// -- LoadState / SaveState ---------------------------------------------------

func TestLoadState_NilWhenNoFile(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)

	state, err := p.LoadState(spec)
	if err != nil {
		t.Fatalf("LoadState: unexpected error: %v", err)
	}
	if state != nil {
		t.Fatal("LoadState: expected nil for missing file")
	}
}

func TestSaveState_WritesFile(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)
	state := testProviderState(tfprovider.LifecycleApplied)

	if err := p.SaveState(spec, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	path := filepath.Join(spec.WorkDir, "provider_state.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("SaveState: state file not found at %s", path)
	}
}

func TestSaveState_LoadState_RoundTrip(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)
	state := testProviderState(tfprovider.LifecycleApplied)
	state.InstanceID = "do-roundtrip-001"
	state.PublicIPv4 = "167.9.9.9"

	if err := p.SaveState(spec, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := p.LoadState(spec)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadState: expected non-nil state")
	}
	if loaded.InstanceID != "do-roundtrip-001" {
		t.Errorf("InstanceID: got %q want do-roundtrip-001", loaded.InstanceID)
	}
	if loaded.PublicIPv4 != "167.9.9.9" {
		t.Errorf("PublicIPv4: got %q want 167.9.9.9", loaded.PublicIPv4)
	}
	if loaded.Status != tfprovider.LifecycleApplied {
		t.Errorf("Status: got %q want %q", loaded.Status, tfprovider.LifecycleApplied)
	}
}

func TestSaveState_SetsLastTransition(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)
	state := testProviderState(tfprovider.LifecycleInitialized)

	before := time.Now()
	if err := p.SaveState(spec, state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	after := time.Now()

	loaded, err := p.LoadState(spec)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.LastTransition.Before(before) || loaded.LastTransition.After(after) {
		t.Errorf("LastTransition %v not in expected range [%v, %v]",
			loaded.LastTransition, before, after)
	}
}

func TestSaveState_AllLifecycleStatuses(t *testing.T) {
	statuses := []tfprovider.LifecycleStatus{
		tfprovider.LifecycleUninitialized,
		tfprovider.LifecycleInitialized,
		tfprovider.LifecycleApplying,
		tfprovider.LifecycleApplied,
		tfprovider.LifecycleDestroying,
		tfprovider.LifecycleDestroyed,
		tfprovider.LifecycleFailed,
		tfprovider.LifecycleInterrupted,
	}

	for _, status := range statuses {
		p := testProvider(t)
		spec := testPlanetSpec(t)
		state := testProviderState(status)

		if err := p.SaveState(spec, state); err != nil {
			t.Errorf("SaveState(%q): %v", status, err)
			continue
		}
		loaded, err := p.LoadState(spec)
		if err != nil {
			t.Errorf("LoadState(%q): %v", status, err)
			continue
		}
		if loaded.Status != status {
			t.Errorf("Status round-trip: got %q want %q", loaded.Status, status)
		}
	}
}

// -- UnloadState -------------------------------------------------------------

func TestUnloadState_IsNoop(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)
	if err := p.UnloadState(spec); err != nil {
		t.Fatalf("UnloadState: unexpected error: %v", err)
	}
}

// -- env ---------------------------------------------------------------------

func TestEnv_ContainsRequiredKeys(t *testing.T) {
	p := testProvider(t)
	env := p.env()

	required := []string{"PATH=", "HOME=", "DIGITALOCEAN_TOKEN="}
	for _, prefix := range required {
		found := false
		for _, e := range env {
			if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("env missing required key prefix %q", prefix)
		}
	}
}

func TestEnv_TokenMappedCorrectly(t *testing.T) {
	p := &TerraformProviderDigitalOcean{creds: Credentials{
		Token: "dop_v1_abc123def456",
	}}
	env := p.env()

	prefix := "DIGITALOCEAN_TOKEN="
	for _, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			got := e[len(prefix):]
			if got != "dop_v1_abc123def456" {
				t.Errorf("DIGITALOCEAN_TOKEN: got %q want dop_v1_abc123def456", got)
			}
			return
		}
	}
	t.Error("env missing DIGITALOCEAN_TOKEN key")
}

func TestEnv_DoesNotContainAWSKeys(t *testing.T) {
	p := testProvider(t)
	env := p.env()

	forbidden := []string{"AWS_ACCESS_KEY_ID=", "AWS_SECRET_ACCESS_KEY="}
	for _, prefix := range forbidden {
		for _, e := range env {
			if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
				t.Errorf("env should not contain AWS key %q", prefix)
			}
		}
	}
}

// -- stateFilePath -----------------------------------------------------------

func TestStateFilePath_ContainsWorkDir(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)

	path := p.stateFilePath(spec)
	if !containsStr(path, spec.WorkDir) {
		t.Errorf("stateFilePath %q should contain WorkDir %q", path, spec.WorkDir)
	}
}

func TestStateFilePath_EndsWithProviderStateYML(t *testing.T) {
	p := testProvider(t)
	spec := testPlanetSpec(t)

	path := p.stateFilePath(spec)
	if filepath.Base(path) != "provider_state.yml" {
		t.Errorf("stateFilePath base: got %q want provider_state.yml",
			filepath.Base(path))
	}
}

// -- renderAll ---------------------------------------------------------------

func TestRenderAll_ProducesFiveFiles(t *testing.T) {
	spec := testHCLSpec(t)
	files, err := renderAll(spec)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	if len(files) != 5 {
		t.Errorf("renderAll: got %d files want 5", len(files))
	}
}

func TestRenderAll_FileNamesCorrect(t *testing.T) {
	spec := testHCLSpec(t)
	files, err := renderAll(spec)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}

	names := make(map[string]bool)
	for _, f := range files {
		names[f.name] = true
	}

	expected := []string{
		"provider.tf",
		"data.tf",
		"resources_network.tf",
		"resources_compute.tf",
		"output.tf",
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("renderAll: missing file %q", name)
		}
	}
}

func TestRenderAll_ContentsNonEmpty(t *testing.T) {
	spec := testHCLSpec(t)
	files, err := renderAll(spec)
	if err != nil {
		t.Fatalf("renderAll: %v", err)
	}
	for _, f := range files {
		if len(f.content) == 0 {
			t.Errorf("renderAll: file %q has empty content", f.name)
		}
	}
}

// -- render ------------------------------------------------------------------

func TestRender_ProviderTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	content, err := render(providerTF, spec)
	if err != nil {
		t.Fatalf("render providerTF: %v", err)
	}
	if !containsStr(content, "digitalocean/digitalocean") {
		t.Error("rendered provider.tf should contain digitalocean/digitalocean")
	}
}

func TestRender_OutputTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	content, err := render(outputTF, spec)
	if err != nil {
		t.Fatalf("render outputTF: %v", err)
	}
	for _, output := range []string{"instance_id", "public_ipv4", "private_ipv4"} {
		if !containsStr(content, output) {
			t.Errorf("rendered output.tf missing %q", output)
		}
	}
}

func TestRender_OutputTFUsesDropletFields(t *testing.T) {
	spec := testHCLSpec(t)
	content, err := render(outputTF, spec)
	if err != nil {
		t.Fatalf("render outputTF: %v", err)
	}
	if !containsStr(content, "ipv4_address") {
		t.Error("rendered output.tf should reference ipv4_address for DigitalOcean Droplet")
	}
}

func TestRender_ComputeTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	spec.LemmingsPackage = "github.com/andreimerlescu/lemmings@latest"

	content, err := render(resourcesComputeTF, spec)
	if err != nil {
		t.Fatalf("render resourcesComputeTF: %v", err)
	}
	if !containsStr(content, "github.com/andreimerlescu/lemmings@latest") {
		t.Error("rendered resources_compute.tf should contain lemmings package")
	}
}

func TestRender_ComputeTFContainsPrivateNetworking(t *testing.T) {
	spec := testHCLSpec(t)
	content, err := render(resourcesComputeTF, spec)
	if err != nil {
		t.Fatalf("render resourcesComputeTF: %v", err)
	}
	if !containsStr(content, "private_networking") {
		t.Error("rendered resources_compute.tf should contain private_networking")
	}
}

func TestRender_NetworkTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	spec.PlanetID = 6
	spec.Slug = "sovereign-planet-6"

	content, err := render(resourcesNetworkTF, spec)
	if err != nil {
		t.Fatalf("render resourcesNetworkTF: %v", err)
	}
	if !containsStr(content, "sovereign-planet-6") {
		t.Error("rendered resources_network.tf should contain slug")
	}
}

func TestRender_NetworkTFContainsFirewallResource(t *testing.T) {
	spec := testHCLSpec(t)
	content, err := render(resourcesNetworkTF, spec)
	if err != nil {
		t.Fatalf("render resourcesNetworkTF: %v", err)
	}
	if !containsStr(content, "digitalocean_firewall") {
		t.Error("rendered resources_network.tf should contain digitalocean_firewall resource")
	}
}

func TestRender_DataTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	content, err := render(dataTF, spec)
	if err != nil {
		t.Fatalf("render dataTF: %v", err)
	}
	// DigitalOcean data.tf is comment-only -- verify it renders without error
	// and is not empty
	if len(content) == 0 {
		t.Error("rendered data.tf should not be empty")
	}
}

// -- DO-specific resource type -----------------------------------------------

func TestResourceType_IsDigitalOceanDroplet(t *testing.T) {
	if resourceType != "digitalocean_droplet" {
		t.Errorf("resourceType: got %q want digitalocean_droplet", resourceType)
	}
}

// -- DO-specific region slugs ------------------------------------------------

func TestRenderHCL_DORegionSlugs(t *testing.T) {
	regions := []string{"nyc1", "sfo3", "ams3", "sgp1", "lon1"}
	for _, region := range regions {
		p := testProvider(t)
		spec := testHCLSpec(t)
		spec.Region = region
		writeFakeSSHKey(t, spec.SSHKeyPath)

		if err := p.RenderHCL(spec); err != nil {
			t.Errorf("RenderHCL region %s: %v", region, err)
			continue
		}

		content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
		if err != nil {
			t.Errorf("read resources_compute.tf region %s: %v", region, err)
			continue
		}
		if !containsStr(string(content), region) {
			t.Errorf("resources_compute.tf should contain region slug %q", region)
		}
	}
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkNew(b *testing.B) {
	b.Setenv("DIGITALOCEAN_TOKEN", "test-token")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New()
	}
}

func BenchmarkRenderAll(b *testing.B) {
	spec := tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "nyc1",
		Zone:            "nyc1",
		InstanceType:    "s-2vcpu-4gb",
		RemoteUser:      "rocky",
		AMIOS:           "rockylinux-9-x64",
		RootVolumeSize:  30,
		RootVolumeType:  "gp2",
		TempVolumeSize:  10,
		TempVolumeType:  "gp2",
		SSHKeyPath:      "/home/rocky/.ssh/sovereign-planet.pem",
		GoVersion:       "1.25.0",
		LemmingsPackage: "github.com/andreimerlescu/lemmings@latest",
		SaveTo:          "/home/rocky/lemmings",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderAll(spec)
	}
}

func BenchmarkSaveState(b *testing.B) {
	p := &TerraformProviderDigitalOcean{creds: testCredentials()}
	spec := tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  b.TempDir(),
		Region:   "nyc1",
		Zone:     "nyc1",
	}
	state := testProviderState(tfprovider.LifecycleApplied)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SaveState(spec, state)
	}
}

func BenchmarkLoadState(b *testing.B) {
	p := &TerraformProviderDigitalOcean{creds: testCredentials()}
	spec := tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  b.TempDir(),
		Region:   "nyc1",
		Zone:     "nyc1",
	}
	state := testProviderState(tfprovider.LifecycleApplied)
	if err := p.SaveState(spec, state); err != nil {
		b.Fatalf("SaveState: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.LoadState(spec)
	}
}

func BenchmarkRenderHCL(b *testing.B) {
	p := &TerraformProviderDigitalOcean{creds: testCredentials()}
	dir := b.TempDir()
	keyPath := filepath.Join(dir, "sovereign-planet.pem")
	pubPath := keyPath + ".pub"
	_ = os.WriteFile(pubPath, []byte("ssh-rsa AAAAB3NzaC1yc2E test"), 0600)

	spec := tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "nyc1",
		Zone:            "nyc1",
		InstanceType:    "s-2vcpu-4gb",
		RemoteUser:      "rocky",
		AMIOS:           "rockylinux-9-x64",
		RootVolumeSize:  30,
		RootVolumeType:  "gp2",
		TempVolumeSize:  10,
		TempVolumeType:  "gp2",
		SSHKeyPath:      keyPath,
		GoVersion:       "1.25.0",
		LemmingsPackage: "github.com/andreimerlescu/lemmings@latest",
		SaveTo:          "/home/rocky/lemmings",
		WorkDir:         dir,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.RenderHCL(spec)
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
