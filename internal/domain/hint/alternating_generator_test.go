package hint_test

import (
	"context"
	"image"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

const (
	leftHandCharacters  = "QWERTASDFGZXCVB"
	rightHandCharacters = "YUIOPHJKLNM"
)

func TestAlphabetGenerator_AlternatingHands_LabelsAtEveryDepth(t *testing.T) {
	testCases := []struct {
		name   string
		count  int
		length int
	}{
		{"two characters", 16, 2},
		{"three characters", 166, 3},
		{"four characters", 2476, 4},
		{"five characters", 27226, 5},
	}

	for _, direction := range []hint.LabelDirection{
		hint.LabelDirectionNormal,
		hint.LabelDirectionReverse,
	} {
		generator, err := hint.NewAlphabetGenerator("asdf", direction)
		if err != nil {
			t.Fatalf("NewAlphabetGenerator: %v", err)
		}

		generator.UpdateAlternateHands(true)

		for _, testCase := range testCases {
			t.Run(direction.String()+"/"+testCase.name, func(t *testing.T) {
				labels := generator.LabelsForTesting(testCase.count)
				if len(labels) != testCase.count {
					t.Fatalf("got %d labels, want %d", len(labels), testCase.count)
				}

				assertLabelsUniqueAndPrefixFree(t, labels)
				assertAlternatingHands(t, labels)

				found := false
				for _, label := range labels {
					if len([]rune(label)) == testCase.length {
						found = true

						break
					}
				}

				if !found {
					t.Errorf("no %d-character label generated", testCase.length)
				}
			})
		}
	}
}

func TestAlphabetGenerator_AlternatingHands_GeneratesManyTargets(t *testing.T) {
	const targetCount = 2476 // One more than the three-character alternating capacity.

	generator, err := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator: %v", err)
	}

	generator.UpdateAlternateHands(true)

	if generator.MaxHints() < targetCount {
		t.Fatalf("MaxHints() = %d, want at least %d", generator.MaxHints(), targetCount)
	}

	elements := make([]*element.Element, targetCount)
	for index := range elements {
		elements[index], err = element.NewElement(
			element.ID(strconv.Itoa(index)),
			image.Rect(index, 0, index+1, 1),
			element.RoleButton,
		)
		if err != nil {
			t.Fatalf("NewElement(%d): %v", index, err)
		}
	}

	hints, err := generator.Generate(context.Background(), elements)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(hints) != targetCount {
		t.Fatalf("Generate returned %d hints, want %d", len(hints), targetCount)
	}

	labels := make([]string, len(hints))
	for i, generated := range hints {
		labels[i] = generated.Label()
	}

	assertLabelsUniqueAndPrefixFree(t, labels)
	assertAlternatingHands(t, labels)
}

func TestAlphabetGenerator_AlternatingHands_ManagerFiltersAndSelects(t *testing.T) {
	generator, err := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator: %v", err)
	}

	generator.UpdateAlternateHands(true)

	elements := make([]*element.Element, 16)
	for index := range elements {
		elements[index], err = element.NewElement(
			element.ID(strconv.Itoa(index)),
			image.Rect(index, 0, index+1, 1),
			element.RoleButton,
		)
		if err != nil {
			t.Fatalf("NewElement(%d): %v", index, err)
		}
	}

	hints, err := generator.Generate(context.Background(), elements)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	label := ""
	for _, generated := range hints {
		if len(generated.Label()) == 2 {
			label = generated.Label()

			break
		}
	}

	if label == "" {
		t.Fatal("expected a two-character label")
	}

	manager := hint.NewManager(nil, nil)

	err = manager.SetHints(hint.NewCollection(hints))
	if err != nil {
		t.Fatalf("SetHints: %v", err)
	}

	match, found, _, err := manager.HandleInput(strings.ToLower(label[:1]))
	if err != nil {
		t.Fatalf("HandleInput first character: %v", err)
	}

	if found || match != nil {
		t.Fatalf("selected %v after only the first character of %q", match, label)
	}

	if filtered := manager.FilteredHints(); len(filtered) != 2 {
		t.Errorf(
			"FilteredHints() returned %d hints after prefix %q, want 2",
			len(filtered),
			label[:1],
		)
	}

	match, found, _, err = manager.HandleInput(strings.ToLower(label[1:]))
	if err != nil {
		t.Fatalf("HandleInput second character: %v", err)
	}

	if !found || match == nil || match.Label() != label {
		t.Errorf("selected %v after typing %q, want that exact hint", match, label)
	}
}

func TestAlphabetGenerator_AlternatingHands_ToggleRestoresConfiguredCharacters(t *testing.T) {
	for _, direction := range []hint.LabelDirection{
		hint.LabelDirectionNormal,
		hint.LabelDirectionReverse,
	} {
		t.Run(direction.String(), func(t *testing.T) {
			generator, err := hint.NewAlphabetGenerator("abcd", direction)
			if err != nil {
				t.Fatalf("NewAlphabetGenerator: %v", err)
			}

			legacy := slices.Clone(generator.LabelsForTesting(20))
			generator.UpdateAlternateHands(true)

			if got := generator.Characters(); got != leftHandCharacters+rightHandCharacters {
				t.Errorf("Characters() = %q with alternation enabled", got)
			}

			assertAlternatingHands(t, generator.LabelsForTesting(20))

			err = generator.UpdateCharacters("xyz")
			if err != nil {
				t.Fatalf("UpdateCharacters: %v", err)
			}

			assertAlternatingHands(t, generator.LabelsForTesting(20))

			generator.UpdateAlternateHands(false)

			if got := generator.Characters(); got != "XYZ" {
				t.Errorf("Characters() = %q after disabling alternation, want XYZ", got)
			}

			if got := generator.MaxHints(); got != 27 {
				t.Errorf("MaxHints() = %d after disabling alternation, want 27", got)
			}

			err = generator.UpdateCharacters("abcd")
			if err != nil {
				t.Fatalf("UpdateCharacters: %v", err)
			}

			if got := generator.LabelsForTesting(20); !slices.Equal(got, legacy) {
				t.Errorf(
					"legacy labels changed after toggling alternation: got %v, want %v",
					got,
					legacy,
				)
			}
		})
	}
}

func assertAlternatingHands(t *testing.T, labels []string) {
	t.Helper()

	for _, label := range labels {
		for depth, character := range label {
			alphabet := leftHandCharacters
			if depth%2 == 1 {
				alphabet = rightHandCharacters
			}

			if !strings.ContainsRune(alphabet, character) {
				t.Fatalf(
					"label %q has %q at depth %d, want a character from %q",
					label,
					character,
					depth,
					alphabet,
				)
			}
		}
	}
}
