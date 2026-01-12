package main

import (
	"fmt"
	"os"

	"dbruntime/pkg/core"
)

func main() {
	kv := core.NewKVStore()

	// Put some sample data
	kv.Set("name", "KV Store")
	kv.Set("version", "1.0")
	kv.Set("author", "Quadgate")

	// Simple CLI: kvstore set <key> <value> | kvstore get <key> | kvstore delete <key>
	if len(os.Args) < 2 {
		fmt.Println("Usage: kvstore <set|get|delete> [key] [value]")
		return
	}

	switch os.Args[1] {
	case "set":
		if len(os.Args) != 4 {
			fmt.Println("Usage: kvstore set <key> <value>")
			return
		}
		kv.Set(os.Args[2], os.Args[3])
		fmt.Printf("Set key '%s' to '%s'\n", os.Args[2], os.Args[3])
	case "get":
		if len(os.Args) != 3 {
			fmt.Println("Usage: kvstore get <key>")
			return
		}
		val, ok := kv.Get(os.Args[2])
		if ok {
			fmt.Printf("Value: %v\n", val)
		} else {
			fmt.Println("Key not found")
		}
	case "delete":
		if len(os.Args) != 3 {
			fmt.Println("Usage: kvstore delete <key>")
			return
		}
		kv.Delete(os.Args[2])
		fmt.Printf("Deleted key '%s'\n", os.Args[2])
	default:
		fmt.Println("Unknown command. Use set, get, or delete.")
	}
}
