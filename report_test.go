package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// -- percentile --------------------------------------------------------------

// TestPercentile_EmptySlice verifies that an empty slice returns zero.
func TestPercentile_EmptySlice(t *testing.T) {
	got := percentile([]time.Duration{}, 50)
	requireEqual(t, got, time.Duration(0), "percentile empty slice")
}

// TestPercentile_SingleElement verifies that a single-element slice
// returns that element for any percentile.
func TestPercentile_SingleElement(t *testing.T) {
	d := 100 * time.Millisecond
	got := percentile([]time.Duration{d}, 99)
	requireEqual(t, got, d, "percentile single element")
}

// TestPercentile_P50 verifies the median of a sorted slice.
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

// TestPercentile_P99 verifies the 99th percentile returns a value
// near the end of the sorted slice.
func TestPercentile_P99(t *testing.T) {
	sorted := make([]time.Duration, 100)
	for i := range sorted {
		sorted[i] = time.Duration(i+1) * time.Millisecond
	}
	got := percentile(sorted, 99)
	requireEqual(t, got, 99*time.Millisecond, "percentile p99")
}

// -- domainFromURL -----------------------------------------------------------

// TestDomainFromURL_HTTPS verifies that the https scheme is stripped.
func TestDomainFromURL_HTTPS(t *testing.T) {
	got := domainFromURL("https://example.com/path")
	requireEqual(t, got, "example.com", "domainFromURL https")
}

// TestDomainFromURL_HTTP verifies that the http scheme is stripped.
func TestDomainFromURL_HTTP(t *testing.T) {
	got := domainFromURL("http://localhost:8080/")
	requireEqual(t, got, "localhost-8080", "domainFromURL http with port")
}

// TestDomainFromURL_EmptyString verifies that an empty string
// returns an empty string without panicking.
func TestDomainFromURL_EmptyString(t *testing.T) {
	got := domainFromURL("")
	requireEqual(t, got, "", "domainFromURL empty string")
}

// TestDomainFromURL_PortReplacedWithDash verifies that colons in
// the host are replaced with dashes for filesystem safety.
func TestDomainFromURL_PortReplacedWithDash(t *testing.T) {
	got := domainFromURL("http://myapp.com:9090/")
	requireEqual(t, got, "myapp.com-9090", "domainFromURL port replaced")
}

// -- formatBytes -------------------------------------------------------------

// TestFormatBytes_Bytes verifies formatting of values under 1KB.
func TestFormatBytes_Bytes(t *testing.T) {
	got := formatBytes(500)
	requireEqual(t, got, "500 B", "formatBytes bytes")
}

// TestFormatBytes_Kilobytes verifies formatting of KB values.
func TestFormatBytes_Kilobytes(t *testing.T) {
	got := formatBytes(2048)
	requireEqual(t, got, "2.00 KB", "formatBytes kilobytes")
}

// TestFormatBytes_Megabytes verifies formatting of MB values.
func TestFormatBytes_Megabytes(t *testing.T) {
	got := formatBytes(1048576)
	requireEqual(t, got, "1.00 MB", "formatBytes megabytes")
}

// TestFormatBytes_Gigabytes verifies formatting of GB values.
func TestFormatBytes_Gigabytes(t *testing.T) {
	got := formatBytes(1073741824)
	requireEqual(t, got, "1.00 GB", "formatBytes gigabytes")
}

// TestFormatBytes_Zero verifies that zero bytes formats correctly.
func TestFormatBytes_Zero(t *testing.T) {
	got := formatBytes(0)
	requireEqual(t, got, "0 B", "formatBytes zero")
}

// -- renderUnifiedMarkdown ---------------------------------------------------

// TestRenderUnifiedMarkdown_NonEmpty verifies that the markdown
// renderer returns a non-empty string.
func TestRenderUnifiedMarkdown_NonEmpty(t *testing.T) {
	unified := testUnifiedReport(t)
	md, err := renderUnifiedMarkdown(unified)
	requireNoError(t, err, "renderUnifiedMarkdown non-empty")
	requireTrue(t, len(md) > 0, "renderUnifiedMarkdown non-empty string")
}

// TestRenderUnifiedMarkdown_ContainsVersion verifies that the version
// appears in the rendered markdown.
func TestRenderUnifiedMarkdown_ContainsVersion(t *testing.T) {
	unified := testUnifiedReport(t)
	md, err := renderUnifiedMarkdown(unified)
	requireNoError(t, err, "renderUnifiedMarkdown contains version")
	requireTrue(t, strings.Contains(md, unified.Version),
		"renderUnifiedMarkdown contains version string")
}

// TestRenderUnifiedMarkdown_ContainsTarget verifies that the target
// URL appears in the rendered markdown.
func TestRenderUnifiedMarkdown_ContainsTarget(t *testing.T) {
	unified := testUnifiedReport(t)
	md, err := renderUnifiedMarkdown(unified)
	requireNoError(t, err, "renderUnifiedMarkdown contains target")
	requireTrue(t, strings.Contains(md, unified.Config.Hit),
		"renderUnifiedMarkdown contains target URL")
}

// -- renderUnifiedHTML -------------------------------------------------------

// TestRenderUnifiedHTML_NonEmpty verifies that the HTML renderer
// returns a non-empty string.
func TestRenderUnifiedHTML_NonEmpty(t *testing.T) {
	unified := testUnifiedReport(t)
	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML non-empty")
	requireTrue(t, len(html) > 0, "renderUnifiedHTML non-empty string")
}

// TestRenderUnifiedHTML_IsHTML verifies that the output begins with
// a DOCTYPE declaration.
func TestRenderUnifiedHTML_IsHTML(t *testing.T) {
	unified := testUnifiedReport(t)
	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML is HTML")
	requireTrue(t, strings.HasPrefix(html, "<!DOCTYPE html>"),
		"renderUnifiedHTML starts with DOCTYPE")
}

// TestRenderUnifiedHTML_ContainsVersion verifies the version appears
// in the rendered HTML.
func TestRenderUnifiedHTML_ContainsVersion(t *testing.T) {
	unified := testUnifiedReport(t)
	html, err := renderUnifiedHTML(unified)
	requireNoError(t, err, "renderUnifiedHTML contains version")
	requireTrue(t, strings.Contains(html, unified.Version),
		"renderUnifiedHTML contains version")
}

// -- readPlanetReport --------------------------------------------------------

// TestReadPlanetReport_ValidReport verifies that a well-formed lemmings
// report file is parsed correctly.
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

// TestReadPlanetReport_EmptyReportPath verifies that an empty
// ReportPath returns an error.
func TestReadPlanetReport_EmptyReportPath(t *testing.T) {
	s := testSovereign(t)
	planet := testDonePlanet(t, 1)
	planet.ReportPath = ""

	_, err := s.readPlanetReport(planet)
	requireError(t, err, "readPlanetReport empty report path")
}

// TestReadPlanetReport_NoReportFile verifies that a directory with
// no lemmings report file returns an error.
func TestReadPlanetReport_NoReportFile(t *testing.T) {
	s := testSovereign(t)
	planet := testDonePlanet(t, 1)
	// ReportPath exists but contains no lemmings yml file

	_, err := s.readPlanetReport(planet)
	requireError(t, err, "readPlanetReport no report file")
}

// -- writeReport -------------------------------------------------------------

// TestWriteReport_WritesMarkdownAndHTML verifies that writeReport
// creates both .md and .html files in the destination directory.
func TestWriteReport_WritesMarkdownAndHTML(t *testing.T) {
	dest := t.TempDir()
	err := writeReport(dest, "sovereign.2026.04.22.example.com",
		"# test markdown", "<html>test</html>")
	requireNoError(t, err, "writeReport writes files")

	mdPath := filepath.Join(dest, "sovereign.2026.04.22.example.com.md")
	htmlPath := filepath.Join(dest, "sovereign.2026.04.22.example.com.html")

	if _, statErr := os.Stat(mdPath); statErr != nil {
		t.Fatalf("writeReport: markdown file not created at %s", mdPath)
	}
	if _, statErr := os.Stat(htmlPath); statErr != nil {
		t.Fatalf("writeReport: html file not created at %s", htmlPath)
	}
}

// TestWriteReport_ContentPreserved verifies that the file contents
// match what was passed to writeReport.
func TestWriteReport_ContentPreserved(t *testing.T) {
	dest := t.TempDir()
	md := "# sovereign report\n\ntest content"
	html := "<!DOCTYPE html><html><body>test</body></html>"

	err := writeReport(dest, "test-report", md, html)
	requireNoError(t, err, "writeReport content preserved")

	mdData, readErr := os.ReadFile(filepath.Join(dest, "test-report.md"))
	requireNoError(t, readErr, "writeReport read md")
	requireEqual(t, string(mdData), md, "writeReport md content matches")

	htmlData, readErr := os.ReadFile(filepath.Join(dest, "test-report.html"))
	requireNoError(t, readErr, "writeReport read html")
	requireEqual(t, string(htmlData), html, "writeReport html content matches")
}

// TestWriteReport_CreatesDestDirectory verifies that writeReport
// creates the destination directory if it does not exist.
func TestWriteReport_CreatesDestDirectory(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "nested", "report", "dir")
	err := writeReport(dest, "test-report", "# md", "<html/>")
	requireNoError(t, err, "writeReport creates dest directory")

	if _, statErr := os.Stat(dest); statErr != nil {
		t.Fatalf("writeReport: dest directory not created at %s", dest)
	}
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkRenderUnifiedMarkdown measures markdown rendering throughput.
func BenchmarkRenderUnifiedMarkdown(b *testing.B) {
	unified := testUnifiedReport(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderUnifiedMarkdown(unified)
	}
}

// BenchmarkRenderUnifiedHTML measures HTML rendering throughput.
func BenchmarkRenderUnifiedHTML(b *testing.B) {
	unified := testUnifiedReport(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderUnifiedHTML(unified)
	}
}

// -- helpers -----------------------------------------------------------------

// testUnifiedReport builds a minimal UnifiedReport for render tests.
// t may be nil for benchmark use where no temp dirs are needed.
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
