package sdk_test

import (
	"strings"
	"testing"

	"icarosu/internal/sdk"
)

func validManifest() sdk.Manifest {
	return sdk.Manifest{
		APIVersion: 1,
		ID:         "tool.demo",
		Version:    "0.1.0",
		Kind:       sdk.KindTool,
		Name:       "demo",
		Provides:   []string{"tool.demo"},
	}
}

func TestManifestValidate(t *testing.T) {
	if err := validManifest().Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	cases := []struct {
		name string
		edit func(*sdk.Manifest)
		want string
	}{
		{"empty kind", func(m *sdk.Manifest) { m.Kind = "" }, "kind is empty"},
		{"unknown kind", func(m *sdk.Manifest) { m.Kind = "wat" }, "unknown plugin kind"},
		{"id prefix", func(m *sdk.Manifest) { m.ID = "tool" }, "must start with"},
		{"empty provides", func(m *sdk.Manifest) { m.Provides = nil }, "provides is empty"},
		{"duplicate provides", func(m *sdk.Manifest) { m.Provides = []string{"x", "x"} }, "duplicate provides"},
		{"duplicate requires", func(m *sdk.Manifest) { m.Requires = []string{"kernel.llm", "kernel.llm"} }, "duplicate requires"},
		{"negative timeout", func(m *sdk.Manifest) {
			m.Tools = []sdk.ToolSpec{{Name: "demo", TimeoutMS: -1}}
		}, "negative timeout"},
		{"path escape", func(m *sdk.Manifest) { m.Manual = "../../etc/passwd" }, "escapes plugin directory"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest()
			tc.edit(&m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateToolName(t *testing.T) {
	for _, name := range []string{"time_now", "a1", "x"} {
		if err := sdk.ValidateToolName(name); err != nil {
			t.Fatalf("%q rejected: %v", name, err)
		}
	}
	for _, name := range []string{"", "1x", "x-y", "x y"} {
		if err := sdk.ValidateToolName(name); err == nil {
			t.Fatalf("%q accepted", name)
		}
	}
}
