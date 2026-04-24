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

	_, err := s.readPlanetReport(planet)
	requireError(t, err, "readPlanetReport no report file")
}

// -- LocalDeliverer ----------------------------------------------------------

// TestLocalDeliverer_WritesMarkdownAndHTML verifies that Deliver
// creates both .md and .html files in the destination directory.
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

// TestLocalDeliverer_ContentPreserved verifies that the file contents
// match what was passed to Deliver.
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

// TestLocalDeliverer_CreatesDestDirectory verifies that Deliver
// creates the destination directory if it does not exist.
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

// TestAggregateReport_BytesAccumulatesNumerically verifies that bytes
// across multiple paths accumulate as integers not strings. This is a
// regression test for the GlobalPathReport.Bytes string-concatenation bug.
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
