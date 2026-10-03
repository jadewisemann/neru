package loader_test

import (
	"strconv"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

func TestSetField_HintLabelOptions(t *testing.T) {
	tests := []struct {
		path  string
		value string
		check func(config.HintsConfig) bool
	}{
		{
			path:  "hints.first_hand",
			value: config.FirstHandRight,
			check: func(hints config.HintsConfig) bool { return hints.FirstHand == config.FirstHandRight },
		},
		{
			path: "hints.allow_repeated_keys", value: strconv.FormatBool(true),
			check: func(hints config.HintsConfig) bool { return hints.AllowRepeatedKeys },
		},
		{
			path:  "hints.label_order",
			value: config.LabelOrderPriorityFirst,
			check: func(hints config.HintsConfig) bool { return hints.LabelOrder == config.LabelOrderPriorityFirst },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.path, func(t *testing.T) {
			cfg := config.DefaultConfig()

			err := loader.SetField(cfg, testCase.path, testCase.value)
			if err != nil {
				t.Fatalf("SetField() error: %v", err)
			}

			if !testCase.check(cfg.Hints) {
				t.Fatalf("SetField() did not set %s to %q", testCase.path, testCase.value)
			}
		})
	}
}
