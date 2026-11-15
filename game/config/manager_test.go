package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/wricardo/OneTDraw-Solver/game/engine"
)

func writeMap(t *testing.T, dir, name string, cfg engine.MapConfig) {
	t.Helper()
	blob, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), blob, 0o644); err != nil {
		t.Fatal(err)
	}
}

func triConfig() engine.MapConfig {
	return engine.MapConfig{
		ID: "tri", Name: "Triangle", Description: "three", Difficulty: "easy",
		Points: []engine.PointConfig{
			engine.NewPointConfig(1, 1, false), engine.NewPointConfig(2, 1, false), engine.NewPointConfig(3, 1, false),
		},
		Edges: []engine.EdgeConfig{
			{PointA: 1, PointB: 2}, {PointA: 2, PointB: 3}, {PointA: 3, PointB: 1},
		},
	}
}

func TestNewManagerLoadsMaps(t *testing.T) {
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())

	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	list := mgr.List()
	if len(list) != 1 || list[0].ID != "tri" {
		t.Fatalf("list = %+v, want one tri map", list)
	}
	got, err := mgr.Get("tri")
	if err != nil {
		t.Fatal(err)
	}
	if got.Par != 3 || got.StepCount != 3 {
		t.Errorf("loaded map: par=%d steps=%d", got.Par, got.StepCount)
	}
}

func TestNewManagerMissingDirIsCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "maps")
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatalf("a missing directory should be created, got %v", err)
	}
	if len(mgr.List()) != 0 {
		t.Error("a fresh directory should hold no maps")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory was not created: %v", err)
	}
}

// An id/name mismatch is caught in strict mode; in the default lenient mode the
// file is skipped and logged so the server still boots. `make validate` runs the
// strict check, which is where a mismatch must be fixed.
func TestNewManagerRejectsIDMismatch(t *testing.T) {
	dir := t.TempDir()
	cfg := triConfig()
	cfg.ID = "other"
	writeMap(t, dir, "tri.json", cfg)
	if _, err := newManager(dir, true); err == nil {
		t.Fatal("strict mode should reject a map whose id disagrees with its file name")
	}
	// Lenient mode (the default at runtime) skips it instead of failing.
	mgr, err := newManager(dir, false)
	if err != nil {
		t.Fatalf("lenient mode should skip the file, not fail: %v", err)
	}
	if len(mgr.List()) != 0 {
		t.Errorf("the mismatched map was loaded: %d maps", len(mgr.List()))
	}
}

func TestListReturnsCopies(t *testing.T) {
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())
	mgr, _ := NewManager(dir)

	list := mgr.List()
	list[0].Edges[0].Count = 99
	again := mgr.List()
	if again[0].Edges[0].Count == 99 {
		t.Error("mutating a listed map changed the stored one")
	}
}

func TestSaveAndCreate(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager(dir)

	if _, err := mgr.Create(triConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Create(triConfig()); err == nil {
		t.Error("Create over an existing id should fail")
	}
	// Save replaces.
	cfg := triConfig()
	cfg.Name = "Renamed"
	got, err := mgr.Save(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", got.Name)
	}

	// The change must be on disk, so a restart sees it.
	reopened, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	back, err := reopened.Get("tri")
	if err != nil {
		t.Fatal(err)
	}
	if back.Name != "Renamed" {
		t.Errorf("after reload name = %q, want Renamed", back.Name)
	}
}

func TestSaveRejectsInvalidMap(t *testing.T) {
	mgr, _ := NewManager(t.TempDir())
	if _, err := mgr.Save(engine.MapConfig{ID: "x", Name: "X"}); err == nil {
		t.Fatal("saving a map with no edges should fail")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())
	mgr, _ := NewManager(dir)

	if err := mgr.Delete("tri"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tri.json")); !os.IsNotExist(err) {
		t.Error("the map file is still on disk after delete")
	}
	if err := mgr.Delete("tri"); err == nil {
		t.Error("deleting a missing map should fail")
	}
}

func TestConcurrentGetDuringSave(t *testing.T) {
	mgr, _ := NewManager(t.TempDir())
	if _, err := mgr.Create(triConfig()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 40; i++ {
			_, _ = mgr.Save(triConfig())
		}
		close(done)
	}()
	for i := 0; i < 40; i++ {
		if _, err := mgr.Get("tri"); err != nil {
			t.Errorf("Get during Save: %v", err)
		}
	}
	<-done
}

// A corrupt map file must not take the server down. This is not hypothetical:
// deploying from macOS put AppleDouble `._name.json` siblings in maps/ and the
// service crash-looped at startup, so the skip path is load-bearing.
func TestBadMapIsSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())
	// A binary resource-fork file, exactly what tar from macOS leaves behind.
	if err := os.WriteFile(filepath.Join(dir, "._tri.json"), []byte{0x00, 0x05, 0x16, 0x07, 0x00, 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	// A file that looks like JSON but is not a map.
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{"id":"broken"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr, err := newManager(dir, false)
	if err != nil {
		t.Fatalf("one bad map must not fail the load: %v", err)
	}
	if got := len(mgr.List()); got != 1 {
		t.Errorf("loaded %d maps, want just the valid one", got)
	}
	if _, err := mgr.Get("tri"); err != nil {
		t.Errorf("the valid map was not loaded: %v", err)
	}
}

func TestAppleDoubleFilesAreIgnoredSilently(t *testing.T) {
	// These are tar noise, not user error: no map, no warning, no failure.
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())
	for _, name := range []string{"._tri.json", "._maps.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte{0x00, 0x01}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mgr, err := newManager(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(mgr.List()); got != 1 {
		t.Errorf("loaded %d maps, want 1", got)
	}
}

func TestStrictModeFailsOnABadMap(t *testing.T) {
	// STRICT_MAPS=1 restores fail-fast for a deliberate check in CI.
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newManager(dir, true); err == nil {
		t.Fatal("strict mode accepted a corrupt map")
	}
}

func TestStrictModeStillIgnoresAppleDouble(t *testing.T) {
	// Resource forks are never maps, so strict mode skips them too.
	dir := t.TempDir()
	writeMap(t, dir, "tri.json", triConfig())
	if err := os.WriteFile(filepath.Join(dir, "._tri.json"), []byte{0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := newManager(dir, true)
	if err != nil {
		t.Fatalf("strict mode should ignore AppleDouble files: %v", err)
	}
	if len(mgr.List()) != 1 {
		t.Errorf("got %d maps, want 1", len(mgr.List()))
	}
}

func TestEmptyMapsDirStartsUp(t *testing.T) {
	// A fresh host with no maps must still serve the UI rather than refuse to boot.
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(mgr.List()) != 0 {
		t.Errorf("expected no maps, got %d", len(mgr.List()))
	}
}
