# jobwatch — new grad job alerter

Polls company ATS boards every 4 minutes, pings Discord when a **software
engineering new grad** role opens in NYC, SF Bay, Chicago, LA, Boston or
Seattle. Runs on a Fly.io machine for ~$2/month.

A full cycle across the 63 boards scans ~10,400 postings in ~60 seconds, using
well under 100 MB of RSS. Against the 4-minute interval that is ~25% duty
cycle, leaving room for roughly 150 boards before it gets tight. Past that,
drop the 300 ms per-company stagger in `cycle()` — at 63 boards it is already
~19s of the 60s, and it is pure sleeping.

**Workday is the exception.** It hard-caps pages at 20 postings (`limit: 50+`
returns zero), so Salesforce alone was 73 requests and 160 seconds — 93% of the
entire cycle. It is also the only ATS exposing no department metadata. It was
dropped for that reason; the adapter is still there if you want it back.

---

## 1. Make the Discord webhook

In Discord: **Server Settings → Integrations → Webhooks → New Webhook**.
Pick a channel, **Copy Webhook URL**. That URL is the whole auth mechanism —
anyone who has it can post to your channel, so never commit it.

## 2. Verify your company slugs

Every slug in `companies.json` needs to be real. Check each one with curl before
deploying — this is the step that saves you debugging later:

```bash
curl -s "https://boards-api.greenhouse.io/v1/boards/stripe/jobs" | head -c 200
curl -s "https://api.lever.co/v0/postings/spotify?mode=json" | head -c 200
curl -s "https://api.ashbyhq.com/posting-api/job-board/ramp" | head -c 200
curl -s "https://ats.rippling.com/api/v2/board/rippling/jobs?pageSize=5" | head -c 200
```

A 404 means the slug is wrong or the company left that ATS. Open their careers
page, watch the browser Network tab (filter: Fetch/XHR), reload, and read the
real endpoint off the request.

For **Workday**, the slug is `tenant/wdN/SiteName`, all three read off the
careers URL:
`salesforce.wd12.myworkdayjobs.com/External_Career_Site`
→ `salesforce/wd12/External_Career_Site`

## 3. Tune the filters

A job has to clear four gates in `matches()` (`main.go`). All four, or no alert:

| Gate | What it does |
|---|---|
| `locOK()` | one of six target metros, or remote **not pinned to another country**. See below |
| `EmpType != "Intern"` | Ashby/Lever report employment type structurally |
| `notRe` | drops interns, recruiters, sales, ops, PhD, and anything senior/staff/lead |
| `earlyRe` + (`sweRe` or `engDeptRe`) | early-career **and** software engineering |

The last one is the important pair. Seniority and role-type are *separate*
axes — filtering on "new grad" alone pulls in `Head of Early Career Recruiting`
and `Business Development Associate, Early Career`. `sweRe` reads the title;
`engDeptRe` reads the board's department field as a fallback for eng titles
that don't spell out the role.

Scored against a live pull of 3,772 postings, this returns exactly one job:
`Stripe · Software Engineer, New Grad · San Francisco, Seattle, New York`.
That is not a bug. Genuine NYC SWE new grad reqs are rare outside the autumn
hiring window; a filter that returns more than a handful is matching noise.

### Why location takes several regexes

`locRe` used to be one pattern with `remote` in it. That silently matched
Affirm's `Remote Spain` and `Remote Poland` postings as NYC — 4 of 6 alerts
were European roles. `locOK()` now splits it:

- `cityRe` or `abbrevRe` matches a target metro → accept, done
- not remote → reject
- remote **and** carries a US marker (`US`, `USA`, `United States`) → accept
- remote and names a foreign country/city → reject
- bare `Remote` with no country → accept (US companies usually mean US-remote)

Metro abbreviations (`NYC`, `NY`, `SF`, `SEA`, `LA`) are matched
**case-sensitively** — boards always write them uppercase, and lowercasing
would match `sea` and `ny` inside ordinary words. `LA` is additionally checked
against a Louisiana guard, since it is also that state's code: `SF, LA, NYC`
is Los Angeles, `New Orleans, LA` is not. Louisiana is rejected *before* the
remote rule too, so `Louisiana - Remote` does not sneak in as generic
US-remote — but a posting listing both (`New Orleans, LA; New York, NY`)
still matches on the target metro.

Built against a 29-case table of real location strings, including
`US-Remote; Canada-Remote` (accept), `Taiwan - Remote` (reject) and
`Palo Alto, CA` (reject — California is not the filter, San Francisco is).

**Where the metadata comes from:**

| ATS | Department | Employment type |
|---|---|---|
| Greenhouse | ✅ via `/departments` (the `/jobs` endpoint has none) | — |
| Ashby | ✅ `department` / `team` | ✅ `FullTime` / `Intern` |
| Lever | ✅ `categories.department` | ✅ `commitment` |
| Rippling | ✅ `department.name` | — |
| Workday | ❌ none exposed — title regex only | — |

Don't trust `employmentType` on its own: Notion currently tags
`Data Science Intern (Winter 2027)` as `FullTime`. `notRe` is the backstop.

## 4. Build locally first

```bash
go build -o jobwatch .
STATE_PATH=./seen.json DISCORD_WEBHOOK="your-url" ./jobwatch
```

`STATE_PATH` overrides the default `/data/seen.json` so you don't need a `/data`
mount locally. First run seeds silently and sends nothing — that's correct.
Expect it to sit quiet for ~3 minutes, then log:

```
first run: scanned 10372 jobs, seeded 2 matches, no alerts sent
```

To prove alerts actually fire, kill it, `rm seen.json`, and loosen `sweRe` to
something broad like `engineer`.

## 5. Deploy to Fly

```bash
fly launch --no-deploy          # say NO to a public IP / HTTP service
fly volumes create jobwatch_data --size 1 --region ewr
fly secrets set DISCORD_WEBHOOK="your-url"
fly deploy
fly logs
```

**Decline the dedicated IPv4.** It costs $2/month and doubles your bill — this
app only makes outbound requests, it serves nothing. If `fly launch` allocated
one anyway: `fly ips list` then `fly ips release <addr>`.

## 6. Confirm it's alive

```bash
fly logs                        # should show a cycle every 4 min
fly status
```

---

## Cost

| Item | Monthly |
|---|---|
| shared-cpu-1x, 256MB, always on | ~$2.02 |
| 1GB volume | $0.15 |
| Bandwidth (<1GB) | ~$0.02 |

Polling frequency doesn't change this — you pay for the machine being on, not
for what it does.

## Gotchas already handled

- First run seeds state instead of dumping every open job into your channel
- One board failing (404, 500, timeout) logs and continues, never kills the cycle
- Alerts capped at 20/cycle so a bad slug can't flood the channel
- State written atomically after every cycle, so a restart doesn't re-alert
- Ashby's `isListed: false` roles are skipped
- Workday paginates and gets a slower request cadence than the rest
- Rippling pages to `totalPages`, Workday to the response's `total` — both used
  to silently truncate at 100 and 400 postings respectively
- Every cycle logs `N jobs scanned, M boards failed`, so a board going 404 is
  visible in `fly logs` instead of just quietly returning nothing

## Adding companies

Cycle time is not the limit — 29 boards ran in 40s. The limit is **slug rot**,
and it is silent. Of 21 candidate slugs probed, 2 returned `200 OK` with
`{"jobs":[]}`: a dead slug is indistinguishable from a quiet day unless you
look. The cycle now logs `WARN ... board returned 0 jobs` for exactly this.

**Counting results is not enough — read them.** Two traps, both hit during setup:

- **Lever returns `[false, false]` for any unknown slug.** Length 2, so a
  count-based check passes. `google`, `apple` and `microsoft` all "worked".
- **Sandbox boards return real JSON with fake jobs.** `greenhouse/linkedin`
  serves 53 postings led by *"Unicorn job post title"*; `lever/linkedin` serves
  24 like *"Anirban jobReq 3 - public"*. Both parse perfectly.

Always eyeball a title before trusting a slug:

```bash
curl -s "https://api.ashbyhq.com/posting-api/job-board/<slug>" | jq -r '.jobs[0].title'
curl -s "https://boards-api.greenhouse.io/v1/boards/<slug>/departments" | jq -r '[.departments[].jobs[]][0].title'
curl -s "https://api.lever.co/v0/postings/<slug>?mode=json" | jq -r '.[0].text'
```

If that prints a plausible job title, the slug is good.

Known-bad, do not re-add: `lever/latch` (2 postings, one titled *"I don't see
the right role"*), `greenhouse/linkedin` and `lever/linkedin` (sandbox data),
`ashby/mercury` and `ashby/deel` (empty boards).

Most large tech companies aren't reachable this way at all — Google, Apple,
Microsoft, Meta, Amazon, Oracle, Uber, Tesla and Netflix returned nothing on
any of the four ATSes. They run custom sites or Workday.

## Not covered

Bloomberg, Two Sigma, Citadel and similar run custom careers sites
with no public JSON. (**Jane Street is on Greenhouse** — slug `janestreet` —
despite what you might assume.) Options, best first:

1. Check for an embedded blob: `curl <careers-url> | grep -o '__NEXT_DATA__'`.
   If it's there, the full job list is usually in it — write another adapter.
2. Poll `SimplifyJobs/New-Grad-Positions` on GitHub as a catch-all. Lags a few
   hours, covers the long tail, ~30 lines.
3. Just bookmark them and check manually.
