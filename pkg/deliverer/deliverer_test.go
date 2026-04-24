package deliverer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimerlescu/sovereign/pkg/commander"
)

// -- helpers -----------------------------------------------------------------

const (
	testFilename = "sovereign.2026.04.24.localhost-8080"
	testMD       = "# sovereign report\n\ntest markdown content"
	testHTML     = "<!DOCTYPE html><html><body>test</body></html>"
)

func testCmd() *commander.MockCommander {
	return commander.NewMockCommander()
}

// -- New ---------------------------------------------------------------------

func TestNew_LocalBarePathReturnsLocalDeliverer(t *testing.T) {
	d, err := New("/tmp/reports", "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := d.(*LocalDeliverer); !ok {
		t.Errorf("expected *LocalDeliverer, got %T", d)
	}
}

func TestNew_LocalDotReturnsLocalDeliverer(t *testing.T) {
	d, err := New(".", "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := d.(*LocalDeliverer); !ok {
		t.Errorf("expected *LocalDeliverer, got %T", d)
	}
}

func TestNew_FileSchemeReturnsLocalDeliverer(t *testing.T) {
	d, err := New("file:///tmp/reports", "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := d.(*LocalDeliverer); !ok {
		t.Errorf("expected *LocalDeliverer, got %T", d)
	}
}

func TestNew_S3SchemeReturnsS3Deliverer(t *testing.T) {
	d, err := New("s3://my-bucket/reports", "", testCmd())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := d.(*S3Deliverer); !ok {
		t.Errorf("expected *S3Deliverer, got %T", d)
	}
}

func TestNew_SSHSchemeReturnsSSHDeliverer(t *testing.T) {
	d, err := New("ssh://rocky@1.2.3.4:/reports", "/path/to/key.pem", testCmd())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := d.(*SSHDeliverer); !ok {
		t.Errorf("expected *SSHDeliverer, got %T", d)
	}
}

func TestNew_RelativePathReturnsLocalDeliverer(t *testing.T) {
	d, err := New("./reports", "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := d.(*LocalDeliverer); !ok {
		t.Errorf("expected *LocalDeliverer, got %T", d)
	}
}

// -- LocalDeliverer ----------------------------------------------------------

func TestLocalDeliverer_DeliversMarkdownAndHTML(t *testing.T) {
	dir := t.TempDir()
	d, err := newLocalDeliverer(dir)
	if err != nil {
		t.Fatalf("newLocalDeliverer: %v", err)
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	mdPath := filepath.Join(dir, testFilename+".md")
	htmlPath := filepath.Join(dir, testFilename+".html")

	if _, err := os.Stat(mdPath); err != nil {
		t.Errorf("md file not found: %v", err)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Errorf("html file not found: %v", err)
	}
}

func TestLocalDeliverer_ContentPreserved(t *testing.T) {
	dir := t.TempDir()
	d, err := newLocalDeliverer(dir)
	if err != nil {
		t.Fatalf("newLocalDeliverer: %v", err)
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	mdData, err := os.ReadFile(filepath.Join(dir, testFilename+".md"))
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	if string(mdData) != testMD {
		t.Errorf("md content: got %q want %q", string(mdData), testMD)
	}

	htmlData, err := os.ReadFile(filepath.Join(dir, testFilename+".html"))
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	if string(htmlData) != testHTML {
		t.Errorf("html content: got %q want %q", string(htmlData), testHTML)
	}
}

func TestLocalDeliverer_CreatesDirectoryIfAbsent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "report", "dir")
	d, err := newLocalDeliverer(dir)
	if err != nil {
		t.Fatalf("newLocalDeliverer: %v", err)
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

func TestLocalDeliverer_EmptyPathReturnsError(t *testing.T) {
	_, err := newLocalDeliverer("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestLocalDeliverer_StringReturnsDir(t *testing.T) {
	d, err := newLocalDeliverer("/tmp/reports")
	if err != nil {
		t.Fatalf("newLocalDeliverer: %v", err)
	}
	if d.String() != "/tmp/reports" {
		t.Errorf("String: got %q want /tmp/reports", d.String())
	}
}

func TestLocalDeliverer_FileSchemeStripped(t *testing.T) {
	d, err := New("file:///tmp/reports", "", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	local := d.(*LocalDeliverer)
	if local.dir != "/tmp/reports" {
		t.Errorf("dir: got %q want /tmp/reports", local.dir)
	}
}

// -- S3Deliverer -------------------------------------------------------------

func TestS3Deliverer_ParsesBucketAndPrefix(t *testing.T) {
	d, err := newS3Deliverer("s3://my-bucket/reports/2026", testCmd())
	if err != nil {
		t.Fatalf("newS3Deliverer: %v", err)
	}
	if d.bucket != "my-bucket" {
		t.Errorf("bucket: got %q want my-bucket", d.bucket)
	}
	if d.prefix != "reports/2026" {
		t.Errorf("prefix: got %q want reports/2026", d.prefix)
	}
}

func TestS3Deliverer_ParsesBucketOnlyNoPrefixTrailingSlash(t *testing.T) {
	d, err := newS3Deliverer("s3://my-bucket/", testCmd())
	if err != nil {
		t.Fatalf("newS3Deliverer: %v", err)
	}
	if d.bucket != "my-bucket" {
		t.Errorf("bucket: got %q want my-bucket", d.bucket)
	}
	if d.prefix != "" {
		t.Errorf("prefix: got %q want empty", d.prefix)
	}
}

func TestS3Deliverer_ParsesBucketNoPrefix(t *testing.T) {
	d, err := newS3Deliverer("s3://my-bucket", testCmd())
	if err != nil {
		t.Fatalf("newS3Deliverer: %v", err)
	}
	if d.bucket != "my-bucket" {
		t.Errorf("bucket: got %q want my-bucket", d.bucket)
	}
	if d.prefix != "" {
		t.Errorf("prefix: got %q want empty", d.prefix)
	}
}

func TestS3Deliverer_EmptyBucketReturnsError(t *testing.T) {
	_, err := newS3Deliverer("s3://", testCmd())
	if err == nil {
		t.Fatal("expected error for empty bucket")
	}
}

func TestS3Deliverer_NilCommanderReturnsError(t *testing.T) {
	_, err := newS3Deliverer("s3://my-bucket/reports", nil)
	if err == nil {
		t.Fatal("expected error for nil commander")
	}
}

func TestS3Deliverer_S3Key_WithPrefix(t *testing.T) {
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports"}
	got := d.s3Key(testFilename, ".md")
	want := "reports/" + testFilename + ".md"
	if got != want {
		t.Errorf("s3Key: got %q want %q", got, want)
	}
}

func TestS3Deliverer_S3Key_NoPrefix(t *testing.T) {
	d := &S3Deliverer{bucket: "my-bucket", prefix: ""}
	got := d.s3Key(testFilename, ".md")
	want := testFilename + ".md"
	if got != want {
		t.Errorf("s3Key: got %q want %q", got, want)
	}
}

func TestS3Deliverer_S3URL_WithPrefix(t *testing.T) {
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports"}
	got := d.s3URL(testFilename, ".html")
	want := "s3://my-bucket/reports/" + testFilename + ".html"
	if got != want {
		t.Errorf("s3URL: got %q want %q", got, want)
	}
}

func TestS3Deliverer_String_WithPrefix(t *testing.T) {
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports"}
	if d.String() != "s3://my-bucket/reports/" {
		t.Errorf("String: got %q want s3://my-bucket/reports/", d.String())
	}
}

func TestS3Deliverer_String_NoPrefix(t *testing.T) {
	d := &S3Deliverer{bucket: "my-bucket", prefix: ""}
	if d.String() != "s3://my-bucket/" {
		t.Errorf("String: got %q want s3://my-bucket/", d.String())
	}
}

func TestS3Deliverer_DeliverCallsAWSCPTwice(t *testing.T) {
	cmd := testCmd()
	d := &S3Deliverer{
		bucket: "my-bucket",
		prefix: "reports",
		cmd:    cmd,
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if cmd.CallCount("aws") != 2 {
		t.Errorf("expected 2 aws calls, got %d", cmd.CallCount("aws"))
	}
}

func TestS3Deliverer_DeliverUploadsMarkdownFirst(t *testing.T) {
	cmd := testCmd()
	d := &S3Deliverer{
		bucket: "my-bucket",
		prefix: "reports",
		cmd:    cmd,
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	calls := cmd.Calls("aws")
	if len(calls) < 1 {
		t.Fatal("expected at least one aws call")
	}
	found := false
	for _, arg := range calls[0].Args {
		if containsStr(arg, ".md") {
			found = true
			break
		}
	}
	if !found {
		t.Error("first aws call should reference .md file")
	}
}

func TestS3Deliverer_DeliverUploadsHTMLSecond(t *testing.T) {
	cmd := testCmd()
	d := &S3Deliverer{
		bucket: "my-bucket",
		prefix: "reports",
		cmd:    cmd,
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	calls := cmd.Calls("aws")
	if len(calls) < 2 {
		t.Fatal("expected at least two aws calls")
	}
	found := false
	for _, arg := range calls[1].Args {
		if containsStr(arg, ".html") {
			found = true
			break
		}
	}
	if !found {
		t.Error("second aws call should reference .html file")
	}
}

func TestS3Deliverer_DeliverPassesContentTypeMarkdown(t *testing.T) {
	cmd := testCmd()
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports", cmd: cmd}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	calls := cmd.Calls("aws")
	mdCall := calls[0]
	found := false
	for i, arg := range mdCall.Args {
		if arg == "--content-type" && i+1 < len(mdCall.Args) &&
			mdCall.Args[i+1] == "text/markdown" {
			found = true
			break
		}
	}
	if !found {
		t.Error("md upload should pass --content-type text/markdown")
	}
}

func TestS3Deliverer_DeliverPassesContentTypeHTML(t *testing.T) {
	cmd := testCmd()
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports", cmd: cmd}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	calls := cmd.Calls("aws")
	htmlCall := calls[1]
	found := false
	for i, arg := range htmlCall.Args {
		if arg == "--content-type" && i+1 < len(htmlCall.Args) &&
			htmlCall.Args[i+1] == "text/html" {
			found = true
			break
		}
	}
	if !found {
		t.Error("html upload should pass --content-type text/html")
	}
}

func TestS3Deliverer_DeliverReturnsErrorOnAWSFailure(t *testing.T) {
	cmd := testCmd()
	cmd.Register("aws", commander.MockResponse{Err: errors.New("aws: no credentials")})
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports", cmd: cmd}

	err := d.Deliver(context.Background(), testFilename, testMD, testHTML)
	if err == nil {
		t.Fatal("expected error when aws command fails")
	}
}

// -- SSHDeliverer ------------------------------------------------------------

func TestSSHDeliverer_ParsesUserHostPath(t *testing.T) {
	d, err := newSSHDeliverer("ssh://rocky@1.2.3.4:/reports/sovereign", "", testCmd())
	if err != nil {
		t.Fatalf("newSSHDeliverer: %v", err)
	}
	if d.user != "rocky" {
		t.Errorf("user: got %q want rocky", d.user)
	}
	if d.host != "1.2.3.4" {
		t.Errorf("host: got %q want 1.2.3.4", d.host)
	}
	if d.dir != "/reports/sovereign" {
		t.Errorf("dir: got %q want /reports/sovereign", d.dir)
	}
}

func TestSSHDeliverer_ParsesIdentity(t *testing.T) {
	d, err := newSSHDeliverer("ssh://rocky@1.2.3.4:/reports",
		"/home/rocky/.ssh/key.pem", testCmd())
	if err != nil {
		t.Fatalf("newSSHDeliverer: %v", err)
	}
	if d.identity != "/home/rocky/.ssh/key.pem" {
		t.Errorf("identity: got %q want /home/rocky/.ssh/key.pem", d.identity)
	}
}

func TestSSHDeliverer_MissingColonReturnsError(t *testing.T) {
	_, err := newSSHDeliverer("ssh://rocky@1.2.3.4/reports", "", testCmd())
	if err == nil {
		t.Fatal("expected error for missing colon separator")
	}
}

func TestSSHDeliverer_MissingUserReturnsError(t *testing.T) {
	_, err := newSSHDeliverer("ssh://1.2.3.4:/reports", "", testCmd())
	if err == nil {
		t.Fatal("expected error for missing user")
	}
}

func TestSSHDeliverer_MissingHostReturnsError(t *testing.T) {
	_, err := newSSHDeliverer("ssh://rocky@:/reports", "", testCmd())
	if err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestSSHDeliverer_MissingRemotePathReturnsError(t *testing.T) {
	_, err := newSSHDeliverer("ssh://rocky@1.2.3.4:", "", testCmd())
	if err == nil {
		t.Fatal("expected error for missing remote path")
	}
}

func TestSSHDeliverer_NilCommanderReturnsError(t *testing.T) {
	_, err := newSSHDeliverer("ssh://rocky@1.2.3.4:/reports", "", nil)
	if err == nil {
		t.Fatal("expected error for nil commander")
	}
}

func TestSSHDeliverer_DeliverCallsSCPTwice(t *testing.T) {
	cmd := testCmd()
	d := &SSHDeliverer{
		user: "rocky",
		host: "1.2.3.4",
		dir:  "/reports",
		cmd:  cmd,
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	if cmd.CallCount("scp") != 2 {
		t.Errorf("expected 2 scp calls, got %d", cmd.CallCount("scp"))
	}
}

func TestSSHDeliverer_DeliverIncludesIdentityFlag(t *testing.T) {
	cmd := testCmd()
	d := &SSHDeliverer{
		user:     "rocky",
		host:     "1.2.3.4",
		dir:      "/reports",
		identity: "/home/rocky/.ssh/key.pem",
		cmd:      cmd,
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	calls := cmd.Calls("scp")
	for _, call := range calls {
		found := false
		for i, arg := range call.Args {
			if arg == "-i" && i+1 < len(call.Args) &&
				call.Args[i+1] == "/home/rocky/.ssh/key.pem" {
				found = true
				break
			}
		}
		if !found {
			t.Error("scp call should include -i /home/rocky/.ssh/key.pem")
		}
	}
}

func TestSSHDeliverer_DeliverNoIdentityOmitsFlag(t *testing.T) {
	cmd := testCmd()
	d := &SSHDeliverer{
		user: "rocky",
		host: "1.2.3.4",
		dir:  "/reports",
		cmd:  cmd,
	}

	if err := d.Deliver(context.Background(), testFilename, testMD, testHTML); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	calls := cmd.Calls("scp")
	for _, call := range calls {
		for _, arg := range call.Args {
			if arg == "-i" {
				t.Error("scp call should not include -i when identity is empty")
			}
		}
	}
}

func TestSSHDeliverer_DeliverReturnsErrorOnSCPFailure(t *testing.T) {
	cmd := testCmd()
	cmd.Register("scp", commander.MockResponse{Err: errors.New("connection refused")})
	d := &SSHDeliverer{
		user: "rocky",
		host: "1.2.3.4",
		dir:  "/reports",
		cmd:  cmd,
	}

	err := d.Deliver(context.Background(), testFilename, testMD, testHTML)
	if err == nil {
		t.Fatal("expected error when scp fails")
	}
}

func TestSSHDeliverer_StringFormat(t *testing.T) {
	d := &SSHDeliverer{user: "rocky", host: "1.2.3.4", dir: "/reports"}
	want := "ssh://rocky@1.2.3.4:/reports/"
	if d.String() != want {
		t.Errorf("String: got %q want %q", d.String(), want)
	}
}

func TestSSHDeliverer_ScpArgs_WithIdentity(t *testing.T) {
	d := &SSHDeliverer{
		user:     "rocky",
		host:     "1.2.3.4",
		dir:      "/reports",
		identity: "/path/to/key.pem",
	}
	args := d.scpArgs("/tmp/file.md", "/reports", testFilename, ".md")
	if args[0] != "scp" {
		t.Errorf("first arg should be scp, got %q", args[0])
	}
	foundI := false
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) && args[i+1] == "/path/to/key.pem" {
			foundI = true
			break
		}
	}
	if !foundI {
		t.Error("scpArgs should include -i /path/to/key.pem")
	}
}

func TestSSHDeliverer_ScpArgs_RemotePathContainsHostAndDir(t *testing.T) {
	d := &SSHDeliverer{
		user: "rocky",
		host: "1.2.3.4",
		dir:  "/reports",
	}
	args := d.scpArgs("/tmp/file.md", "/reports", testFilename, ".md")
	last := args[len(args)-1]
	if !containsStr(last, "1.2.3.4") {
		t.Errorf("remote path should contain host, got %q", last)
	}
	if !containsStr(last, "/reports") {
		t.Errorf("remote path should contain dir, got %q", last)
	}
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkLocalDeliverer_Deliver(b *testing.B) {
	dir := b.TempDir()
	d, err := newLocalDeliverer(dir)
	if err != nil {
		b.Fatalf("newLocalDeliverer: %v", err)
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Deliver(ctx, testFilename, testMD, testHTML)
	}
}

func BenchmarkS3Deliverer_Deliver(b *testing.B) {
	cmd := commander.NewMockCommander()
	d := &S3Deliverer{bucket: "my-bucket", prefix: "reports", cmd: cmd}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Deliver(ctx, testFilename, testMD, testHTML)
	}
}

func BenchmarkSSHDeliverer_Deliver(b *testing.B) {
	cmd := commander.NewMockCommander()
	d := &SSHDeliverer{user: "rocky", host: "1.2.3.4", dir: "/reports", cmd: cmd}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Deliver(ctx, testFilename, testMD, testHTML)
	}
}

func BenchmarkNew_Local(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New("/tmp/reports", "", nil)
	}
}

func BenchmarkNew_S3(b *testing.B) {
	cmd := commander.NewMockCommander()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New("s3://my-bucket/reports", "", cmd)
	}
}

func BenchmarkNew_SSH(b *testing.B) {
	cmd := commander.NewMockCommander()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		New("ssh://rocky@1.2.3.4:/reports", "/path/to/key.pem", cmd)
	}
}

// -- helpers -----------------------------------------------------------------

// containsStr returns true if substr appears within s.
func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	return strings.Contains(s, substr)
}

// readAll drains an io.Reader and returns the content as a string.
func readAll(r io.Reader) string {
	if r == nil {
		return ""
	}
	b, _ := io.ReadAll(r)
	return string(b)
}
