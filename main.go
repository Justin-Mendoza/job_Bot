package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// ---------- config ----------

type Company struct {
	Name string `json:"name"`
	ATS  string `json:"ats"`  // greenhouse | lever | ashby | workday | rippling
	Slug string `json:"slug"` // for workday: "tenant/wd12/SiteName"
}

type Job struct {
	Company    string
	ID         string
	Title      string
	Location   string
	URL        string
	Department string // "" when the board doesn't expose one (workday)
	EmpType    string // ashby only: FullTime | Intern | Contract | Temporary
}

func (j Job) Key() string { return j.Company + ":" + j.ID }

var (
	// axis 1 — is this early-career?
	earlyRe = regexp.MustCompile(`(?i)\bnew.?grad|\bgrad(uate)?\s+(program|role|opportunit)|university\s+(grad|hire)|new\s+college\s+grad|early.?career|entry.?level|campus\s+hire|software engineer\s*(i|1)\b|swe\s*(i|1)\b`)
	// axis 2 — is this actually software engineering?
	sweRe = regexp.MustCompile(`(?i)software\s+eng|software\s+dev|\bswe\b|backend|back.end|frontend|front.end|full.?stack|infrastructure eng|platform eng|systems eng|security eng|machine learning eng|\bml\s+eng|android eng|ios eng|mobile eng|site reliability|\bsre\b`)
	// axis 2b — department fallback, for eng titles with no role keyword
	engDeptRe = regexp.MustCompile(`(?i)engineer|software|infrastructure|platform|developer|technology`)
	// axis 3 — hard excludes: interns, recruiting, sales, ops, and anything senior
	notRe = regexp.MustCompile(`(?i)\bintern\b|internship|\bco.?op\b|recruit|talent acquisition|\bsales\b|business development|account exec|marketing|\bphd\b|apprentice|program manager|head of|director|\bmanager\b|principal|\bstaff\b|senior|\bsr\.?\b|\blead\b`)

	// Location is free text and wildly inconsistent, so it takes three regexes.
	// "remote" alone is not enough: Affirm posts "Remote Spain" and "Remote
	// Poland", which the old single-regex version happily matched as NYC.
	nycRe     = regexp.MustCompile(`(?i)new york|nyc|ny,|, ny\b`)
	remoteRe  = regexp.MustCompile(`(?i)\bremote\b`)
	usRe      = regexp.MustCompile(`(?i)\b(us|usa|u\.s\.|united states)\b`)
	foreignRe = regexp.MustCompile(`(?i)\b(spain|poland|india|ireland|germany|france|uk|united kingdom|canada|brazil|mexico|singapore|japan|australia|netherlands|portugal|romania|israel|italy|sweden|switzerland|china|korea|taiwan|emea|apac|latam|london|dublin|berlin|paris|toronto|vancouver|bangalore|tokyo|sydney|amsterdam|lisbon|bucharest|barcelona|madrid|warsaw|tel aviv|seoul|milan|gurugram)\b`)
)

const (
	// Ashby ships whole job descriptions inline; OpenAI's board alone is 12.2 MiB.
	// Cap generously, but fail loudly instead of handing json.Unmarshal a
	// truncated body and getting "unexpected end of JSON input".
	maxBody = 64 << 20

	maxPerCycle  = 20
	pollInterval = 4 * time.Minute // a full cycle is ~60s at 63 boards
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
			EmpType:    j.Categories.Commitment,
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
			EmpType:    j.EmploymentType,
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

func notify(webhook string, j Job) error {
	desc := fmt.Sprintf("**%s** · %s", j.Company, j.Location)
	if j.Department != "" {
		desc += fmt.Sprintf("\n%s", j.Department)
	}
	payload := map[string]any{
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
		return err
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
// in NYC. Structured board metadata wins where it exists; the title regexes
// are the fallback for boards that expose nothing (workday) and the backstop
// for boards where companies fill the fields in wrong (Notion tags some
// interns as FullTime).
// locOK accepts NYC outright. A remote role counts only when it is not
// pinned to another country — "Remote, US" yes, "Remote Spain" no, bare
// "Remote" yes (US companies usually mean US-remote).
func locOK(loc string) bool {
	if nycRe.MatchString(loc) {
		return true
	}
	if !remoteRe.MatchString(loc) {
		return false
	}
	return usRe.MatchString(loc) || !foreignRe.MatchString(loc)
}

func matches(j Job) bool {
	if !locOK(j.Location) {
		return false
	}
	if strings.EqualFold(j.EmpType, "intern") {
		return false
	}
	if notRe.MatchString(j.Title) {
		return false
	}
	if !earlyRe.MatchString(j.Title) {
		return false
	}
	return sweRe.MatchString(j.Title) || engDeptRe.MatchString(j.Department)
}

func cycle(companies []Company, seen map[string]bool, seeded bool, webhook string) {
	var fresh []Job
	scanned, failed := 0, 0
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
	log.Printf("cycle: %d jobs scanned, %d boards failed, %d sent, %d retrying", scanned, failed, sent, retry)

	saveState(seen)
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	webhook := os.Getenv("DISCORD_WEBHOOK")
	if webhook == "" {
		log.Fatal("DISCORD_WEBHOOK not set")
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
