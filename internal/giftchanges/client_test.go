package giftchanges

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPinnedClientBlocksForeignRedirect(t *testing.T) {
	t.Parallel()
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`["x"]`))
	}))
	defer evil.Close()

	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/gifts", http.StatusFound)
	}))
	defer hop.Close()

	c := &GiftChanges{
		Base: hop.URL,
		HTTP: newPinnedClient(),
	}
	var names []string
	err := c.getJSON(t.Context(), "/gifts", &names)
	if err == nil {
		t.Fatal("expected redirect blocked")
	}
}
