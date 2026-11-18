package session

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

// isolate points the user config directory at a temp dir and returns it.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	t.Setenv("MEGA_CONFIG_DIR", "")
	return home
}

// roundTrip saves, loads and deletes a session through the active store.
func roundTrip(t *testing.T) {
	t.Helper()
	if _, err := Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Load on empty store = %v, want ErrNoSession", err)
	}
	key := []byte{1, 2, 3, 4}
	if err := Save(New("a@b.c", "sid", key)); err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Key()
	if err != nil || !bytes.Equal(got, key) || s.ID != "sid" || s.Email != "a@b.c" {
		t.Fatalf("loaded %+v, key %v, err %v", s, got, err)
	}
	if err := Delete(); err != nil {
		t.Fatal(err)
	}
	if err := Delete(); err != nil {
		t.Fatalf("second Delete = %v", err)
	}
	if _, err := Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Load after Delete = %v, want ErrNoSession", err)
	}
}

// TestKeychainRoundTrip checks the default keychain-backed store.
func TestKeychainRoundTrip(t *testing.T) {
	isolate(t)
	keyring.MockInit()
	if Location() != "OS keychain" {
		t.Fatalf("Location = %q", Location())
	}
	roundTrip(t)
}

// TestFileRoundTrip checks the MEGA_CONFIG_DIR file store and its perms.
func TestFileRoundTrip(t *testing.T) {
	isolate(t)
	keyring.MockInitWithError(errors.New("keychain must not be used"))
	dir := t.TempDir()
	t.Setenv("MEGA_CONFIG_DIR", dir)
	if err := Save(New("a@b.c", "sid", []byte{1})); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, fileName))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("session file perms = %v, %v", fi, err)
	}
	if err := Delete(); err != nil {
		t.Fatal(err)
	}
	roundTrip(t)
}

// TestMigratesLegacyFile checks a pre-keychain session file is imported
// into the keychain and removed from disk.
func TestMigratesLegacyFile(t *testing.T) {
	isolate(t)
	keyring.MockInit()
	legacy, ok := legacyFile()
	if !ok {
		t.Fatal("no legacy path")
	}
	if err := legacy.set([]byte(`{"email":"a@b.c","session_id":"old","master_key":"AQ=="}`)); err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil || s.ID != "old" {
		t.Fatalf("Load = %+v, %v", s, err)
	}
	if _, err := os.Stat(legacy.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy file still present: %v", err)
	}
	if _, err := keyring.Get(keyringService, keyringUser); err != nil {
		t.Fatalf("session not in keychain: %v", err)
	}
}

// TestKeychainErrorSurfaces checks keychain failures are not reported as
// a missing session.
func TestKeychainErrorSurfaces(t *testing.T) {
	isolate(t)
	keyring.MockInitWithError(errors.New("locked"))
	if _, err := Load(); err == nil || errors.Is(err, ErrNoSession) {
		t.Fatalf("Load = %v, want keychain error", err)
	}
}
