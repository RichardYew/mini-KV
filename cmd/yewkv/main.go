package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/RichardYew/mini-KV/internal/kvstore"
)

const usage = `=== YewKV 内存键值存储 ===
支持命令:
  set <key> <value> [ttl]  写入键值对，ttl 可选，如 10s / 2m / 1h
  get <key>                读取键值
  del <key>                删除键
  keys                     列出所有未过期的键
  exit                     退出并持久化到 data.json
示例: set name Tom 10s | get name | del name
------------------------`

func main() {
	store := kvstore.NewKVStore("data.json")
	defer store.Close()

	fmt.Println(usage)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		switch strings.ToLower(parts[0]) {
		case "exit", "quit":
			fmt.Println("Bye~ 数据已持久化到 data.json")
			return
		case "set":
			handleSet(store, parts)
		case "get":
			handleGet(store, parts)
		case "del":
			handleDel(store, parts)
		case "keys":
			handleKeys(store, parts)
		default:
			fmt.Println("未知命令，支持: set / get / del / keys / exit")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "读取输入失败:", err)
	}
}

func handleSet(store *kvstore.KVStore, parts []string) {
	if len(parts) < 3 {
		fmt.Println("用法: set <key> <value> [ttl]，示例: set name Tom 30s")
		return
	}

	key, value := parts[1], parts[2]
	var ttl time.Duration

	if len(parts) >= 4 {
		var err error
		ttl, err = time.ParseDuration(parts[3])
		if err != nil {
			fmt.Println("过期时间格式错误，示例: 10s, 2m, 1h")
			return
		}
	}

	store.Set(key, value, ttl)
	fmt.Println("OK")
}

func handleGet(store *kvstore.KVStore, parts []string) {
	if len(parts) != 2 {
		fmt.Println("用法: get <key>")
		return
	}

	value, err := store.Get(parts[1])
	if err != nil {
		fmt.Printf("(nil) - %v\n", err)
		return
	}
	fmt.Println(value)
}

func handleDel(store *kvstore.KVStore, parts []string) {
	if len(parts) != 2 {
		fmt.Println("用法: del <key>")
		return
	}

	store.Del(parts[1])
	fmt.Println("OK")
}

func handleKeys(store *kvstore.KVStore, parts []string) {
	if len(parts) != 1 {
		fmt.Println("用法: keys")
		return
	}

	keys := store.Keys()
	if len(keys) == 0 {
		fmt.Println("(empty)")
		return
	}
	sort.Strings(keys)
	fmt.Println(strings.Join(keys, " "))
}
