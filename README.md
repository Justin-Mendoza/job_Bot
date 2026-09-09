# jobwatch — new grad job alerter

Polls company ATS boards every 4 minutes, pings Discord when a **software
engineering new grad** role opens in NYC, SF Bay, Chicago, LA, Boston or
Seattle. Runs on a Fly.io machine for ~$2/month.

A full cycle across the 68 boards scans ~11,200 postings in ~35 seconds, using
well under 100 MB of RSS. Against the 4-minute interval that is ~15% duty
cycle, leaving room for roughly 150 boards before it gets tight. Past that,
drop the 300 ms per-company stagger in `cycle()` — at 68 boards it is already
~20s of the 35s, and it is pure sleeping.

**Workday is the exception.** It hard-caps pages at 20 postings (`limit: 50+`
returns zero), so Salesforce alone was 73 requests and 160 seconds — 93% of the
entire cycle. It is also the only ATS exposing no department metadata. It was
dropped for that reason; the adapter is still there if you want it back.

**Snap is a one-off.** It sits on Workday underneath, but on a different host
and path shape (`wd1.myworkdaysite.com/recruiting/...`) than `fetchWorkday`
builds. Its careers site publishes the whole board as one flat JSON feed with
department and location attached, so `ats: "snap"` goes through that instead —
one request rather than nine paginated Workday POSTs. The slug is the careers
host, not a board name.

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
| `notRe` | drops interns, fellowships, recruiters, sales, ops, PhD, and anything senior/staff/lead |
| (`earlyRe` or `earlyDeptRe`) + (`sweRe` or `engDeptRe`) | early-career **and** software engineering |

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

**Check `fly.toml` after `fly launch`.** It will try to add an `[http_service]`
block with `auto_stop_machines = 'stop'` and `min_machines_running = 0`. That
is fatal here: this app never receives HTTP traffic, so Fly stops the machine
shortly after boot, the poll ticker dies, and you get no alerts and no error —
it just looks like a quiet job market. Delete the whole block. The correct
`fly.toml` has no `[http_service]` and no `[[services]]`, only `[build]`,
`[[mounts]]` and `[[vm]]`.

## 6. Confirm it's alive

```bash
fly logs                        # should show a cycle every 4 min
fly status
```

The first cycle logs `first run: ... no alerts sent` and sends nothing. That is
correct — it records the currently-open matches so you don't get pinged for
jobs that have been up for weeks. Real alerts start on cycle two.

## 7. Auto-deploy from GitHub

`.github/workflows/fly-deploy.yml` redeploys on every push to `main`, so the
watchlist can be extended by editing `companies.json` in the GitHub web UI —
no local checkout, no `fly deploy`. Takes about 90 seconds end to end.

One-time setup (the token is piped straight into GitHub so it never lands in
your shell history or terminal):

```bash
fly tokens create deploy -x 8760h | tail -1 | tr -d '\n' \
  | gh secret set FLY_API_TOKEN --repo <you>/job_Bot
```

The workflow gates deploy behind a test job — `companies.json` must parse and
every entry must name a known ATS, plus `gofmt`, `go vet` and `go build`. A
typo like `"ats": "greehouse"` fails CI instead of shipping a bot that
silently skips that company.

Redeploying is safe: `seen.json` lives on the Fly volume, not in the image, so
a restart never re-alerts jobs already sent. Worst case you miss one poll.

Path filters mean README-only edits don't trigger a deploy. To deploy without
pushing, use the workflow's **Run workflow** button (`workflow_dispatch`).

---

## Cost

| Item | Monthly |
|---|---|
| shared-cpu-1x, 256MB, always on | ~$2.02 |
| 1GB volume | $0.15 |
| Outbound bandwidth (~1.4 GiB) | ~$0.03 |
| **Total** | **~$2.20** |

Measured at 68 boards on a 4-minute interval: 54 MiB pulled per cycle, about
572 GiB/month — but **inbound transfer is free on Fly**, and outbound is only
request headers plus the occasional Discord POST.

Neither polling frequency nor company count moves this much; you pay for the
machine being on, not for what it does. Going from 8 to 68 boards changed the
bill by pennies. The one thing that would double it is a dedicated IPv4.

## Gotchas already handled

- First run seeds state instead of dumping every open job into your channel
- One board failing (404, 500, timeout) logs and continues, never kills the cycle
- Alerts capped at 20/cycle so a bad slug can't flood the channel
- State written atomically after every cycle, so a restart doesn't re-alert
- Ashby's `isListed: false` roles are skipped
- The Discord embed carries an explicit **Apply →** link. The title is already
  a hyperlink via the embed's `url`, but it renders the same colour as plain
  text on most themes and gets missed, so the link is repeated visibly
- Workday paginates and gets a slower request cadence than the rest
- Rippling pages to `totalPages`, Workday to the response's `total` — both used
  to silently truncate at 100 and 400 postings respectively
- Every cycle logs `N jobs scanned, M boards failed`, so a board going 404 is
  visible in `fly logs` instead of just quietly returning nothing

### Snap levels its titles

Snap writes seniority as `Level N`, not as words: *"Software Engineer, Level 3"*
is the new grad role, *"Level 4"* and up are not. `earlyRe` matches neither, and
`notRe` catches none of them either, so **Snap alerts on nothing** — it is
watched for its board, not for its hits.

**This is deliberate, not a gap to fix.** Adding `level\s*3\b` to `earlyRe` was
considered and rejected: Level 3 is not consistently entry-level across Snap's
teams, so the rule would pull in mid-level roles for the sake of one board. If
that ever changes, that one-line addition is all it takes.

### What the early-career gate actually keys on

`earlyRe` reads the **title** and nothing else, matching one of: `new grad`,
`graduate program/role`, `university graduate/hire`, `new college grad`,
`early career`, `early in career`, `entry level`, `campus hire`, `recent grad`,
`class of 20NN`, or a trailing level marker (`Software Engineer I`, `SWE 1` —
the trailing `\b` correctly rejects `II` and `III`).

`earlyDeptRe` is the fallback for boards that carry the signal only in the
**department**: Coinbase files new grad roles under *"Internships & Emerging
Talent Positions"* with titles as plain as `Software Engineer`. The intern gates
still apply, so the internships sitting in that same department stay filtered.

**What it still misses, by design.** Measured across all 68 boards: of 704
in-metro, non-senior engineering roles, only 13 pass. The other 691 are mostly
bare `Software Engineer`, `Software Engineer, Backend`, `Site Reliability
Engineer` — titles where seniority lives in the description, not the name.
Widening further means accepting false positives, because an unqualified
"Software Engineer" is genuinely ambiguous. `associate` and `junior` were tried
and rejected for this reason: on real boards they are mostly seniority-neutral
business titles that slip through via the `engDeptRe` department fallback.

Two live examples of why the gate stays narrow:

- **Snap** writes seniority as `Level N`, so nothing on its board matches (see
  below).
- **Fellowships** looked early-career and were not — DoorDash's *"AI Research
  Fellowship (Summer and Fall 2026)"* entered through the department fallback
  before `fellowship|\bfellow\b` was added to `notRe`. ("Fellow" is also a
  *senior* IC title at some companies, so excluding it cuts both ways.)

## Adding companies

Cycle time is not the limit — 68 boards scan 11,200 postings in ~35s, well
inside the 4-minute interval. The limit is **slug rot**,
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
curl -s "https://<careers-host>/api/jobs" | jq -r '.body[0]._source.title'   # snap
```

If that prints a plausible job title, the slug is good.

Known-bad, do not re-add: `lever/latch` (2 postings, one titled *"I don't see
the right role"*), `greenhouse/linkedin` and `lever/linkedin` (sandbox data),
`ashby/mercury` and `ashby/deel` (empty boards), `ashby/bumble` (empty — the
live Bumble board is `ashby/bumbleinc`).

Most large tech companies aren't reachable this way at all — Google, Apple,
Microsoft, Meta, Amazon, Oracle, Uber, Tesla and Netflix returned nothing on
any of the four ATSes. They run custom sites or Workday.

## Not covered

Bloomberg, Two Sigma, Citadel and similar run custom careers sites
with no public JSON. (**Jane Street is on Greenhouse** — slug `janestreet` —
despite what you might assume.)

**Hinge** is the same story and was probed thoroughly: 404 on greenhouse, lever
and ashby under every plausible slug, and `hinge.co/careers` renders its
openings from Sanity CMS with no ATS host anywhere in the page or its JS
bundles. `lever/matchgroup` is a live board (22 NYC roles) but it is Match
Group corporate and Hyperconnect, not Hinge. Nothing to point an adapter at.

Options, best first:

1. Check for an embedded blob: `curl <careers-url> | grep -o '__NEXT_DATA__'`.
   If it's there, the full job list is usually in it — write another adapter.
2. Poll `SimplifyJobs/New-Grad-Positions` on GitHub as a catch-all. Lags a few
   hours, covers the long tail, ~30 lines.
3. Just bookmark them and check manually.
