package remote

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// fakeAPI serves the given responses in order and records requests.
func fakeAPI(t *testing.T, responses ...string) (*atomic.Int32, *atomic.Value) {
	t.Helper()
	var calls atomic.Int32
	var lastSID atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(calls.Add(1)) - 1
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/cs" || string(body) != `[{"a":"sml"}]` {
			t.Errorf("unexpected request %s %q", r.URL.Path, body)
		}
		lastSID.Store(r.URL.Query().Get("sid"))
		if i >= len(responses) {
			i = len(responses) - 1
		}
		io.WriteString(w, responses[i])
	}))
	t.Cleanup(srv.Close)
	t.Setenv("X_MEGA_API_URL", srv.URL)
	return &calls, &lastSID
}

// TestLogoutOK checks a successful logout sends the session ID.
func TestLogoutOK(t *testing.T) {
	calls, sid := fakeAPI(t, "[0]")
	if err := Logout("abc+/="); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || sid.Load() != "abc+/=" {
		t.Fatalf("calls=%d sid=%v", calls.Load(), sid.Load())
	}
}

// TestLogoutExpiredSession checks an already-invalid session is success.
func TestLogoutExpiredSession(t *testing.T) {
	fakeAPI(t, "-15")
	if err := Logout("x"); err != nil {
		t.Fatalf("Logout = %v, want nil", err)
	}
}

// TestLogoutRetriesEAGAIN checks "try again" responses are retried.
func TestLogoutRetriesEAGAIN(t *testing.T) {
	calls, _ := fakeAPI(t, "[-3]", "[0]")
	if err := Logout("x"); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

// TestLogoutError checks other API errors are reported.
func TestLogoutError(t *testing.T) {
	fakeAPI(t, "[-9]")
	if err := Logout("x"); err == nil {
		t.Fatal("expected error")
	}
}
