package deliverer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andreimerlescu/sovereign/pkg/commander"
)

// ReportDeliverer delivers a rendered sovereign report to a destination.
// Deliver is called once per save-to destination with the base filename
// and the fully rendered markdown and HTML content.
type ReportDeliverer interface {
	// Deliver writes or uploads the markdown and HTML report files.
	// filename is the base name without extension, e.g.
	// "sovereign.2026.04.24.example-com". The deliverer appends
	// ".md" and ".html" as appropriate for the destination.
	Deliver(ctx context.Context, filename, md, html string) error

	// String returns a human-readable description of the destination
	// for use in log output.
	String() string
}

// New parses a save-to destination string and returns the appropriate
// ReportDeliverer. The destination may be:
//
//	s3://bucket/prefix        -- upload via aws s3 cp
//	ssh://user@host:/path     -- upload via scp, optionally with identity
//	file:///path              -- write to local filesystem
//	/path or ./path or .      -- write to local filesystem
//
// The cmd argument is used for S3 and SSH deliverers. It may be nil
// for local deliverers.
//
// The identity argument is the path to an SSH private key used by the
// SSH deliverer. If empty the SSH deliverer falls back to the default
// SSH agent or key discovery.
func New(dest, identity string, cmd commander.Commander) (ReportDeliverer, error) {
	switch {
	case strings.HasPrefix(dest, "s3://"):
		return newS3Deliverer(dest, cmd)
	case strings.HasPrefix(dest, "ssh://"):
		return newSSHDeliverer(dest, identity, cmd)
	case strings.HasPrefix(dest, "file://"):
		path := strings.TrimPrefix(dest, "file://")
		return newLocalDeliverer(path)
	default:
		return newLocalDeliverer(dest)
	}
}

// -- LocalDeliverer ----------------------------------------------------------

// LocalDeliverer writes report files to the local filesystem.
type LocalDeliverer struct {
	dir string
}

func newLocalDeliverer(dir string) (*LocalDeliverer, error) {
	if dir == "" {
		return nil, fmt.Errorf("deliverer: local: destination path is empty")
	}
	return &LocalDeliverer{dir: dir}, nil
}

// Deliver writes filename.md and filename.html into the configured
// directory, creating it if it does not exist.
func (d *LocalDeliverer) Deliver(_ context.Context, filename, md, html string) error {
	if err := os.MkdirAll(d.dir, 0755); err != nil {
		return fmt.Errorf("deliverer: local: mkdir %s: %w", d.dir, err)
	}

	mdPath := filepath.Join(d.dir, filename+".md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("deliverer: local: write md: %w", err)
	}

	htmlPath := filepath.Join(d.dir, filename+".html")
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("deliverer: local: write html: %w", err)
	}

	return nil
}

// String returns the local destination path.
func (d *LocalDeliverer) String() string {
	return d.dir
}

// -- S3Deliverer -------------------------------------------------------------

// S3Deliverer uploads report files to Amazon S3 via the aws CLI.
// Credentials are never managed by sovereign -- the aws CLI reads them
// from the subprocess environment, ~/.aws/credentials, or instance
// metadata as it normally would.
type S3Deliverer struct {
	bucket string
	prefix string
	cmd    commander.Commander
}

func newS3Deliverer(dest string, cmd commander.Commander) (*S3Deliverer, error) {
	if cmd == nil {
		return nil, fmt.Errorf("deliverer: s3: commander is required")
	}
	// dest is s3://bucket/prefix/path
	// strip the scheme to get bucket/prefix
	without := strings.TrimPrefix(dest, "s3://")
	if without == "" {
		return nil, fmt.Errorf("deliverer: s3: destination %q is missing bucket", dest)
	}

	parts := strings.SplitN(without, "/", 2)
	bucket := parts[0]
	prefix := ""
	if len(parts) == 2 {
		prefix = strings.TrimSuffix(parts[1], "/")
	}

	if bucket == "" {
		return nil, fmt.Errorf("deliverer: s3: destination %q is missing bucket", dest)
	}

	return &S3Deliverer{
		bucket: bucket,
		prefix: prefix,
		cmd:    cmd,
	}, nil
}

// s3Key constructs the full S3 key for a given filename and extension.
func (d *S3Deliverer) s3Key(filename, ext string) string {
	if d.prefix == "" {
		return filename + ext
	}
	return d.prefix + "/" + filename + ext
}

// s3URL constructs the full s3:// URL for a given filename and extension.
func (d *S3Deliverer) s3URL(filename, ext string) string {
	return "s3://" + d.bucket + "/" + d.s3Key(filename, ext)
}

// Deliver uploads filename.md and filename.html to S3 via aws s3 cp.
// The files are written to a temp directory first then uploaded.
func (d *S3Deliverer) Deliver(ctx context.Context, filename, md, html string) error {
	tmp, err := os.MkdirTemp("", "sovereign-s3-*")
	if err != nil {
		return fmt.Errorf("deliverer: s3: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	mdPath := filepath.Join(tmp, filename+".md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("deliverer: s3: write temp md: %w", err)
	}

	htmlPath := filepath.Join(tmp, filename+".html")
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("deliverer: s3: write temp html: %w", err)
	}

	// upload markdown
	_, _, err = d.cmd.Run(ctx, "", nil,
		"aws", "s3", "cp",
		mdPath,
		d.s3URL(filename, ".md"),
		"--content-type", "text/markdown",
	)
	if err != nil {
		return fmt.Errorf("deliverer: s3: upload md to %s: %w",
			d.s3URL(filename, ".md"), err)
	}

	// upload html
	_, _, err = d.cmd.Run(ctx, "", nil,
		"aws", "s3", "cp",
		htmlPath,
		d.s3URL(filename, ".html"),
		"--content-type", "text/html",
	)
	if err != nil {
		return fmt.Errorf("deliverer: s3: upload html to %s: %w",
			d.s3URL(filename, ".html"), err)
	}

	return nil
}

// String returns the S3 destination as a human-readable string.
func (d *S3Deliverer) String() string {
	if d.prefix == "" {
		return "s3://" + d.bucket + "/"
	}
	return "s3://" + d.bucket + "/" + d.prefix + "/"
}

// -- SSHDeliverer ------------------------------------------------------------

// SSHDeliverer uploads report files to a remote host via scp.
// The destination is parsed from an ssh://user@host:/path URL.
// If identity is non-empty it is passed as the -i flag to scp.
type SSHDeliverer struct {
	user     string
	host     string
	dir      string
	identity string
	cmd      commander.Commander
}

func newSSHDeliverer(dest, identity string, cmd commander.Commander) (*SSHDeliverer, error) {
	if cmd == nil {
		return nil, fmt.Errorf("deliverer: ssh: commander is required")
	}

	// dest is ssh://user@host:/path/to/dir
	without := strings.TrimPrefix(dest, "ssh://")
	if without == "" {
		return nil, fmt.Errorf("deliverer: ssh: destination %q is missing host", dest)
	}

	// split user@host from :/path
	// the colon separates host from remote path, matching scp convention
	colonIdx := strings.Index(without, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing colon separator between host and path", dest)
	}

	userHost := without[:colonIdx]
	remotePath := without[colonIdx+1:]

	if remotePath == "" {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing remote path", dest)
	}

	// split user@host
	atIdx := strings.Index(userHost, "@")
	if atIdx < 0 {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing user -- use ssh://user@host:/path", dest)
	}

	user := userHost[:atIdx]
	host := userHost[atIdx+1:]

	if user == "" {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing user", dest)
	}
	if host == "" {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing host", dest)
	}

	return &SSHDeliverer{
		user:     user,
		host:     host,
		dir:      strings.TrimSuffix(remotePath, "/"),
		identity: identity,
		cmd:      cmd,
	}, nil
}

// scpArgs builds the scp argument list for a single file upload.
// If identity is set it is passed as -i.
func (d *SSHDeliverer) scpArgs(localPath, remoteDir, filename, ext string) []string {
	args := []string{"scp", "-o", "StrictHostKeyChecking=no"}
	if d.identity != "" {
		args = append(args, "-i", d.identity)
	}
	args = append(args,
		localPath,
		fmt.Sprintf("%s@%s:%s/%s%s", d.user, d.host, remoteDir, filename, ext),
	)
	return args
}

// Deliver uploads filename.md and filename.html to the remote host
// via scp. Files are written to a temp directory first then uploaded.
func (d *SSHDeliverer) Deliver(ctx context.Context, filename, md, html string) error {
	tmp, err := os.MkdirTemp("", "sovereign-ssh-*")
	if err != nil {
		return fmt.Errorf("deliverer: ssh: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	mdPath := filepath.Join(tmp, filename+".md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("deliverer: ssh: write temp md: %w", err)
	}

	htmlPath := filepath.Join(tmp, filename+".html")
	if err := os.WriteFile(htmlPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("deliverer: ssh: write temp html: %w", err)
	}

	// upload markdown
	mdArgs := d.scpArgs(mdPath, d.dir, filename, ".md")
	_, _, err = d.cmd.Run(ctx, "", nil, mdArgs[0], mdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("deliverer: ssh: upload md to %s@%s:%s: %w",
			d.user, d.host, d.dir, err)
	}

	// upload html
	htmlArgs := d.scpArgs(htmlPath, d.dir, filename, ".html")
	_, _, err = d.cmd.Run(ctx, "", nil, htmlArgs[0], htmlArgs[1:]...)
	if err != nil {
		return fmt.Errorf("deliverer: ssh: upload html to %s@%s:%s: %w",
			d.user, d.host, d.dir, err)
	}

	return nil
}

// String returns the SSH destination as a human-readable string.
func (d *SSHDeliverer) String() string {
	return fmt.Sprintf("ssh://%s@%s:%s/", d.user, d.host, d.dir)
}
