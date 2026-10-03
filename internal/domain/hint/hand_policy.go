package hint

import (
	"slices"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	firstHandLeft            = "left"
	firstHandRight           = "right"
	firstHandBoth            = "both"
	labelOrderLengthFirst    = "length_first"
	labelOrderPriorityFirst  = "priority_first"
	primaryLeftCharacters    = "ASDF"
	secondaryLeftCharacters  = "ZXCVWERG"
	primaryRightCharacters   = "JKLP"
	secondaryRightCharacters = "UIONM,.H"
	handLeftCharacters       = primaryLeftCharacters + secondaryLeftCharacters
	handRightCharacters      = primaryRightCharacters + secondaryRightCharacters
	handCharacters           = handLeftCharacters + handRightCharacters
	primaryHandCharacters    = primaryLeftCharacters + primaryRightCharacters
	qwertyLeftCharacters     = "QWERTASDFGZXCVB"
	qwertyRightCharacters    = "YUIOPHJKLNM,."
	maxHandLabelLength       = 5
	handTierCombinations     = 1 << maxHandLabelLength
	handRankBucketCount      = 2 * maxHandLabelLength * handTierCombinations
)

// HandOptions controls hint starting hands, alternation, and key priority.
// Empty FirstHand and LabelOrder select left and length_first respectively.
// AlternateHands always prevents repeated keys, regardless of AllowRepeatedKeys.
type HandOptions struct {
	AlternateHands    bool
	FirstHand         string
	AllowRepeatedKeys bool
	LabelOrder        string
}

type handPolicy struct {
	options      HandOptions
	characters   []rune
	roots        []rune
	primaryRoots []rune
	children     map[rune][]rune
	branch       int
	prioritize   bool
}

// UpdateHandOptions applies hand constraints without changing hint_characters.
// Unlike the legacy UpdateAlternateHands API, disabling alternation retains
// FirstHand, AllowRepeatedKeys, and LabelOrder constraints.
func (g *AlphabetGenerator) UpdateHandOptions(options HandOptions) error {
	policy, err := newHandPolicy(g.uppercaseChars, options)
	if err != nil {
		return err
	}

	g.handPolicy = policy
	g.alternateHands = false
	g.maxHints = policy.capacity(len(policy.roots))

	return nil
}

func newHandPolicy(configured string, options HandOptions) (*handPolicy, error) {
	if options.FirstHand == "" {
		options.FirstHand = firstHandLeft
	}

	if options.LabelOrder == "" {
		options.LabelOrder = labelOrderLengthFirst
	}

	if options.FirstHand != firstHandLeft && options.FirstHand != firstHandRight &&
		options.FirstHand != firstHandBoth {
		return nil, derrors.Newf(
			derrors.CodeInvalidInput,
			"invalid first_hand %q",
			options.FirstHand,
		)
	}

	if options.LabelOrder != labelOrderLengthFirst &&
		options.LabelOrder != labelOrderPriorityFirst {
		return nil, derrors.Newf(
			derrors.CodeInvalidInput,
			"invalid label_order %q",
			options.LabelOrder,
		)
	}

	if options.AlternateHands {
		configured = handCharacters
		options.AllowRepeatedKeys = false
	}

	policy := &handPolicy{
		options:    options,
		characters: []rune(configured),
		children:   make(map[rune][]rune),
	}

	for _, character := range policy.characters {
		if options.FirstHand == firstHandBoth ||
			(options.FirstHand == firstHandLeft && strings.ContainsRune(qwertyLeftCharacters, character)) ||
			(options.FirstHand == firstHandRight && strings.ContainsRune(qwertyRightCharacters, character)) {
			policy.roots = append(policy.roots, character)
		}
	}

	if len(policy.roots) == 0 {
		return nil, derrors.Newf(
			derrors.CodeInvalidInput,
			"hint_characters has no characters for first_hand %q",
			options.FirstHand,
		)
	}

	prioritize := options.AlternateHands || options.FirstHand != firstHandBoth ||
		!options.AllowRepeatedKeys || options.LabelOrder != labelOrderLengthFirst
	policy.prioritize = prioritize

	if prioritize {
		policy.roots = prioritizeHandCharacters(policy.roots)
	}

	for _, character := range policy.roots {
		if strings.ContainsRune(primaryHandCharacters, character) {
			policy.primaryRoots = append(policy.primaryRoots, character)
		}
	}

	for _, previous := range policy.characters {
		alphabet := policy.characters
		if options.AlternateHands {
			alphabet = []rune(handRightCharacters)
			if strings.ContainsRune(handRightCharacters, previous) {
				alphabet = []rune(handLeftCharacters)
			}
		}

		for _, character := range alphabet {
			if options.AllowRepeatedKeys || character != previous {
				policy.children[previous] = append(policy.children[previous], character)
			}
		}

		if prioritize {
			policy.children[previous] = prioritizeHandCharacters(policy.children[previous])
		}
	}

	policy.branch = len(policy.children[policy.characters[0]])

	return policy, nil
}

func prioritizeHandCharacters(characters []rune) []rune {
	result := make([]rune, 0, len(characters))
	for _, character := range characters {
		if strings.ContainsRune(primaryHandCharacters, character) {
			result = append(result, character)
		}
	}

	for _, character := range characters {
		if !strings.ContainsRune(primaryHandCharacters, character) {
			result = append(result, character)
		}
	}

	return result
}

func (p *handPolicy) capacity(rootCount int) int {
	capacity := rootCount
	for range maxHandLabelLength - 1 {
		capacity *= p.branch
	}

	return capacity
}

func (p *handPolicy) cacheKey() string {
	return ":hands:" + p.options.FirstHand + ":" + p.options.LabelOrder + ":" +
		strconv.FormatBool(
			p.options.AlternateHands,
		) + ":" + strconv.FormatBool(p.options.AllowRepeatedKeys)
}

func (p *handPolicy) labels(count int, direction LabelDirection) []string {
	if count <= 0 {
		return nil
	}

	count = min(count, p.capacity(len(p.roots)))

	var labels []string

	if p.options.LabelOrder == labelOrderPriorityFirst && len(p.primaryRoots) > 0 {
		labels = p.priorityLabels(count, direction)
	} else {
		labels = p.labelsForRoots(count, p.roots, direction)
	}

	if p.prioritize {
		labels = p.rankLabels(labels)
	}

	return labels
}

func (p *handPolicy) labelsForRoots(count int, roots []rune, direction LabelDirection) []string {
	if direction == LabelDirectionReverse {
		return p.reverseLabels(count, roots)
	}

	return p.normalLabels(count, roots)
}

func (p *handPolicy) priorityLabels(count int, direction LabelDirection) []string {
	length := 1
	fullCapacity := len(p.roots)

	primaryCapacity := len(p.primaryRoots)
	for fullCapacity < count || (length == 1 && primaryCapacity < count) {
		fullCapacity *= p.branch
		primaryCapacity *= p.branch
		length++
	}

	primaryCount := min(count, primaryCapacity)

	labels := p.labelsForRoots(primaryCount, p.primaryRoots, direction)
	if primaryCount < count {
		secondaryRoots := p.roots[len(p.primaryRoots):]
		labels = append(labels, p.labelsForRoots(count-primaryCount, secondaryRoots, direction)...)
	}

	return labels
}

func (p *handPolicy) rankLabels(labels []string) []string {
	var buckets [handRankBucketCount][]string

	for _, label := range labels {
		characters := []rune(label)
		signature := 0

		for _, character := range characters {
			signature <<= 1
			if !strings.ContainsRune(primaryHandCharacters, character) {
				signature++
			}
		}

		bucket := (len(characters)-1)*handTierCombinations + signature
		if p.options.LabelOrder == labelOrderPriorityFirst {
			firstTier := signature >> (len(characters) - 1)
			suffixMask := (1 << (len(characters) - 1)) - 1
			bucket = firstTier*maxHandLabelLength*handTierCombinations +
				(len(characters)-1)*handTierCombinations + (signature & suffixMask)
		}

		buckets[bucket] = append(buckets[bucket], label)
	}

	ordered := make([]string, 0, len(labels))
	for _, bucket := range buckets {
		ordered = append(ordered, bucket...)
	}

	return ordered
}

func (p *handPolicy) reverseLabels(count int, roots []rune) []string {
	length := 1

	capacity := len(roots)
	for capacity < count {
		capacity *= p.branch
		length++
	}

	labels := make([]string, 0, count)

	digits := make([]int, length)
	for index := range count {
		value := index
		digits[0] = value % len(roots)
		value /= len(roots)

		for depth := 1; depth < length; depth++ {
			digits[depth] = value % p.branch
			value /= p.branch
		}

		labels = append(labels, p.labelForDigits(digits, roots))
	}

	return labels
}

func (p *handPolicy) normalLabels(count int, roots []rune) []string {
	counts := make([]int, 0, maxHandLabelLength)
	remaining := count

	slots := len(roots)
	for remaining > 0 {
		nextCapacity := slots * p.branch
		keep := 0

		switch {
		case slots >= remaining:
			keep = remaining
		case nextCapacity >= remaining:
			keep = (nextCapacity - remaining) / (p.branch - 1)
		}

		counts = append(counts, keep)
		remaining -= keep
		slots = (slots - keep) * p.branch
	}

	labels := make([]string, 0, count)

	cursor := []int{0}
	for level, keep := range counts {
		for len(cursor) <= level {
			cursor = append(cursor, 0)
		}

		for range keep {
			labels = append(labels, p.labelForDigits(cursor, roots))
			p.incrementDigits(cursor, len(roots))
		}
	}

	return labels
}

func (p *handPolicy) labelForDigits(digits []int, roots []rune) string {
	characters := make([]rune, len(digits))

	characters[0] = roots[digits[0]]
	for depth := 1; depth < len(digits); depth++ {
		characters[depth] = p.children[characters[depth-1]][digits[depth]]
	}

	return string(characters)
}

func (p *handPolicy) incrementDigits(digits []int, rootCount int) {
	for depth := range slices.Backward(digits) {
		digits[depth]++

		base := p.branch
		if depth == 0 {
			base = rootCount
		}

		if digits[depth] < base {
			return
		}

		digits[depth] = 0
	}
}
