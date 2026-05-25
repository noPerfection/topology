package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()

	a, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if a.Services == nil {
		t.Fatal("Services is nil")
	}
	if len(a.Services) != 0 {
		t.Fatalf("len(Services) = %d, want 0", len(a.Services))
	}
}

func TestGetService(t *testing.T) {
	a := SdsService{}
	sample := New("api", IndependentType)
	if err := a.SetService(*sample); err != nil {
		t.Fatalf("SetService: %v", err)
	}

	found, err := a.GetService("api")
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if found.Name != "api" {
		t.Fatalf("Name = %q, want api", found.Name)
	}

	if _, err := a.GetService("missing"); err == nil {
		t.Fatal("GetService missing service returned nil error")
	}
}

func TestGetByType(t *testing.T) {
	a := SdsService{}
	services := []Service{
		*New("api", IndependentType),
		*New("worker", IndependentType),
		*New("proxy", ProxyType),
	}
	for _, s := range services {
		if err := a.SetService(s); err != nil {
			t.Fatalf("SetService: %v", err)
		}
	}

	if _, err := a.GetByType(Type("invalid")); err == nil {
		t.Fatal("GetByType with invalid type returned nil error")
	}
	if _, err := a.GetByType(ExtensionType); err == nil {
		t.Fatal("GetByType with missing type returned nil error")
	}

	found, err := a.GetByType(IndependentType)
	if err != nil {
		t.Fatalf("GetByType independent: %v", err)
	}
	if found.Name != "api" {
		t.Fatalf("Name = %q, want api", found.Name)
	}
}

func TestFilterByType(t *testing.T) {
	a := SdsService{}
	services := []Service{
		*New("api", IndependentType),
		*New("worker", IndependentType),
		*New("proxy", ProxyType),
	}
	for _, s := range services {
		if err := a.SetService(s); err != nil {
			t.Fatalf("SetService: %v", err)
		}
	}

	if _, err := a.FilterByType(Type("invalid")); err == nil {
		t.Fatal("FilterByType with invalid type returned nil error")
	}
	if _, err := a.FilterByType(ExtensionType); err == nil {
		t.Fatal("FilterByType with missing type returned nil error")
	}

	found, err := a.FilterByType(IndependentType)
	if err != nil {
		t.Fatalf("FilterByType independent: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("len(found) = %d, want 2", len(found))
	}
	if found[0].Name != "api" {
		t.Fatalf("first service = %q, want api", found[0].Name)
	}
	if found[1].Name != "worker" {
		t.Fatalf("second service = %q, want worker", found[1].Name)
	}
}

func TestCountByType(t *testing.T) {
	a := SdsService{}
	services := []Service{
		*New("api", IndependentType),
		*New("worker", IndependentType),
		*New("proxy", ProxyType),
	}
	for _, s := range services {
		if err := a.SetService(s); err != nil {
			t.Fatalf("SetService: %v", err)
		}
	}

	if count := a.CountByType(IndependentType); count != 2 {
		t.Fatalf("CountByType independent = %d, want 2", count)
	}
	if count := a.CountByType(ExtensionType); count != 0 {
		t.Fatalf("CountByType extension = %d, want 0", count)
	}
	if count := a.CountByType(Type("invalid")); count != 0 {
		t.Fatalf("CountByType invalid = %d, want 0", count)
	}
}

func TestSetService(t *testing.T) {
	a := SdsService{}
	first := New("api", IndependentType)
	second := New("proxy", ProxyType)

	if err := a.SetService(*first); err != nil {
		t.Fatalf("SetService first: %v", err)
	}
	if err := a.SetService(*second); err != nil {
		t.Fatalf("SetService second: %v", err)
	}
	if len(a.Services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(a.Services))
	}

	updated := *first
	updated.StartCommand = "go run ./cmd/api"
	if err := a.SetService(updated); err != nil {
		t.Fatalf("SetService update: %v", err)
	}
	if len(a.Services) != 2 {
		t.Fatalf("len(Services) after update = %d, want 2", len(a.Services))
	}

	found, err := a.GetService("api")
	if err != nil {
		t.Fatalf("GetService updated: %v", err)
	}
	if found.StartCommand != "go run ./cmd/api" {
		t.Fatalf("StartCommand = %q, want go run ./cmd/api", found.StartCommand)
	}
}

func TestRemoveService(t *testing.T) {
	a := SdsService{}
	first := New("api", IndependentType)
	second := New("proxy", ProxyType)
	if err := a.SetService(*first); err != nil {
		t.Fatalf("SetService first: %v", err)
	}
	if err := a.SetService(*second); err != nil {
		t.Fatalf("SetService second: %v", err)
	}

	if err := a.RemoveService(""); err == nil {
		t.Fatal("RemoveService with empty name returned nil error")
	}
	if err := a.RemoveService("missing"); err == nil {
		t.Fatal("RemoveService with missing service returned nil error")
	}

	if err := a.RemoveService("api"); err != nil {
		t.Fatalf("RemoveService: %v", err)
	}
	if len(a.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(a.Services))
	}
	if a.Services[0].Name != "proxy" {
		t.Fatalf("remaining service = %q, want proxy", a.Services[0].Name)
	}
}

func TestLoadSave(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "app.json")
	original, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	sample := New("api", IndependentType)
	sample.Handlers = []Handler{
		{
			Type:     ReplierType,
			Category: "api",
			Socket:   Socket{Id: "api_1", Port: 4101},
		},
	}
	if err := original.SetService(*sample); err != nil {
		t.Fatalf("SetService: %v", err)
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if !jsonLooksIndented(data) {
		t.Fatalf("written JSON is not indented: %s", string(data))
	}

	loaded, err := Load(filePath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Services) != 1 {
		t.Fatalf("len(Services) = %d, want 1", len(loaded.Services))
	}
	if loaded.Services[0].Name != "api" {
		t.Fatalf("Name = %q, want api", loaded.Services[0].Name)
	}
	if loaded.Services[0].Handlers[0].Socket.Port != 4101 {
		t.Fatalf("Port = %d, want 4101", loaded.Services[0].Handlers[0].Socket.Port)
	}
}

func TestSaveWithoutFilePath(t *testing.T) {
	if err := (SdsService{}).Save(); err == nil {
		t.Fatal("Save without file path returned nil error")
	}
}

func jsonLooksIndented(data []byte) bool {
	for _, b := range data {
		if b == '\n' {
			return true
		}
	}
	return false
}
