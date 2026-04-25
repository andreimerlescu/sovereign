package deliverer

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/commander"
)

// ReportDeliverer delivers a rendered sovereign report to a destination.
type ReportDeliverer interface {
	Deliver(ctx context.Context, filename, md, html string) error
	String() string
}

// New parses a save-to destination string and returns the appropriate
// ReportDeliverer. Supported destination formats:
//
//	s3://bucket/prefix            upload via aws s3 cp
//	ssh://user@host:/path         upload via scp
//	ftp://host/path               upload via FTP
//	file:///path                  write to local filesystem
//	/path or ./path or .          write to local filesystem
//
// The cmd argument is used by S3 and SSH deliverers for subprocess
// execution. It may be nil for local and FTP deliverers.
//
// The identity argument is the SSH private key path for SSHDeliverer.
// The ftpUsername and ftpPassword arguments are credentials for FTPDeliverer.
func New(dest, identity string, cmd commander.Commander, opts ...DelivererOption) (ReportDeliverer, error) {
	o := delivererOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	switch {
	case strings.HasPrefix(dest, "s3://"):
		return newS3Deliverer(dest, cmd)
	case strings.HasPrefix(dest, "ssh://"):
		return newSSHDeliverer(dest, identity, cmd)
	case strings.HasPrefix(dest, "ftp://"):
		return newFTPDeliverer(dest, o.ftpUsername, o.ftpPassword)
	case strings.HasPrefix(dest, "file://"):
		path := strings.TrimPrefix(dest, "file://")
		return newLocalDeliverer(path)
	default:
		return newLocalDeliverer(dest)
	}
}

// DelivererOption configures optional deliverer parameters.
type DelivererOption func(*delivererOptions)

type delivererOptions struct {
	ftpUsername string
	ftpPassword string
}

// WithFTPCredentials sets the username and password for FTP deliverers.
func WithFTPCredentials(username, password string) DelivererOption {
	return func(o *delivererOptions) {
		o.ftpUsername = username
		o.ftpPassword = password
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
type S3Deliverer struct {
	bucket string
	prefix string
	cmd    commander.Commander
}

func newS3Deliverer(dest string, cmd commander.Commander) (*S3Deliverer, error) {
	if cmd == nil {
		return nil, fmt.Errorf("deliverer: s3: commander is required")
	}
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

func (d *S3Deliverer) s3Key(filename, ext string) string {
	if d.prefix == "" {
		return filename + ext
	}
	return d.prefix + "/" + filename + ext
}

func (d *S3Deliverer) s3URL(filename, ext string) string {
	return "s3://" + d.bucket + "/" + d.s3Key(filename, ext)
}

// Deliver uploads filename.md and filename.html to S3 via aws s3 cp.
// On failure the temp directory is always cleaned up -- the defer runs
// regardless of which upload failed.
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

func (d *S3Deliverer) String() string {
	if d.prefix == "" {
		return "s3://" + d.bucket + "/"
	}
	return "s3://" + d.bucket + "/" + d.prefix + "/"
}

// -- SSHDeliverer ------------------------------------------------------------

// SSHDeliverer uploads report files to a remote host via scp.
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

	without := strings.TrimPrefix(dest, "ssh://")
	if without == "" {
		return nil, fmt.Errorf("deliverer: ssh: destination %q is missing host", dest)
	}

	colonIdx := strings.Index(without, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing colon separator between host and path", dest)
	}

	userHost := without[:colonIdx]
	remotePath := without[colonIdx+1:]

	if remotePath == "" {
		return nil, fmt.Errorf("deliverer: ssh: destination %q missing remote path", dest)
	}

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
// via scp. The temp directory is always removed on return.
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

	mdArgs := d.scpArgs(mdPath, d.dir, filename, ".md")
	_, _, err = d.cmd.Run(ctx, "", nil, mdArgs[0], mdArgs[1:]...)
	if err != nil {
		return fmt.Errorf("deliverer: ssh: upload md to %s@%s:%s: %w",
			d.user, d.host, d.dir, err)
	}

	htmlArgs := d.scpArgs(htmlPath, d.dir, filename, ".html")
	_, _, err = d.cmd.Run(ctx, "", nil, htmlArgs[0], htmlArgs[1:]...)
	if err != nil {
		return fmt.Errorf("deliverer: ssh: upload html to %s@%s:%s: %w",
			d.user, d.host, d.dir, err)
	}

	return nil
}

func (d *SSHDeliverer) String() string {
	return fmt.Sprintf("ssh://%s@%s:%s/", d.user, d.host, d.dir)
}

// -- FTPDeliverer ------------------------------------------------------------

// FTPDeliverer uploads report files to a remote host via FTP using
// the standard net package. No external dependency is required.
//
// The FTP connection is established per Deliver call and closed on return.
// Passive mode is used by default for compatibility with most firewalls.
//
// Credentials are passed at construction time and never logged.
type FTPDeliverer struct {
	host     string
	dir      string
	username string
	password string
}

func newFTPDeliverer(dest, username, password string) (*FTPDeliverer, error) {
	without := strings.TrimPrefix(dest, "ftp://")
	if without == "" {
		return nil, fmt.Errorf("deliverer: ftp: destination %q is missing host", dest)
	}

	// Parse host and path: ftp://host/path or ftp://host:21/path
	slashIdx := strings.Index(without, "/")
	host := without
	dir := "/"
	if slashIdx >= 0 {
		host = without[:slashIdx]
		dir = without[slashIdx:]
	}
	if dir == "" {
		dir = "/"
	}

	if host == "" {
		return nil, fmt.Errorf("deliverer: ftp: destination %q is missing host", dest)
	}

	// Ensure the host has a port; default to 21.
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = net.JoinHostPort(host, "21")
	}

	if username == "" {
		username = "anonymous"
	}

	return &FTPDeliverer{
		host:     host,
		dir:      strings.TrimSuffix(dir, "/"),
		username: username,
		password: password,
	}, nil
}

// Deliver connects to the FTP server and uploads filename.md and
// filename.html using the raw FTP protocol via net.Conn.
//
// The implementation uses a minimal subset of RFC 959: USER, PASS,
// PASV, STOR, and QUIT. It does not depend on any external FTP library.
func (d *FTPDeliverer) Deliver(ctx context.Context, filename, md, html string) error {
	if err := d.upload(ctx, filename+".md", []byte(md)); err != nil {
		return fmt.Errorf("deliverer: ftp: upload md: %w", err)
	}
	if err := d.upload(ctx, filename+".html", []byte(html)); err != nil {
		return fmt.Errorf("deliverer: ftp: upload html: %w", err)
	}
	return nil
}

// upload opens an FTP control connection, authenticates, enters passive
// mode, and transfers a single file. The connection is closed on return.
func (d *FTPDeliverer) upload(ctx context.Context, name string, data []byte) error {
	dialer := &net.Dialer{}
	ctrl, err := dialer.DialContext(ctx, "tcp", d.host)
	if err != nil {
		return fmt.Errorf("connect %s: %w", d.host, err)
	}
	defer ctrl.Close()

	// Set a generous deadline for the overall operation.
	_ = ctrl.SetDeadline(time.Now().Add(60 * time.Second))

	send := func(cmd string) error {
		_, err := fmt.Fprintf(ctrl, "%s\r\n", cmd)
		return err
	}
	recv := func() (string, error) {
		buf := make([]byte, 1024)
		n, err := ctrl.Read(buf)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(buf[:n])), nil
	}
	expect := func(prefix string) error {
		line, err := recv()
		if err != nil {
			return err
		}
		if !strings.HasPrefix(line, prefix) {
			return fmt.Errorf("ftp: unexpected response %q (want %s...)", line, prefix)
		}
		return nil
	}

	// greeting
	if err := expect("220"); err != nil {
		return err
	}
	// auth
	if err := send("USER " + d.username); err != nil {
		return err
	}
	if err := expect("331"); err != nil {
		return err
	}
	if err := send("PASS " + d.password); err != nil {
		return err
	}
	if err := expect("230"); err != nil {
		return err
	}
	// binary mode
	if err := send("TYPE I"); err != nil {
		return err
	}
	if err := expect("200"); err != nil {
		return err
	}
	// passive mode
	if err := send("PASV"); err != nil {
		return err
	}
	pasvLine, err := recv()
	if err != nil {
		return err
	}
	dataAddr, err := parsePASV(pasvLine)
	if err != nil {
		return fmt.Errorf("ftp: parse PASV: %w", err)
	}

	// open data connection
	dataConn, err := dialer.DialContext(ctx, "tcp", dataAddr)
	if err != nil {
		return fmt.Errorf("ftp: open data connection %s: %w", dataAddr, err)
	}

	remotePath := d.dir + "/" + name
	if err := send("STOR " + remotePath); err != nil {
		dataConn.Close()
		return err
	}
	if err := expect("150"); err != nil {
		dataConn.Close()
		return err
	}

	if _, err := io.Copy(dataConn, strings.NewReader(string(data))); err != nil {
		dataConn.Close()
		return fmt.Errorf("ftp: write data: %w", err)
	}
	dataConn.Close()

	if err := expect("226"); err != nil {
		return err
	}

	_ = send("QUIT")
	return nil
}

// parsePASV parses the address from an FTP 227 PASV response.
// The response format is: 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2)
func parsePASV(line string) (string, error) {
	start := strings.Index(line, "(")
	end := strings.LastIndex(line, ")")
	if start < 0 || end < 0 || end <= start {
		return "", fmt.Errorf("cannot find parentheses in %q", line)
	}
	parts := strings.Split(line[start+1:end], ",")
	if len(parts) != 6 {
		return "", fmt.Errorf("expected 6 comma-separated values, got %d", len(parts))
	}
	ip := strings.Join(parts[:4], ".")
	p1, p2 := 0, 0
	fmt.Sscanf(parts[4], "%d", &p1)
	fmt.Sscanf(parts[5], "%d", &p2)
	port := p1*256 + p2
	return net.JoinHostPort(ip, fmt.Sprintf("%d", port)), nil
}

func (d *FTPDeliverer) String() string {
	return fmt.Sprintf("ftp://%s%s/", d.host, d.dir)
}
