package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerExposesPollMetrics(t *testing.T) {
	ObservePoll("getgems", 120*time.Millisecond, nil)
	SetListings("getgems", 7)
	SetTGSession(true)

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	got := string(body)
	for _, want := range []string{
		"orderbot_poll_seconds",
		"orderbot_listings",
		`market="getgems"`,
		"orderbot_tg_session",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("metrics missing %q\n%s", want, got)
		}
	}
}

func TestObservePollCountsErrors(t *testing.T) {
	ObservePoll("mrkt", time.Millisecond, io.EOF)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "orderbot_poll_errors_total") {
		t.Fatal("missing poll errors")
	}
	if !strings.Contains(string(body), `market="mrkt"`) {
		t.Fatal("missing mrkt label")
	}
}

func TestObservePaperAlert(t *testing.T) {
	ObservePaperAlert("getgems", nil)
	ObservePaperAlert("mrkt", io.EOF)

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	got := rec.Body.String()
	for _, want := range []string{
		"orderbot_paper_alerts_total",
		`market="getgems"`,
		"orderbot_paper_alert_errors_total",
		`market="mrkt"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("metrics missing %q\n%s", want, got)
		}
	}
}
