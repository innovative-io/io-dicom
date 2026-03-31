package network

import (
	"sync"
	"testing"
)

func TestResetuniq(t *testing.T) {
	Resetuniq()
	v1 := Uniq8odd()
	Resetuniq()
	v2 := Uniq8odd()
	if v1 != v2 {
		t.Errorf("Resetuniq: after reset first values should be equal, got %d and %d", v1, v2)
	}
}

func TestUniq8odd_NonZero(t *testing.T) {
	Resetuniq()
	for i := 0; i < 20; i++ {
		v := Uniq8odd()
		if v == 0 {
			t.Errorf("Uniq8odd() returned zero at iteration %d", i)
		}
	}
}

func TestUniq16odd_NonZero(t *testing.T) {
	Resetuniq()
	for i := 0; i < 20; i++ {
		v := Uniq16odd()
		if v == 0 {
			t.Errorf("Uniq16odd() returned zero at iteration %d", i)
		}
	}
}

func TestUniq8_Increments(t *testing.T) {
	Resetuniq()
	v1 := Uniq8()
	v2 := Uniq8()
	if v2 <= v1 && v2 != 0 { // allow wrap-around to 0 on overflow
		if v2 != 0 {
			t.Errorf("Uniq8: v2 (%d) should be > v1 (%d)", v2, v1)
		}
	}
}

func TestUniq16_Increments(t *testing.T) {
	Resetuniq()
	v1 := Uniq16()
	v2 := Uniq16()
	if v2 <= v1 && v2 != 0 {
		t.Errorf("Uniq16: v2 (%d) should be > v1 (%d)", v2, v1)
	}
}

func TestUniq8odd_Concurrent(t *testing.T) {
	Resetuniq()
	const goroutines = 50
	results := make([]byte, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = Uniq8odd()
		}()
	}
	wg.Wait()
	// All values should be non-zero
	for _, v := range results {
		if v == 0 {
			t.Errorf("Uniq8odd concurrent: got zero value")
		}
	}
}

func TestUniq16odd_Concurrent(t *testing.T) {
	Resetuniq()
	const goroutines = 50
	results := make([]uint16, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = Uniq16odd()
		}()
	}
	wg.Wait()
	// All values should be non-zero
	for _, v := range results {
		if v == 0 {
			t.Errorf("Uniq16odd concurrent: got zero value")
		}
	}
}
