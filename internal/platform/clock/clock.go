// Package clock is the only place recurin reads the wall clock.
//
// Billing logic asks a Clock for the time instead of calling time.Now, so tests
// (and, later, API test clocks) can move time forward through a whole
// subscription lifecycle in milliseconds. A lint rule forbids time.Now,
// time.Since and time.Until everywhere outside this package.
package clock

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// Clock tells the current time. All times it returns are in UTC.
type Clock interface {
	Now() time.Time
}

// Real returns the system clock.
func Real() Clock { return realClock{} }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

// ErrBackwards is returned when a Fake clock is asked to move back in time.
// Billing state only moves forward, so a test that rewinds time is a bug.
var ErrBackwards = errors.New("clock: cannot move backwards")

// Fake is a manually driven clock for tests. It never moves on its own.
// It is safe for concurrent use.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a Fake clock set to start (converted to UTC).
func NewFake(start time.Time) *Fake {
	return &Fake{now: start.UTC()}
}

// Now returns the fake current time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock forward by d and returns the new time.
// A negative d returns ErrBackwards and leaves the clock unchanged.
func (f *Fake) Advance(d time.Duration) (time.Time, error) {
	if d < 0 {
		return time.Time{}, fmt.Errorf("%w: advance by %s", ErrBackwards, d)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	return f.now, nil
}

// Set moves the clock to t. Moving to an earlier time returns ErrBackwards
// and leaves the clock unchanged.
func (f *Fake) Set(t time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t.Before(f.now) {
		return fmt.Errorf("%w: from %s to %s", ErrBackwards, f.now.Format(time.RFC3339Nano), t.Format(time.RFC3339Nano))
	}
	f.now = t.UTC()
	return nil
}
