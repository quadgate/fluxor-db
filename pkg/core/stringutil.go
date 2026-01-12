package core

import "strings"

// StringUtil provides utility functions for string operations.
type StringUtil struct{}

// Reverse returns the reversed string.
func (StringUtil) Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Split splits the string s by the given separator and returns a slice of substrings.
func (StringUtil) Split(s, sep string) []string {
	return strings.Split(s, sep)
}

// ToUpper returns the string in uppercase.
func (StringUtil) ToUpper(s string) string {
	result := []rune(s)
	for i, c := range result {
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		}
	}
	return string(result)
}

// ToLower returns the string in lowercase.
func (StringUtil) ToLower(s string) string {
	result := []rune(s)
	for i, c := range result {
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		}
	}
	return string(result)
}
