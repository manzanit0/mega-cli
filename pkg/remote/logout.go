package remote

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"

	mega "github.com/t3rm1n4l/go-mega"
)

const (
	// errAgain is MEGA's "try again" API error code.
	errAgain = -3
	// errSID is MEGA's "invalid or expired session" API error code.
	errSID = -15
	// logoutAttempts bounds retries of the logout request.
	logoutAttempts = 3
)

// apiBaseURL mirrors go-mega's endpoint selection, including the
// X_MEGA_API_URL override.
func apiBaseURL() string {
	if u := os.Getenv("X_MEGA_API_URL"); u != "" {
		return u
	}
	return mega.API_URL
}

// Logout invalidates a session on MEGA's servers using the "sml" command,
// the same call the official SDK issues. A session that is already invalid
// is treated as logged out. go-mega has no logout, so the request is made
// directly without fetching the filesystem.
func Logout(sessionID string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	var err error
	for i := 0; i < logoutAttempts; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i) * time.Second)
		}
		var retry bool
		retry, err = logoutOnce(client, sessionID)
		if !retry {
			return err
		}
	}
	return err
}

// logoutOnce sends a single logout request and reports whether it should
// be retried.
func logoutOnce(client *http.Client, sessionID string) (bool, error) {
	seq, err := rand.Int(rand.Reader, big.NewInt(1<<32))
	if err != nil {
		return false, err
	}
	u := fmt.Sprintf("%s/cs?id=%d&sid=%s", apiBaseURL(), seq, url.QueryEscape(sessionID))
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBufferString(`[{"a":"sml"}]`))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return true, fmt.Errorf("logout request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return true, err
	}
	if resp.StatusCode >= 500 {
		return true, fmt.Errorf("logout: HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("logout: HTTP %d", resp.StatusCode)
	}
	return interpretLogout(body)
}

// interpretLogout parses a logout response body, which is either a bare
// error code or a one-element array such as "[0]".
func interpretLogout(body []byte) (bool, error) {
	var codes []int
	if err := json.Unmarshal(body, &codes); err != nil || len(codes) == 0 {
		var code int
		if err := json.Unmarshal(body, &code); err != nil {
			return false, fmt.Errorf("logout: unexpected response %q", body)
		}
		codes = []int{code}
	}
	switch codes[0] {
	case 0, errSID:
		return false, nil
	case errAgain:
		return true, fmt.Errorf("logout: MEGA asked to try again")
	default:
		return false, fmt.Errorf("logout: MEGA API error %d", codes[0])
	}
}
