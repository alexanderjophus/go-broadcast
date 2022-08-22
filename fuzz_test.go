package broadcast

import (
	"testing"
)

func FuzzBroadcast(f *testing.F) {
	f.Add(0)

	b := NewBroadcaster[int](1)
	defer b.Close()

	cch := make(chan int)
	b.Register(cch)
	defer b.Unregister(cch)

	f.Fuzz(func(t *testing.T, a int) {
		b.Submit(a)
		if a != <-cch {
			t.Fatalf("Submit/Receive mismatch expect=%v actual=%v", a, <-cch)
		}
	})
}
