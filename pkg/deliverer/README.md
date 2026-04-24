# deliverer

> Report delivery destinations for sovereign.
> A ReportDeliverer interface with pluggable backends for local
> filesystem, Amazon S3, and remote hosts via SCP.

---

## What This Package Is

This package implements the --save-to destination logic for sovereign.
After aggregating a unified report, sovereign delivers the rendered
markdown and HTML to every configured save-to destination. Each
destination string is parsed into a ReportDeliverer that handles
the mechanics of writing or uploading the files.

This package has one dependency outside the standard library:
pkg/commander for subprocess execution in S3 and SSH deliverers.

---

## Files

    deliverer.go       ReportDeliverer interface, New factory,
                       LocalDeliverer, S3Deliverer, SSHDeliverer
    deliverer_test.go  full test and benchmark coverage

---

## ReportDeliverer Interface

    type ReportDeliverer interface {
        Deliver(ctx context.Context, filename, md, html string) error
        String() string
    }

Deliver is called once per destination with the base filename
(without extension) and the fully rendered markdown and HTML strings.
Each deliverer appends the appropriate extension and writes or
uploads the files.

String returns a human-readable destination description for log output.

---

## Destination URL Formats

    s3://bucket/prefix        S3Deliverer -- uploads via aws s3 cp
    ssh://user@host:/path     SSHDeliverer -- uploads via scp
    file:///path              LocalDeliverer -- writes to filesystem
    /path or ./path or .      LocalDeliverer -- writes to filesystem

---

## New

Parses a destination string and returns the appropriate deliverer:

    d, err := deliverer.New(dest, identity, cmd)

The identity argument is the SSH private key path used by SSHDeliverer.
It is ignored by LocalDeliverer and S3Deliverer. If empty the SSH
deliverer omits the -i flag and relies on the SSH agent.

The cmd argument is the Commander used by S3Deliverer and SSHDeliverer
for subprocess execution. It may be nil for local destinations.

---

## LocalDeliverer

Writes filename.md and filename.html into the configured directory.
Creates the directory if it does not exist. No external dependencies.

    // all equivalent
    deliverer.New(".", "", nil)
    deliverer.New("/tmp/reports", "", nil)
    deliverer.New("file:///tmp/reports", "", nil)

---

## S3Deliverer

Uploads via aws s3 cp. Sovereign never manages AWS credentials -- the
aws CLI reads them from the subprocess environment, ~/.aws/credentials,
or instance metadata as it normally would.

Two separate aws s3 cp calls are made, one for .md and one for .html,
with --content-type set appropriately for each.

    deliverer.New("s3://my-bucket/reports/2026", "", cmd)

Uploads to:

    s3://my-bucket/reports/2026/sovereign.2026.04.24.example-com.md
    s3://my-bucket/reports/2026/sovereign.2026.04.24.example-com.html

---

## SSHDeliverer

Uploads via scp. If identity is non-empty it is passed as -i.
StrictHostKeyChecking is disabled for consistency with how sovereign
connects to planets.

    deliverer.New("ssh://rocky@1.2.3.4:/reports", "/path/to/key.pem", cmd)

Uploads to:

    rocky@1.2.3.4:/reports/sovereign.2026.04.24.example-com.md
    rocky@1.2.3.4:/reports/sovereign.2026.04.24.example-com.html

---

## Adding a New Deliverer

Implement ReportDeliverer and add a case to New:

    type MyDeliverer struct { ... }

    func (d *MyDeliverer) Deliver(ctx context.Context, filename, md, html string) error { ... }
    func (d *MyDeliverer) String() string { ... }

Then in New:

    case strings.HasPrefix(dest, "myscheme://"):
        return newMyDeliverer(dest, cmd)

---

## Import Path

    import "github.com/andreimerlescu/sovereign/pkg/deliverer"

---

## Testing

    go test -race ./pkg/deliverer/...
    go test -bench=. -benchmem ./pkg/deliverer/...