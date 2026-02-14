package intents

type AddMessage struct {
	Text   string
	Err    error
	Sticky bool
}

func (AddMessage) isIntent() {}

type DismissOldest struct{}

func (DismissOldest) isIntent() {}
