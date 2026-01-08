package clipboard

import (
	"errors"

	"golang.design/x/clipboard"
)

var (
	// ErrNotInitialized is returned when clipboard is not initialized
	ErrNotInitialized = errors.New("clipboard not initialized")
	
	initialized bool
)

// Init initializes the clipboard. Must be called before any clipboard operations.
// This is safe to call multiple times.
func Init() error {
	if initialized {
		return nil
	}
	
	err := clipboard.Init()
	if err != nil {
		return err
	}
	
	initialized = true
	return nil
}

// WriteText writes text to the clipboard.
// Returns error if clipboard is not initialized or write fails.
func WriteText(text string) error {
	if !initialized {
		if err := Init(); err != nil {
			return err
		}
	}
	
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}

// Clear clears the clipboard by writing an empty string.
func Clear() error {
	return WriteText("")
}

// ReadText reads text from the clipboard.
// Returns empty string if clipboard is empty or not initialized.
func ReadText() (string, error) {
	if !initialized {
		if err := Init(); err != nil {
			return "", err
		}
	}
	
	data := clipboard.Read(clipboard.FmtText)
	return string(data), nil
}



