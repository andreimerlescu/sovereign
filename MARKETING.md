# Sovereign as a Service

> Know your limits before the market does.

**[Sign up at lemmingsaas.com](https://lemmingsaas.com)**

---

## The Problem Every Builder Faces Before Launch

You built something. Maybe it was with Claude, Gemini, Grok, or your
own fine-tuned model. Maybe it took three months. Maybe it took three
weeks because the AI did most of the heavy lifting. Either way, you
have a product. You have users waiting. You have a launch date.

And you have no idea if it will hold.

This is not a hypothetical problem. It is the problem that kills
launches. Not bad code. Not bad design. Not bad marketing. Load.
Unexpected, asymmetric, geographically distributed load from real
users who all showed up at once because you did your job well and
someone important shared your link.

The question is not whether your application can serve one user.
The question is whether it can serve ten thousand users from six
continents simultaneously, and whether you will find out the answer
before or after your launch day.

Sovereign answers that question before launch day.

---

## What Sovereign Actually Does

Sovereign provisions real cloud infrastructure -- actual EC2 instances,
actual Droplets, actual OVH compute nodes -- in multiple regions
simultaneously. It installs lemmings on each of those instances. It
runs a coordinated load test across all of them at the same time.
It collects the results and delivers a unified report.

Then it destroys everything, down to the last instance, and tells you
exactly what it cost.

You are not simulating load from a laptop. You are generating real
HTTP traffic from real servers in real geographic regions, the same
way real users generate it. The latency is real. The connection pools
are real. The TCP handshakes are real. The CDN edge behavior is real.

A typical five-planet sovereign run against a production target
generates hundreds of thousands of concurrent lemmings hitting your
application from us-east-1, us-west-1, eu-west-1, ap-southeast-1,
and sa-east-1 at the same time. You see exactly how your application
performs under that load, broken down by region, by path, by response
code, and by timing percentile.

---

## Who This Is For

### The AI-native builder

You shipped your MVP in two weeks with Claude writing 80% of the code.
The architecture works. The demo is clean. You have a waitlist of three
thousand people and a launch date in ten days.

You have never load tested anything in your life because every project
you shipped before this one was too small to matter.

This one is different. Sovereign tells you whether your infrastructure
matches your ambition before you find out the hard way.

### The indie developer preparing a launch

You have been building alone for six months. You have a Barron's
feature dropping next Tuesday. You do not know if your servers can
handle the traffic that comes from that kind of press.

You have one shot at the first impression. Sovereign lets you rehearse
it. You run the test, you read the report, you make the fixes, you
run it again. By the time Tuesday arrives you know exactly what your
ceiling is and you have either raised it or accepted it and planned
accordingly.

### The small team shipping a B2B product

You have ten engineers and a Fortune 500 pilot launching in six weeks.
Your lead architect is confident the system will hold. Your CTO wants
proof. Your customer's security team wants a performance report before
they sign the MSA.

Sovereign generates that proof. The unified report -- with per-region
breakdown, per-path timing percentiles, response code distribution,
and projected infrastructure cost -- is the document you hand to
their procurement team.

### The platform team at a growing startup

You have product-market fit. You have Series A money. You have a
growth team that is about to run a campaign that could 10x your daily
active users over a weekend.

You need to know whether your infrastructure scales horizontally the
way your architecture diagrams say it does. Sovereign finds out.

### The enterprise DevOps team

You are migrating a monolith to microservices. The new architecture
looks correct on paper. You need to validate it under realistic
multi-region load before you flip the DNS and cut over production.

Sovereign gives you that validation without touching production.

---

## Real Applications

### Pre-launch rehearsal

A developer building a Go-based API gateway for financial data is
preparing for a press release. The application serves market data to
mobile clients. The press release is expected to drive 50,000 new
signups in 48 hours.

The developer runs sovereign with 5 planets, 100 terrain groups per
planet, and 50 lemmings per terrain group. That is 25,000 concurrent
simulated users hitting the API from five AWS regions simultaneously
for 30 minutes.

The report reveals that the /quotes endpoint begins returning 429s
after the connection pool on the primary database exhausts at roughly
8,000 concurrent users. The developer has three weeks to fix it. They
double the connection pool size, add a read replica, and add a Redis
cache layer in front of the most expensive queries. They run sovereign
again. The 429s are gone. The P99 latency at 25,000 concurrent users
is 340ms. The launch goes ahead on schedule.

### Regional performance validation

A SaaS company is expanding from US-only to global. Their application
has been running on us-east-1 for two years. They are launching in
Europe and Asia Pacific next quarter and they need to know whether
their current architecture will serve European and Asian users with
acceptable latency.

They run sovereign with three planets in eu-west-1, ap-southeast-1,
and us-east-1 and direct all traffic at a single production-equivalent
staging environment. The report shows that European users see P99
latency of 180ms and Asian users see P99 latency of 620ms -- well
above their 400ms SLA target.

The team adds CloudFront with edge caching for static assets and moves
session storage to a globally replicated DynamoDB table. A second
sovereign run confirms P99 latency drops to 210ms for Asian users.
They proceed with the global launch.

### AI product scale validation

An AI startup built a document analysis product on top of Claude's
API. The product ingests PDFs, extracts structured data, and returns
JSON responses. They are onboarding their first enterprise customer
who expects to process 10,000 documents per day during business hours.

The team has no idea whether their queue-based processing architecture
will handle the burst load when the customer uploads a batch of 2,000
documents at 9am. They use sovereign to simulate that burst pattern
with a custom lemmings plan that models the upload and processing
request distribution.

The test reveals that their SQS consumer fleet does not scale fast
enough during the initial burst and document processing latency spikes
to 45 minutes for the last documents in a large batch. They tune the
auto-scaling policy and rerun. Processing latency for the tail of the
batch drops to 8 minutes. The customer SLA requires 15 minutes.
They sign the contract.

### Pre-acquisition due diligence

A private equity firm is acquiring a SaaS company and needs to
validate the target's infrastructure claims. The target claims their
platform supports 50,000 concurrent users. The acquiring firm runs
sovereign against the target's staging environment as part of
technical due diligence.

The test reveals the platform degrades significantly above 12,000
concurrent users. The acquiring firm uses this finding to negotiate
a reduced purchase price that accounts for the infrastructure
remediation cost. The target company's actual capacity was less than
a quarter of what they claimed.

### Gaming backend pre-launch

A mobile game studio is launching a new title with expected day-one
installs of 500,000 based on their app store pre-registration numbers.
Their matchmaking and leaderboard backend needs to handle the surge
that comes in the first hour after the App Store feature goes live.

They use sovereign with 10 planets across multiple regions to simulate
50,000 concurrent game clients making matchmaking requests and
leaderboard queries. The test reveals a deadlock in their leaderboard
write path that only appears under high concurrency. The fix takes
four hours to write and deploy. Without sovereign, they would have
discovered it live in front of half a million players.

---

## Pricing

Sovereign as a Service is available at
**[lemmingsaas.com](https://lemmingsaas.com)**.

| Tier | Price | Who it is for |
|------|-------|---------------|
| Individual | **$99 / developer / year** | Solo builders, indie developers, consultants |
| Team | **$70 / developer / year** | Teams of 10 or more developers |
| Enterprise | **$50 / developer / year** | Organizations with 100 or more developers |

Your subscription grants access to the sovereign portal where you
configure and run sovereign tests, review historical reports, manage
cloud spend, and invite team members.

Cloud infrastructure costs are billed separately and transparently
to your account at cost. Sovereign shows you the projected cost before
provisioning any infrastructure and requires confirmation before
spending a dollar. A typical five-planet 30-minute test on t3.medium
instances costs between $0.50 and $2.00 in cloud compute. You control
the instance type, the region selection, the number of planets, and
the test duration.

There are no hidden markups on cloud costs. You pay what AWS,
DigitalOcean, or OVH charges for the compute time sovereign uses
on your behalf. Sovereign is the orchestration layer. The cloud
bill is yours and it is always shown to you in advance.

---

## Why the Numbers Make Sense

A production outage during a major launch costs more than $99.

A failed enterprise pilot because your API could not handle the
demo load costs more than $99.

A press cycle that turns negative because your site went down when
Barron's, TechCrunch, or Product Hunt sent you traffic costs more
than $99.

One sovereign test run that catches a database connection pool
exhaustion issue three weeks before your launch costs between one
and two dollars in cloud compute and whatever fraction of your
annual subscription that test represents.

The math is not complicated.

---

## The Technical Foundation

Sovereign is built in Go. It is the orchestration layer above
lemmings, which is also built in Go. Both tools were built by a
engineer with 17 years of experience spanning Cisco Systems, Oracle
America, Warner Bros. Games, and independent consulting engagements
that have delivered hundreds of thousands of dollars in measurable
infrastructure savings.

The architecture is designed around one principle: every event that
happens during a sovereign run is written to a crash-proof audit
ledger on disk before the next event begins. If sovereign is
interrupted mid-run -- by a signal, a power failure, a network
partition -- it resumes exactly where it stopped. Planets that were
already provisioned are not re-provisioned. Results that were already
collected are not re-collected. Cloud resources are not abandoned.

This is not a testing tool that was built in a weekend. It is a tool
that was built because the person who built it needed it and could
not find anything that did what it does at a price that made sense
for a solo developer preparing a launch.

---

## Supported Cloud Providers

Sovereign provisions infrastructure on the provider you choose.
Switch providers per run or mix providers across planets.

    AWS              EC2 instances, any region, Rocky Linux 9
    DigitalOcean     Droplets, global regions, Rocky Linux 9
    OVH              OpenStack compute, European and global regions

All three providers are fully implemented. No partial support.
No experimental flags. If the provider is listed it works.

---

## The Report You Get

Every sovereign run produces a unified report delivered to wherever
you configure: local filesystem, S3, a remote server via SCP, or an
FTP server. The report is available in both markdown and HTML.

The HTML report is self-contained with no external dependencies.
You can open it in any browser, email it to a stakeholder, or attach
it to a JIRA ticket without any additional tooling.

The report includes:

- total request count and total bytes transferred across all planets
- per-region breakdown showing geographic performance differences
- per-path breakdown showing which endpoints are slowest under load
- response code distribution (2xx, 3xx, 4xx, 5xx) globally and per planet
- timing percentiles: P50, P90, P95, P99, fastest, and slowest
- waiting room metrics showing how many lemmings were held back by
  your rate limiting or queuing infrastructure
- lost planet summary with instance IDs if any planets failed,
  so you can inspect them manually if needed
- projected infrastructure cost for the run

---

## Getting Started

**[Sign up at lemmingsaas.com](https://lemmingsaas.com)**

Individual access is $99 per year. You can run your first test the
same day you sign up. No sales call required. No contract negotiation.
No procurement process. Sign up, configure your cloud credentials in
the portal, point sovereign at your application, and run.

If you are evaluating sovereign for a team of 10 or more, the team
tier applies automatically when you add seats. Volume pricing for
enterprise organizations of 100 or more is applied at the account
level. Contact us through the portal for custom arrangements for
very large organizations or for academic and non-profit pricing.

---

## The Name

A lemming is a sovereign's agent. It is born with a predetermined
destiny. It follows its plan. It reports back. It has no free will
of its own.

The sovereign is the engineer -- the one who decides the plan,
commands the terrain, and reads the report.

You are the sovereign. Your application is the terrain. The question
is whether your terrain holds when the lemmings arrive.

**Find out before your users do.**

**[lemmingsaas.com](https://lemmingsaas.com)**
