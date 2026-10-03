package config

const (
	// FirstHandLeft restricts the first hint character to the left hand.
	FirstHandLeft = "left"
	// FirstHandRight restricts the first hint character to the right hand.
	FirstHandRight = "right"
	// FirstHandBoth permits either hand for the first hint character.
	FirstHandBoth = "both"
	// LabelOrderLengthFirst favors shorter hint labels.
	LabelOrderLengthFirst = "length_first"
	// LabelOrderPriorityFirst favors primary starting keys before label length.
	LabelOrderPriorityFirst = "priority_first"
)
