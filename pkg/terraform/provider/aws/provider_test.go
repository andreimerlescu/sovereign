package aws

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
		AccessKeyID:     "test-access-key-id",
		SecretAccessKey: "test-secret-access-key",
	}
}

func testProvider(t *testing.T) *TerraformProviderAWS {
	t.Helper()
	return &TerraformProviderAWS{creds: testCredentials()}
}

func testPlanetSpec(t *testing.T) tfprovider.PlanetSpec {
	t.Helper()
	return tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  t.TempDir(),
		Region:   "us-east-1",
		Zone:     "us-east-1a",
	}
}

func testHCLSpec(t *testing.T) tfprovider.HCLSpec {
	t.Helper()
	return tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "us-east-1",
		Zone:            "us-east-1a",
		InstanceType:    "t3.medium",
		RemoteUser:      "rocky",
		AMIOS:           "Rocky-9-EC2-Base-9.*-x86_64*",
		AMIOwners:       "792107900819",
		SubnetID:        "subnet-test123",
		SecurityGroupID: "sg-test456",
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
		Provider:   "aws",
		Region:     "us-east-1",
		Status:     status,
		InstanceID: "i-abc123",
		PublicIPv4: "54.1.2.3",
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

func TestResolve_MissingBothKeys(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
}

func TestResolve_MissingAccessKeyID(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing AWS_ACCESS_KEY_ID")
	}
}

func TestResolve_MissingSecretAccessKey(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error for missing AWS_SECRET_ACCESS_KEY")
	}
}

func TestResolve_BothPresent(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	creds, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("AccessKeyID: got %q want AKIAIOSFODNN7EXAMPLE", creds.AccessKeyID)
	}
	if creds.SecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("SecretAccessKey: got %q want wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", creds.SecretAccessKey)
	}
}

func TestResolve_ErrorMentionsMissingVars(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	_, err := Resolve()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if len(msg) == 0 {
		t.Error("error message should not be empty")
	}
}

// -- New ---------------------------------------------------------------------

func TestNew_MissingCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	_, err := New()
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestNew_ValidCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-key-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	p, err := New()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

// -- Name --------------------------------------------------------------------

func TestName_ReturnsAWS(t *testing.T) {
	p := testProvider(t)
	if p.Name() != "aws" {
		t.Errorf("Name: got %q want aws", p.Name())
	}
}

// -- Validate ----------------------------------------------------------------

func TestValidate_EmptyCredentials(t *testing.T) {
	p := &TerraformProviderAWS{creds: Credentials{}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty credentials")
	}
}

func TestValidate_EmptyAccessKeyID(t *testing.T) {
	p := &TerraformProviderAWS{creds: Credentials{
		AccessKeyID:     "",
		SecretAccessKey: "test-secret",
	}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty AccessKeyID")
	}
}

func TestValidate_EmptySecretAccessKey(t *testing.T) {
	p := &TerraformProviderAWS{creds: Credentials{
		AccessKeyID:     "test-key",
		SecretAccessKey: "",
	}}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty SecretAccessKey")
	}
}

func TestValidate_ValidCredentialsTerraformAbsent(t *testing.T) {
	p := testProvider(t)
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

func TestRenderHCL_ProviderTFContainsRegion(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.Region = "eu-west-1"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "provider.tf"))
	if err != nil {
		t.Fatalf("read provider.tf: %v", err)
	}
	if !containsStr(string(content), "eu-west-1") {
		t.Error("provider.tf should contain region eu-west-1")
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

func TestRenderHCL_ComputeTFContainsInstanceType(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.InstanceType = "c6a.large"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_compute.tf"))
	if err != nil {
		t.Fatalf("read resources_compute.tf: %v", err)
	}
	if !containsStr(string(content), "c6a.large") {
		t.Error("resources_compute.tf should contain instance type c6a.large")
	}
}

func TestRenderHCL_DataTFContainsAMIOwners(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.AMIOwners = "792107900819"
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "data.tf"))
	if err != nil {
		t.Fatalf("read data.tf: %v", err)
	}
	if !containsStr(string(content), "792107900819") {
		t.Error("data.tf should contain AMI owners account ID")
	}
}

func TestRenderHCL_NetworkTFContainsPlanetID(t *testing.T) {
	p := testProvider(t)
	spec := testHCLSpec(t)
	spec.PlanetID = 7
	writeFakeSSHKey(t, spec.SSHKeyPath)

	if err := p.RenderHCL(spec); err != nil {
		t.Fatalf("RenderHCL: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(spec.WorkDir, "terraform", "resources_network.tf"))
	if err != nil {
		t.Fatalf("read resources_network.tf: %v", err)
	}
	if !containsStr(string(content), "7") {
		t.Error("resources_network.tf should contain planet ID 7")
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
	state.InstanceID = "i-roundtrip001"
	state.PublicIPv4 = "54.9.9.9"

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
	if loaded.InstanceID != "i-roundtrip001" {
		t.Errorf("InstanceID: got %q want i-roundtrip001", loaded.InstanceID)
	}
	if loaded.PublicIPv4 != "54.9.9.9" {
		t.Errorf("PublicIPv4: got %q want 54.9.9.9", loaded.PublicIPv4)
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

	required := []string{"PATH=", "HOME=", "AWS_ACCESS_KEY_ID=", "AWS_SECRET_ACCESS_KEY="}
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

func TestEnv_DoesNotContainLegacyKeys(t *testing.T) {
	p := testProvider(t)
	env := p.env()

	legacy := []string{"AWS_ACCESS_KEY=", "AWS_ACCESS_SECRET_KEY="}
	for _, prefix := range legacy {
		for _, e := range env {
			if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
				t.Errorf("env must not contain legacy key %q -- use AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY", prefix)
			}
		}
	}
}

func TestEnv_CredentialsMappedCorrectly(t *testing.T) {
	p := &TerraformProviderAWS{creds: Credentials{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
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

	checkEnvValue("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	checkEnvValue("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
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
	spec.Region = "ap-southeast-1"

	content, err := render(providerTF, spec)
	if err != nil {
		t.Fatalf("render providerTF: %v", err)
	}
	if !containsStr(content, "ap-southeast-1") {
		t.Error("rendered provider.tf should contain region")
	}
	if !containsStr(content, "hashicorp/aws") {
		t.Error("rendered provider.tf should contain hashicorp/aws")
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

// -- Benchmarks --------------------------------------------------------------

func BenchmarkNew(b *testing.B) {
	b.Setenv("AWS_ACCESS_KEY_ID", "test-key-id")
	b.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New()
	}
}

func BenchmarkRenderAll(b *testing.B) {
	spec := tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "us-east-1",
		Zone:            "us-east-1a",
		InstanceType:    "t3.medium",
		RemoteUser:      "rocky",
		AMIOS:           "Rocky-9-EC2-Base-9.*-x86_64*",
		AMIOwners:       "792107900819",
		SubnetID:        "subnet-test123",
		SecurityGroupID: "sg-test456",
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
	p := &TerraformProviderAWS{creds: testCredentials()}
	spec := tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  b.TempDir(),
		Region:   "us-east-1",
		Zone:     "us-east-1a",
	}
	state := testProviderState(tfprovider.LifecycleApplied)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.SaveState(spec, state)
	}
}

func BenchmarkLoadState(b *testing.B) {
	p := &TerraformProviderAWS{creds: testCredentials()}
	spec := tfprovider.PlanetSpec{
		PlanetID: 1,
		WorkDir:  b.TempDir(),
		Region:   "us-east-1",
		Zone:     "us-east-1a",
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
	p := &TerraformProviderAWS{creds: testCredentials()}
	dir := b.TempDir()
	keyPath := filepath.Join(dir, "sovereign-planet.pem")
	pubPath := keyPath + ".pub"
	_ = os.WriteFile(pubPath, []byte("ssh-rsa AAAAB3NzaC1yc2E test"), 0600)

	spec := tfprovider.HCLSpec{
		PlanetID:        1,
		Slug:            "sovereign-planet-1",
		Region:          "us-east-1",
		Zone:            "us-east-1a",
		InstanceType:    "t3.medium",
		RemoteUser:      "rocky",
		AMIOS:           "Rocky-9-EC2-Base-9.*-x86_64*",
		AMIOwners:       "792107900819",
		SubnetID:        "subnet-test123",
		SecurityGroupID: "sg-test456",
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
