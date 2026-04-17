package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ErrNotFound is returned when a state file does not exist yet.
var ErrNotFound = errors.New("state file not found")

// writeSecure atomically writes data as JSON to path with mode 0600.
// It writes to a temp file alongside the target and renames to avoid
// partial writes leaving a corrupt file.
func writeSecure(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(v); encErr != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("encode JSON: %w", encErr)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename to target: %w", err)
	}
	return nil
}

// readJSON reads path and decodes JSON into v.
// Returns ErrNotFound if the file does not exist.
func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// SaveAccount writes acc to the account.json file in d.Config.
// If force is false and the file already exists, it returns an error.
func SaveAccount(d Dirs, acc AccountState, force bool) error {
	if !force {
		if _, err := os.Stat(d.AccountFile()); err == nil {
			return fmt.Errorf("account state already exists at %s; use --force to overwrite", d.AccountFile())
		}
	}
	acc.Normalize()
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return writeSecure(d.AccountFile(), acc)
}

// LoadAccount reads account.json from d.Config.
// Returns ErrNotFound if no account has been registered yet.
func LoadAccount(d Dirs) (AccountState, error) {
	var acc AccountState
	err := readJSON(d.AccountFile(), &acc)
	if err != nil {
		return acc, err
	}
	acc.Normalize()
	return acc, nil
}

// SaveLastGoodAccount writes acc to the daemon's last-known-good snapshot file.
func SaveLastGoodAccount(d Dirs, acc AccountState) error {
	acc.Normalize()
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return writeSecure(d.LastGoodAccountFile(), acc)
}

// LoadLastGoodAccount reads the daemon's last-known-good account snapshot.
func LoadLastGoodAccount(d Dirs) (AccountState, error) {
	var acc AccountState
	err := readJSON(d.LastGoodAccountFile(), &acc)
	if err != nil {
		return acc, err
	}
	acc.Normalize()
	return acc, nil
}

// SaveSettings writes s to settings.json in d.Config.
func SaveSettings(d Dirs, s Settings) error {
	s.Normalize()
	if err := os.MkdirAll(d.Config, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return writeSecure(d.SettingsFile(), s)
}

// LoadSettings reads settings.json from d.Config.
// Returns defaults with ErrNotFound if the file does not exist.
func LoadSettings(d Dirs) (Settings, error) {
	s := DefaultSettings()
	err := readJSON(d.SettingsFile(), &s)
	if errors.Is(err, ErrNotFound) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, err
	}
	s.Normalize()
	return s, nil
}

// SaveRuntime writes rt to runtime.json in d.Runtime.
func SaveRuntime(d Dirs, rt RuntimeState) error {
	rt.Normalize()
	if err := os.MkdirAll(d.Runtime, 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	return writeSecure(d.RuntimeFile(), rt)
}

// LoadRuntime reads runtime.json from d.Runtime.
// Returns ErrNotFound if no backend has been started.
func LoadRuntime(d Dirs) (RuntimeState, error) {
	var rt RuntimeState
	err := readJSON(d.RuntimeFile(), &rt)
	if err != nil {
		return rt, err
	}
	rt.Normalize()
	return rt, nil
}

// ClearRuntime removes the runtime.json and backend config files.
func ClearRuntime(d Dirs) error {
	for _, path := range []string{d.RuntimeFile(), d.BackendConfigFile()} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}
