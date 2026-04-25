package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/deliverer"
)

// -- percentile --------------------------------------------------------------

func TestPercentile_EmptySlice(t *testing.T) {
	got := percentile([]time.Duration{}, 50)
	requireEqual(t, got, time.Duration(0), "percentile empty slice")
}

func TestPercentile_SingleElement(t *testing.T) {
	d := 100 * time.Millisecond
	got := percentile([]time.Duration{d}, 99)
	requireEqual(t, got, d, "percentile single element")
}

func TestPercentile_P50(t *testing.T) {
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}
	got := percentile(sorted, 50)
	requireEqual(t, got, 30*time.Millisecond, "percentile p50")
}

func TestPercentile_P99(t *testing.T) {
	sorted := make([]time.Duration, 100)
	for i := range sorted {
		sorted[i] = time.Duration(i+1) * time.Millisecond
	}
	got := percentile(sorted, 99)
	requireEqual(t, got, 99*time.Millisecond, "percentile p99")
}

// -- domainFromURL -----------------------------------------------------------

func TestDomainFromURL_HTTPS(t *testing.T) {
	got := domainFromURL("https://example.com/path")
	requireEqual(t, got, "example.com", "domainFromURL https")
}

func TestDomainFromURL_HTTP(t *testing.T) {
	got := domainFromURL("http://localhost:8080/")
	requireEqual(t, got, "localhost-8080", "domainFromURL http with port")
}

func TestDomainFromURL_EmptyString(t *testing.T) {
	got := domainFromURL("")
	requireEqual(t, got, "", "domainFromURL empty string")
}

func TestDomainFromURL_PortReplacedWithDash(t *testing.T) {
	got := domainFromURL("http://myapp.com:9090/")
	requireEqual(t, got, "myapp.com-9090", "domainFromURL port replaced")
}

// -- formatBytes -------------------------------------------------------------

func TestFormatBytes_Bytes(t *testing.T) {
	got := formatBytes(500)
	requireEqual(t, got, "500 B", "formatBytes bytes")
}

func TestFormatBytes_Kilobytes(t *testing.T) {
	got := formatBytes(2048)
	requireEqual(t, got, "2.00 KB", "formatBytes kilobytes")
}

func TestFormatBytes_Megabytes(t *testing.T) {
	got := formatBytes(1048576)
	requireEqual(t, got, "1.00 MB", "formatBytes megabytes")
}

func TestFormatBytes_Gigabytes(t *testing.T) {
	got := formatBytes(1073741824)
	requireEqual(t, got, "1.00 GB", "formatBytes gigabytes")
}

func TestFormatBytes_Zero(t *testing.T) {
	got := formatBytes(0)
	requireEqual(t, got, "0 B", "formatBytes zero")
}

// -- renderUnifiedMarkdown ---------------------------------------------------

func TestRenderUnifiedMarkdown_NonEmpty(t *testing.T) {
	unified := testUnifiedReport(t)
	md, err := renderUnifiedMarkdown(unified)
	requireNoError(t, err, "renderUnifiedMarkdown non-empty")
	requireTrue(t, len(md) > 0, "renderUnifiedMarkdown non-empty string")
}

func TestRenderUnifiedMarkdown_ContainsVersion(t *testing.T) {
	unified := testUnifiedReport(t)
	md, err := renderUnifiedMarkdown(unified)
	requireNoError(t, err, "renderUnifiedMarkdown contains version")
	requireTrue(t, strings.Contains(md, unified.Version),
		"renderUnifiedMarkdown contains version string")
}

func TestRenderUnifiedMarkdown_ContainsTarget(t *testing.T) {
	unified := testUnifiedReport(t)
	md, err := renderUnifiedMarkdown(unified)
	requireNoError(t, err, "renderUnifiedMarkdown contains target")
	requireTrue(t, strings.Contains(md, unified.Config.Hit),
		"renderUnifiedMarkdown contains target URL")
}

// -- renderUnifiedHTML -------------------------------------------------------

func TestRenderUnifiedHTML_NonEmpty(t *testing.T) {
	unified := testUnifiedReport(t)
	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML non-empty")
	requireTrue(t, len(html) > 0, "renderUnifiedHTML non-empty string")
}

func TestRenderUnifiedHTML_IsHTML(t *testing.T) {
	unified := testUnifiedReport(t)
	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML is HTML")
	requireTrue(t, strings.HasPrefix(html, "<!DOCTYPE html>"),
		"renderUnifiedHTML starts with DOCTYPE")
}

func TestRenderUnifiedHTML_ContainsVersion(t *testing.T) {
	unified := testUnifiedReport(t)
	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML contains version")
	requireTrue(t, strings.Contains(html, unified.Version),
		"renderUnifiedHTML contains version")
}

// TestRenderUnifiedHTML_EscapesUserContent verifies that the html/template
// renderer escapes content that would be dangerous in raw fmt.Sprintf output.
// A target URL containing angle brackets must be escaped, not injected as
// raw HTML.
func TestRenderUnifiedHTML_EscapesUserContent(t *testing.T) {
	unified := testUnifiedReport(t)
	unified.Config.Hit = `http://example.com/<script>alert(1)</script>`

	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML escapes user content")
	requireFalse(t, strings.Contains(html, "<script>alert(1)</script>"),
		"renderUnifiedHTML must not emit raw script tag from user content")
	requireTrue(t, strings.Contains(html, "&lt;script&gt;"),
		"renderUnifiedHTML HTML-escapes angle brackets from user content")
}

// TestRenderUnifiedHTML_EscapesInstanceID verifies that instance IDs
// containing special characters are escaped.
func TestRenderUnifiedHTML_EscapesInstanceID(t *testing.T) {
	unified := testUnifiedReport(t)
	unified.LostPlanets = []*Planet{
		{
			ID:         99,
			Region:     "us-east-1",
			InstanceID: `i-<dangerous>"value"`,
		},
	}

	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML escapes instance ID")
	requireFalse(t, strings.Contains(html, `i-<dangerous>`),
		"renderUnifiedHTML must not emit raw angle brackets in instance ID")
}

// -- readPlanetReport --------------------------------------------------------

func TestReadPlanetReport_ValidReport(t *testing.T) {
	s := testSovereign(t)
	planet := testDonePlanet(t, 1)
	writeMockLemmingsReport(t, planet.ReportPath, 1)

	report, err := s.readPlanetReport(planet)
	requireNoError(t, err, "readPlanetReport valid report")
	requireTrue(t, report != nil, "readPlanetReport non-nil")
	requireTrue(t, report.TotalVisits > 0,
		"readPlanetReport TotalVisits populated")
}

func TestReadPlanetReport_EmptyReportPath(t *testing.T) {
	s := testSovereign(t)
	planet := testDonePlanet(t, 1)
	planet.ReportPath = ""

	_, err := s.readPlanetReport(planet)
	requireError(t, err, "readPlanetReport empty report path")
}

func TestReadPlanetReport_NoReportFile(t *testing.T) {
	s := testSovereign(t)
	planet := testDonePlanet(t, 1)

	_, err := s.readPlanetReport(planet)
	requireError(t, err, "readPlanetReport no report file")
}

// -- deliverReport -----------------------------------------------------------

// TestDeliverReport_LocalDestination verifies that deliverReport writes
// both .md and .html files to a local directory destination.
func TestDeliverReport_LocalDestination(t *testing.T) {
	dest := t.TempDir()
	s := testSovereign(t)
	s.cfg.SaveTo = []string{dest}

	unified := testUnifiedReport(t)
	err := s.deliverReport(unified)
	requireNoError(t, err, "deliverReport local destination")

	entries, err := os.ReadDir(dest)
	requireNoError(t, err, "read dest directory")
	requireTrue(t, len(entries) == 2, "deliverReport writes two files")

	var hasMD, hasHTML bool
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			hasMD = true
		}
		if strings.HasSuffix(e.Name(), ".html") {
			hasHTML = true
		}
	}
	requireTrue(t, hasMD, "deliverReport writes markdown file")
	requireTrue(t, hasHTML, "deliverReport writes html file")
}

// TestDeliverReport_InvalidDestinationLogsWarning verifies that an
// unrecognised save-to destination logs a warning but does not return
// an error, so a bad destination does not abort delivery to valid ones.
func TestDeliverReport_InvalidDestinationLogsWarning(t *testing.T) {
	goodDest := t.TempDir()
	s := testSovereign(t)
	// unsupported scheme that passes validation (scheme check is in validate,
	// construction-time error in deliverer.New)
	s.cfg.SaveTo = []string{"badscheme://somewhere", goodDest}

	unified := testUnifiedReport(t)
	err := s.deliverReport(unified)

	// deliverReport never returns an error for individual delivery failures.
	requireNoError(t, err, "deliverReport bad destination does not abort")

	// The good destination should still have received the report.
	entries, readErr := os.ReadDir(goodDest)
	requireNoError(t, readErr, "read good dest directory")
	requireTrue(t, len(entries) == 2,
		"deliverReport delivers to good destination even when one destination fails")
}

// TestDeliverReport_MultipleDestinations verifies that deliverReport
// delivers to all configured destinations.
func TestDeliverReport_MultipleDestinations(t *testing.T) {
	dest1 := t.TempDir()
	dest2 := t.TempDir()
	s := testSovereign(t)
	s.cfg.SaveTo = []string{dest1, dest2}

	unified := testUnifiedReport(t)
	err := s.deliverReport(unified)
	requireNoError(t, err, "deliverReport multiple destinations")

	for _, dest := range []string{dest1, dest2} {
		entries, err := os.ReadDir(dest)
		requireNoError(t, err, "read dest directory")
		requireTrue(t, len(entries) == 2,
			"deliverReport delivers to each destination")
	}
}

// TestDeliverReport_EmptyDestinationsIsNoop verifies that deliverReport
// with no destinations does not error.
func TestDeliverReport_EmptyDestinationsIsNoop(t *testing.T) {
	s := testSovereign(t)
	s.cfg.SaveTo = []string{}

	unified := testUnifiedReport(t)
	err := s.deliverReport(unified)
	requireNoError(t, err, "deliverReport empty destinations is noop")
}

// -- LocalDeliverer ----------------------------------------------------------

func TestLocalDeliverer_WritesMarkdownAndHTML(t *testing.T) {
	dest := t.TempDir()
	d, err := deliverer.New(dest, "", nil)
	requireNoError(t, err, "New local deliverer")

	err = d.Deliver(context.Background(),
		"sovereign.2026.04.22.example.com",
		"# test markdown", "<html>test</html>")
	requireNoError(t, err, "Deliver writes files")

	mdPath := filepath.Join(dest, "sovereign.2026.04.22.example.com.md")
	htmlPath := filepath.Join(dest, "sovereign.2026.04.22.example.com.html")

	if _, statErr := os.Stat(mdPath); statErr != nil {
		t.Fatalf("Deliver: markdown file not created at %s", mdPath)
	}
	if _, statErr := os.Stat(htmlPath); statErr != nil {
		t.Fatalf("Deliver: html file not created at %s", htmlPath)
	}
}

func TestLocalDeliverer_ContentPreserved(t *testing.T) {
	dest := t.TempDir()
	d, err := deliverer.New(dest, "", nil)
	requireNoError(t, err, "New local deliverer")

	md := "# sovereign report\n\ntest content"
	html := "<!DOCTYPE html><html><body>test</body></html>"

	err = d.Deliver(context.Background(), "test-report", md, html)
	requireNoError(t, err, "Deliver content preserved")

	mdData, readErr := os.ReadFile(filepath.Join(dest, "test-report.md"))
	requireNoError(t, readErr, "read md")
	requireEqual(t, string(mdData), md, "md content matches")

	htmlData, readErr := os.ReadFile(filepath.Join(dest, "test-report.html"))
	requireNoError(t, readErr, "read html")
	requireEqual(t, string(htmlData), html, "html content matches")
}

func TestLocalDeliverer_CreatesDestDirectory(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nested", "report", "dir")
	d, err := deliverer.New(dest, "", nil)
	requireNoError(t, err, "New local deliverer")

	err = d.Deliver(context.Background(), "test-report", "# md", "<html/>")
	requireNoError(t, err, "Deliver creates dest directory")

	if _, statErr := os.Stat(dest); statErr != nil {
		t.Fatalf("Deliver: dest directory not created at %s", dest)
	}
}

// -- validateSaveToDestination -----------------------------------------------

func TestValidateSaveToDestination_SupportedSchemes(t *testing.T) {
	supported := []string{
		"s3://my-bucket/reports",
		"ssh://rocky@1.2.3.4:/reports",
		"ftp://ftpserver.example.com/reports",
		"file:///tmp/reports",
		"/tmp/reports",
		"./reports",
		".",
	}
	for _, dest := range supported {
		err := validateSaveToDestination(dest)
		requireNoError(t, err, "validateSaveToDestination should accept "+dest)
	}
}

func TestValidateSaveToDestination_UnsupportedScheme(t *testing.T) {
	unsupported := []string{
		"gopher://old.server.com/reports",
		"smb://fileserver/share",
		"rsync://backup.server.com/path",
	}
	for _, dest := range unsupported {
		err := validateSaveToDestination(dest)
		requireError(t, err, "validateSaveToDestination should reject "+dest)
	}
}

// -- validateConfig save-to --------------------------------------------------

func TestValidateConfig_FTPDestinationAccepted(t *testing.T) {
	cfg := testConfig()
	cfg.SaveTo = []string{"ftp://ftpserver.example.com/reports"}
	err := validateConfig(&cfg)
	requireNoError(t, err, "validateConfig accepts ftp:// save-to")
}

func TestValidateConfig_HTTPHitInvalidScheme(t *testing.T) {
	cfg := testConfig()
	cfg.Hit = "httpevil://malicious.com"
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig rejects non-http/https scheme in hit URL")
}

func TestValidateConfig_HitWithNoHost(t *testing.T) {
	cfg := testConfig()
	cfg.Hit = "http://"
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig rejects hit URL with empty host")
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkRenderUnifiedMarkdown(b *testing.B) {
	unified := testUnifiedReport(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderUnifiedMarkdown(unified)
	}
}

func BenchmarkRenderUnifiedHTML(b *testing.B) {
	unified := testUnifiedReport(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderUnifiedHTML(unified)
	}
}

// -- helpers -----------------------------------------------------------------

func testUnifiedReport(t *testing.T) *UnifiedReport {
	cfg := SovereignConfig{
		Hit:     testHit,
		Version: testVersion,
	}
	planet := &Planet{
		ID:         1,
		Region:     testRegion,
		PublicIPv4: "54.1.10.1",
		InstanceID: "i-00000000000000001",
	}
	pr := &PlanetReport{
		Planet:      planet,
		TotalVisits: 1000,
		TotalBytes:  1048576,
		Total2xx:    950,
		Total3xx:    30,
		Total4xx:    15,
		Total5xx:    5,
		P50:         20 * time.Millisecond,
		P90:         100 * time.Millisecond,
		P95:         200 * time.Millisecond,
		P99:         400 * time.Millisecond,
		Fastest:     1 * time.Millisecond,
		Slowest:     500 * time.Millisecond,
	}
	return &UnifiedReport{
		Version:      testVersion,
		GeneratedAt:  time.Now(),
		Duration:     30 * time.Second,
		Config:       cfg,
		TotalPlanets: 1,
		TotalVisits:  1000,
		TotalBytes:   1048576,
		Total2xx:     950,
		Total3xx:     30,
		Total4xx:     15,
		Total5xx:     5,
		Fastest:      1 * time.Millisecond,
		Slowest:      500 * time.Millisecond,
		P50:          20 * time.Millisecond,
		P90:          100 * time.Millisecond,
		P95:          200 * time.Millisecond,
		P99:          400 * time.Millisecond,
		Planets:      []*PlanetReport{pr},
		LostPlanets:  []*Planet{},
	}
}

func TestAggregateReport_BytesAccumulatesNumerically(t *testing.T) {
	s := testSovereign(t)

	planet1 := testDonePlanet(t, 1)
	planet2 := testDonePlanet(t, 2)
	writeMockLemmingsReport(t, planet1.ReportPath, 1)
	writeMockLemmingsReport(t, planet2.ReportPath, 2)

	s.planets = []*Planet{planet1, planet2}
	s.state.Planets = s.planets

	_ = s.aggregateReport()

	report1, err := s.readPlanetReport(planet1)
	requireNoError(t, err, "readPlanetReport planet1")

	report2, err := s.readPlanetReport(planet2)
	requireNoError(t, err, "readPlanetReport planet2")

	pathMap := make(map[string]*GlobalPathReport)
	for _, path := range report1.Paths {
		gp := &GlobalPathReport{URL: path.URL}
		gp.Bytes += path.Bytes
		pathMap[path.URL] = gp
	}
	for _, path := range report2.Paths {
		gp, ok := pathMap[path.URL]
		if !ok {
			gp = &GlobalPathReport{URL: path.URL}
			pathMap[path.URL] = gp
		}
		gp.Bytes += path.Bytes
	}

	for url, gp := range pathMap {
		if gp.Bytes <= 0 {
			t.Errorf("path %s: Bytes = %d, want > 0", url, gp.Bytes)
		}
		if gp.Bytes > 10_000_000 {
			t.Errorf("path %s: Bytes = %d looks like string concatenation, not integer addition",
				url, gp.Bytes)
		}
	}
}
