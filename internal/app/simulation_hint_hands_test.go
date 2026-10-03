package app_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

func TestSimulation_RightHandPunctuationHintMovesCursor(t *testing.T) {
	for _, key := range []string{",", "."} {
		t.Run(key, func(t *testing.T) {
			cfg := simConfig()
			cfg.Hints.AlternateHands = true
			cfg.Hints.FirstHand = config.FirstHandRight
			cfg.Hints.LabelOrder = config.LabelOrderLengthFirst

			elements := manyButtons(t, 12)
			sim := newSimHarness(t, cfg, elements)
			sim.pressHotkey(hintsHotkey)
			sim.waitMode(domain.ModeHints)
			sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

			labels := sim.overlay.lastHintLabels()
			targetIndex := -1

			for index, label := range labels {
				if label == key {
					targetIndex = index

					break
				}
			}

			if targetIndex < 0 {
				t.Fatalf("generated hints do not include %q: %v", key, labels)
			}

			movesBefore := sim.cursor.moveCount()
			sim.press(key)
			sim.waitFor("punctuation hint selected", func() bool {
				return sim.cursor.moveCount() > movesBefore
			})

			if got, want := sim.cursor.position(), elements[targetIndex].Center(); got != want {
				t.Errorf("punctuation hint moved cursor to %v, want %v", got, want)
			}
		})
	}
}
