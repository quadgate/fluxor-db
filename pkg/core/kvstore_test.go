package core

import (
	"reflect"
	"testing"
)

func TestKVStore_SetGetDelete(t *testing.T) {
	kv := NewKVStore()

	// Test Set and Get
	kv.Set("foo", "bar")
	val, ok := kv.Get("foo")
	if !ok {
		t.Fatalf("expected key 'foo' to exist")
	}
	if val != "bar" {
		t.Errorf("expected value 'bar', got '%v'", val)
	}

	// Test overwrite
	kv.Set("foo", 123)
	val, ok = kv.Get("foo")
	if !ok || val != 123 {
		t.Errorf("expected value 123, got '%v'", val)
	}

	// Test Delete
	kv.Delete("foo")
	_, ok = kv.Get("foo")
	if ok {
		t.Errorf("expected key 'foo' to be deleted")
	}

	// Test non-existent key
	_, ok = kv.Get("notfound")
	if ok {
		t.Errorf("expected key 'notfound' to not exist")
	}
}

type TestStruct struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestMapJSONToStruct(t *testing.T) {
	jsonStr := `{"name":"test","value":42}`
	var result TestStruct
	err := MapJSONToStruct(jsonStr, &result)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := TestStruct{Name: "test", Value: 42}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
