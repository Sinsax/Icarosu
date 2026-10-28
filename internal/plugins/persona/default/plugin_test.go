package persona_test

import (
	"context"
	"strings"
	"testing"

	"icarosu/internal/plugins/persona/default"
	"icarosu/internal/sdk/testkit"
)

func TestPersonaBuildsPrompt(t *testing.T) {
	p := persona.New().(*persona.Plugin)
	if err := p.Init(context.Background(), testkit.NewFakeHost()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	persona, err := p.Persona(context.Background())
	if err != nil {
		t.Fatalf("Persona: %v", err)
	}
	if persona.Name != "小智" || !strings.Contains(persona.SystemPrompt, "普通群友") {
		t.Fatalf("unexpected persona: %+v", persona)
	}
}
