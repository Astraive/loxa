package safe

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Go runs fn in a goroutine and recovers any panic, writing it to stderr.
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "[loza] panic recovered: %v\n%s\n", r, debug.Stack())
			}
		}()
		fn()
	}()
}

// SafeGo runs fn in a goroutine and recovers panics.
func SafeGo(fn func()) {
	Go(fn)
}

// Call runs fn and recovers any panic, returning it as an error.
func Call(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	fn()
	return nil
}
