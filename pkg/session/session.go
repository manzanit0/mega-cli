// Package session persists and restores MEGA login sessions.
//
// Sessions are stored in the OS keychain (macOS Keychain, Secret Service on
// Linux, Credential Manager on Windows). Setting MEGA_CONFIG_DIR switches to
// a plain file in that directory, for headless machines without a keychain.
package session

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// ErrNoSession is returned when no stored session exists.
var ErrNoSession = errors.New("not logged in: run `mega login`")

const (
	// keyringService is the keychain service name entries are stored under.
	keyringService = "mega-cli"
	// keyringUser is the keychain account name of the session entry.
	keyringUser = "session"
	// fileName is the session file name used by the file store.
	fileName = "session.json"
)

// Session holds the credentials needed to resume a MEGA session.
type Session struct {
	// Email is the account email the session belongs to.
	Email string `json:"email"`
	// ID is the opaque MEGA session identifier.
	ID string `json:"session_id"`
	// MasterKey is the base64-encoded account master key.
	MasterKey string `json:"master_key"`
}

// Key returns the decoded master key.
func (s *Session) Key() ([]byte, error) {
	return base64.StdEncoding.DecodeString(s.MasterKey)
}

// New builds a Session from raw login material.
func New(email, id string, key []byte) *Session {
	return &Session{
		Email:     email,
		ID:        id,
		MasterKey: base64.StdEncoding.EncodeToString(key),
	}
}

// store reads and writes the serialized session.
type store interface {
	// get returns the stored data, or ErrNoSession.
	get() ([]byte, error)
	// set replaces the stored data.
	set(data []byte) error
	// del removes the stored data; missing data is not an error.
	del() error
	// String describes where the session lives, for messages.
	String() string
}

// keychainStore keeps the session in the OS keychain.
type keychainStore struct{}

// get implements store.
func (keychainStore) get() ([]byte, error) {
	s, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("reading OS keychain: %w", err)
	}
	return []byte(s), nil
}

// set implements store.
func (keychainStore) set(data []byte) error {
	if err := keyring.Set(keyringService, keyringUser, string(data)); err != nil {
		return fmt.Errorf("writing OS keychain (set MEGA_CONFIG_DIR to use a file): %w", err)
	}
	return nil
}

// del implements store.
func (keychainStore) del() error {
	err := keyring.Delete(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// String implements store.
func (keychainStore) String() string {
	return "OS keychain"
}

// fileStore keeps the session in an owner-only file.
type fileStore struct {
	// path is the session file location.
	path string
}

// get implements store.
func (f fileStore) get() ([]byte, error) {
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoSession
	}
	return b, err
}

// set implements store.
func (f fileStore) set(data []byte) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o600)
}

// del implements store.
func (f fileStore) del() error {
	err := os.Remove(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// String implements store.
func (f fileStore) String() string {
	return f.path
}

// active returns the store selected by the environment.
func active() store {
	if d := os.Getenv("MEGA_CONFIG_DIR"); d != "" {
		return fileStore{path: filepath.Join(d, fileName)}
	}
	return keychainStore{}
}

// legacyFile returns the file store used by versions before keychain
// support, or false when the config directory is unknown.
func legacyFile() (fileStore, bool) {
	base, err := os.UserConfigDir()
	if err != nil {
		return fileStore{}, false
	}
	return fileStore{path: filepath.Join(base, "mega-cli", fileName)}, true
}

// Location describes where sessions are stored.
func Location() string {
	return active().String()
}

// Load reads the stored session, migrating a legacy session file into the
// keychain when one is found.
func Load() (*Session, error) {
	st := active()
	b, err := st.get()
	if errors.Is(err, ErrNoSession) {
		b, err = migrate(st)
	}
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("corrupt session in %s, run `mega login`: %w", st, err)
	}
	return &s, nil
}

// migrate moves a legacy session file into st and removes the file.
func migrate(st store) ([]byte, error) {
	if _, ok := st.(keychainStore); !ok {
		return nil, ErrNoSession
	}
	legacy, ok := legacyFile()
	if !ok {
		return nil, ErrNoSession
	}
	b, err := legacy.get()
	if err != nil {
		return nil, err
	}
	if err := st.set(b); err != nil {
		return nil, err
	}
	return b, legacy.del()
}

// Save stores the session.
func Save(s *Session) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return active().set(b)
}

// Delete removes the stored session, including any legacy session file.
func Delete() error {
	if err := active().del(); err != nil {
		return err
	}
	if legacy, ok := legacyFile(); ok {
		return legacy.del()
	}
	return nil
}
