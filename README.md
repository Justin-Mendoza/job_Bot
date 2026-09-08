# jobwatch — new grad job alerter

Polls company ATS boards every 2 minutes, pings Discord when a new grad / early
career role opens in NYC. Runs on a Fly.io machine for ~$2/month.

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

In `main.go`:

- `titleRe` — what counts as a new grad role
- `locRe` — what counts as NYC

Greenhouse locations are free text (`New York`, `New York, NY`, `US-Remote`,
`NYC`), so keep the location regex loose. Start looser than you think and
tighten once you see what actually comes through.

## 4. Build locally first

```bash
go build -o jobwatch . && DISCORD_WEBHOOK="your-url" ./jobwatch
```

Point `statePath` at `./seen.json` while testing so you don't need `/data`.
First run seeds silently and sends nothing — that's correct. Kill it, delete
`seen.json`, and change `titleRe` to something broad like `engineer` if you
want to confirm alerts actually fire.

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
fly logs                        # should show a cycle every 2 min
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

## Not covered

Bloomberg, Jane Street, Two Sigma, Citadel and similar run custom careers sites
with no public JSON. Options, best first:

1. Check for an embedded blob: `curl <careers-url> | grep -o '__NEXT_DATA__'`.
   If it's there, the full job list is usually in it — write another adapter.
2. Poll `SimplifyJobs/New-Grad-Positions` on GitHub as a catch-all. Lags a few
   hours, covers the long tail, ~30 lines.
3. Just bookmark them and check manually.
