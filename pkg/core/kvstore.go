package core

import (
	"encoding/json"
	"errors"
	"reflect"
)

// MapJSONToStruct unmarshals a JSON string into the provided target struct.
func MapJSONToStruct(jsonStr string, target interface{}) error {
	return json.Unmarshal([]byte(jsonStr), target)
}

// MapJSONToStructAsList unmarshals a JSON array into the provided target struct,
// mapping array elements to struct fields in order.
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

// KVStore is a simple in-memory key-value store.
// ListString is a slice of strings for convenience.
type ListString []string

type KVStore struct {
	store map[string]interface{}
}

// NewKVStore creates and returns a new KVStore.
func NewKVStore() *KVStore {
	return &KVStore{
		store: make(map[string]interface{}),
	}
}

// Set sets the value for a key in the store.
func (kv *KVStore) Set(key string, value interface{}) {
	kv.store[key] = value
}

// Get retrieves the value for a key from the store.
// Returns the value and a boolean indicating if the key exists.
func (kv *KVStore) Get(key string) (interface{}, bool) {
	val, ok := kv.store[key]
	return val, ok
}

// Delete removes a key from the store.
func (kv *KVStore) Delete(key string) {
	delete(kv.store, key)
}
