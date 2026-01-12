package core

import (
	"reflect"
	"strconv"
	"testing"
)

func TestArrayStore_ConcurrentAdd(t *testing.T) {
	s := NewArrayStore()
	done := make(chan struct{}, 1000)
	for i := 1; i <= 1000; i++ {
		go func(i int) {
			s.Add("Goroutine " + strconv.Itoa(i))
			done <- struct{}{}
		}(i)
	}
	// Wait for all goroutines to finish
	for i := 0; i < 1000; i++ {
		<-done
	}
	arr := s.Get()
	if len(arr) != 1000 {
		t.Fatalf("Expected 1000 elements, got %d", len(arr))
	}
	// Optionally, check for duplicates
	seen := make(map[string]bool)
	for _, v := range arr {
		if seen[v] {
			t.Errorf("Duplicate value: %q", v)
		}
		seen[v] = true
	}
}

func TestArrayStore_AddThousandAndQueryAll(t *testing.T) {
	s := NewArrayStore()
	for i := 1; i <= 1000; i++ {
		s.Add("Line " + strconv.Itoa(i))
	}

	arr := s.Get()
	if len(arr) != 1000 {
		t.Fatalf("Expected 1000 elements, got %d", len(arr))
	}
	// Check first and last element
	if arr[0] != "Line 1" || arr[999] != "Line 1000" {
		t.Errorf("Unexpected values: got first=%q last=%q", arr[0], arr[999])
	}
}

func TestArrayStore_AddAndGet(t *testing.T) {
	s := NewArrayStore()
	s.Add("foo")
	s.Add("bar")
	got := s.Get()
	want := []string{"foo", "bar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Add/Get failed: got %v, want %v", got, want)
	}
}

func TestArrayStore_SetAndClear(t *testing.T) {
	s := NewArrayStore()
	arr := []string{"a", "b", "c"}
	s.Set(arr)
	if !reflect.DeepEqual(s.Get(), arr) {
		t.Errorf("Set failed: got %v, want %v", s.Get(), arr)
	}
	s.Clear()
	if len(s.Get()) != 0 {
		t.Errorf("Clear failed: got %v, want []", s.Get())
	}
}

func TestArrayStore_Sort(t *testing.T) {
	s := NewArrayStore()
	unsorted := []string{"banana", "apple", "cherry"}
	s.Set(unsorted)
	s.Sort()
	got := s.Get()
	want := []string{"apple", "banana", "cherry"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Sort failed: got %v, want %v", got, want)
	}
}
