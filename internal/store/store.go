package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"frp-helper/internal/model"
)

const homeOverrideEnv = "FRP_HELPER_HOME"

type Layout struct {
	Home      string
	BinRoot   string
	ConfigDir string
	StateDir  string
	LogsDir   string
}

type Store struct {
	Layout Layout
}

func New() (*Store, error) {
	layout, err := ResolveLayout()
	if err != nil {
		return nil, err
	}
	if err := ensureDirs(layout); err != nil {
		return nil, err
	}
	return &Store{Layout: layout}, nil
}

func ResolveLayout() (Layout, error) {
	if override := os.Getenv(homeOverrideEnv); override != "" {
		return layoutFromHome(override), nil
	}

	configHome, err := os.UserConfigDir()
	if err != nil {
		return Layout{}, fmt.Errorf("resolve user config dir: %w", err)
	}
	return layoutFromHome(filepath.Join(configHome, model.AppName)), nil
}

func layoutFromHome(home string) Layout {
	return Layout{
		Home:      home,
		BinRoot:   filepath.Join(home, "bin"),
		ConfigDir: filepath.Join(home, "config"),
		StateDir:  filepath.Join(home, "state"),
		LogsDir:   filepath.Join(home, "logs"),
	}
}

func ensureDirs(layout Layout) error {
	dirs := []string{
		layout.Home,
		layout.BinRoot,
		layout.ConfigDir,
		layout.StateDir,
		layout.LogsDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

func (s *Store) ManifestPath() string {
	return filepath.Join(s.Layout.ConfigDir, "manifest.json")
}

func (s *Store) ConfigPath() string {
	return filepath.Join(s.Layout.ConfigDir, "frpc.toml")
}

func (s *Store) RuntimePath() string {
	return filepath.Join(s.Layout.StateDir, "runtime.json")
}

func (s *Store) LogPath() string {
	return filepath.Join(s.Layout.LogsDir, "frpc.log")
}

func (s *Store) FRPCPath(version string) string {
	name := "frpc"
	if isWindows() {
		name += ".exe"
	}
	return filepath.Join(s.Layout.BinRoot, "frpc", version, name)
}

func (s *Store) ReadManifest() (model.Manifest, error) {
	var manifest model.Manifest
	if err := readJSON(s.ManifestPath(), &manifest, true); err != nil {
		return model.Manifest{}, err
	}
	return manifest, nil
}

func (s *Store) WriteManifest(manifest model.Manifest) error {
	return writeJSONAtomic(s.ManifestPath(), manifest, 0o600)
}

func (s *Store) ReadRuntime() (model.RuntimeState, error) {
	var state model.RuntimeState
	err := readJSON(s.RuntimePath(), &state, false)
	if errors.Is(err, os.ErrNotExist) {
		return model.RuntimeState{}, nil
	}
	if err != nil {
		return model.RuntimeState{}, err
	}
	if state.Services == nil {
		state.Services = map[string]model.ServiceRuntime{}
	}
	return state, nil
}

func (s *Store) WriteRuntime(state model.RuntimeState) error {
	if state.Services == nil {
		state.Services = map[string]model.ServiceRuntime{}
	}
	return writeJSONAtomic(s.RuntimePath(), state, 0o600)
}

func (s *Store) WriteConfig(content string) error {
	return writeFileAtomic(s.ConfigPath(), []byte(content), 0o600)
}

func readJSON(path string, target any, strict bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func writeJSONAtomic(path string, value any, perm os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, perm)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file for %s: %w", path, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file for %s: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
