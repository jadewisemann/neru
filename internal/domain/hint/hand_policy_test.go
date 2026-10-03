package hint_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/hint"
)

const (
	priorityFirstOrder        = "priority_first"
	lengthFirstOrder          = "length_first"
	handStartLeft             = "left"
	handStartRight            = "right"
	handStartBoth             = "both"
	accessibleLeftCharacters  = "ASDFZXCVWERG"
	accessibleRightCharacters = "JKLPUIONM,.H"
	legacyHomeRowCharacters   = "asdfg" + "hjkl"
)

func TestAlphabetGenerator_UpdateHandOptions_AlternatesFromEitherHandAtEveryDepth(t *testing.T) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		for _, firstHand := range []string{handStartLeft, handStartRight, handStartBoth} {
			for level, count := range []int{13, 145, 1729, 20737} {
				if firstHand == handStartBoth {
					count = 2*(count-1) + 1
				}

				generator := newHandGenerator(t, "qwerty", direction, hint.HandOptions{
					AlternateHands:    true,
					FirstHand:         firstHand,
					AllowRepeatedKeys: true,
				})

				labels := generator.LabelsForTesting(count)
				if len(labels) != count {
					t.Fatalf(
						"%s/%s: got %d labels, want %d",
						direction,
						firstHand,
						len(labels),
						count,
					)
				}

				assertLabelsUniqueAndPrefixFree(t, labels)
				assertHandPolicy(t, labels, firstHand, true, false)

				foundDepth := false
				for _, label := range labels {
					if len(label) == level+2 {
						foundDepth = true
					}
				}

				if !foundDepth {
					t.Errorf(
						"%s/%s: no %d-key label at count %d",
						direction,
						firstHand,
						level+2,
						count,
					)
				}
			}
		}
	}
}

func TestAlphabetGenerator_UpdateHandOptions_FirstHandBothUsesBothStarts(t *testing.T) {
	generator := newHandGenerator(t, "asdf", hint.LabelDirectionNormal, hint.HandOptions{
		AlternateHands: true,
		FirstHand:      handStartBoth,
	})
	labels := generator.LabelsForTesting(24)

	want := "ASDFJKLPZXCVWERGUIONM,.H"
	if got := strings.Join(labels, ""); got != want {
		t.Errorf("single-key labels = %q, want %q", got, want)
	}
}

func TestAlphabetGenerator_UpdateHandOptions_RepeatedKeyOptionInFreeMode(t *testing.T) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		for _, allowRepeated := range []bool{false, true} {
			generator := newHandGenerator(t, "ajk", direction, hint.HandOptions{
				FirstHand:         handStartBoth,
				AllowRepeatedKeys: allowRepeated,
			})
			labels := generator.LabelsForTesting(7)
			assertLabelsUniqueAndPrefixFree(t, labels)

			foundRepeat := false
			for _, label := range labels {
				for depth := 1; depth < len(label); depth++ {
					if label[depth] == label[depth-1] {
						foundRepeat = true
					}
				}
			}

			if foundRepeat != allowRepeated {
				t.Errorf(
					"%s: repeated keys found = %t, allowed = %t",
					direction,
					foundRepeat,
					allowRepeated,
				)
			}
		}
	}
}

func TestAlphabetGenerator_UpdateHandOptions_FreeModeRestrictsOnlyFirstCharacter(t *testing.T) {
	for _, firstHand := range []string{handStartLeft, handStartRight} {
		generator := newHandGenerator(t, "qybm", hint.LabelDirectionReverse, hint.HandOptions{
			FirstHand: firstHand,
		})
		labels := generator.LabelsForTesting(6)
		assertLabelsUniqueAndPrefixFree(t, labels)
		assertHandPolicy(t, labels, firstHand, false, false)

		foundSameHand := false
		for _, label := range labels {
			if len(label) == 2 && isLeftHand(rune(label[0])) == isLeftHand(rune(label[1])) {
				foundSameHand = true
			}
		}

		if !foundSameHand {
			t.Errorf("%s: free mode did not emit a same-hand pair", firstHand)
		}
	}
}

func TestAlphabetGenerator_UpdateHandOptions_PriorityFirstExpandsPrimaryStarts(t *testing.T) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		lengthGenerator := newHandGenerator(
			t,
			"asdf",
			direction,
			hint.HandOptions{AlternateHands: true},
		)
		priorityGenerator := newHandGenerator(t, "asdf", direction, hint.HandOptions{
			AlternateHands: true,
			LabelOrder:     priorityFirstOrder,
		})
		lengthLabels := lengthGenerator.LabelsForTesting(12)

		priorityLabels := priorityGenerator.LabelsForTesting(12)
		if slices.Equal(lengthLabels, priorityLabels) {
			t.Fatalf("%s: priority_first did not change allocation", direction)
		}

		for _, label := range lengthLabels {
			if len(label) != 1 {
				t.Errorf("%s: length_first generated %q instead of a single key", direction, label)
			}
		}

		for _, label := range priorityLabels {
			if !strings.ContainsRune("ASDF", rune(label[0])) {
				t.Errorf("%s: priority_first used secondary start %q", direction, label)
			}
		}

		assertLabelsUniqueAndPrefixFree(t, priorityLabels)
	}
}

func TestAlphabetGenerator_UpdateHandOptions_PriorityFirstUsesSecondaryStartsBeforeThirdDepth(
	t *testing.T,
) {
	const count = 49

	generator := newHandGenerator(t, "asdf", hint.LabelDirectionNormal, hint.HandOptions{
		AlternateHands: true,
		LabelOrder:     priorityFirstOrder,
	})

	labels := generator.LabelsForTesting(count)
	if len(labels) != count {
		t.Fatalf("got %d labels, want %d", len(labels), count)
	}

	assertLabelsUniqueAndPrefixFree(t, labels)
	assertHandPolicy(t, labels, handStartLeft, true, false)

	foundSecondary := false
	for _, label := range labels {
		if len(label) > 2 {
			t.Errorf("generated %q while two-character capacity is available", label)
		}

		if !strings.ContainsRune("ASDF", rune(label[0])) {
			foundSecondary = true
		}
	}

	if !foundSecondary {
		t.Fatal("count beyond the primary two-character pool did not use a secondary start")
	}
}

func TestAlphabetGenerator_UpdateHandOptions_GlobalTierOrderWithinEachLength(t *testing.T) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		for _, order := range []string{lengthFirstOrder, priorityFirstOrder} {
			generator := newHandGenerator(t, "asdf", direction, hint.HandOptions{
				AlternateHands: true,
				LabelOrder:     order,
			})
			labels := generator.LabelsForTesting(144)
			assertLabelsUniqueAndPrefixFree(t, labels)

			previousTier := -1

			counts := [4]int{}
			for _, label := range labels {
				if len(label) != 2 {
					t.Fatalf("%s/%s: expected two-key labels, got %q", direction, order, label)
				}

				tier := 0
				if !strings.ContainsRune("ASDF", rune(label[0])) {
					tier += 2
				}

				if !strings.ContainsRune("JKLP", rune(label[1])) {
					tier++
				}

				if tier < previousTier {
					t.Errorf(
						"%s/%s: tier %d label %q followed tier %d",
						direction,
						order,
						tier,
						label,
						previousTier,
					)
				}

				counts[tier]++
				previousTier = tier
			}

			if counts != [4]int{16, 32, 32, 64} {
				t.Errorf("%s/%s: tier counts = %v, want [16 32 32 64]", direction, order, counts)
			}
		}
	}
}

func TestAlphabetGenerator_UpdateHandOptions_CacheAndLegacyIsolation(t *testing.T) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		generator, err := hint.NewAlphabetGenerator("asdfjklp", direction)
		if err != nil {
			t.Fatalf("NewAlphabetGenerator: %v", err)
		}

		legacy := slices.Clone(generator.LabelsForTesting(20))
		options := hint.HandOptions{AlternateHands: true}

		err = generator.UpdateHandOptions(options)
		if err != nil {
			t.Fatalf("UpdateHandOptions: %v", err)
		}

		leftLabels := slices.Clone(generator.LabelsForTesting(20))
		options.AllowRepeatedKeys = true

		err = generator.UpdateHandOptions(options)
		if err != nil {
			t.Fatalf("UpdateHandOptions: %v", err)
		}

		if got := generator.LabelsForTesting(20); !slices.Equal(got, leftLabels) {
			t.Errorf("%s: alternation did not ignore the repeat setting", direction)
		}

		options.FirstHand = handStartRight

		err = generator.UpdateHandOptions(options)
		if err != nil {
			t.Fatalf("UpdateHandOptions: %v", err)
		}

		if got := generator.LabelsForTesting(20); slices.Equal(got, leftLabels) {
			t.Errorf("%s: right-first reused left-first labels", direction)
		}

		generator.UpdateAlternateHands(false)

		if got := generator.LabelsForTesting(20); !slices.Equal(got, legacy) {
			t.Errorf("%s: disabling hand policies changed legacy labels", direction)
		}
	}
}

func TestAlphabetGenerator_UpdateHandOptions_InvalidOptionsLeavePolicyIntact(t *testing.T) {
	generator := newHandGenerator(
		t,
		"ajk",
		hint.LabelDirectionNormal,
		hint.HandOptions{FirstHand: handStartRight},
	)

	before := slices.Clone(generator.LabelsForTesting(7))
	for _, options := range []hint.HandOptions{
		{FirstHand: "invalid"},
		{LabelOrder: "invalid"},
	} {
		err := generator.UpdateHandOptions(options)
		if err == nil {
			t.Errorf("UpdateHandOptions(%+v) accepted invalid input", options)
		}
	}

	err := generator.UpdateCharacters("asdf")
	if err == nil {
		t.Error("UpdateCharacters accepted an empty right-hand pool")
	}

	if got := generator.LabelsForTesting(7); !slices.Equal(got, before) {
		t.Errorf("failed update changed labels: got %v, want %v", got, before)
	}
}

func TestAlphabetGenerator_UpdateHandOptions_TwoKeysWithoutRepeatsHaveFiniteCapacity(t *testing.T) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		generator := newHandGenerator(
			t,
			"aj",
			direction,
			hint.HandOptions{FirstHand: handStartBoth},
		)
		if got := generator.MaxHints(); got != 2 {
			t.Errorf("%s: MaxHints() = %d, want 2", direction, got)
		}

		if got := generator.LabelsForTesting(100); !slices.Equal(got, []string{"A", "J"}) {
			t.Errorf("%s: saturated two-key labels = %v, want [A J]", direction, got)
		}
	}
}

func TestAlphabetGenerator_UpdateHandOptions_UnrestrictedLengthFirstPreservesLegacyOrder(
	t *testing.T,
) {
	for _, direction := range []hint.LabelDirection{hint.LabelDirectionNormal, hint.LabelDirectionReverse} {
		for _, characters := range []string{"abcd", legacyHomeRowCharacters} {
			generator, err := hint.NewAlphabetGenerator(characters, direction)
			if err != nil {
				t.Fatalf("NewAlphabetGenerator: %v", err)
			}

			for _, count := range []int{1, 10, 20} {
				legacy := slices.Clone(generator.LabelsForTesting(count))

				configured := newHandGenerator(t, characters, direction, hint.HandOptions{
					FirstHand:         handStartBoth,
					AllowRepeatedKeys: true,
					LabelOrder:        lengthFirstOrder,
				})
				if got := configured.LabelsForTesting(count); !slices.Equal(got, legacy) {
					t.Errorf(
						"%s/%s/%d: got %v, want legacy %v",
						direction,
						characters,
						count,
						got,
						legacy,
					)
				}
			}
		}
	}
}

func newHandGenerator(
	t *testing.T,
	characters string,
	direction hint.LabelDirection,
	options hint.HandOptions,
) *hint.AlphabetGenerator {
	t.Helper()

	generator, err := hint.NewAlphabetGenerator(characters, direction)
	if err != nil {
		t.Fatalf("NewAlphabetGenerator: %v", err)
	}

	err = generator.UpdateHandOptions(options)
	if err != nil {
		t.Fatalf("UpdateHandOptions: %v", err)
	}

	return generator
}

func assertHandPolicy(
	t *testing.T,
	labels []string,
	firstHand string,
	alternate, allowRepeated bool,
) {
	t.Helper()

	for _, label := range labels {
		characters := []rune(label)

		startsLeft := isLeftHand(characters[0])
		if (firstHand == handStartLeft && !startsLeft) ||
			(firstHand == handStartRight && startsLeft) {
			t.Fatalf("first_hand=%s generated %q", firstHand, label)
		}

		for depth, character := range characters {
			if alternate &&
				!strings.ContainsRune(
					accessibleLeftCharacters+accessibleRightCharacters,
					character,
				) {
				t.Fatalf("alternating label %q contains excluded key %q", label, character)
			}

			if depth == 0 {
				continue
			}

			if alternate && isLeftHand(character) == isLeftHand(characters[depth-1]) {
				t.Fatalf("label %q repeats the same hand at depth %d", label, depth)
			}

			if !allowRepeated && character == characters[depth-1] {
				t.Fatalf("label %q repeats key %q at depth %d", label, character, depth)
			}
		}
	}
}

func isLeftHand(character rune) bool {
	return strings.ContainsRune(leftHandCharacters, character)
}
