package main

import (
	"runtime"
	"testing"
)

// --- SCENARIO A: The "Boring" Struct ---
type TodoStruct struct {
	ID   int
	Text string
	Done bool
}

type UserSessionStruct struct {
	UserID int
	Todos  []TodoStruct // Contiguous memory! GC loves this.
}

// --- SCENARIO B: The Signal Graph ---
type Signal[T any] struct {
	val  T
	subs []func() // Pointers to closures! GC hates this.
}

func NewSignal[T any](val T) *Signal[T] {
	return &Signal[T]{val: val}
}

type UserSessionSignals struct {
	UserID *Signal[int]
	Todos  []*Signal[TodoStruct] // Slice of pointers to objects
}

// --- BENCHMARKS ---

func BenchmarkGC_Structs(b *testing.B) {
	// Setup: Create 10k users with 50 items each
	sessions := make([]UserSessionStruct, 10_000)
	for i := range sessions {
		sessions[i] = UserSessionStruct{
			UserID: i,
			Todos:  make([]TodoStruct, 50),
		}
	}

	// Force GC to run and measure pause times
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.GC()
	}
}

func BenchmarkGC_Signals(b *testing.B) {
	// Setup: Create 10k users with 50 signals each
	sessions := make([]UserSessionSignals, 10_000)
	for i := range sessions {
		s := UserSessionSignals{
			UserID: NewSignal(i),
			Todos:  make([]*Signal[TodoStruct], 50),
		}
		for j := range s.Todos {
			// Allocating a new object for every single item
			s.Todos[j] = NewSignal(TodoStruct{ID: j})
		}
		sessions[i] = s
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.GC()
	}
}
