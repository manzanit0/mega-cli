package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/manzanit0/mega-cli/internal/remote"
	"github.com/manzanit0/mega-cli/internal/session"
	"github.com/manzanit0/mega-cli/internal/ui"
)

// stdin is a shared buffered reader over standard input for prompts.
var stdin = bufio.NewReader(os.Stdin)

// prompt asks for a line of input on stderr.
func prompt(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptSecret asks for input without echoing it when on a terminal.
func promptSecret(label string) (string, error) {
	if !ui.IsTerminal(os.Stdin) {
		return prompt(label)
	}
	fmt.Fprint(os.Stderr, label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}

// valueOr returns v, falling back to env, then to an interactive prompt.
func valueOr(v, env string, ask func() (string, error)) (string, error) {
	if v != "" {
		return v, nil
	}
	if e := os.Getenv(env); e != "" {
		return e, nil
	}
	return ask()
}

// newLoginCmd builds `mega login`.
func newLoginCmd() *cobra.Command {
	var email, mfa string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in and store a session",
		Long: `Log in and store a reusable session (no password is stored).

Email and password are read from --email / MEGA_EMAIL and MEGA_PASSWORD,
or prompted for. A 2FA code is prompted for when the account requires one,
or can be given with --mfa / MEGA_MFA.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogin(email, mfa)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "account email")
	cmd.Flags().StringVar(&mfa, "mfa", "", "2FA code")
	return cmd
}

// runLogin performs the interactive login flow.
func runLogin(email, mfa string) error {
	email, err := valueOr(email, "MEGA_EMAIL", func() (string, error) {
		return prompt("Email: ")
	})
	if err != nil {
		return err
	}
	pass, err := valueOr("", "MEGA_PASSWORD", func() (string, error) {
		return promptSecret("Password: ")
	})
	if err != nil {
		return err
	}
	mfa, _ = valueOr(mfa, "MEGA_MFA", func() (string, error) { return "", nil })
	c, err := remote.Login(email, pass, mfa, options())
	if errors.Is(err, remote.ErrMFARequired) && mfa == "" {
		if mfa, err = prompt("2FA code: "); err != nil {
			return err
		}
		c, err = remote.Login(email, pass, mfa, options())
	}
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if err := session.Save(session.New(email, c.SessionID(), c.MasterKey())); err != nil {
		return err
	}
	logf("Logged in as %s (session stored in %s)", email, session.Location())
	return nil
}

// newLogoutCmd builds `mega logout`.
func newLogoutCmd() *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "End the session on MEGA and remove it locally",
		Long: `End the session on MEGA's servers and remove it from this machine.

If MEGA cannot be reached the local session is kept so you can retry; use
--local to forget it anyway (it then stays valid until revoked from the
MEGA web app).`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runLogout(local)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "only remove the local session")
	return cmd
}

// runLogout revokes the stored session server-side, then deletes it.
func runLogout(local bool) error {
	s, err := session.Load()
	if errors.Is(err, session.ErrNoSession) {
		logf("Not logged in")
		return nil
	}
	if err != nil && !local {
		return fmt.Errorf("%w (use --local to discard it)", err)
	}
	if !local {
		if err := remote.Logout(s.ID); err != nil {
			return fmt.Errorf("%w; local session kept, retry or use --local", err)
		}
	}
	if err := session.Delete(); err != nil {
		return err
	}
	if local {
		logf("Removed local session (still valid on MEGA until revoked)")
		return nil
	}
	logf("Logged out of %s", s.Email)
	return nil
}

// account is the output of `mega whoami`.
type account struct {
	// Email is the account email.
	Email string `json:"email"`
	// Name is the account display name.
	Name string `json:"name"`
	// Handle is the MEGA user handle.
	Handle string `json:"handle"`
}

// newWhoamiCmd builds `mega whoami`.
func newWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the logged-in account",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			u, err := c.User()
			if err != nil {
				return err
			}
			a := account{Email: u.Email, Name: u.Name, Handle: u.U}
			if flags.json {
				return ui.JSON(out, a)
			}
			fmt.Fprintf(out, "%s (%s)\n", a.Email, a.Name)
			return nil
		},
	}
}

// quota is the output of `mega quota`.
type quota struct {
	// Used is the number of bytes stored.
	Used uint64 `json:"used"`
	// Total is the account capacity in bytes.
	Total uint64 `json:"total"`
	// Free is the remaining capacity in bytes.
	Free uint64 `json:"free"`
}

// newQuotaCmd builds `mega quota`.
func newQuotaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quota",
		Short: "Show storage usage",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			c, err := connect()
			if err != nil {
				return err
			}
			r, err := c.Quota()
			if err != nil {
				return err
			}
			q := quota{Used: r.Cstrg, Total: r.Mstrg}
			if q.Total > q.Used {
				q.Free = q.Total - q.Used
			}
			if flags.json {
				return ui.JSON(out, q)
			}
			pct := 0.0
			if q.Total > 0 {
				pct = float64(q.Used) * 100 / float64(q.Total)
			}
			fmt.Fprintf(out, "%s / %s used (%.1f%%), %s free\n",
				ui.Bytes(int64(q.Used)), ui.Bytes(int64(q.Total)), pct,
				ui.Bytes(int64(q.Free)))
			return nil
		},
	}
}
