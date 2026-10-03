package app

import (
	"context"
	"image"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

const labelDirNormal = "normal"

// TestRegisterOppositeLabelDirectionGenerator_PreSeedsBothDirections verifies
// that the app initializer registers a generator for the direction opposite
// to the configured one, so the per-activation override path
// (`hints --label-direction <opposite>`) resolves to a real generator instead
// of silently falling back to the default.
func TestRegisterOppositeLabelDirectionGenerator_PreSeedsBothDirections(t *testing.T) {
	tests := []struct {
		name             string
		configuredRaw    string
		expectedPrimary  hint.LabelDirection
		expectedOpposite hint.LabelDirection
	}{
		{
			name:             "default (normal) -> opposite is reverse",
			configuredRaw:    "",
			expectedPrimary:  hint.LabelDirectionNormal,
			expectedOpposite: hint.LabelDirectionReverse,
		},
		{
			name:             "explicit normal -> opposite is reverse",
			configuredRaw:    labelDirNormal,
			expectedPrimary:  hint.LabelDirectionNormal,
			expectedOpposite: hint.LabelDirectionReverse,
		},
		{
			name:             "explicit reverse -> opposite is normal",
			configuredRaw:    "reverse",
			expectedPrimary:  hint.LabelDirectionReverse,
			expectedOpposite: hint.LabelDirectionNormal,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.LabelDirection = testCase.configuredRaw

			primaryGen, primaryGenErr := hint.NewAlphabetGenerator(
				cfg.Hints.HintCharacters,
				hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
			)
			if primaryGenErr != nil {
				t.Fatalf("NewAlphabetGenerator(primary) error: %v", primaryGenErr)
			}

			hintService := services.NewHintService(
				&mocks.MockAccessibilityPort{},
				&mocks.MockOverlayPort{},
				&mocks.MockSystemPort{},
				primaryGen,
				cfg.Hints,
				logger.Get(),
				nil,
			)

			app := &App{
				ctx:    context.Background(),
				logger: zap.NewNop(),
			}

			registerOppositeLabelDirectionGenerator(app, hintService, cfg)

			// Both directions must be resolvable.
			gotPrimary := hintService.Generator(testCase.expectedPrimary.String())
			if gotPrimary == nil {
				t.Fatalf("Generator(%s) returned nil", testCase.expectedPrimary)
			}

			if gotPrimary.LabelDirection() != testCase.expectedPrimary {
				t.Errorf(
					"Generator(%s).LabelDirection() = %v, want %v",
					testCase.expectedPrimary,
					gotPrimary.LabelDirection(),
					testCase.expectedPrimary,
				)
			}

			gotOpposite := hintService.Generator(testCase.expectedOpposite.String())
			if gotOpposite == nil {
				t.Fatalf("Generator(%s) returned nil", testCase.expectedOpposite)
			}

			if gotOpposite.LabelDirection() != testCase.expectedOpposite {
				t.Errorf(
					"Generator(%s).LabelDirection() = %v, want %v",
					testCase.expectedOpposite,
					gotOpposite.LabelDirection(),
					testCase.expectedOpposite,
				)
			}

			// The two generators must be distinct instances.
			if gotPrimary == gotOpposite {
				t.Errorf(
					"expected distinct generators for %s and %s, got the same instance",
					testCase.expectedPrimary,
					testCase.expectedOpposite,
				)
			}
		})
	}
}

// TestHintService_OverrideWithoutOppositeRegistrationFallsBack reproduces
// the user-reported bug: when only the configured direction is registered,
// asking for the opposite via the per-activation override silently returns
// the default generator. This is the regression check the
// `registerOppositeLabelDirectionGenerator` helper exists to prevent.
func TestHintService_OverrideWithoutOppositeRegistrationFallsBack(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.LabelDirection = labelDirNormal

	primaryGen, primaryGenErr := hint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
	)
	if primaryGenErr != nil {
		t.Fatalf("NewAlphabetGenerator(primary) error: %v", primaryGenErr)
	}

	hintService := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		primaryGen,
		cfg.Hints,
		logger.Get(),
		nil,
	)

	// No `UpdateGenerator` call here — the helper is intentionally NOT
	// invoked. This mirrors the buggy state where only the configured
	// direction is registered.
	gotReverse := hintService.Generator(hint.LabelDirectionReverse.String())
	if gotReverse == nil {
		t.Fatal("Generator(reverse) returned nil (expected default fallback)")
	}

	if gotReverse.LabelDirection() != hint.LabelDirectionNormal {
		t.Errorf(
			"Generator(reverse) without opposite registration returned direction %v, want normal (default fallback)",
			gotReverse.LabelDirection(),
		)
	}
}

// TestRegisterOppositeLabelDirectionGenerator_FixesUserBugScenario is the
// end-to-end regression test: with the helper invoked, the per-activation
// override (`hints --label-direction reverse`) must resolve to a real
// `reverse`-direction generator, not the configured `normal` one.
func TestRegisterOppositeLabelDirectionGenerator_FixesUserBugScenario(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.LabelDirection = labelDirNormal

	primaryGen, primaryGenErr := hint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
	)
	if primaryGenErr != nil {
		t.Fatalf("NewAlphabetGenerator(primary) error: %v", primaryGenErr)
	}

	hintService := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		primaryGen,
		cfg.Hints,
		logger.Get(),
		nil,
	)

	app := &App{
		ctx:    context.Background(),
		logger: zap.NewNop(),
	}

	// Simulate the production initialization path.
	registerOppositeLabelDirectionGenerator(app, hintService, cfg)

	// The per-activation `hints --label-direction reverse` override must
	// resolve to a generator with direction Reverse.
	gotReverse := hintService.Generator(hint.LabelDirectionReverse.String())
	if gotReverse == nil {
		t.Fatal("Generator(reverse) returned nil after registration")
	}

	if gotReverse.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(reverse) after registration has direction %v, want %v (this is the user-reported bug)",
			gotReverse.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}
}

func TestHintGenerators_AlternateHandsAcrossStartupAndReload(t *testing.T) {
	const labelCount = 200

	cfg := config.DefaultConfig()
	cfg.Hints.AlternateHands = true

	hintService, gridService, actionService, scrollService, indicators, err := initializeServices(
		cfg,
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("initializeServices() error: %v", err)
	}

	if gridService == nil || actionService == nil || scrollService == nil ||
		indicators.mode == nil || indicators.sticky == nil || indicators.virtualPointer == nil {
		t.Fatal("initializeServices() returned an incomplete service set")
	}

	app := &App{
		ctx:         context.Background(),
		logger:      zap.NewNop(),
		hintService: hintService,
	}
	registerOppositeLabelDirectionGenerator(app, hintService, cfg)

	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		generator, ok := hintService.Generator(direction.String()).(*hint.AlphabetGenerator)
		if !ok {
			t.Fatalf("Generator(%s) is not an alphabet generator", direction)
		}

		assertAlternatingHintLabels(t, generator.LabelsForTesting(labelCount), labelCount)
	}

	cfg.Hints.AlternateHands = false
	cfg.Hints.FirstHand = config.FirstHandBoth
	cfg.Hints.AllowRepeatedKeys = true
	cfg.Hints.LabelOrder = config.LabelOrderLengthFirst
	app.updateServiceConfigs(cfg)

	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		generator, ok := hintService.Generator(direction.String()).(*hint.AlphabetGenerator)
		if !ok {
			t.Fatalf("Generator(%s) after reload is not an alphabet generator", direction)
		}

		legacy, legacyErr := hint.NewAlphabetGenerator(cfg.Hints.HintCharacters, direction)
		if legacyErr != nil {
			t.Fatalf("NewAlphabetGenerator(%s) error: %v", direction, legacyErr)
		}

		got := generator.LabelsForTesting(labelCount)
		want := legacy.LabelsForTesting(labelCount)

		if !reflect.DeepEqual(got, want) {
			t.Errorf(
				"Generator(%s) after disabling alternate hands differs from legacy labels",
				direction,
			)
		}
	}

	cfg.Hints.AlternateHands = true
	cfg.Hints.FirstHand = config.FirstHandLeft
	cfg.Hints.AllowRepeatedKeys = false
	app.updateServiceConfigs(cfg)

	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		generator, ok := hintService.Generator(direction.String()).(*hint.AlphabetGenerator)
		if !ok {
			t.Fatalf("Generator(%s) after re-enabling is not an alphabet generator", direction)
		}

		assertAlternatingHintLabels(t, generator.LabelsForTesting(labelCount), labelCount)
	}
}

func assertAlternatingHintLabels(t *testing.T, labels []string, wantCount int) {
	t.Helper()

	if len(labels) != wantCount {
		t.Fatalf("got %d labels, want %d", len(labels), wantCount)
	}

	const (
		left  = "asdfzxcvwerg"
		right = "jklpuionm,.h"
	)

	seenThreeCharacters := false
	for _, label := range labels {
		characters := []rune(strings.ToLower(label))
		if len(characters) >= 3 {
			seenThreeCharacters = true
		}

		for depth, character := range characters {
			alphabet := left
			if depth%2 == 1 {
				alphabet = right
			}

			if !strings.ContainsRune(alphabet, character) {
				t.Errorf("label %q has invalid character %q at depth %d", label, character, depth)
			}
		}
	}

	if !seenThreeCharacters {
		t.Error("expected labels to extend to at least three characters")
	}
}

func TestHintGenerators_HandOptionsAcrossStartupAndReload(t *testing.T) {
	const labelCount = 200

	tests := []struct {
		name      string
		alternate bool
		firstHand string
		repeated  bool
		order     string
	}{
		{"left alternating", true, config.FirstHandLeft, false, config.LabelOrderLengthFirst},
		{
			"right alternating ignores repeats",
			true,
			config.FirstHandRight,
			true,
			config.LabelOrderPriorityFirst,
		},
		{"both alternating", true, config.FirstHandBoth, false, config.LabelOrderLengthFirst},
		{"left unrestricted", false, config.FirstHandLeft, true, config.LabelOrderPriorityFirst},
		{
			"right without repeats",
			false,
			config.FirstHandRight,
			false,
			config.LabelOrderLengthFirst,
		},
		{
			"both without repeats",
			false,
			config.FirstHandBoth,
			false,
			config.LabelOrderPriorityFirst,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.AlternateHands = testCase.alternate
			cfg.Hints.FirstHand = testCase.firstHand
			cfg.Hints.AllowRepeatedKeys = testCase.repeated
			cfg.Hints.LabelOrder = testCase.order

			hintService, _, _, _, _, err := initializeServices(
				cfg,
				&mocks.MockAccessibilityPort{},
				&mocks.MockOverlayPort{},
				&mocks.MockSystemPort{},
				zap.NewNop(),
			)
			if err != nil {
				t.Fatalf("initializeServices() error: %v", err)
			}

			app := &App{
				ctx:         context.Background(),
				logger:      zap.NewNop(),
				hintService: hintService,
			}
			registerOppositeLabelDirectionGenerator(app, hintService, cfg)
			assertHintGeneratorOptions(t, hintService, cfg.Hints, labelCount)

			cfg.Hints.FirstHand = config.FirstHandLeft
			if testCase.firstHand == config.FirstHandLeft {
				cfg.Hints.FirstHand = config.FirstHandRight
			}

			cfg.Hints.AlternateHands = !testCase.alternate
			cfg.Hints.AllowRepeatedKeys = !testCase.repeated
			cfg.Hints.LabelOrder = config.LabelOrderPriorityFirst

			if testCase.order == config.LabelOrderPriorityFirst {
				cfg.Hints.LabelOrder = config.LabelOrderLengthFirst
			}

			app.updateServiceConfigs(cfg)
			assertHintGeneratorOptions(t, hintService, cfg.Hints, labelCount)
		})
	}
}

func assertHintGeneratorOptions(
	t *testing.T,
	hintService *services.HintService,
	cfg config.HintsConfig,
	labelCount int,
) {
	t.Helper()

	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		generator, ok := hintService.Generator(direction.String()).(*hint.AlphabetGenerator)
		if !ok {
			t.Fatalf("Generator(%s) is not an alphabet generator", direction)
		}

		labels := generator.LabelsForTesting(labelCount)
		if len(labels) != labelCount {
			t.Fatalf(
				"Generator(%s) returned %d labels, want %d",
				direction,
				len(labels),
				labelCount,
			)
		}

		configured, err := newHintGenerator(cfg, direction)
		if err != nil {
			t.Fatalf("newHintGenerator(%s) error: %v", direction, err)
		}

		if !reflect.DeepEqual(labels, configured.LabelsForTesting(labelCount)) {
			t.Errorf("Generator(%s) does not reflect the current hand options", direction)
		}

		for _, label := range labels {
			characters := []rune(strings.ToLower(label))
			firstLeft := strings.ContainsRune("asdfzxcvwerg", characters[0])

			if cfg.FirstHand == config.FirstHandLeft && !firstLeft ||
				cfg.FirstHand == config.FirstHandRight && firstLeft {
				t.Errorf(
					"label %q does not start with the configured hand %q",
					label,
					cfg.FirstHand,
				)
			}

			for depth := 1; depth < len(characters); depth++ {
				if !cfg.AllowRepeatedKeys && characters[depth] == characters[depth-1] {
					t.Errorf("label %q repeats the same key at depth %d", label, depth)
				}

				if cfg.AlternateHands {
					previousLeft := strings.ContainsRune("asdfzxcvwerg", characters[depth-1])
					currentLeft := strings.ContainsRune("asdfzxcvwerg", characters[depth])

					if previousLeft == currentLeft {
						t.Errorf("label %q repeats the same hand at depth %d", label, depth)
					}
				}
			}
		}
	}
}

func TestHintGenerators_RightHandPunctuationCanBeSelected(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.FirstHand = config.FirstHandRight

	generator, err := newHintGenerator(cfg.Hints, hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("newHintGenerator() error: %v", err)
	}

	elem, err := element.NewElement("button", image.Rect(0, 0, 20, 20), element.RoleButton)
	if err != nil {
		t.Fatalf("NewElement() error: %v", err)
	}

	elements := make([]*element.Element, 12)
	for index := range elements {
		elements[index] = elem
	}

	hints, err := generator.Generate(context.Background(), elements)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	manager := hint.NewManager(zap.NewNop(), nil)

	err = manager.SetHints(hint.NewCollection(hints))
	if err != nil {
		t.Fatalf("SetHints() error: %v", err)
	}

	for _, key := range []string{",", "."} {
		err = manager.Reset()
		if err != nil {
			t.Fatalf("Reset() error: %v", err)
		}

		matched, complete, _, inputErr := manager.HandleInput(key)
		if inputErr != nil {
			t.Fatalf("HandleInput(%q) error: %v", key, inputErr)
		}

		if !complete || matched == nil || matched.Label() != key {
			t.Errorf("HandleInput(%q) did not select its punctuation hint", key)
		}
	}
}
