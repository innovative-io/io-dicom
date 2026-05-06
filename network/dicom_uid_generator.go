package network

import "sync/atomic"

var uniqid int64 = 1

// Resetuniq resets the unique ID counter. Callers are responsible for
// ensuring no concurrent calls to Uniq* are in flight when this is called.
func Resetuniq() {
	atomic.StoreInt64(&uniqid, 1)
}

// uniq8 returns a new unique 8-bit value.
func uniq8() byte {
	v := atomic.AddInt64(&uniqid, 1)
	return byte(v & 0xff)
}

// Uniq8odd returns a new unique 8-bit value using the same increment pattern
// as the original implementation, but with atomic read-modify-write semantics.
func Uniq8odd() byte {
	for {
		old := atomic.LoadInt64(&uniqid)
		var next int64
		if old&0x01 == 1 {
			next = old + 1
		} else {
			next = old + 2
		}
		if atomic.CompareAndSwapInt64(&uniqid, old, next) {
			return byte(next & 0xff)
		}
	}
}

// Uniq16odd returns a new unique 16-bit value using the same increment pattern
// as the original implementation, but with atomic read-modify-write semantics.
func Uniq16odd() uint16 {
	for {
		old := atomic.LoadInt64(&uniqid)
		var next int64
		if old&0x01 == 1 {
			next = old + 1
		} else {
			next = old + 2
		}
		if atomic.CompareAndSwapInt64(&uniqid, old, next) {
			return uint16(next & 0xffff)
		}
	}
}
