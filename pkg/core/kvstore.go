package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
)

// MapJSONToStruct unmarshals a JSON string into the provided target struct.
//
// jsonStr: the JSON string to unmarshal.
// target: a pointer to the struct to populate.
// Returns an error if unmarshaling fails.
func MapJSONToStruct(jsonStr string, target interface{}) error {
	return json.Unmarshal([]byte(jsonStr), target)
}

// MapJSONToStructAsList unmarshals a JSON array into the provided target struct,
// mapping array elements to struct fields in order.
//
// jsonStr: the JSON array string to unmarshal.
// target: a pointer to the struct to populate.
// Returns an error if unmarshaling fails or the target is not a pointer to struct.
func MapJSONToStructAsList(jsonStr string, target interface{}) error {
	var list []interface{}
	err := json.Unmarshal([]byte(jsonStr), &list)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return errors.New("target must be a pointer to struct")
	}

	v = v.Elem()
	for i := 0; i < v.NumField() && i < len(list); i++ {
		field := v.Field(i)
		if field.CanSet() {
			val := reflect.ValueOf(list[i])
			if val.Type().AssignableTo(field.Type()) {
				field.Set(val)
			} else if val.CanConvert(field.Type()) {
				field.Set(val.Convert(field.Type()))
			}
		}
	}
	return nil
}

// ListString is a slice of strings for convenience.
type ListString []string

// KVStore is a simple, thread-safe in-memory key-value store.
type ListString []string

type KVStore struct {
	store map[string]interface{}
	mu    sync.RWMutex
}

// NewKVStore creates and returns a new KVStore.
//
// Returns a pointer to a new KVStore instance.
func NewKVStore() *KVStore {
	return &KVStore{
		store: make(map[string]interface{}),
	}
}

// Set sets the value for a key in the store.
//
// key: the key to set.
// value: the value to associate with the key.
func (kv *KVStore) Set(key string, value interface{}) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.store[key] = value
}

// Get retrieves the value for a key from the store.
//
// key: the key to retrieve.
// Returns the value and a boolean indicating if the key exists.
func (kv *KVStore) Get(key string) (interface{}, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	val, ok := kv.store[key]
	return val, ok
}

// Delete removes a key from the store.
func (kv *KVStore) Delete(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.store, key)
}
