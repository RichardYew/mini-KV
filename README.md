# mini-KV（YewKV）

一个用纯 Go 标准库实现的**命令行内存键值存储**，支持 `set / get / del / keys` 交互命令、TTL 自动过期、JSON 快照持久化，是 Go 筑基阶段的练手项目，也是后续实现 Mini Redis 的地基。

- 无第三方依赖，只用标准库
- 并发安全（`sync.RWMutex` + 后台清理 goroutine）
- 退出自动持久化，重启自动加载

## 目录结构

```
mini-KV/
├── cmd/
│   └── yewkv/
│       └── main.go            # 命令行 REPL 入口
├── internal/
│   └── kvstore/
│       ├── kvstore.go         # 核心存储：Set/Get/Del/Keys + 过期清理协程
│       ├── persist.go         # JSON 持久化：Save/load（原子写入）
│       └── kvstore_test.go    # 单元测试（表驱动 + 并发压测）
├── go.mod
├── .gitignore
├── LICENSE
└── README.md
```

- `cmd/`：Go 社区约定，存放可执行程序入口。
- `internal/`：语言级私有包语义，外部项目无法导入，核心逻辑与入口解耦，方便后续改造成 TCP 服务。

## 快速开始

环境要求：Go 1.20+（开发环境为 Go 1.27.1 / WSL2 Ubuntu 24.04）。

```bash
# 克隆并进入项目
git clone https://github.com/RichardYew/mini-KV.git
cd mini-KV

# 运行
go run ./cmd/yewkv

# 或先编译再运行
go build -o yewkv ./cmd/yewkv
./yewkv
```

## 命令说明

| 命令 | 说明 | 示例 |
| --- | --- | --- |
| `set <key> <value> [ttl]` | 写入键值对，ttl 可选（如 `10s`、`2m`、`1h`），不填则永不过期 | `set name Tom 30s` |
| `get <key>` | 读取键值，键不存在返回 `key not found`，已过期返回 `key expired` | `get name` |
| `del <key>` | 删除键 | `del name` |
| `keys` | 列出所有未过期的键 | `keys` |
| `exit` / `quit` | 退出并把未过期数据写入 `data.json` | `exit` |

会话示例：

```text
=== YewKV 内存键值存储 ===
支持命令:
  set <key> <value> [ttl]  写入键值对，ttl 可选，如 10s / 2m / 1h
  get <key>                读取键值
  del <key>                删除键
  keys                     列出所有未过期的键
  exit                     退出并持久化到 data.json
示例: set name Tom 10s | get name | del name
------------------------
> set name Tom
OK
> set age 25 30s
OK
> get name
Tom
> keys
age name
> del name
OK
> get name
(nil) - key not found
> exit
Bye~ 数据已持久化到 data.json
```

## 持久化

退出时未过期的数据写入工作目录下的 `data.json`（启动时自动加载），格式为缩进 JSON：

```json
{
  "name": {
    "value": "Tom",
    "expire_at": "0001-01-01T00:00:00Z"
  },
  "age": {
    "value": "25",
    "expire_at": "2026-10-03T00:30:00+08:00"
  }
}
```

- `expire_at` 为 Go 时间零值（`0001-01-01T00:00:00Z`）表示永不过期。
- 保存只写未过期的键，加载时也会过滤已过期条目。
- 写入采用「临时文件 + rename」两步策略，避免进程中途失败留下残缺文件。

## 测试

```bash
# 全量测试（含 -race 数据竞争检测）
go test -race ./...

# 详细输出 / 只跑某个用例
go test ./internal/kvstore -v
go test -run TestExpire -v

# 静态检查与格式化
go vet ./...
gofmt -l .
```

覆盖用例：基础读写、键不存在、TTL 过期、后台协程清理、删除、键列表、持久化重启恢复、并发读写压测。

## 实现要点

| 模块 | 做法 | 对应知识点 |
| --- | --- | --- |
| 并发安全 | `map` + `sync.RWMutex`，读用 `RLock`、写用 `Lock`，`defer Unlock` | Go map 非线程安全；读写锁读共享写互斥 |
| 自动过期 | `Set` 时记录 `ExpireAt`；`Get/Keys` 惰性判断；后台 goroutine 每秒 `time.Ticker` 扫描删除 | goroutine、`select` 多路复用、channel 信号 |
| 优雅关闭 | `Close()` 关闭 `stopChan` 广播停止 → `WaitGroup` 等待协程退出 → `Save()` 落盘 | channel 关闭即广播、`WaitGroup` 类似 `CountDownLatch` |
| 持久化 | `encoding/json` 序列化 + `os.ReadFile` / 临时文件 rename | 显式错误返回、`defer` 释放资源 |
| CLI | `bufio.Scanner` 逐行读 `os.Stdin`，`strings.Fields` 切分后 `switch` 分发 | 切片、字符串处理、REPL 模式 |
| 工程 | `go mod` + `cmd/` `internal/` 分层 + 表驱动测试 | 模块化、包封装（小写私有）、`go test` |

### Go vs Java 差异速记

- **并发模型**：goroutine 是运行时调度的轻量级协程，通过 channel 通信（CSP），比线程便宜得多。
- **错误处理**：返回 `error` 显式处理，没有异常栈；错误用 `errors.New` 定义、`errors.Is` 判断。
- **defer**：绑定函数作用域、后进先出，类似 `finally` 但粒度不同。
- **引用类型**：`map`、切片、channel 必须 `make` 初始化，直接用 nil 会 panic。
- **接口**：非侵入式，实现全部方法即自动实现接口，无需声明 `implements`。
- **模块**：`go mod` 是原生依赖管理，没有 Maven 中央仓库的概念。

## 进阶方向

- 命令扩展：`incr`、`exists`、`flushall`、`ttl`
- 持久化增强：AOF 追加日志替代全量快照；按 TTL 定期落盘
- 网络化：改造成 TCP 服务并解析 RESP 协议，即 Mini Redis
- 性能：分段锁降低锁粒度；时间轮 / 最小堆优化过期扫描
- 数据结构：支持 list、hash、set、zset

## License

[MIT](LICENSE)
