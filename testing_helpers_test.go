package main

import (
	"testing"
)

// assertNoErr fails the test if err is non-nil.
func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// assertErr fails the test if err is nil.
func assertErr(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// assertTrue fails the test if v is false.
func assertTrue(t *testing.T, v bool, msg string) {
	t.Helper()
	if !v {
		t.Fatal(msg)
	}
}

// assertFalse fails the test if v is true.
func assertFalse(t *testing.T, v bool, msg string) {
	t.Helper()
	if v {
		t.Fatal(msg)
	}
}

// assertEqual fails the test if want != got.
func assertEqual[T comparable](t *testing.T, want, got T) {
	t.Helper()
	if want != got {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// assertNotEmpty fails the test if s is empty.
func assertNotEmpty(t *testing.T, s, msg string) {
	t.Helper()
	if s == "" {
		t.Fatal(msg)
	}
}
