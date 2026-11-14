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

func TestNewManagerRejectsIDMismatch(t *testing.T) {
	dir := t.TempDir()
	cfg := triConfig()
	cfg.ID = "other"
	writeMap(t, dir, "tri.json", cfg)
	if _, err := NewManager(dir); err == nil {
		t.Fatal("a file whose map id disagrees with its name should be rejected")
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
