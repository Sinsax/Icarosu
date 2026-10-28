package main

import (
	"os"
	"path/filepath"
	"testing"

	"icarosu/internal/config"
)

func TestLoadSectionFromPluginFile(t *testing.T) {
	dir := t.TempDir()
	content := "version: 1\nenabled: true\nconfig:\n  name: 测试\n  speaking_style: 短句\n"
	if err := os.WriteFile(filepath.Join(dir, "persona.default.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var out struct {
		Name          string `json:"name"`
		SpeakingStyle string `json:"speaking_style"`
	}
	if err := loadSection(&config.Config{}, dir, "persona.default", &out); err != nil {
		t.Fatalf("loadSection: %v", err)
	}
	if out.Name != "测试" || out.SpeakingStyle != "短句" {
		t.Fatalf("unexpected config: %+v", out)
	}
}

func TestLoadSectionFallsBackToMainConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Bot.Name = "小智"
	var out struct {
		Name string `json:"name"`
	}
	if err := loadSection(cfg, filepath.Join(t.TempDir(), "missing"), "persona.default", &out); err != nil {
		t.Fatalf("loadSection: %v", err)
	}
	if out.Name != "小智" {
		t.Fatalf("expected fallback name, got %q", out.Name)
	}
}

func TestWritePluginConfigs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Bot.Name = "迁移测试"
	cfg.Bot.Personality = "懒"
	cfg.Filter.Mode = "deny"
	dir := t.TempDir()
	if err := writePluginConfigs(cfg, dir); err != nil {
		t.Fatalf("writePluginConfigs: %v", err)
	}
	var out struct {
		Name string `json:"name"`
	}
	if err := loadSection(cfg, dir, "persona.default", &out); err != nil {
		t.Fatalf("loadSection: %v", err)
	}
	if out.Name != "迁移测试" {
		t.Fatalf("unexpected migrated name: %q", out.Name)
	}
}
