package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/manzanit0/mega-cli/internal/ui"
	"github.com/manzanit0/mega-cli/pkg/auth"
	"github.com/manzanit0/mega-cli/pkg/remote"
	"github.com/manzanit0/mega-cli/pkg/session"
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
	_, err = auth.Login(email, pass, mfa, options())
	if errors.Is(err, remote.ErrMFARequired) && mfa == "" {
		if mfa, err = prompt("2FA code: "); err != nil {
			return err
		}
		_, err = auth.Login(email, pass, mfa, options())
	}
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
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
	email, err := auth.Logout(local)
	if errors.Is(err, auth.ErrNotLoggedIn) {
		logf("Not logged in")
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w; local session kept, retry or use --local", err)
	}
	if local {
		logf("Removed local session (still valid on MEGA until revoked)")
		return nil
	}
	logf("Logged out of %s", email)
	return nil
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
			a, err := c.Account()
			if err != nil {
				return err
			}
			if flags.json {
				return ui.JSON(out, a)
			}
			fmt.Fprintf(out, "%s (%s)\n", a.Email, a.Name)
			return nil
		},
	}
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
			q, err := c.Storage()
			if err != nil {
				return err
			}
			if flags.json {
				return ui.JSON(out, q)
			}
			fmt.Fprintf(out, "%s / %s used (%.1f%%), %s free\n",
				ui.Bytes(int64(q.Used)), ui.Bytes(int64(q.Total)), q.Percent(),
				ui.Bytes(int64(q.Free)))
			return nil
		},
	}
}
