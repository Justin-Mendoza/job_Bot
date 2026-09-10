package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// ---------- config ----------

type Company struct {
	Name string `json:"name"`
	ATS  string `json:"ats"`  // greenhouse | lever | ashby | workday | rippling | snap
	Slug string `json:"slug"` // for workday: "tenant/wd12/SiteName"
}

type Job struct {
	Company    string
	ID         string
	Title      string
	Location   string
	URL        string
	Department string // "" when the board doesn't expose one (workday)
}

func (j Job) Key() string { return j.Company + ":" + j.ID }

var (
	// axis 1 — is this early-career? Title first.
	// "associate"/"junior" are deliberately NOT here: on real boards they are
	// mostly seniority-neutral business titles (Associate General Counsel), and
	// adding them let senior non-eng roles through the engDeptRe fallback.
	earlyRe = regexp.MustCompile(`(?i)\bintern\b|internship|\bco.?op\b|\bnew.?grad|\bgrad(uate)?\s+(program|role|opportunit)|university\s+(grad|hire)|new\s+college\s+grad|early.?career|early\s+in\s+career|entry.?level|campus\s+hire|recent\s+grad|class\s+of\s+20\d\d|software engineer\s*(i|1)\b|swe\s*(i|1)\b`)
	// axis 1b — department fallback. Some boards carry the early-career signal
	// only in the department: Coinbase files new grad roles under "Internships &
	// Emerging Talent Positions" with a title as plain as "Software Engineer".
	// Internships in that department are wanted too, so nothing extra is
	// needed to let them through.
	earlyDeptRe = regexp.MustCompile(`(?i)emerging\s+talent|early\s+career|new\s+grad|university\s+(recruit|program|hir)|campus\s+(recruit|hir)`)
	// axis 2 — is this actually software engineering?
	sweRe = regexp.MustCompile(`(?i)software\s+eng|software\s+dev|\bswe\b|backend|back.end|frontend|front.end|full.?stack|infrastructure eng|platform eng|systems eng|security eng|machine learning eng|\bml\s+eng|android eng|ios eng|mobile eng|site reliability|\bsre\b`)
	// axis 2b — department fallback, for eng titles with no role keyword
	engDeptRe = regexp.MustCompile(`(?i)engineer|software|infrastructure|platform|developer|technology`)
	// axis 3 — hard excludes: recruiting, sales, ops, PhD, and anything senior
	notRe = regexp.MustCompile(`(?i)fellowship|\bfellow\b|recruit|talent acquisition|\bsales\b|business development|account exec|marketing|\bphd\b|apprentice|program manager|head of|director|\bmanager\b|principal|\bstaff\b|senior|\bsr\.?\b|\blead\b`)

	// Location is free text and wildly inconsistent. Scope is the US and Canada,
	// so the gate is exclusion-based: accept anything that does not name a place
	// outside them. This inverts the old six-metro allowlist — with a whole
	// continent in scope, enumerating acceptable cities is hopeless, while the
	// set of places to rule out is finite.
	//
	// foreignRe is now load-bearing. It used to run only on remote postings
	// ("Remote Spain"), so a short list was fine; it is now the only location
	// gate, and anything it misses is a false positive.
	foreignRe = regexp.MustCompile(`(?i)\b(` +
		// countries and regions
		`spain|poland|india|ireland|germany|france|uk|united kingdom|england|scotland|wales|` +
		`brazil|mexico|singapore|japan|australia|new zealand|netherlands|portugal|romania|` +
		`israel|italy|sweden|switzerland|china|korea|taiwan|philippines|vietnam|thailand|` +
		`indonesia|malaysia|argentina|colombia|chile|peru|egypt|nigeria|kenya|south africa|` +
		`africa|uae|qatar|saudi|turkey|greece|norway|denmark|finland|belgium|austria|czechia|` +
		`czech|hungary|bulgaria|croatia|serbia|ukraine|estonia|latvia|lithuania|iceland|` +
		`luxembourg|malta|cyprus|hong kong|pakistan|bangladesh|sri lanka|morocco|russia|` +
		`slovakia|slovenia|emea|apac|latam|mena|` +
		// cities. Deliberately omitted because a US or Canadian city shares the
		// name: Cambridge, Birmingham, Manchester, Bristol, Naples, Athens,
		// Lima, Rome, Victoria, Hamilton, Waterloo, Windsor.
		`dublin|berlin|paris|bangalore|bengaluru|tokyo|osaka|sydney|melbourne|brisbane|perth|` +
		`auckland|wellington|amsterdam|lisbon|porto|bucharest|barcelona|madrid|valencia|` +
		`seville|warsaw|krakow|gdansk|wroclaw|tel aviv|jerusalem|haifa|seoul|milan|gurugram|` +
		`gurgaon|mumbai|delhi|noida|hyderabad|pune|chennai|kolkata|manila|jakarta|bangkok|` +
		`kuala lumpur|ho chi minh|hanoi|zurich|geneva|munich|frankfurt|hamburg|cologne|` +
		`stuttgart|dusseldorf|stockholm|copenhagen|oslo|helsinki|brussels|vienna|prague|` +
		`budapest|sofia|belgrade|kyiv|kiev|riga|tallinn|vilnius|cairo|lagos|nairobi|` +
		`johannesburg|cape town|shanghai|shenzhen|beijing|guangzhou|taipei|dubai|abu dhabi|` +
		`doha|riyadh|istanbul|ankara|sao paulo|rio de janeiro|buenos aires|santiago|bogota|` +
		`edinburgh|glasgow|leeds|belfast|cork|galway` +
		`)\b`)
	// "London" is ambiguous: London, Ontario is Canadian. Treat it as foreign
	// unless an Ontario marker sits alongside it — the same shape as the old
	// LA/Louisiana guard, which the six-metro filter needed for exactly this
	// reason.
	londonRe  = regexp.MustCompile(`(?i)\blondon\b`)
	ontarioRe = regexp.MustCompile(`(?i)\bontario\b|,\s*on\b`)
)

const (
	// Ashby ships whole job descriptions inline; OpenAI's board alone is 12.2 MiB.
	// Cap generously, but fail loudly instead of handing json.Unmarshal a
	// truncated body and getting "unexpected end of JSON input".
	maxBody = 64 << 20

	// Raised from 20 when internships were let in. Turning them on takes the
	// steady-state match count from 15 to ~38, and the overflow is *discarded*,
	// not deferred — at 20 the first cycle after that change would have marked
	// ~18 real internships seen without ever alerting on them.
	maxPerCycle  = 50
	pollInterval = 4 * time.Minute // a full cycle is 58-105s at 83 boards
)

var statePath = envOr("STATE_PATH", "/data/seen.json")

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var client = &http.Client{Timeout: 20 * time.Second}

// ---------- adapters ----------

// Greenhouse's /jobs endpoint omits departments entirely, so go through
// /departments instead — same job IDs, same shape, plus the department name.
func fetchGreenhouse(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/departments", c.Slug)
	var body struct {
		Departments []struct {
			Name string `json:"name"`
			Jobs []struct {
				ID          int64  `json:"id"`
				Title       string `json:"title"`
				AbsoluteURL string `json:"absolute_url"`
				Location    struct {
					Name string `json:"name"`
				} `json:"location"`
			} `json:"jobs"`
		} `json:"departments"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	var out []Job
	seen := map[int64]bool{} // a job can be listed under more than one department
	for _, d := range body.Departments {
		for _, j := range d.Jobs {
			if seen[j.ID] {
				continue
			}
			seen[j.ID] = true
			out = append(out, Job{
				Company:    c.Name,
				ID:         fmt.Sprint(j.ID),
				Title:      j.Title,
				Location:   j.Location.Name,
				URL:        j.AbsoluteURL,
				Department: d.Name,
			})
		}
	}
	return out, nil
}

func fetchLever(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json", c.Slug)
	var body []struct {
		ID         string `json:"id"`
		Text       string `json:"text"`
		HostedURL  string `json:"hostedUrl"`
		Categories struct {
			Location   string `json:"location"`
			Department string `json:"department"`
			Team       string `json:"team"`
			Commitment string `json:"commitment"`
		} `json:"categories"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(body))
	for _, j := range body {
		dept := j.Categories.Department
		if dept == "" {
			dept = j.Categories.Team
		}
		out = append(out, Job{
			Company:    c.Name,
			ID:         j.ID,
			Title:      j.Text,
			Location:   j.Categories.Location,
			URL:        j.HostedURL,
			Department: dept,
		})
	}
	return out, nil
}

func fetchAshby(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s", c.Slug)
	var body struct {
		Jobs []struct {
			ID             string `json:"id"`
			Title          string `json:"title"`
			Location       string `json:"location"`
			JobURL         string `json:"jobUrl"`
			IsListed       bool   `json:"isListed"`
			Department     string `json:"department"`
			Team           string `json:"team"`
			EmploymentType string `json:"employmentType"`
		} `json:"jobs"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		if !j.IsListed {
			continue
		}
		dept := j.Department
		if dept == "" {
			dept = j.Team
		}
		out = append(out, Job{
			Company:    c.Name,
			ID:         j.ID,
			Title:      j.Title,
			Location:   j.Location,
			URL:        j.JobURL,
			Department: dept,
		})
	}
	return out, nil
}

func fetchRippling(c Company) ([]Job, error) {
	var out []Job
	for page := 0; page < 50; page++ { // hard stop; real boards are ~7 pages
		url := fmt.Sprintf("https://ats.rippling.com/api/v2/board/%s/jobs?page=%d&pageSize=100", c.Slug, page)
		var body struct {
			TotalPages int `json:"totalPages"`
			Items      []struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				URL          string `json:"url"`
				WorkLocation struct {
					Label string `json:"label"`
				} `json:"workLocation"`
				Locations []struct {
					Label string `json:"label"`
				} `json:"locations"`
				Department struct {
					Name string `json:"name"`
				} `json:"department"`
			} `json:"items"`
		}
		if err := getJSON(url, &body); err != nil {
			return nil, err
		}
		for _, j := range body.Items {
			u := j.URL
			if u == "" {
				u = fmt.Sprintf("https://ats.rippling.com/%s/jobs/%s", c.Slug, j.ID)
			}
			loc := j.WorkLocation.Label
			if loc == "" {
				parts := make([]string, 0, len(j.Locations))
				for _, l := range j.Locations {
					parts = append(parts, l.Label)
				}
				loc = strings.Join(parts, "; ")
			}
			out = append(out, Job{
				Company:    c.Name,
				ID:         j.ID,
				Title:      j.Name,
				Location:   loc,
				URL:        u,
				Department: j.Department.Name,
			})
		}
		if len(body.Items) == 0 || page+1 >= body.TotalPages {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	return out, nil
}

// Snap runs Workday underneath (wd1.myworkdaysite.com — note: a different host
// and path shape than fetchWorkday handles), but its careers site publishes the
// whole board as one flat JSON feed, department and location attached. One
// request instead of nine paginated Workday POSTs, so go through the front door.
// Slug is the careers host, e.g. "careers.snap.com".
func fetchSnap(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://%s/api/jobs", c.Slug)
	var body struct {
		Body []struct {
			Source struct {
				ID              string `json:"id"`
				Title           string `json:"title"`
				AbsoluteURL     string `json:"absolute_url"`
				Departments     string `json:"departments"`
				EmploymentType  string `json:"employment_type"`
				PrimaryLocation string `json:"primary_location"`
				Offices         []struct {
					Location string `json:"location"`
				} `json:"offices"`
			} `json:"_source"`
		} `json:"body"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(body.Body))
	for _, h := range body.Body {
		j := h.Source
		// A role open in several offices lists them all; primary_location is a
		// bare city ("New York") and only names one of them.
		loc := j.PrimaryLocation
		if len(j.Offices) > 0 {
			parts := make([]string, 0, len(j.Offices))
			for _, o := range j.Offices {
				parts = append(parts, o.Location)
			}
			loc = strings.Join(parts, "; ")
		}
		out = append(out, Job{
			Company:    c.Name,
			ID:         j.ID,
			Title:      j.Title,
			Location:   loc,
			URL:        j.AbsoluteURL,
			Department: j.Departments,
		})
	}
	return out, nil
}

func fetchWorkday(c Company) ([]Job, error) {
	parts := strings.Split(c.Slug, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("workday slug must be tenant/wdN/site, got %q", c.Slug)
	}
	tenant, dc, site := parts[0], parts[1], parts[2]
	base := fmt.Sprintf("https://%s.%s.myworkdayjobs.com", tenant, dc)
	endpoint := fmt.Sprintf("%s/wday/cxs/%s/%s/jobs", base, tenant, site)

	const pageSize = 20
	const hardCap = 3000 // don't walk a 20k-posting board forever

	var out []Job
	total := hardCap
	for offset := 0; offset < total && offset < hardCap; offset += pageSize {
		payload := map[string]any{
			"appliedFacets": map[string]any{},
			"limit":         pageSize,
			"offset":        offset,
			"searchText":    "",
		}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("workday %s: status %d", tenant, resp.StatusCode)
		}

		var page struct {
			Total       int `json:"total"`
			JobPostings []struct {
				Title         string `json:"title"`
				ExternalPath  string `json:"externalPath"`
				LocationsText string `json:"locationsText"`
			} `json:"jobPostings"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		if page.Total > 0 {
			total = page.Total // the board tells us how far to go
		}
		for _, j := range page.JobPostings {
			out = append(out, Job{
				Company:  c.Name,
				ID:       j.ExternalPath,
				Title:    j.Title,
				Location: j.LocationsText,
				URL:      base + j.ExternalPath,
			})
		}
		if len(page.JobPostings) < pageSize {
			break
		}
		time.Sleep(1500 * time.Millisecond) // workday rate-limits harder
	}
	return out, nil
}

func fetch(c Company) ([]Job, error) {
	switch c.ATS {
	case "greenhouse":
		return fetchGreenhouse(c)
	case "lever":
		return fetchLever(c)
	case "ashby":
		return fetchAshby(c)
	case "rippling":
		return fetchRippling(c)
	case "workday":
		return fetchWorkday(c)
	case "snap":
		return fetchSnap(c)
	}
	return nil, fmt.Errorf("unknown ats %q", c.ATS)
}

func getJSON(url string, v any) error {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "jobwatch/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return err
	}
	if len(raw) > maxBody {
		return fmt.Errorf("body exceeds %d MiB cap", maxBody>>20)
	}
	return json.Unmarshal(raw, v)
}

// ---------- state ----------

func loadState() (map[string]bool, bool) {
	b, err := os.ReadFile(statePath)
	if err != nil {
		return map[string]bool{}, false // first run
	}
	var keys []string
	if err := json.Unmarshal(b, &keys); err != nil {
		log.Printf("state corrupt, reseeding: %v", err)
		return map[string]bool{}, false
	}
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m, true
}

func saveState(m map[string]bool) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	b, _ := json.Marshal(keys)
	tmp := statePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("write state: %v", err)
		return
	}
	if err := os.Rename(tmp, statePath); err != nil {
		log.Printf("rename state: %v", err)
	}
}

// ---------- notify ----------

// redact strips the webhook out of an error message, keeping the useful part.
func redact(err error, secret string) error {
	if err == nil {
		return nil
	}
	return errors.New(strings.ReplaceAll(err.Error(), secret, "<webhook>"))
}

func notify(webhook string, j Job) error {
	desc := fmt.Sprintf("**%s** · %s", j.Company, j.Location)
	if j.Department != "" {
		desc += fmt.Sprintf("\n%s", j.Department)
	}
	// The embed title is already a hyperlink via "url", but it does not read as
	// one — same colour as plain text on most themes, so it gets missed. Repeat
	// it as an explicit masked link so there is something obviously clickable.
	desc += fmt.Sprintf("\n\n**[Apply →](%s)**", j.URL)
	// No @everyone. It was fine at ~15 new-grad matches; with internships the
	// steady state is ~38 and a burst pings the whole server for roles most of
	// it does not care about. allowed_mentions stays, set to nothing, so a job
	// title that happens to contain "@everyone" can never ping either.
	payload := map[string]any{
		"allowed_mentions": map[string]any{
			"parse": []string{},
		},
		"embeds": []map[string]any{{
			"title":       j.Title,
			"url":         j.URL,
			"color":       0x5865F2,
			"description": desc,
		}},
	}
	b, _ := json.Marshal(payload)
	resp, err := client.Post(webhook, "application/json", bytes.NewReader(b))
	if err != nil {
		// net/http errors embed the full URL, and the webhook URL *is* the
		// credential — anyone holding it can post to the channel. Never let it
		// reach the logs.
		return redact(err, webhook)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord status %d", resp.StatusCode)
	}
	return nil
}

// ---------- main ----------

// matches keeps a job only if it is an early-career software engineering role
// in the US or Canada. Structured board metadata wins where it exists; the
// title regexes are the fallback for boards that expose nothing (workday) and
// the backstop for boards where companies fill the fields in wrong (Notion
// tags some interns as FullTime).

// locOK accepts a posting anywhere in the US or Canada. Boards list multiple
// offices separated by ";" or "|", and a role open in both Toronto and London
// is still one you can take, so a single in-scope segment carries the posting.
// An empty location is accepted: every board here belongs to a US or Canadian
// company, so "unspecified" is far more often in scope than not.
func locOK(loc string) bool {
	for _, seg := range strings.FieldsFunc(loc, func(r rune) bool { return r == ';' || r == '|' }) {
		if inScope(seg) {
			return true
		}
	}
	// FieldsFunc returns nothing for an empty or separator-only string.
	return strings.TrimSpace(loc) == ""
}

func inScope(seg string) bool {
	if londonRe.MatchString(seg) && ontarioRe.MatchString(seg) {
		return true // London, Ontario
	}
	if londonRe.MatchString(seg) {
		return false
	}
	return !foreignRe.MatchString(seg)
}

func matches(j Job) bool {
	if !locOK(j.Location) {
		return false
	}
	if notRe.MatchString(j.Title) {
		return false
	}
	if !earlyRe.MatchString(j.Title) && !earlyDeptRe.MatchString(j.Department) {
		return false
	}
	return sweRe.MatchString(j.Title) || engDeptRe.MatchString(j.Department)
}

func cycle(companies []Company, seen map[string]bool, seeded bool, webhook string) {
	var fresh []Job
	scanned, failed := 0, 0
	// live holds every posting seen on a board this cycle; prunable holds the
	// companies whose boards answered well enough to be trusted about what is
	// no longer on them. See prune().
	live := make(map[string]bool, 12000)
	prunable := make(map[string]bool, len(companies))
	for _, c := range companies {
		jobs, err := fetch(c)
		if err != nil {
			failed++
			log.Printf("%s (%s): %v", c.Name, c.ATS, err)
			continue // one bad board never kills the run
		}
		if len(jobs) == 0 {
			// 200 OK with an empty list means the slug is stale or the company
			// left that ATS. Silent at 8 companies, invisible at 60.
			log.Printf("WARN %s (%s): board returned 0 jobs — check the slug", c.Name, c.ATS)
		} else {
			prunable[c.Name] = true
		}
		for _, j := range jobs {
			live[j.Key()] = true
		}
		scanned += len(jobs)
		for _, j := range jobs {
			if !matches(j) || seen[j.Key()] {
				continue
			}
			fresh = append(fresh, j)
		}
		time.Sleep(300 * time.Millisecond) // stagger
	}

	// First run records everything and alerts on nothing, so you don't get the
	// entire back catalogue dumped into the channel.
	if !seeded {
		for _, j := range fresh {
			seen[j.Key()] = true
		}
		log.Printf("first run: scanned %d jobs, seeded %d matches, no alerts sent", scanned, len(fresh))
		saveState(seen)
		return
	}

	// Flood guard: a bad filter change could match hundreds. Drop the overflow
	// outright rather than dribbling it into the channel for the next hour.
	if len(fresh) > maxPerCycle {
		for _, j := range fresh[maxPerCycle:] {
			seen[j.Key()] = true
		}
		log.Printf("capping %d alerts to %d, dropping %d", len(fresh), maxPerCycle, len(fresh)-maxPerCycle)
		fresh = fresh[:maxPerCycle]
	}

	sent, retry := 0, 0
	for _, j := range fresh {
		if err := notify(webhook, j); err != nil {
			// Leave it unseen. A 429 or a brief Discord outage should delay an
			// alert, never lose it.
			retry++
			log.Printf("notify %s: %v (will retry next cycle)", j.Title, err)
			continue
		}
		seen[j.Key()] = true
		sent++
		log.Printf("ALERT %s — %s (%s)", j.Company, j.Title, j.Location)
		time.Sleep(400 * time.Millisecond) // discord rate limit
	}
	if n := prune(seen, live, prunable); n > 0 {
		log.Printf("pruned %d closed postings from state", n)
	}
	log.Printf("cycle: %d jobs scanned, %d boards failed, %d sent, %d retrying", scanned, failed, sent, retry)

	saveState(seen)
}

// prune forgets postings that have left their board, so a role that is closed
// and later reopened alerts again instead of being suppressed forever by a
// state file that only ever grows.
//
// It only touches companies in prunable — boards that fetched successfully AND
// returned a non-empty list. A failed or empty board must never flush its keys:
// a stale slug or a transient 404 would drop every posting that company has,
// and the whole back catalogue would re-alert on the next cycle that works.
//
// A posting that vanishes for one cycle and comes back (board flakiness,
// pagination glitch) costs one duplicate alert. That is the right way round —
// this trades a rare duplicate for never silently missing a reopened role.
func prune(seen, live, prunable map[string]bool) int {
	n := 0
	for k := range seen {
		company, _, ok := strings.Cut(k, ":")
		if !ok || !prunable[company] || live[k] {
			continue
		}
		delete(seen, k)
		n++
	}
	return n
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	// TrimSpace is not cosmetic: a secret set from a file or a shell heredoc
	// picks up a trailing newline, and net/url rejects the URL as containing a
	// control character. Every single delivery then fails while the board scan
	// keeps reporting success, so the bot looks healthy and silently sends
	// nothing.
	webhook := strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK"))
	if webhook == "" {
		log.Fatal("DISCORD_WEBHOOK not set")
	}
	// Fail loudly at startup rather than once per matched job forever.
	if _, err := url.Parse(webhook); err != nil {
		log.Fatalf("DISCORD_WEBHOOK is not a valid URL: %v", redact(err, webhook))
	}

	// TEST_ALERT=1 sends one synthetic alert and exits — proves the webhook URL
	// works and the embed renders, without waiting for a real posting.
	if os.Getenv("TEST_ALERT") != "" {
		j := Job{
			Company:    "jobwatch",
			ID:         "test",
			Title:      "Test Alert — Software Engineer, New Grad",
			Location:   "New York, NY",
			URL:        "https://github.com",
			Department: "Engineering",
		}
		if err := notify(webhook, j); err != nil {
			log.Fatalf("TEST FAILED: %v", err)
		}
		log.Print("TEST OK — alert delivered, check your Discord channel")
		return
	}

	raw, err := os.ReadFile("companies.json")
	if err != nil {
		log.Fatalf("read companies.json: %v", err)
	}
	var companies []Company
	if err := json.Unmarshal(raw, &companies); err != nil {
		log.Fatalf("parse companies.json: %v", err)
	}
	log.Printf("watching %d companies every %s (state: %s)", len(companies), pollInterval, statePath)

	seen, seeded := loadState()
	cycle(companies, seen, seeded, webhook)

	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for range t.C {
		cycle(companies, seen, true, webhook)
	}
}
