package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/deliverer"
	"gopkg.in/yaml.v3"
)

// PlanetReport holds the aggregated metrics from a single planet's
// lemmings run.
type PlanetReport struct {
	Planet      *Planet
	TotalVisits int64
	TotalBytes  int64
	Total2xx    int64
	Total3xx    int64
	Total4xx    int64
	Total5xx    int64
	WaitingRoom int64
	Errors      int64
	Fastest     time.Duration
	Slowest     time.Duration
	P50         time.Duration
	P90         time.Duration
	P95         time.Duration
	P99         time.Duration
	Paths       []PlanetPathReport
}

// PlanetPathReport holds per-path metrics from a single planet.
type PlanetPathReport struct {
	URL         string
	Hits        int64
	Bytes       int64
	XX2         int64
	XX3         int64
	XX4         int64
	XX5         int64
	WaitingRoom int64
	Errors      int64
	P50         time.Duration
	P99         time.Duration
}

// UnifiedReport is the aggregated report across all planets.
type UnifiedReport struct {
	Version     string
	GeneratedAt time.Time
	Duration    time.Duration
	Config      SovereignConfig
	Planets     []*PlanetReport
	LostPlanets []*Planet

	TotalPlanets int
	TotalVisits  int64
	TotalBytes   int64
	Total2xx     int64
	Total3xx     int64
	Total4xx     int64
	Total5xx     int64
	WaitingRoom  int64
	Errors       int64

	Fastest time.Duration
	Slowest time.Duration
	P50     time.Duration
	P90     time.Duration
	P95     time.Duration
	P99     time.Duration

	Paths []GlobalPathReport
}

// GlobalPathReport holds per-path metrics aggregated across all planets.
type GlobalPathReport struct {
	URL         string
	Hits        int64
	Bytes       int64
	XX2         int64
	XX3         int64
	XX4         int64
	XX5         int64
	WaitingRoom int64
	Errors      int64
	P50         time.Duration
	P99         time.Duration
}

// lemmingsReportFile is the structure of the lemmings YAML report file.
type lemmingsReportFile struct {
	Version     string        `yaml:"Version"`
	GeneratedAt time.Time     `yaml:"GeneratedAt"`
	Duration    time.Duration `yaml:"Duration"`
	TotalVisits int64         `yaml:"TotalVisits"`
	TotalBytes  int64         `yaml:"TotalBytes"`
	Total2xx    int64         `yaml:"Total2xx"`
	Total3xx    int64         `yaml:"Total3xx"`
	Total4xx    int64         `yaml:"Total4xx"`
	Total5xx    int64         `yaml:"Total5xx"`
	WaitingRoom int64         `yaml:"TotalWaitingRoom"`
	Errors      int64         `yaml:"TotalErrors"`
	Fastest     time.Duration `yaml:"Fastest"`
	Slowest     time.Duration `yaml:"Slowest"`
	P50         time.Duration `yaml:"P50"`
	P90         time.Duration `yaml:"P90"`
	P95         time.Duration `yaml:"P95"`
	P99         time.Duration `yaml:"P99"`
	Paths       []struct {
		URL         string        `yaml:"URL"`
		Hits        int64         `yaml:"Hits"`
		Bytes       int64         `yaml:"Bytes"`
		XX2         int64         `yaml:"XX2"`
		XX3         int64         `yaml:"XX3"`
		XX4         int64         `yaml:"XX4"`
		XX5         int64         `yaml:"XX5"`
		WaitingRoom int64         `yaml:"WaitingRoom"`
		Errors      int64         `yaml:"Errors"`
		P50         time.Duration `yaml:"P50"`
		P99         time.Duration `yaml:"P99"`
	} `yaml:"Paths"`
}

// aggregateReport reads the lemmings report files collected from each
// completed planet, merges them into a UnifiedReport, and delivers
// the unified report to all configured save-to destinations.
func (s *Sovereign) aggregateReport() error {
	completed := s.completedPlanets()
	if len(completed) == 0 {
		return fmt.Errorf("no planets completed -- no report to aggregate")
	}

	unified := &UnifiedReport{
		Version:      Version(),
		GeneratedAt:  time.Now(),
		Duration:     time.Since(s.startedAt),
		Config:       s.cfg,
		TotalPlanets: len(s.planets),
		LostPlanets:  s.lostPlanets(),
		Fastest:      time.Duration(math.MaxInt64),
	}

	pathMap := make(map[string]*GlobalPathReport)
	var allDurations []time.Duration

	for _, planet := range completed {
		report, err := s.readPlanetReport(planet)
		if err != nil {
			fmt.Fprintf(s.out, "  sovereign: warning: could not read report for planet %d: %v\n",
				planet.ID, err)
			continue
		}

		planetReport := &PlanetReport{
			Planet:      planet,
			TotalVisits: report.TotalVisits,
			TotalBytes:  report.TotalBytes,
			Total2xx:    report.Total2xx,
			Total3xx:    report.Total3xx,
			Total4xx:    report.Total4xx,
			Total5xx:    report.Total5xx,
			WaitingRoom: report.WaitingRoom,
			Errors:      report.Errors,
			Fastest:     report.Fastest,
			Slowest:     report.Slowest,
			P50:         report.P50,
			P90:         report.P90,
			P95:         report.P95,
			P99:         report.P99,
		}

		unified.TotalVisits += report.TotalVisits
		unified.TotalBytes += report.TotalBytes
		unified.Total2xx += report.Total2xx
		unified.Total3xx += report.Total3xx
		unified.Total4xx += report.Total4xx
		unified.Total5xx += report.Total5xx
		unified.WaitingRoom += report.WaitingRoom
		unified.Errors += report.Errors

		if report.Fastest < unified.Fastest {
			unified.Fastest = report.Fastest
		}
		if report.Slowest > unified.Slowest {
			unified.Slowest = report.Slowest
		}

		allDurations = append(allDurations,
			report.P50, report.P90, report.P95, report.P99)

		for _, path := range report.Paths {
			gp, ok := pathMap[path.URL]
			if !ok {
				gp = &GlobalPathReport{URL: path.URL}
				pathMap[path.URL] = gp
			}
			gp.Hits += path.Hits
			gp.Bytes += path.Bytes
			gp.XX2 += path.XX2
			gp.XX3 += path.XX3
			gp.XX4 += path.XX4
			gp.XX5 += path.XX5
			gp.WaitingRoom += path.WaitingRoom
			gp.Errors += path.Errors
			if path.P50 > gp.P50 {
				gp.P50 = path.P50
			}
			if path.P99 > gp.P99 {
				gp.P99 = path.P99
			}

			planetReport.Paths = append(planetReport.Paths, PlanetPathReport{
				URL:         path.URL,
				Hits:        path.Hits,
				Bytes:       path.Bytes,
				XX2:         path.XX2,
				XX3:         path.XX3,
				XX4:         path.XX4,
				XX5:         path.XX5,
				WaitingRoom: path.WaitingRoom,
				Errors:      path.Errors,
				P50:         path.P50,
				P99:         path.P99,
			})
		}

		unified.Planets = append(unified.Planets, planetReport)
	}

	if len(allDurations) > 0 {
		sort.Slice(allDurations, func(i, j int) bool {
			return allDurations[i] < allDurations[j]
		})
		unified.P50 = percentile(allDurations, 50)
		unified.P90 = percentile(allDurations, 90)
		unified.P95 = percentile(allDurations, 95)
		unified.P99 = percentile(allDurations, 99)
	}

	for _, gp := range pathMap {
		unified.Paths = append(unified.Paths, *gp)
	}
	sort.Slice(unified.Paths, func(i, j int) bool {
		return unified.Paths[i].Hits > unified.Paths[j].Hits
	})

	return s.deliverReport(unified)
}

// readPlanetReport finds and parses the lemmings YAML report file
// from a planet's collected report directory.
func (s *Sovereign) readPlanetReport(planet *Planet) (*lemmingsReportFile, error) {
	reportDir := planet.ReportPath
	if reportDir == "" {
		return nil, fmt.Errorf("planet %d has no report path", planet.ID)
	}

	var reportFile string
	err := filepath.WalkDir(reportDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".yml") &&
			strings.Contains(path, "lemmings.") {
			reportFile = path
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk report dir %s: %w", reportDir, err)
	}
	if reportFile == "" {
		return nil, fmt.Errorf("no lemmings report file found in %s", reportDir)
	}

	contents, err := os.ReadFile(reportFile)
	if err != nil {
		return nil, fmt.Errorf("read report file %s: %w", reportFile, err)
	}

	var report lemmingsReportFile
	if err := yaml.Unmarshal(contents, &report); err != nil {
		return nil, fmt.Errorf("parse report file %s: %w", reportFile, err)
	}

	return &report, nil
}

// deliverReport renders the unified report as markdown and HTML and
// delivers it to all configured save-to destinations.
//
// A delivery failure to one destination is logged as a warning and
// does not block delivery to the others.
func (s *Sovereign) deliverReport(unified *UnifiedReport) error {
	md, err := renderUnifiedMarkdown(unified)
	if err != nil {
		return fmt.Errorf("render unified markdown: %w", err)
	}

	html, err := renderUnifiedHTML(unified)
	if err != nil {
		return fmt.Errorf("render unified html: %w", err)
	}

	filename := fmt.Sprintf("sovereign.%s.%s",
		time.Now().Format("2006.01.02"),
		domainFromURL(s.cfg.Hit),
	)

	for _, dest := range s.cfg.SaveTo {
		dest = strings.TrimSpace(dest)
		if dest == "" {
			continue
		}
		d, err := deliverer.New(dest, s.cfg.SaveToKey, s.cmd,
			deliverer.WithFTPCredentials(s.cfg.FTPUsername, s.cfg.FTPPassword),
		)
		if err != nil {
			fmt.Fprintf(s.out, "  sovereign: warning: invalid save-to destination %q: %v\n",
				dest, err)
			continue
		}
		if err := d.Deliver(s.ctx, filename, md, html); err != nil {
			fmt.Fprintf(s.out, "  sovereign: warning: failed to deliver report to %s: %v\n",
				d.String(), err)
		} else {
			fmt.Fprintf(s.out, "  sovereign: report delivered to %s\n", d.String())
		}
	}

	return nil
}

// percentile computes the Nth percentile of a pre-sorted duration slice.
func percentile(sorted []time.Duration, n int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(n)/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// domainFromURL extracts a filesystem-safe domain string from a URL.
func domainFromURL(hit string) string {
	hit = strings.TrimPrefix(hit, "https://")
	hit = strings.TrimPrefix(hit, "http://")
	hit = strings.Split(hit, "/")[0]
	hit = strings.ReplaceAll(hit, ":", "-")
	return hit
}

// formatBytes renders a byte count as a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// -- Markdown template -------------------------------------------------------

var unifiedMarkdownTemplate = texttemplate.Must(
	texttemplate.New("unified-md").Funcs(texttemplate.FuncMap{
		"formatBytes": formatBytes,
		"formatInt":   func(n int64) string { return fmt.Sprintf("%d", n) },
	}).Parse(`
# sovereign v{{.Version}} unified report

**generated:** {{.GeneratedAt.Format "2006-01-02 15:04:05 MST"}}
**duration:**  {{.Duration.Round 1000000000}}
**target:**    {{.Config.Hit}}
**planets:**   {{.TotalPlanets}} provisioned / {{len .LostPlanets}} lost

## global summary

| metric | value |
|--------|-------|
| total visits    | {{formatInt .TotalVisits}} |
| total bytes     | {{formatBytes .TotalBytes}} |
| waiting room    | {{formatInt .WaitingRoom}} lemmings held |
| errors          | {{formatInt .Errors}} |

## global response codes

| code | count |
|------|-------|
| 2xx  | {{formatInt .Total2xx}} |
| 3xx  | {{formatInt .Total3xx}} |
| 4xx  | {{formatInt .Total4xx}} |
| 5xx  | {{formatInt .Total5xx}} |

## global timing

| percentile | duration |
|------------|----------|
| fastest    | {{.Fastest}} |
| p50        | {{.P50}} |
| p90        | {{.P90}} |
| p95        | {{.P95}} |
| p99        | {{.P99}} |
| slowest    | {{.Slowest}} |

## per-planet breakdown

{{range .Planets}}
### planet {{.Planet.ID}} -- {{.Planet.Region}} ({{.Planet.PublicIPv4}})

| metric | value |
|--------|-------|
| visits | {{formatInt .TotalVisits}} |
| bytes  | {{formatBytes .TotalBytes}} |
| 2xx    | {{formatInt .Total2xx}} |
| 3xx    | {{formatInt .Total3xx}} |
| 4xx    | {{formatInt .Total4xx}} |
| 5xx    | {{formatInt .Total5xx}} |
| p50    | {{.P50}} |
| p99    | {{.P99}} |
{{end}}

{{if .LostPlanets}}
## lost planets

The following planets did not complete successfully and may require
manual intervention to verify termination.

| planet | region | instance id |
|--------|--------|-------------|
{{range .LostPlanets}}| {{.ID}} | {{.Region}} | {{.InstanceID}} |
{{end}}
{{end}}
`))

func renderUnifiedMarkdown(unified *UnifiedReport) (string, error) {
	var buf bytes.Buffer
	if err := unifiedMarkdownTemplate.Execute(&buf, unified); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// -- HTML template -----------------------------------------------------------
//
// html/template is used here instead of strings.Builder with raw
// fmt.Sprintf calls. The template engine automatically escapes all
// user-controlled values (URLs, region names, instance IDs, error
// strings) so that content containing <, >, &, or " cannot break
// the HTML structure or create injection vectors.

var unifiedHTMLTemplate = template.Must(
	template.New("unified-html").Funcs(template.FuncMap{
		"formatBytes": formatBytes,
		"formatInt":   func(n int64) string { return fmt.Sprintf("%d", n) },
		"roundSec":    func(d time.Duration) string { return d.Round(time.Second).String() },
	}).Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>sovereign unified report</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root { --bg:#0f1117; --surface:#1a1d27; --border:#2a2d3a;
  --accent:#6c8ef5; --accent2:#a78bfa; --text:#e2e8f0;
  --muted:#64748b; --success:#34d399; --warn:#f59e0b; --danger:#ef4444; }
body { background:var(--bg); color:var(--text);
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  padding:2rem; max-width:1100px; margin:0 auto; }
h1 { font-size:1.6rem; margin-bottom:0.25rem;
  background:linear-gradient(135deg,var(--accent),var(--accent2));
  -webkit-background-clip:text; -webkit-text-fill-color:transparent;
  background-clip:text; }
h2 { font-size:1.1rem; margin:2rem 0 0.75rem; color:var(--accent);
  border-bottom:1px solid var(--border); padding-bottom:0.4rem; }
h3 { font-size:0.95rem; margin:1.5rem 0 0.5rem; color:var(--accent2); }
.meta { color:var(--muted); font-size:0.85rem; margin-bottom:2rem; }
table { width:100%; border-collapse:collapse; margin-bottom:1rem;
  font-size:0.9rem; }
th { text-align:left; padding:0.5rem 0.75rem; background:var(--surface);
  border:1px solid var(--border); color:var(--muted); font-weight:500;
  font-size:0.8rem; text-transform:uppercase; letter-spacing:0.05em; }
td { padding:0.5rem 0.75rem; border:1px solid var(--border); }
tr:hover td { background:var(--surface); }
.lost { background:rgba(239,68,68,0.1); border:1px solid var(--danger);
  border-radius:6px; padding:0.75rem 1rem; margin-bottom:1rem;
  font-size:0.85rem; color:var(--danger); }
footer { margin-top:3rem; color:var(--muted); font-size:0.8rem;
  border-top:1px solid var(--border); padding-top:1rem; }
a { color:var(--accent); }
</style>
</head>
<body>
<h1>sovereign v{{.Version}} unified report</h1>
<p class="meta">
  {{.GeneratedAt.Format "2006-01-02 15:04:05 MST"}}
  &nbsp;·&nbsp;
  {{roundSec .Duration}}
  &nbsp;·&nbsp;
  <a href="{{.Config.Hit}}">{{.Config.Hit}}</a>
</p>

<h2>global summary</h2>
<table>
  <tr><th>metric</th><th>value</th></tr>
  <tr><td>total visits</td><td>{{formatInt .TotalVisits}}</td></tr>
  <tr><td>total bytes</td><td>{{formatBytes .TotalBytes}}</td></tr>
  <tr><td>waiting room</td><td>{{formatInt .WaitingRoom}} lemmings held</td></tr>
  <tr><td>errors</td><td>{{formatInt .Errors}}</td></tr>
</table>

<h2>global response codes</h2>
<table>
  <tr><th>code</th><th>count</th></tr>
  <tr><td>2xx</td><td>{{formatInt .Total2xx}}</td></tr>
  <tr><td>3xx</td><td>{{formatInt .Total3xx}}</td></tr>
  <tr><td>4xx</td><td>{{formatInt .Total4xx}}</td></tr>
  <tr><td>5xx</td><td>{{formatInt .Total5xx}}</td></tr>
</table>

<h2>global timing</h2>
<table>
  <tr><th>percentile</th><th>duration</th></tr>
  <tr><td>fastest</td><td>{{.Fastest}}</td></tr>
  <tr><td>p50</td><td>{{.P50}}</td></tr>
  <tr><td>p90</td><td>{{.P90}}</td></tr>
  <tr><td>p95</td><td>{{.P95}}</td></tr>
  <tr><td>p99</td><td>{{.P99}}</td></tr>
  <tr><td>slowest</td><td>{{.Slowest}}</td></tr>
</table>

<h2>per-planet breakdown</h2>
{{range .Planets}}
<h3>planet {{.Planet.ID}} &mdash; {{.Planet.Region}} ({{.Planet.PublicIPv4}})</h3>
<table>
  <tr><th>metric</th><th>value</th></tr>
  <tr><td>visits</td><td>{{formatInt .TotalVisits}}</td></tr>
  <tr><td>bytes</td><td>{{formatBytes .TotalBytes}}</td></tr>
  <tr><td>2xx</td><td>{{formatInt .Total2xx}}</td></tr>
  <tr><td>3xx</td><td>{{formatInt .Total3xx}}</td></tr>
  <tr><td>4xx</td><td>{{formatInt .Total4xx}}</td></tr>
  <tr><td>5xx</td><td>{{formatInt .Total5xx}}</td></tr>
  <tr><td>p50</td><td>{{.P50}}</td></tr>
  <tr><td>p99</td><td>{{.P99}}</td></tr>
</table>
{{end}}

{{if .LostPlanets}}
<h2>lost planets</h2>
<div class="lost">The following planets did not complete successfully
and may require manual intervention to verify termination.</div>
<table>
  <tr><th>planet</th><th>region</th><th>instance id</th></tr>
  {{range .LostPlanets}}
  <tr><td>{{.ID}}</td><td>{{.Region}}</td><td>{{.InstanceID}}</td></tr>
  {{end}}
</table>
{{end}}

<footer>generated by sovereign v{{.Version}} &nbsp;·&nbsp; {{.GeneratedAt.Format "2006-01-02 15:04:05 MST"}}</footer>
</body>
</html>
`))

func renderUnifiedHTML(unified *UnifiedReport) (string, error) {
	var buf bytes.Buffer
	if err := unifiedHTMLTemplate.Execute(&buf, unified); err != nil {
		return "", err
	}
	return buf.String(), nil
}
