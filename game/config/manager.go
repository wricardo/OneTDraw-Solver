// Package config loads, lists and saves map files (maps/*.json).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
	"github.com/wricardo/OneTDraw-Solver/game/service"
)

// Manager is a file-backed service.MapStore. All maps are held in memory and
// guarded by an RWMutex; callers always receive deep copies.
type Manager struct {
	dir  string
	mu   sync.RWMutex
	maps map[string]*engine.Map
}

var _ service.MapStore = (*Manager)(nil)

// NewManager loads every *.json file in dir, creating dir if missing. A file
// that is not a valid map is skipped with a logged warning rather than failing
// startup, so one bad file cannot take the whole game down.
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create maps dir: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read maps dir: %w", err)
	}
	m := &Manager{dir: dir, maps: make(map[string]*engine.Map)}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		var cfg engine.MapConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		want := strings.TrimSuffix(e.Name(), ".json")
		if cfg.ID == "" {
			cfg.ID = want
		}
		if cfg.ID != want {
			return nil, fmt.Errorf("%s: map id %q does not match file name %q", path, cfg.ID, want)
		}
		parsed, err := engine.NewMap(cfg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		m.maps[parsed.ID] = parsed
	}
	return m, nil
}

// List returns copies of all maps sorted by id.
func (m *Manager) List() []*engine.Map {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*engine.Map, 0, len(m.maps))
	for _, mp := range m.maps {
		out = append(out, mp.Clone())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns a copy of the map or a NOT_FOUND *service.Error.
func (m *Manager) Get(id string) (*engine.Map, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mp, ok := m.maps[id]
	if !ok {
		return nil, service.Errorf(service.CodeNotFound, "map %q not found", id)
	}
	return mp.Clone(), nil
}

// Save validates cfg, writes it atomically to <dir>/<id>.json and stores it,
// replacing any existing map with the same id.
func (m *Manager) Save(cfg engine.MapConfig) (*engine.Map, error) {
	return m.store(cfg, false)
}

// Create is Save that fails with INVALID_ARGUMENT when the id already exists.
func (m *Manager) Create(cfg engine.MapConfig) (*engine.Map, error) {
	return m.store(cfg, true)
}

func (m *Manager) store(cfg engine.MapConfig, createOnly bool) (*engine.Map, error) {
	if !engine.ValidID(cfg.ID) {
		return nil, service.Errorf(service.CodeInvalidArgument, "invalid map id %q: must match ^[a-z0-9_-]{1,64}$", cfg.ID)
	}
	parsed, err := engine.NewMap(cfg)
	if err != nil {
		return nil, service.Errorf(service.CodeInvalidArgument, "invalid map: %v", err)
	}
	data, err := json.MarshalIndent(parsed.Config(), "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.maps[parsed.ID]; createOnly && exists {
		return nil, service.Errorf(service.CodeInvalidArgument, "map %q already exists", parsed.ID)
	}
	if err := writeAtomic(m.dir, parsed.ID+".json", data); err != nil {
		return nil, fmt.Errorf("save map: %w", err)
	}
	m.maps[parsed.ID] = parsed
	return parsed.Clone(), nil
}

// Delete removes the map and its file. NOT_FOUND if absent.
func (m *Manager) Delete(id string) error {
	if !engine.ValidID(id) {
		return service.Errorf(service.CodeInvalidArgument, "invalid map id %q", id)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.maps[id]; !ok {
		return service.Errorf(service.CodeNotFound, "map %q not found", id)
	}
	if err := os.Remove(filepath.Join(m.dir, id+".json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete map: %w", err)
	}
	if err := syncDir(m.dir); err != nil {
		return fmt.Errorf("delete map: %w", err)
	}
	delete(m.maps, id)
	return nil
}

// writeAtomic writes data to dir/name via a fsynced temp file and a rename,
// then fsyncs the directory so the rename itself is durable.
func writeAtomic(dir, name string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		os.Remove(tmpName)
		return err
	}
	return syncDir(dir)
}

// syncDir fsyncs a directory so renames and removals in it are durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		d.Close()
		return err
	}
	return d.Close()
}
