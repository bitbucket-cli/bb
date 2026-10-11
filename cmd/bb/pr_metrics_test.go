package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	api "github.com/bitbucket-cli/bb/pkg/bitbucket"
)

func TestRunPRMetrics(t *testing.T) {
	defer func() { prNewAPIClient = api.NewClient; prMetricsLimit = 50 }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"values":[
			{"created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-02T00:00:00Z"},
			{"created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-01T06:00:00Z"}
		]}`))
	}))
	defer server.Close()

	prNewAPIClient = mockAPIClientFactory(server)
	if err := runPRMetrics(prMetricsCmd, []string{"myws/myrepo"}); err != nil {
		t.Errorf("runPRMetrics error: %v", err)
	}
}

func TestMedianDuration(t *testing.T) {
	odd := []time.Duration{3 * time.Hour, time.Hour, 2 * time.Hour}
	if got := medianDuration(odd); got != 2*time.Hour {
		t.Errorf("median(odd) = %v, want 2h", got)
	}
	even := []time.Duration{4 * time.Hour, 2 * time.Hour}
	if got := medianDuration(even); got != 3*time.Hour {
		t.Errorf("median(even) = %v, want 3h", got)
	}
}

func TestRoundDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Minute: "30m",
		90 * time.Minute: "1.5h",
		36 * time.Hour:   "1.5d",
	}
	for d, want := range cases {
		if got := roundDuration(d); got != want {
			t.Errorf("roundDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
