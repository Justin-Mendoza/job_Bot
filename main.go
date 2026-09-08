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
	Company  string
	ID       string
	Title    string
	Location string
	URL      string
}

func (j Job) Key() string { return j.Company + ":" + j.ID }

var (
	titleRe = regexp.MustCompile(`(?i)new\s?grad|university|new college|early career|entry.?level|campus|graduate program|associate software|software engineer i\b|swe i\b|20(26|27)`)
	locRe   = regexp.MustCompile(`(?i)new york|nyc|ny,|, ny\b|remote`)
)

const (
	statePath    = "/data/seen.json"
	maxPerCycle  = 20
	pollInterval = 2 * time.Minute
)

var client = &http.Client{Timeout: 20 * time.Second}

// ---------- adapters ----------

func fetchGreenhouse(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs", c.Slug)
	var body struct {
		Jobs []struct {
			ID          int64  `json:"id"`
			Title       string `json:"title"`
			AbsoluteURL string `json:"absolute_url"`
			Location    struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"jobs"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(body.Jobs))
	for _, j := range body.Jobs {
		out = append(out, Job{c.Name, fmt.Sprint(j.ID), j.Title, j.Location.Name, j.AbsoluteURL})
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
			Location string `json:"location"`
		} `json:"categories"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(body))
	for _, j := range body {
		out = append(out, Job{c.Name, j.ID, j.Text, j.Categories.Location, j.HostedURL})
	}
	return out, nil
}

func fetchAshby(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s", c.Slug)
	var body struct {
		Jobs []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Location string `json:"location"`
			JobURL   string `json:"jobUrl"`
			IsListed bool   `json:"isListed"`
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
		out = append(out, Job{c.Name, j.ID, j.Title, j.Location, j.JobURL})
	}
	return out, nil
}

func fetchRippling(c Company) ([]Job, error) {
	url := fmt.Sprintf("https://ats.rippling.com/api/v2/board/%s/jobs?page=0&pageSize=100", c.Slug)
	var body struct {
		Items []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			URL          string `json:"url"`
			WorkLocation struct {
				Label string `json:"label"`
			} `json:"workLocation"`
		} `json:"items"`
	}
	if err := getJSON(url, &body); err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(body.Items))
	for _, j := range body.Items {
		u := j.URL
		if u == "" {
			u = fmt.Sprintf("https://ats.rippling.com/%s/jobs/%s", c.Slug, j.ID)
		}
		out = append(out, Job{c.Name, j.ID, j.Name, j.WorkLocation.Label, u})
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

	var out []Job
	for offset := 0; offset < 400; offset += 20 {
		payload := map[string]any{
			"appliedFacets": map[string]any{},
			"limit":         20,
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
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("workday %s: status %d", tenant, resp.StatusCode)
		}

		var page struct {
			JobPostings []struct {
				Title         string `json:"title"`
				ExternalPath  string `json:"externalPath"`
				LocationsText string `json:"locationsText"`
			} `json:"jobPostings"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		for _, j := range page.JobPostings {
			out = append(out, Job{c.Name, j.ExternalPath, j.Title, j.LocationsText, base + j.ExternalPath})
		}
		if len(page.JobPostings) < 20 {
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
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
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
	payload := map[string]any{
		"embeds": []map[string]any{{
			"title":       j.Title,
			"url":         j.URL,
			"color":       0x5865F2,
			"description": fmt.Sprintf("**%s** · %s", j.Company, j.Location),
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

func matches(j Job) bool {
	return titleRe.MatchString(j.Title) && locRe.MatchString(j.Location)
}

func cycle(companies []Company, seen map[string]bool, seeded bool, webhook string) bool {
	var fresh []Job
	for _, c := range companies {
		jobs, err := fetch(c)
		if err != nil {
			log.Printf("%s (%s): %v", c.Name, c.ATS, err)
			continue // one bad board never kills the run
		}
		for _, j := range jobs {
			if !matches(j) || seen[j.Key()] {
				continue
			}
			seen[j.Key()] = true
			fresh = append(fresh, j)
		}
		time.Sleep(300 * time.Millisecond) // stagger
	}

	if !seeded {
		log.Printf("first run: seeded %d matching jobs, no alerts sent", len(fresh))
	} else {
		if len(fresh) > maxPerCycle {
			log.Printf("capping %d alerts to %d", len(fresh), maxPerCycle)
			fresh = fresh[:maxPerCycle]
		}
		for _, j := range fresh {
			if err := notify(webhook, j); err != nil {
				log.Printf("notify %s: %v", j.Title, err)
			}
			log.Printf("ALERT %s — %s (%s)", j.Company, j.Title, j.Location)
			time.Sleep(400 * time.Millisecond) // discord rate limit
		}
	}

	saveState(seen)
	return true
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	webhook := os.Getenv("DISCORD_WEBHOOK")
	if webhook == "" {
		log.Fatal("DISCORD_WEBHOOK not set")
	}

	raw, err := os.ReadFile("companies.json")
	if err != nil {
		log.Fatalf("read companies.json: %v", err)
	}
	var companies []Company
	if err := json.Unmarshal(raw, &companies); err != nil {
		log.Fatalf("parse companies.json: %v", err)
	}
	log.Printf("watching %d companies every %s", len(companies), pollInterval)

	seen, seeded := loadState()
	cycle(companies, seen, seeded, webhook)

	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for range t.C {
		cycle(companies, seen, true, webhook)
	}
}
