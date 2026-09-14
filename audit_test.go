package main

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
)

// Opt-in read-only audit: fetches every configured board, never sends alerts
// or touches seen state. JOBWATCH_AUDIT names the output JSON report.
func TestLiveBoardAudit(t *testing.T) {
	path := os.Getenv("JOBWATCH_AUDIT")
	if path == "" {
		t.Skip("set JOBWATCH_AUDIT to run live board checks")
	}
	raw, err := os.ReadFile("companies.json")
	if err != nil {
		t.Fatal(err)
	}
	var companies []Company
	if err := json.Unmarshal(raw, &companies); err != nil {
		t.Fatal(err)
	}
	type result struct {
		Company Company
		Error   string
		Jobs    []Job
		Matches int
	}
	results := make([]result, len(companies))
	var wg sync.WaitGroup
	limit := make(chan struct{}, 6)
	for i, c := range companies {
		wg.Add(1)
		go func(i int, c Company) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()
			jobs, err := fetch(c)
			r := result{Company: c, Jobs: jobs}
			if err != nil {
				r.Error = err.Error()
			}
			for _, j := range jobs {
				if matches(j) {
					r.Matches++
				}
			}
			results[i] = r
		}(i, c)
	}
	wg.Wait()
	raw, err = json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		t.Logf("%s: %d jobs, %d matches, error=%s", r.Company.Name, len(r.Jobs), r.Matches, r.Error)
		if r.Error != "" || len(r.Jobs) == 0 {
			t.Errorf("%s: board failed or returned no listed jobs", r.Company.Name)
		}
	}
}
