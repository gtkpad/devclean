package watch

import "github.com/gen2brain/beeep"

// Notifier sends OS-native desktop notifications.
type Notifier interface {
	Notify(title, message string) error
}

// BeeepNotifier uses gen2brain/beeep for cross-platform notifications.
type BeeepNotifier struct{}

// NewBeeepNotifier creates a new BeeepNotifier.
func NewBeeepNotifier() *BeeepNotifier {
	return &BeeepNotifier{}
}

func (n *BeeepNotifier) Notify(title, message string) error {
	return beeep.Notify(title, message, "")
}

// NoopNotifier discards notifications (for testing and headless environments).
type NoopNotifier struct {
	LastTitle   string
	LastMessage string
	Called      int
}

func (n *NoopNotifier) Notify(title, message string) error {
	n.LastTitle = title
	n.LastMessage = message
	n.Called++
	return nil
}
