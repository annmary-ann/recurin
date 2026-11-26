package clock_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/annmary-ann/recurin/internal/platform/clock"
)

var start = time.Date(2027, time.January, 31, 23, 59, 0, 0, time.UTC)

func TestRealIsUTCAndMoves(t *testing.T) {
	c := clock.Real()
	a := c.Now()
	if a.Location() != time.UTC {
		t.Fatalf("Real().Now() location = %v, want UTC", a.Location())
	}
	if b := c.Now(); b.Before(a) {
		t.Fatalf("Real clock went backwards: %v then %v", a, b)
	}
}

func TestFakeStartsWhereToldInUTC(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	c := clock.NewFake(start.In(ist))
	got := c.Now()
	if !got.Equal(start) || got.Location() != time.UTC {
		t.Fatalf("Now() = %v, want %v in UTC", got, start)
	}
}

func TestFakeAdvance(t *testing.T) {
	c := clock.NewFake(start)
	got, err := c.Advance(2 * time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2027, time.February, 1, 0, 1, 0, 0, time.UTC)
	if !got.Equal(want) || !c.Now().Equal(want) {
		t.Fatalf("after Advance: returned %v, Now %v, want %v", got, c.Now(), want)
	}
}

func TestFakeRefusesToGoBackwards(t *testing.T) {
	c := clock.NewFake(start)
	if _, err := c.Advance(-time.Nanosecond); !errors.Is(err, clock.ErrBackwards) {
		t.Fatalf("Advance(-1ns) error = %v, want ErrBackwards", err)
	}
	if err := c.Set(start.Add(-time.Second)); !errors.Is(err, clock.ErrBackwards) {
		t.Fatalf("Set(earlier) error = %v, want ErrBackwards", err)
	}
	if !c.Now().Equal(start) {
		t.Fatalf("clock moved after refused calls: %v", c.Now())
	}
}

func TestFakeSet(t *testing.T) {
	c := clock.NewFake(start)
	later := start.AddDate(0, 1, 0)
	if err := c.Set(later); err != nil {
		t.Fatal(err)
	}
	if err := c.Set(later); err != nil {
		t.Fatalf("Set to the same time should succeed: %v", err)
	}
	if !c.Now().Equal(later) {
		t.Fatalf("Now() = %v, want %v", c.Now(), later)
	}
}

func TestFakeConcurrentUse(t *testing.T) {
	c := clock.NewFake(start)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := c.Advance(time.Second); err != nil {
				t.Error(err)
			}
			_ = c.Now()
		})
	}
	wg.Wait()
	if want := start.Add(50 * time.Second); !c.Now().Equal(want) {
		t.Fatalf("Now() = %v, want %v", c.Now(), want)
	}
}
