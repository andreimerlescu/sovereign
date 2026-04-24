package ovh

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
		Endpoint:          "ovh-eu",
		ApplicationKey:    "test-application-key",
		ApplicationSecret: "test-application-secret",
		ConsumerKey:       "test-consumer-key",
		AuthURL:           "https://auth.cloud.ovh.net/v3",
		TenantID:          "test-tenant-id",
		TenantName:        "test-tenant-name",
		Username:          "test-username",
		Password:          "test-password",
		RegionName:        "GRA11",
	}
}

func testProvider(t *testing.T) *TerraformProviderOVH {
	t.Helper()
	return &TerraformProviderOVH{creds: testCredentials()}
}

func testPlanetSpec(t *testing.T) tfprovider.PlanetSpec {
	t.Helper()
	return tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  t.TempDir(),
		Region:   "GRA11",
		Zone:     "GRA11",
	}
}

func testHCLSpec(t *testing.T) tfprovider.HCLSpec {
	t.Helper()
	return tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "GRA11",
		Zone:            "GRA11",
		InstanceType:    "b2-7",
		RemoteUser:      "rocky",
		AMIOS:           "Rocky Linux 9",
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
		Provider:   "ovh",
		Region:     "GRA11",
		Status:     status,
		InstanceID: "abc-def-123",
		PublicIPv4: "51.1.2.3",
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

func TestResolve_MissingAllVars(t *testing.T) {
	vars := []string{
		"OVH_ENDPOINT", "OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET",
		"OVH_CONSUMER_KEY", "OS_AUTH_URL", "OS_TENANT_ID", "OS_TENANT_NAME",
		"OS_USERNAME", "OS_PASSWORD", "OS_REGION_NAME",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
}

func TestResolve_MissingEndpoint(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OVH_ENDPOINT", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OVH_ENDPOINT")
	}
}

func TestResolve_MissingApplicationKey(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OVH_APPLICATION_KEY", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OVH_APPLICATION_KEY")
	}
}

func TestResolve_MissingApplicationSecret(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OVH_APPLICATION_SECRET", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OVH_APPLICATION_SECRET")
	}
}

func TestResolve_MissingConsumerKey(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OVH_CONSUMER_KEY", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OVH_CONSUMER_KEY")
	}
}

func TestResolve_MissingAuthURL(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OS_AUTH_URL", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OS_AUTH_URL")
	}
}

func TestResolve_MissingTenantID(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OS_TENANT_ID", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OS_TENANT_ID")
	}
}

func TestResolve_MissingTenantName(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OS_TENANT_NAME", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OS_TENANT_NAME")
	}
}

func TestResolve_MissingUsername(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OS_USERNAME", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OS_USERNAME")
	}
}

func TestResolve_MissingPassword(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OS_PASSWORD", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OS_PASSWORD")
	}
}

func TestResolve_MissingRegionName(t *testing.T) {
	setAllOVHEnv(t)
	t.Setenv("OS_REGION_NAME", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing OS_REGION_NAME")
	}
}

func TestResolve_AllPresent(t *testing.T) {
	setAllOVHEnv(t)
	creds, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Endpoint != "ovh-eu" {
		t.Errorf("Endpoint: got %q want ovh-eu", creds.Endpoint)
	}
	if creds.ApplicationKey != "test-app-key" {
		t.Errorf("ApplicationKey: got %q want test-app-key", creds.ApplicationKey)
	}
	if creds.RegionName != "GRA11" {
		t.Errorf("RegionName: got %q want GRA11", creds.RegionName)
	}
}

func TestResolve_ErrorMentionsMissingVars(t *testing.T) {
	vars := []string{
		"OVH_ENDPOINT", "OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET",
		"OVH_CONSUMER_KEY", "OS_AUTH_URL", "OS_TENANT_ID", "OS_TENANT_NAME",
		"OS_USERNAME", "OS_PASSWORD", "OS_REGION_NAME",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Error()) == 0 {
		t.Error("error message should not be empty")
	}
}

// -- New ---------------------------------------------------------------------

func TestNew_MissingCredentials(t *testing.T) {
	vars := []string{
		"OVH_ENDPOINT", "OVH_APPLICATION_KEY", "OVH_APPLICATION_SECRET",
		"OVH_CONSUMER_KEY", "OS_AUTH_URL", "OS_TENANT_ID", "OS_TENANT_NAME",
		"OS_USERNAME", "OS_PASSWORD", "OS_REGION_NAME",
	}
	for _, v := range vars {
		t.Setenv(v, "")
	}
	_, err := New()
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestNew_ValidCredentials(t *testing.T) {
	setAllOVHEnv(t)
	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

// -- Name --------------------------------------------------------------------

func TestName_ReturnsOVH(t *testing.T) {
	p := testProvider(t)
	if p.Name() != "ovh" {
		t.Errorf("Name: got %q want ovh", p.Name())
	}
}

// -- Validate ----------------------------------------------------------------

func TestValidate_EmptyApplicationKey(t *testing.T) {
	p := &TerraformProviderOVH{creds: Credentials{}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty ApplicationKey")
	}
}

func TestValidate_ValidCredentialsTerraformAbsent(t *testing.T) {
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

func TestRenderHCL_ProviderTFContainsOVHProvider(t *testing.T) {
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
	if !containsStr(string(content), "ovh/ovh") {
		t.Error("provider.tf should contain ovh/ovh provider source")
	}
	if !containsStr(string(content), "openstack") {
		t.Error("provider.tf should contain openstack provider")
	}
}

func TestRenderHCL_ProviderTFContainsRegion(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.Region = "SBG5"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "provider.tf"))
	if err != nil {
		t.Fatalf("read provider.tf: %v", err)
	}
	if !containsStr(string(content), "SBG5") {
		t.Error("provider.tf should contain region SBG5")
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

// TestRenderHCL_DataTFContainsFlavorName verifies that the instance type
// (OVH flavor name) appears in data.tf where the flavor lookup occurs.
// OVH uses a data source lookup for flavors so the instance type string
// appears in data.tf, not resources_compute.tf.
func TestRenderHCL_DataTFContainsFlavorName(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.InstanceType = "b2-15"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "data.tf"))
	if err != nil {
		t.Fatalf("read data.tf: %v", err)
	}
	if !containsStr(string(content), "b2-15") {
		t.Error("data.tf should contain flavor name b2-15 for OVH flavor lookup")
	}
}

func TestRenderHCL_DataTFContainsAMIOS(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.AMIOS = "Rocky Linux 9"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "data.tf"))
	if err != nil {
		t.Fatalf("read data.tf: %v", err)
	}
	if !containsStr(string(content), "Rocky Linux 9") {
		t.Error("data.tf should contain AMIOS image name")
	}
}

func TestRenderHCL_NetworkTFContainsPlanetID(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.PlanetID = 5
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_network.tf"))
	if err != nil {
		t.Fatalf("read resources_network.tf: %v", err)
	}
	if !containsStr(string(content), "5") {
		t.Error("resources_network.tf should contain planet ID 5")
	}
}

func TestRenderHCL_NetworkTFContainsSSHPorts(t *testing.T) {
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

func TestRenderHCL_ComputeTFContainsExtNet(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
	if err != nil {
		t.Fatalf("read resources_compute.tf: %v", err)
	}
	if !containsStr(string(content), "Ext-Net") {
		t.Error("resources_compute.tf should contain Ext-Net network")
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
	state.InstanceID = "ovh-roundtrip-001"
	state.PublicIPv4 = "51.9.9.9"

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
	if loaded.InstanceID != "ovh-roundtrip-001" {
		t.Errorf("InstanceID: got %q want ovh-roundtrip-001", loaded.InstanceID)
	}
	if loaded.PublicIPv4 != "51.9.9.9" {
		t.Errorf("PublicIPv4: got %q want 51.9.9.9", loaded.PublicIPv4)
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

	required := []string{
		"PATH=", "HOME=",
		"OVH_ENDPOINT=", "OVH_APPLICATION_KEY=",
		"OVH_APPLICATION_SECRET=", "OVH_CONSUMER_KEY=",
		"OS_AUTH_URL=", "OS_TENANT_ID=", "OS_TENANT_NAME=",
		"OS_USERNAME=", "OS_PASSWORD=", "OS_REGION_NAME=",
	}
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

func TestEnv_CredentialsMappedCorrectly(t *testing.T) {
	p := &TerraformProviderOVH{creds: Credentials{
		Endpoint:          "ovh-us",
		ApplicationKey:    "my-app-key",
		ApplicationSecret: "my-app-secret",
		ConsumerKey:       "my-consumer-key",
		AuthURL:           "https://auth.example.com",
		TenantID:          "tenant-123",
		TenantName:        "my-tenant",
		Username:          "my-user",
		Password:          "my-pass",
		RegionName:        "DE1",
	}}
	env := p.env()

	checkEnvValue := func(key, want string) {
		t.Helper()
		prefix := key + "="
		for _, e := range env {
			if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
				got := e[len(prefix):]
				if got != want {
					t.Errorf("%s: got %q want %q", key, got, want)
				}
				return
			}
		}
		t.Errorf("env missing key %q", key)
	}

	checkEnvValue("OVH_ENDPOINT", "ovh-us")
	checkEnvValue("OVH_APPLICATION_KEY", "my-app-key")
	checkEnvValue("OVH_APPLICATION_SECRET", "my-app-secret")
	checkEnvValue("OVH_CONSUMER_KEY", "my-consumer-key")
	checkEnvValue("OS_AUTH_URL", "https://auth.example.com")
	checkEnvValue("OS_TENANT_ID", "tenant-123")
	checkEnvValue("OS_TENANT_NAME", "my-tenant")
	checkEnvValue("OS_USERNAME", "my-user")
	checkEnvValue("OS_PASSWORD", "my-pass")
	checkEnvValue("OS_REGION_NAME", "DE1")
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
	spec.Region = "DE1"

	content, err := render(providerTF, spec)
	if err != nil {
		t.Fatalf("render providerTF: %v", err)
	}
	if !containsStr(content, "DE1") {
		t.Error("rendered provider.tf should contain region DE1")
	}
	if !containsStr(content, "ovh/ovh") {
		t.Error("rendered provider.tf should contain ovh/ovh provider")
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

func TestRender_DataTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	spec.AMIOS = "Rocky Linux 9"

	content, err := render(dataTF, spec)
	if err != nil {
		t.Fatalf("render dataTF: %v", err)
	}
	if !containsStr(content, "Rocky Linux 9") {
		t.Error("rendered data.tf should contain AMIOS image name")
	}
	if !containsStr(content, "rocky") {
		t.Error("rendered data.tf should contain os_distro rocky")
	}
}

func TestRender_DataTFContainsFlavorLookup(t *testing.T) {
	spec := testHCLSpec(t)
	spec.InstanceType = "b2-7"

	content, err := render(dataTF, spec)
	if err != nil {
		t.Fatalf("render dataTF: %v", err)
	}
	if !containsStr(content, "b2-7") {
		t.Error("rendered data.tf should contain flavor name for lookup")
	}
	if !containsStr(content, "openstack_compute_flavor_v2") {
		t.Error("rendered data.tf should contain openstack_compute_flavor_v2 data source")
	}
}

func TestRender_NetworkTFTemplate(t *testing.T) {
	spec := testHCLSpec(t)
	spec.PlanetID = 3
	spec.Slug = "sovereign-planet-3"

	content, err := render(resourcesNetworkTF, spec)
	if err != nil {
		t.Fatalf("render resourcesNetworkTF: %v", err)
	}
	if !containsStr(content, "sovereign-planet-3") {
		t.Error("rendered resources_network.tf should contain slug")
	}
}

// -- OVH-specific resource type ----------------------------------------------

func TestResourceType_IsOpenStack(t *testing.T) {
	if resourceType != "openstack_compute_instance_v2" {
		t.Errorf("resourceType: got %q want openstack_compute_instance_v2", resourceType)
	}
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkNew(b *testing.B) {
	b.Setenv("OVH_ENDPOINT", "ovh-eu")
	b.Setenv("OVH_APPLICATION_KEY", "test-key")
	b.Setenv("OVH_APPLICATION_SECRET", "test-secret")
	b.Setenv("OVH_CONSUMER_KEY", "test-consumer")
	b.Setenv("OS_AUTH_URL", "https://auth.cloud.ovh.net/v3")
	b.Setenv("OS_TENANT_ID", "test-tenant-id")
	b.Setenv("OS_TENANT_NAME", "test-tenant")
	b.Setenv("OS_USERNAME", "test-user")
	b.Setenv("OS_PASSWORD", "test-pass")
	b.Setenv("OS_REGION_NAME", "GRA11")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New()
	}
}

func BenchmarkRenderAll(b *testing.B) {
	spec := tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "GRA11",
		Zone:            "GRA11",
		InstanceType:    "b2-7",
		RemoteUser:      "rocky",
		AMIOS:           "Rocky Linux 9",
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
	p := &TerraformProviderOVH{creds: testCredentials()}
	spec := tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  b.TempDir(),
		Region:   "GRA11",
		Zone:     "GRA11",
	}
	state := testProviderState(tfprovider.LifecycleApplied)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SaveState(spec, state)
	}
}

func BenchmarkLoadState(b *testing.B) {
	p := &TerraformProviderOVH{creds: testCredentials()}
	spec := tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  b.TempDir(),
		Region:   "GRA11",
		Zone:     "GRA11",
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
	p := &TerraformProviderOVH{creds: testCredentials()}
	dir := b.TempDir()
	keyPath := filepath.Join(dir, "sovereign-planet.pem")
	pubPath := keyPath + ".pub"
	_ = os.WriteFile(pubPath, []byte("ssh-rsa AAAAB3NzaC1yc2E test"), 0600)

	spec := tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "GRA11",
		Zone:            "GRA11",
		InstanceType:    "b2-7",
		RemoteUser:      "rocky",
		AMIOS:           "Rocky Linux 9",
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

// setAllOVHEnv sets all required OVH and OpenStack environment variables
// for tests that need a complete credential set.
func setAllOVHEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OVH_ENDPOINT", "ovh-eu")
	t.Setenv("OVH_APPLICATION_KEY", "test-app-key")
	t.Setenv("OVH_APPLICATION_SECRET", "test-app-secret")
	t.Setenv("OVH_CONSUMER_KEY", "test-consumer-key")
	t.Setenv("OS_AUTH_URL", "https://auth.cloud.ovh.net/v3")
	t.Setenv("OS_TENANT_ID", "test-tenant-id")
	t.Setenv("OS_TENANT_NAME", "test-tenant-name")
	t.Setenv("OS_USERNAME", "test-username")
	t.Setenv("OS_PASSWORD", "test-password")
	t.Setenv("OS_REGION_NAME", "GRA11")
}

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
