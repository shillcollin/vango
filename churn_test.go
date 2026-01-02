package main

import (
	"math/rand"
	"testing"
)

// --- SHARED TYPES ---

type TodoStruct struct {
	ID   int
	Text string
	Done bool
}

type Signal[T any] struct {
	val  T
	subs []func() // Simulating subscription overhead
}

func NewSignal[T any](val T) *Signal[T] {
	return &Signal[T]{val: val}
}

// --- BENCHMARK A: Struct Churn ---
// Simulates: "Diffing" a simple struct.
// You just replace the struct in the slice.
func BenchmarkChurn_Structs(b *testing.B) {
	// Setup 10k users
	sessions := make([][]TodoStruct, 10_000)
	for i := range sessions {
		sessions[i] = make([]TodoStruct, 50)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate 1,000 active users (10% concurrency) performing 1 action each
		for j := 0; j < 1_000; j++ {
			userID := rand.Intn(10_000)
			// Struct update: Just a memory copy. Zero allocation.
			sessions[userID][0] = TodoStruct{ID: i, Text: "Updated", Done: true}
		}
	}
}

// --- BENCHMARK B: Signal Churn ---
// Simulates: A component re-rendering or a signal updating.
// In a reactive graph, updates often trigger new closures or nodes.
func BenchmarkChurn_Signals(b *testing.B) {
	// Setup 10k users
	sessions := make([][]*Signal[TodoStruct], 10_000)
	for i := range sessions {
		sessions[i] = make([]*Signal[TodoStruct], 50)
		for k := range sessions[i] {
			sessions[i][k] = NewSignal(TodoStruct{})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate 1,000 active users performing 1 action each
		for j := 0; j < 1_000; j++ {
			userID := rand.Intn(10_000)

			// Scenario 1: Just updating the value (Best case for Signals)
			// sessions[userID][0].val = TodoStruct{ID: i, Text: "Updated", Done: true}

			// Scenario 2: Re-allocating the signal/wrapper (Realistic case for Vango re-renders)
			// If Vango re-runs a function component, it likely recreates the signal wrappers
			// or the closures attached to them.
			sessions[userID][0] = NewSignal(TodoStruct{ID: i, Text: "New Node", Done: true})
		}
	}
}
