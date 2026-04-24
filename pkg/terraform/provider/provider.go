package provider

import (
	"context"
	"time"
)

// TerraformProvider is the contract every cloud provider must fulfill
// in order to participate in sovereign's provisioning lifecycle.
//
// Sovereign communicates with providers exclusively through PlanetSpec
// and HCLSpec -- the provider never sees sovereign's internal types.
//
// Each provider owns its own HCL templates. Sovereign owns the values
// that populate them via HCLSpec.
type TerraformProvider interface {
	// Identity
	Name() string
	Validate() error

	// HCL -- provider owns templates, sovereign provides specs
	RenderHCL(spec HCLSpec) error

	// Lifecycle
	Init(ctx context.Context, spec PlanetSpec) error
	InitOrApply(ctx context.Context, spec PlanetSpec) error
	Apply(ctx context.Context, spec PlanetSpec) error
	Destroy(ctx context.Context, spec PlanetSpec) error
	Output(ctx context.Context, spec PlanetSpec) (*ProvisionOutput, error)
	Status(ctx context.Context, spec PlanetSpec) (LifecycleStatus, error)

	// State
	StateBackend(spec PlanetSpec) (string, error)
	LoadState(spec PlanetSpec) (*ProviderState, error)
	SaveState(spec PlanetSpec, state *ProviderState) error
	UnloadState(spec PlanetSpec) error
}

// PlanetSpec is the handoff type from sovereign to a provider.
// It contains only what a provider needs to locate and operate
// on a planet's terraform working directory.
// Sovereign constructs this from *Planet. The provider never sees *Planet.
type PlanetSpec struct {
	PlanetID int
	WorkDir  string
	Region   string
	Zone     string
}

// HCLSpec is the specification sovereign passes to RenderHCL.
// The provider owns the HCL templates. Sovereign owns the values.
// Every field a provider could need across any HCL file is defined
// here. Providers ignore fields they do not use.
type HCLSpec struct {
	// Identity
	PlanetID int
	Slug     string

	// Compute
	Region       string
	Zone         string
	InstanceType string
	RemoteUser   string

	// AMI / Image selection
	AMIOS     string
	AMIOwners string

	// Network
	SubnetID        string
	SecurityGroupID string

	// Storage
	RootVolumeSize int
	RootVolumeType string
	TempVolumeSize int
	TempVolumeType string

	// SSH
	SSHKeyPath string

	// Lemmings runtime
	GoVersion       string
	LemmingsPackage string
	Hit             string
	Terrain         int64
	Pack            int64
	Until           string
	Ramp            string
	PlanRemotePath  string
	SaveTo          string

	// Terraform
	WorkDir       string
	StateFilePath string
}

// ProvisionOutput is what every provider returns after a successful
// apply. It is the only data that flows back from the provider to
// sovereign after provisioning completes.
type ProvisionOutput struct {
	InstanceID  string
	PublicIPv4  string
	PrivateIPv4 string
	Region      string
	Zone        string
	ProviderRef string // provider-specific resource identifier
}

// LifecycleStatus represents the current terraform lifecycle state
// of a single planet. Persisted in ProviderState so sovereign knows
// exactly where to resume after any interruption.
type LifecycleStatus string

const (
	LifecycleUnknown       LifecycleStatus = "unknown"
	LifecycleUninitialized LifecycleStatus = "uninitialized"
	LifecycleInitialized   LifecycleStatus = "initialized"
	LifecycleApplying      LifecycleStatus = "applying"
	LifecycleApplied       LifecycleStatus = "applied"
	LifecycleDestroying    LifecycleStatus = "destroying"
	LifecycleDestroyed     LifecycleStatus = "destroyed"
	LifecycleFailed        LifecycleStatus = "failed"
	LifecycleInterrupted   LifecycleStatus = "interrupted"
)

// ProviderState is the persisted state for a single planet's terraform
// provider. Written to disk after every lifecycle transition so sovereign
// can resume from any interruption without re-provisioning instances
// that are already running.
//
// LoadState reads it from disk.
// SaveState writes it.
// UnloadState removes it from memory without deleting the file.
// The file persists until the planet is fully destroyed and cleaned up.
type ProviderState struct {
	PlanetID       int             `yaml:"PlanetID"`
	Provider       string          `yaml:"Provider"`
	Region         string          `yaml:"Region"`
	Zone           string          `yaml:"Zone"`
	Status         LifecycleStatus `yaml:"Status"`
	InstanceID     string          `yaml:"InstanceID,omitempty"`
	PublicIPv4     string          `yaml:"PublicIPv4,omitempty"`
	PrivateIPv4    string          `yaml:"PrivateIPv4,omitempty"`
	TerraformDir   string          `yaml:"TerraformDir"`
	StateFile      string          `yaml:"StateFile"`
	LastTransition time.Time       `yaml:"LastTransition"`
	InitializedAt  time.Time       `yaml:"InitializedAt,omitempty"`
	AppliedAt      time.Time       `yaml:"AppliedAt,omitempty"`
	DestroyedAt    time.Time       `yaml:"DestroyedAt,omitempty"`
	InterruptedAt  time.Time       `yaml:"InterruptedAt,omitempty"`
	HCLBackups     []HCLBackup     `yaml:"HCLBackups,omitempty"`
}

// HCLBackup records a successful HCL render with a timestamp.
// After every successful apply the rendered HCL files are copied
// to a timestamped backup directory so the exact terraform that
// provisioned each instance is never lost.
type HCLBackup struct {
	Timestamp time.Time `yaml:"Timestamp"`
	Directory string    `yaml:"Directory"`
	Files     []string  `yaml:"Files"`
}
