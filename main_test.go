package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPriorityMatches(t *testing.T) {
	for _, tc := range []struct {
		company, title, location string
		want                     bool
	}{
		{"Harvey", "Software Engineer, Backend", "New York", true},
		{"Cohere", "Member of Technical Staff, Training Performance Engineer", "Toronto", true},
		{"Cohere", "Senior Member of Technical Staff", "Toronto", false},
		{"Cohere", "Staff Software Engineer", "Toronto", false},
		{"Harvey", "Senior Software Engineer", "New York", false},
		{"Harvey", "Software Engineer", "London", false},
		{"Harvey", "Software Engineer", "London; New York", true},
		{"Harvey", "Legal Engineer", "New York", false},
		{"Other", "Software Engineer", "Toronto", false},
		{"Other", "Software Engineer, New Grad", "Toronto", true},
		{"Other", "Software Engineer Intern", "Toronto", true},
		{"Justworks", "Associate Software Engineer, Expenses", "Toronto, Canada", true},
		{"Vestwell", "Associate, Software Engineer", "Austin, TX", true},
		{"AlayaCare", "Junior Fullstack Developer (Python)", "Montréal, Quebec, Canada", true},
		{"Trulioo", "Junior Software Engineer", "San Diego", true},
		{"PointClickCare", "Canada- Jr Software Engineer (SRE)", "Remote or Mississauga", true},
		{"Whoop", "Android Engineer I", "Boston, MA", true},
		{"D2L", "Software Test Developer - New Graduate", "Kitchener, Remote Canada", true},
		{"Other", "Graduate Software Engineer", "Toronto", true},
		{"Other", "Software Developer 1", "Toronto", true},
		{"Other", "Android Engineer II", "Toronto", false},
		{"Other", "Senior Android Engineer I", "Toronto", false},
		{"Other", "Associate Financial Analyst", "Toronto", false},
		{"Other", "Associate General Counsel", "Toronto", false},
		{"Other", "Junior Software Engineer", "Mexico City, Mexico", false},
	} {
		t.Run(tc.company+tc.title+tc.location, func(t *testing.T) {
			if got := matches(Job{Company: tc.company, Title: tc.title, Location: tc.location}); got != tc.want {
				t.Fatalf("matches = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEarlyCareerMetadata(t *testing.T) {
	for _, tc := range []struct {
		job  Job
		want bool
	}{
		{Job{Title: "Software Engineer", Department: "Engineering; University Recruiting"}, true},
		{Job{Title: "Software Engineer", Department: "Internships / Co-ops; Research"}, true},
		{Job{Title: "Software Engineer", EmploymentType: "Intern"}, true},
		{Job{Title: "Cohort 0", Department: "Engineering", EmploymentType: "Intern"}, true},
		{Job{Title: "Software Engineer Intern", EmploymentType: "FullTime"}, true},
		{Job{Title: "Marketing Intern", EmploymentType: "Intern"}, false},
		{Job{Title: "Associate Financial Analyst", Department: "Engineering"}, false},
		{Job{Title: "Senior Software Engineer", Department: "University Recruiting"}, false},
	} {
		if got := matches(tc.job); got != tc.want {
			t.Errorf("%+v: got %v want %v", tc.job, got, tc.want)
		}
	}
}

func TestAdapterMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "departments"):
			w.Write([]byte(`{"departments":[{"name":"Engineering","jobs":[{"id":1,"title":"Software Engineer"}]},{"name":"University Recruiting","jobs":[{"id":1,"title":"Software Engineer"}]}]}`))
		case strings.Contains(r.URL.Path, "postings"):
			w.Write([]byte(`[{"id":"1","text":"Software Engineer","categories":{"location":"London","allLocations":["London","Toronto"],"department":"Engineering","team":"University Recruiting","commitment":"Intern"}}]`))
		default:
			w.Write([]byte(`{"jobs":[{"id":"1","title":"Software Engineer","isListed":true,"department":"Engineering","team":"University Recruiting","employmentType":"Intern"}]}`))
		}
	}))
	defer server.Close()
	old := client
	client = &http.Client{Transport: rewriteTransport{server.Listener.Addr().String()}}
	t.Cleanup(func() { client = old })
	for _, ats := range []string{"greenhouse", "lever", "ashby"} {
		jobs, err := fetch(Company{Name: "Example", ATS: ats, Slug: "example"})
		if err != nil || len(jobs) != 1 {
			t.Fatalf("%s: jobs=%v err=%v", ats, jobs, err)
		}
		if !strings.Contains(jobs[0].Department, "University Recruiting") || !matches(jobs[0]) {
			t.Errorf("%s lost metadata: %+v", ats, jobs[0])
		}
	}
}

type rewriteTransport struct{ target string }

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = r.target
	return http.DefaultTransport.RoundTrip(copy)
}

func TestAshbySecondaryLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jobs":[{"id":"1","title":"Software Engineer","location":"London","secondaryLocations":[{"location":"Toronto"}],"isListed":true},{"id":"2","title":"Software Engineer","isListed":false}]}`))
	}))
	defer server.Close()
	old := client
	client = &http.Client{Transport: rewriteTransport{server.Listener.Addr().String()}}
	t.Cleanup(func() { client = old })
	jobs, err := fetchAshby(Company{Name: "Harvey", Slug: "harvey"})
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs=%v err=%v", jobs, err)
	}
	if !matches(jobs[0]) {
		t.Fatal("secondary Canadian location should match")
	}
}
