// Package auth manages the login lifecycle: logging in, persisting the
// session, reconnecting from it, and logging out.
package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/manzanit0/mega-cli/pkg/remote"
	"github.com/manzanit0/mega-cli/pkg/session"
)

const (
	// EnvEmail names the environment variable holding the account email.
	EnvEmail = "MEGA_EMAIL"
	// EnvPassword names the environment variable holding the password.
	EnvPassword = "MEGA_PASSWORD"
	// EnvMFA names the environment variable holding a 2FA code.
	EnvMFA = "MEGA_MFA"
)

// ErrNotLoggedIn is returned when no session or credentials are available.
var ErrNotLoggedIn = session.ErrNoSession

// Login authenticates and stores a reusable session; the password is never
// stored. It returns remote.ErrMFARequired when the account needs a 2FA
// code and mfa is empty, so callers can ask for one and call Login again.
func Login(email, password, mfa string, o remote.Options) (*remote.Client, error) {
	c, err := remote.Login(email, password, mfa, o)
	if err != nil {
		return nil, err
	}
	if err := session.Save(session.New(email, c.SessionID(), c.MasterKey())); err != nil {
		return nil, err
	}
	return c, nil
}

// Connect returns a logged-in client built from the stored session or, when
// there is none, from the EnvEmail and EnvPassword environment variables
// (and optionally EnvMFA). Environment logins are not persisted. It returns
// ErrNotLoggedIn when neither is available.
func Connect(o remote.Options) (*remote.Client, error) {
	s, err := session.Load()
	if errors.Is(err, session.ErrNoSession) {
		email, pass := os.Getenv(EnvEmail), os.Getenv(EnvPassword)
		if email != "" && pass != "" {
			return remote.Login(email, pass, os.Getenv(EnvMFA), o)
		}
	}
	if err != nil {
		return nil, err
	}
	key, err := s.Key()
	if err != nil {
		return nil, fmt.Errorf("corrupt session: %w", err)
	}
	return remote.Resume(s.ID, key, o)
}

// Logout revokes the stored session on MEGA's servers and deletes it
// locally, returning the account email. If MEGA cannot be reached the local
// session is kept so the call can be retried. With local set, only the
// local session is removed and it stays valid on MEGA until revoked
// elsewhere. It returns ErrNotLoggedIn when no session is stored.
func Logout(local bool) (string, error) {
	s, err := session.Load()
	if errors.Is(err, session.ErrNoSession) {
		return "", ErrNotLoggedIn
	}
	email := ""
	switch {
	case err == nil:
		email = s.Email
	case !local:
		return "", err
	}
	if !local {
		if err := remote.Logout(s.ID); err != nil {
			return email, err
		}
	}
	return email, session.Delete()
}
