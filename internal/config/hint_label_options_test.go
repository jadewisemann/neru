package config_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestDefaultConfig_HintLabelOptions(t *testing.T) {
	cfg := config.DefaultConfig()

	if !cfg.Hints.AlternateHands || cfg.Hints.FirstHand != config.FirstHandLeft ||
		cfg.Hints.AllowRepeatedKeys || cfg.Hints.LabelOrder != config.LabelOrderLengthFirst {
		t.Fatalf("unexpected hint label defaults: %+v", cfg.Hints)
	}

	if cfg.Hints.HintCharacters != "asdfzxcvwergjklpuionm,.h" {
		t.Fatalf("unexpected default hint characters: %q", cfg.Hints.HintCharacters)
	}
}

func TestConfig_ValidateHints_HintLabelOptions(t *testing.T) {
	tests := []struct {
		name      string
		firstHand string
		order     string
		wantErr   string
	}{
		{name: "defaults", firstHand: config.FirstHandLeft, order: config.LabelOrderLengthFirst},
		{
			name:      "right priority",
			firstHand: config.FirstHandRight,
			order:     config.LabelOrderPriorityFirst,
		},
		{name: "both hands", firstHand: config.FirstHandBoth, order: config.LabelOrderLengthFirst},
		{name: "empty enums use defaults"},
		{
			name:      "invalid hand",
			firstHand: "either",
			order:     config.LabelOrderLengthFirst,
			wantErr:   "hints.first_hand",
		},
		{
			name:      "invalid order",
			firstHand: config.FirstHandLeft,
			order:     "home_row",
			wantErr:   "hints.label_order",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.FirstHand = testCase.firstHand
			cfg.Hints.LabelOrder = testCase.order

			err := cfg.ValidateHints(nil)
			if testCase.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateHints() unexpected error: %v", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("ValidateHints() error = %v, want field %q", err, testCase.wantErr)
			}
		})
	}
}

func TestConfig_ValidateHints_FreeModeRequiresStartingHand(t *testing.T) {
	tests := []struct {
		name      string
		chars     string
		firstHand string
		wantErr   bool
	}{
		{name: "left keys", chars: "as", firstHand: config.FirstHandLeft},
		{name: "right keys", chars: "jk", firstHand: config.FirstHandRight},
		{name: "right punctuation", chars: ",.", firstHand: config.FirstHandRight},
		{name: "uppercase left", chars: "AS", firstHand: config.FirstHandLeft},
		{name: "custom both", chars: "12", firstHand: config.FirstHandBoth},
		{name: "missing left", chars: "jk", firstHand: config.FirstHandLeft, wantErr: true},
		{name: "empty hand defaults left", chars: "jk", wantErr: true},
		{name: "missing right", chars: "as", firstHand: config.FirstHandRight, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.AlternateHands = false
			cfg.Hints.HintCharacters = testCase.chars
			cfg.Hints.FirstHand = testCase.firstHand

			err := cfg.ValidateHints(nil)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateHints() error = %v, wantErr %t", err, testCase.wantErr)
			}
		})
	}
}

func TestConfig_ValidateHints_AlternationWarnsAboutRepeatedKeys(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.AllowRepeatedKeys = true

	var warnings config.Warnings

	err := cfg.ValidateHints(&warnings)
	if err != nil {
		t.Fatalf("ValidateHints() unexpected error: %v", err)
	}

	messages := warnings.Messages()
	if len(messages) != 1 ||
		!strings.Contains(messages[0], "hints.allow_repeated_keys is ignored") {
		t.Fatalf("ValidateHints() warnings = %v, want ignored repeated-key setting", messages)
	}

	cfg.Hints.AlternateHands = false
	warnings = config.Warnings{}

	err = cfg.ValidateHints(&warnings)
	if err != nil || len(warnings.Messages()) != 0 {
		t.Fatalf("free mode validation = %v, warnings %v", err, warnings.Messages())
	}
}
