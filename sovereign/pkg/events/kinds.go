package events

// EventKind identifies a sovereign lifecycle event. Typed as a string
// so event logs are human-readable without a lookup table.
//
// Sovereign EventKind constants are grouped by lifecycle phase.
// Callers may define additional kinds as plain string constants --
// the EventBus places no restriction on EventKind values.
type EventKind string

const (
	// -- Provision lifecycle -------------------------------------------------

	// KindProvisionStart is emitted when a planet begins provisioning.
	KindProvisionStart EventKind = "provision.start"

	// KindProvisionComplete is emitted when a planet finishes provisioning
	// successfully and has a public IP and instance ID.
	KindProvisionComplete EventKind = "provision.complete"

	// KindProvisionFailed is emitted when a planet fails to provision.
	// The Event.Error field carries the failure reason.
	KindProvisionFailed EventKind = "provision.failed"

	// -- Terraform init lifecycle --------------------------------------------

	// KindInitStart is emitted when terraform init begins for a planet.
	KindInitStart EventKind = "init.start"

	// KindInitComplete is emitted when terraform init succeeds.
	KindInitComplete EventKind = "init.complete"

	// KindInitFailed is emitted when terraform init fails.
	KindInitFailed EventKind = "init.failed"

	// -- Terraform apply lifecycle -------------------------------------------

	// KindApplyStart is emitted when terraform apply begins for a planet.
	KindApplyStart EventKind = "apply.start"

	// KindApplyComplete is emitted when terraform apply succeeds.
	KindApplyComplete EventKind = "apply.complete"

	// KindApplyFailed is emitted when terraform apply fails.
	KindApplyFailed EventKind = "apply.failed"

	// -- Terraform destroy lifecycle -----------------------------------------

	// KindDestroyStart is emitted when terraform destroy begins for a planet.
	KindDestroyStart EventKind = "destroy.start"

	// KindDestroyComplete is emitted when terraform destroy succeeds.
	KindDestroyComplete EventKind = "destroy.complete"

	// KindDestroyFailed is emitted when terraform destroy fails.
	// The Event.InstanceID field carries the instance ID that may require
	// manual intervention in the cloud provider console.
	KindDestroyFailed EventKind = "destroy.failed"

	// -- Lemmings execution lifecycle ----------------------------------------

	// KindLemmingsStart is emitted when lemmings begins executing on a planet.
	KindLemmingsStart EventKind = "lemmings.start"

	// KindLemmingsComplete is emitted when lemmings finishes successfully.
	KindLemmingsComplete EventKind = "lemmings.complete"

	// KindLemmingsFailed is emitted when lemmings fails or times out.
	KindLemmingsFailed EventKind = "lemmings.failed"

	// -- Collection lifecycle ------------------------------------------------

	// KindCollectStart is emitted when result collection begins for a planet.
	KindCollectStart EventKind = "collect.start"

	// KindCollectComplete is emitted when results have been collected
	// successfully and the planet's ReportPath is populated.
	KindCollectComplete EventKind = "collect.complete"

	// KindCollectFailed is emitted when result collection fails.
	KindCollectFailed EventKind = "collect.failed"

	// -- Run lifecycle -------------------------------------------------------

	// KindRunResumed is emitted when sovereign resumes from an existing
	// state file rather than starting a fresh run.
	KindRunResumed EventKind = "run.resumed"

	// KindRunInterrupted is emitted when sovereign detects that a planet
	// was interrupted mid-lifecycle and is being recovered.
	KindRunInterrupted EventKind = "run.interrupted"

	// -- Validation ----------------------------------------------------------

	// KindValidationFailed is emitted when a planet or provider fails
	// validation before provisioning begins.
	KindValidationFailed EventKind = "validation.failed"

	// KindDataCorruption is emitted when sovereign detects inconsistent
	// state that may indicate data corruption.
	KindDataCorruption EventKind = "data.corruption"
)
