package keymap

// Hint is one entry of the key line at the bottom of the screen.
type Hint struct {
	Key  string
	What string
}

// Section is a named group of keys for the help screen.
type Section struct {
	Title string
	Hints []Hint
}
