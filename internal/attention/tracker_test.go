package attention_test

import (
	"testing"

	"icarosu/internal/attention"
	"icarosu/internal/config"
)

func TestAttentionBoostAndProbability(t *testing.T) {
	tr := attention.New(config.AttentionConfig{
		Enabled:         true,
		MaxScore:        100,
		DecayPerMinute:  5,
		BoostAtBot:      20,
		BoostMention:    10,
		BoostInterest:   15,
		BoostBotReplied: 10,
	})

	tr.Observe(1, true, true, true)
	score := tr.Score(1)
	if score <= 0 {
		t.Fatalf("expected positive score, got %v", score)
	}

	base := 0.05
	prob := tr.EffectiveProbability(1, base)
	if prob <= base {
		t.Fatalf("expected probability to increase, got %v", prob)
	}

	cd := tr.CooldownSeconds(1, 60)
	if cd >= 60 {
		t.Fatalf("expected cooldown to decrease, got %d", cd)
	}
}
