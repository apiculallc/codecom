package tui

type KeyHint struct {
	Key   string
	Label string
}

var KeyHints = []KeyHint{
	{Key: "Tab", Label: "panel"},
	{Key: "Arrows", Label: "nav"},
	{Key: "/", Label: "filter"},
	{Key: "Enter", Label: "open"},
	{Key: "PgUp/PgDn", Label: "page"},
	{Key: "Home/End", Label: "jump"},
	{Key: "F5", Label: "refresh"},
	{Key: "F6", Label: "move"},
	{Key: "Space", Label: "select"},
	{Key: "A", Label: "all"},
	{Key: "U", Label: "undo"},
	{Key: "Y", Label: "copy"},
	{Key: "Q", Label: "quit"},
}
