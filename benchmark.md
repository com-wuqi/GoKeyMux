# GoKeyMux 性能基准报告

- 日期：2026-10-20
- 基线代码：`8f12e5f`（工作区另含本次新增的基准与文档）
- 基准实现：`internal/engine/bench_helpers_test.go`、`internal/input/keymap_bench_test.go`、`internal/engine/engines_bench_test.go`、`internal/service/grpc_bench_test.go`、`internal/engine/realbackend_bench_test.go`、`bench/e2e.ps1`
- 快速上手与摘要：[`README.md` 的“性能基准”章节](README.md#性能基准)

---

## 1. 摘要

| 结论 | 数据 |
|------|------|
| 单次按键 submit 成本 | **约 1.14µs**，3 次分配 / 136B |
| press+release 往返 | **约 2.26µs**，约 44 万对/s |
| 键盘稳态吞吐上限 | 单 worker 串行，约 **88 万次 submit/s** |
| 并发提交 | 16 生产者降至约 **66.5 万 ops/s**，没有收益 |
| `dispatchQueueSize` | 1 → 65536 对稳态吞吐无影响（2265–2298ns） |
| 键鼠并行 | 串行 78.4 万 → 并发 85.5 万 ops/s（**+9%**） |
| 释放 held 键 | 精确命中 20ns；128 键全量扫描 97ns |
| 延迟分布（press+release） | p50 **1.4µs** / p90 **4.5µs** / p99 **8.0µs** |
| gRPC unary（bufconn） | **35.7µs/次**；100 条/流时约 **3µs/条** |
| 关闭并释放 32 个 held 键 | **2.6µs** |
| 排空 1024 条队列 | **74µs** |
| 真实后端单事件 | makc（SendInput）**965µs** / fakerInput **59.2µs** / Interception **3.2µs** |

一句话总结：**引擎本身很快（微秒级），真实场景的吞吐由单键盘 worker 和前端驱动/传输成本决定；`dispatchQueueSize` 只影响突发吸收，gRPC 单次调用成本远高于引擎成本，高频场景应使用长流。**

---

## 2. 测试环境

| 项目 | 值 |
|------|-----|
| OS | Windows 11 build 26200 / amd64 |
| CPU | 13th Gen Intel Core i7-13620H（10 核 / 16 线程，基频 2.4GHz） |
| 内存 | 16GB |
| Go | 1.27.1 |
| `GOMAXPROCS` | 16（默认） |
| 采集参数 | `-benchtime 500ms -count 3`，取中位数 |
| 后端 | `noop`（不做真实注入，可配置模拟延迟） |
| 客户端/服务端 | 同进程（`bufconn`）或同机 loopback（端到端脚本） |

> 本机 `time.Now` 的观测粒度约为 0.5ms（RDP/虚拟机环境常见），因此逐次延迟分布使用 Windows QPC（`QueryPerformanceCounter`）计时。绝对值只适合同机前后对比。

---

## 3. 方法与口径

### 3.1 分层

| 层级 | 覆盖内容 | 是否注入真实输入 |
|------|----------|------------------|
| 映射层 | 键名/rune/chord 解析、三套编码查表 | 否 |
| 引擎层 | submit、队列、held 状态、取消、drain、关闭、键鼠并发 | 否（noop） |
| 服务层 | bufconn 上的 unary / client-stream / mouse-stream | 否（noop） |
| 端到端 | loopback TCP + `cmd/loadtest` | 否（noop） |
| 真实后端 | makc / fakerInput / winput 实际注入 | 是（手动） |

### 3.2 指标含义

- `ns/op`：一次基准迭代的平均耗时。注意不同用例的“一次迭代”含义不同：
  - `EngineSubmitSingle`：1 次 submit；
  - `BenchmarkEngineSubmitNoop` / `BenchmarkEngineSubmitQueueSize` / `BenchmarkEngineKeyChord` / `BenchmarkEngineLatencySweep`：press + release（2 次 submit）；
  - `GRPCStreamKeyService/msgsN`：一条包含 N 条消息的流；
  - `GRPCMouseService`：一条包含 32 个事件的流。
- `ops/s`：由实际完成的操作数除以测量时长，避免多操作迭代造成误读。
- `B/op`、`allocs/op`：由 `-benchmem` 统计。
- `p50-ns` / `p90-ns` / `p99-ns` / `max-ns`：逐次采样后排序得到，仅 `EngineLatencyDistribution` 提供。

### 3.3 稳定性控制

- 每个包内的基准串行执行，不调用 `b.Parallel`；`config.GlobalConfig` 与默认 `slog` logger 在每个基准结束时恢复。跨包采集时使用 `go test -p 1`，避免多个测试二进制并行争抢 CPU。
- 关闭日志输出（`slog.DiscardHandler`），避免关闭时逐键释放的 warn 日志引入 I/O 抖动。
- 每个引擎基准使用独立 engine，测量前完成启动、预按等准备工作。
- 关闭/drain 基准的准备工作在计时器停止时完成；`-benchmem` 仍会统计这部分分配，因此忽略这两个用例的 `B/op`。
- 同一用例跑 3 次取中位数；端到端压测波动更大，仅作趋势参考。

### 3.4 复现

```powershell
# 全量快速跑（约 40s；-p 1 让各包串行）
go test -p 1 -run XXX -bench . -benchmem -benchtime 200ms -count 1 ./...

# 稳定数据（约 5-6 分钟）
go test -p 1 -run XXX -bench . -benchmem -benchtime 500ms -count 3 ./... | Tee-Object bench.txt

# 分层运行
go test -run XXX -bench "BenchmarkEngine" -benchmem ./internal/engine
go test -run XXX -bench "BenchmarkGRPC" -benchmem ./internal/service
go test -run XXX -bench "BenchmarkKeyCodes|BenchmarkSamePhysicalKey" -benchmem ./internal/input

# 并发争用
go test -run XXX -bench "BenchmarkEngineSubmit" -cpu 1,4,16 -count 5 ./internal/engine

# 对比改动
go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
```

---

## 4. 用例矩阵

### 4.1 映射层（`internal/input/keymap_bench_test.go`）

| 基准 | 子用例 | 目的 |
|------|--------|------|
| `KeyCodesFromName` | `lower` / `mixed_case` / `upper` / `miss` | 键名解析与 `strings.ToLower` 分配 |
| `KeyCodesFromRune` | 小写 / 大写 / 控制字符 / 非法 rune | 单 rune 映射 |
| `KeyCodesFromRunes` | 1 / 3 / 8 rune、未命中 | chord 解析的线性成本与分配 |
| `KeyCodesFromMakc` | 命中 / 未命中 | Windows VK 查表 |
| `KeyCodesFromWinput` | 命中 / 未命中 | Interception 扫描码查表 |
| `SamePhysicalKey` | 同键 / 修饰键不同 / 不同键 / 纯修饰键 | 释放路径的键身份比较 |

### 4.2 引擎层（`internal/engine/engines_bench_test.go`）

| 基准 | 目的 |
|------|------|
| `EngineSubmitSingle/{press_held,release_unheld}` | 拆分按下/释放，held-set 命中与未命中 |
| `EngineSubmitQueueSize/{q1,q8,q1024,q65536}` | 队列容量对热路径的影响 |
| `EngineKeyChord/{keys1,keys2,keys4}` | 单次 submit 内多键的线性成本 |
| `EngineEffectiveKeyRelease/{held1,32,128}/{exact_hit,same_physical_scan,miss_scan}` | held-set O(1) 命中与 O(n) 扫描 |
| `EngineLatencySweep/{0,100,1000,2000,5000}us` | 模拟后端成本与串行上限 |
| `EngineKeyboardParallel` / `EngineMouseParallel` | 各自 worker 的并发争用 |
| `EngineMixedWorkload/{serial,concurrent}` | 键鼠串行 vs 并发（验证 worker 独立性） |
| `EngineMouseEvents/{move_rel,move_abs,button_down_held,button_up_unheld,wheel,hwheel}` | 鼠标各事件类型 |
| `EngineCanceledContext` | 预取消 context 的快速失败 |
| `EngineDrainQueue` | 关闭时排空队列（1024 条） |
| `EngineCloseWithHeldKeys/{held1,held32}` | 关闭并释放 held 键 |
| `EngineLatencyDistribution` | QPC 逐次采样，输出 p50/p90/p99/max |
| `EngineSubmitNoop` / `EngineSubmitNoopParallel` | 原有基线（`engines_test.go`） |

### 4.3 服务层（`internal/service/grpc_bench_test.go`）

| 基准 | 目的 |
|------|------|
| `GRPCDispatch/{name,rune,chord4}` | 跳过传输，仅测 `dispatch` + 引擎 |
| `GRPCUnaryKeyServiceDebug` | 完整 unary 往返 |
| `GRPCUnaryParallel` | 并发 unary 客户端 |
| `GRPCStreamKeyService/{msgs1,msgs10,msgs100}` | 建流成本与每消息成本 |
| `GRPCMouseService` | 32 事件/流的鼠标路径 |

### 4.4 端到端与真实后端

| 入口 | 目的 |
|------|------|
| `bench/e2e.ps1` | noop 服务 + loopback TCP + `cmd/loadtest` 矩阵，输出 CSV |
| `internal/engine/realbackend_bench_test.go`（`-tags realbackend`） | makc / fakerInput / winput 的真实注入延迟 |

---

## 5. 结果

> 以下均为 `noop` 后端、`-benchtime 500ms -count 3` 的中位数，除非另行标注。

### 5.1 映射层

| 基准 | ns/op | B/op | allocs/op | 说明 |
|------|------:|-----:|----------:|------|
| `KeyCodesFromName/lower` | 13.2 | 0 | 0 | 小写键名，零分配 |
| `KeyCodesFromName/miss` | 35.8 | 0 | 0 | 长字符串查找，零分配 |
| `KeyCodesFromName/mixed_case` | 36.2 | 8 | 1 | `strings.ToLower` 分配 |
| `KeyCodesFromName/upper` | 34.5 | 8 | 1 | 同上 |
| `KeyCodesFromRune/lower_ascii` | 8.7 | 0 | 0 | map 命中 |
| `KeyCodesFromRune/shifted_ascii` | 8.4 | 0 | 0 | 带 Shift 的 rune |
| `KeyCodesFromRune/control_enter` | 15.7 | 0 | 0 | `\n` |
| `KeyCodesFromRune/unsupported` | 13.2 | 0 | 0 | 未命中 |
| `KeyCodesFromRunes/single` | 16.8 | 0 | 0 | 1 rune |
| `KeyCodesFromRunes/chord3` | 36.9 | 0 | 0 | 3 rune |
| `KeyCodesFromRunes/chord8` | 128.8 | 48 | 1 | 8 rune，slice 扩容 |
| `KeyCodesFromRunes/miss` | 36.1 | 0 | 0 | 中途失败 |
| `KeyCodesFromMakc/hit` | 17.4 | 0 | 0 | VK 查表 |
| `KeyCodesFromMakc/miss` | 15.0 | 0 | 0 | 未命中 |
| `KeyCodesFromWinput/hit` | 17.1 | 0 | 0 | 扫描码查表 |
| `KeyCodesFromWinput/miss` | 14.3 | 0 | 0 | 未命中 |
| `SamePhysicalKey/same_code` | 0.56 | 0 | 0 | 内联后几乎零成本 |
| `SamePhysicalKey/modifier_only` | 0.61 | 0 | 0 | 纯修饰键 |

观察：

- 纯映射成本在**纳秒级**，相对于 1.14µs 的 submit 可以忽略。
- 唯一的常规分配来自键名大小写规范化（8B/次）；高频路径应传小写键名。
- 8 rune 的 chord 才出现 slice 扩容分配，实际使用中很少见。

### 5.2 引擎热路径与队列

| 基准 | ns/op | B/op | allocs/op | ops/s |
|------|------:|-----:|----------:|------:|
| `EngineSubmitSingle/press_held` | 1140 | 136 | 3 | 881,074 |
| `EngineSubmitSingle/release_unheld` | 1141 | 136 | 3 | 876,587 |
| `EngineSubmitNoop`（press+release） | 2256 | 272 | 6 | 约 886,000（2 次 submit） |
| `EngineSubmitQueueSize/q1` | 2298 | 256 | 4 | 878,948 |
| `EngineSubmitQueueSize/q8` | 2285 | 256 | 4 | 875,155 |
| `EngineSubmitQueueSize/q1024` | 2285 | 256 | 4 | 900,572 |
| `EngineSubmitQueueSize/q65536` | 2265 | 256 | 4 | 905,821 |
| `EngineKeyChord/keys1` | 2260 | 256 | 4 | 889,787 |
| `EngineKeyChord/keys2` | 2515 | 256 | 4 | 835,134 |
| `EngineKeyChord/keys4` | 2903 | 256 | 4 | 718,578 |

说明：

- `EngineSubmitSingle` 按真实调用方式每次构造新的 `[]KeyCodes`，所以是 **3 allocs / 136B**；`benchPressRelease` 复用同一个 slice，因此队列/chord 用例是每次 submit 2 allocs / 128B。真实 gRPC 路径每次 `dispatch` 也会新建 slice，与 `GRPCDispatch` 的 3 allocs 一致。
- 队列容量从 1 到 65536，`ns/op` 差异在 ±1.5% 内，**稳态吞吐与队列容量无关**；队列只影响突发吸收和饱和时的排队延迟。
- 4 键 chord 相对单键多约 640ns，约等于 4 次后端调用的线性叠加。

### 5.3 held 状态与释放

| 场景 | held=1 | held=32 | held=128 |
|------|-------:|--------:|---------:|
| 精确命中（map） | 17.6ns | 19.7ns | 20.0ns |
| 同物理键但修饰键不同（扫描） | 13.5ns¹ | 39.1ns | 93.2ns |
| 完全未命中（扫描） | 13.7ns¹ | 37.3ns | 96.9ns |

¹ held=1 时是单元素扫描，成本与一次比较相当。

- 正常客户端“按下什么就释放什么”会命中 map，**O(1)**。
- 只有修饰键标志不一致（例如按下 `Shift+A`、释放 `a`）时才回退到线性扫描；128 个 held 键时仍不足 100ns，不会成为瓶颈。

### 5.4 模拟后端延迟与串行上限

`noopLatencyMicros` 会让 noop 后端在每次事件上 sleep，用来观察单 worker 的串行上限：

| 设置 | ns/op（press+release） | ops/s | 每事件近似 |
|------|-----------------------:|------:|-----------:|
| 0 | 2,379 | 854,685 | 1.19µs |
| 100µs | 1,254,512 | 1,609 | 0.63ms |
| 1ms | 3,181,220 | 636 | 1.59ms |
| 2ms | 4,903,460 | 412 | 2.45ms |
| 5ms | 10,896,918 | 185 | 5.45ms |

- 从 1ms 起基本符合 `2 × latency + 固定开销`，即吞吐 ≈ `1 / (latency + 1.14µs)` 的串行模型。
- 100µs 设置明显偏离，是因为本机 Windows 定时器粒度约 0.5ms；**小于 1ms 的模拟延迟不可信**，延迟扫描应以 ≥1ms 的数据为准。
- 这说明：真实后端的单事件成本直接决定吞吐上限；引擎的 1.14µs 开销在驱动延迟面前通常可以忽略。

### 5.5 并发争用与键鼠并行

| 基准 | ns/op | ops/s | B/op | allocs/op |
|------|------:|------:|-----:|----------:|
| `EngineKeyboardParallel` | 3,130 | 664,711 | 272 | 6 |
| `EngineMouseParallel` | 1,417 | 707,116 | 128 | 2 |
| `EngineMixedWorkload/serial` | 3,827 | 783,898 | 400 | 8 |
| `EngineMixedWorkload/concurrent` | 3,507 | 855,334 | 400 | 8 |

- 16 个生产者竞争同一个键盘 worker，吞吐从单生产者的约 88 万降到 **66.5 万 ops/s**：额外的 goroutine 唤醒与 channel 争用是纯开销。
- 鼠标 worker 独立，鼠标-only 约 **70.7 万 ops/s**。
- 键鼠并发相对串行只提升 **约 9%**，而不是理论上的 2×。两个 worker 确实独立，但 submit 的调度/唤醒成本占了主导；不要把“键鼠双 worker”当成吞吐翻倍的保证。

### 5.6 取消、drain 与关闭

| 基准 | 结果 | 说明 |
|------|------|------|
| `EngineCanceledContext` | 32.5ns / 8B / 1 alloc | 预取消 context，在分配响应 channel 前返回；8B 是调用点的 key slice |
| `EngineDrainQueue` | 74.2µs / 1024 条，约 **1380 万 ops/s** | 关闭时直接调用 `apply`，不再经过 submit 往返 |
| `EngineCloseWithHeldKeys/held1` | 1.81µs | 关闭 worker + 释放 1 个 held 键 |
| `EngineCloseWithHeldKeys/held32` | 2.60µs | 释放 32 个 held 键，约 12.3M keys/s |

关闭路径没有隐藏的 O(n²) 或阻塞点：即使按下几十个键后退出，释放也只需微秒级。

### 5.7 延迟分布

`EngineLatencyDistribution` 用 QPC 对每次 press+release 往返采样，`-benchtime 500ms -count 3`：

| 指标 | 值 |
|------|----|
| 平均（ns/op） | 2.51µs |
| p50 | 1.40µs |
| p90 | 4.50µs |
| p99 | 8.00µs |
| max | 377µs |

- 分布明显右偏：大多数请求在 1–2µs 完成，p99 约 8µs，偶发调度抖动会到几百微秒。
- 采样包含两次 QPC 读取的开销（几十纳秒），以及 Go benchmark 自身的循环成本。

### 5.8 gRPC 服务层（bufconn）

| 基准 | ns/op | B/op | allocs/op | 换算 |
|------|------:|-----:|----------:|------|
| `GRPCDispatch/name` | 1,175 | 136 | 3 | 约 85 万次/s |
| `GRPCDispatch/rune` | 1,260 | 136 | 3 | 约 83 万次/s |
| `GRPCDispatch/chord4` | 1,381 | 152 | 3 | 约 290 万键/s |
| `GRPCUnaryKeyServiceDebug` | 35,671 | 9,182 | 158 | 约 2.8 万次/s |
| `GRPCUnaryParallel` | 8,243 | 9,005 | 144 | 约 12.1 万次/s |
| `GRPCStreamKeyService/msgs1` | 39,865 | 9,658 | 165 | 约 2.5 万次/s |
| `GRPCStreamKeyService/msgs10` | 72,186 | 16,569 | 325 | 约 14.2 万条消息/s |
| `GRPCStreamKeyService/msgs100` | 299,321 | 84,108 | 1,878 | 约 34.3 万条消息/s |
| `GRPCMouseService`（32 事件/流） | 129,085 | 31,127 | 608 | 约 24.8 万事件/s |

观察：

- 进程内 `dispatch` 约 **1.2µs**，与直接调用引擎基本一致；映射和引擎不是瓶颈。
- 一次 unary 往返约 **35.7µs**，是引擎成本的 30 倍以上；单条流的建流成本也在 40µs 量级。
- 把 10 条消息放进一条流，每消息成本从 39.9µs 降到 **7.2µs**；100 条/流时约 **3µs/条**。高频输入应复用长流。
- 并发 unary（16 workers）把单次成本压到 8.2µs，说明服务端可以并行处理多个客户端，但最终仍受单键盘 worker 约束。

### 5.9 端到端 loopback（noop）

`bench/e2e.ps1`，2s/worker，`-Concurrency 1,8`：

| latency_us | mode | c | total | tps/rps | p50 | p99 |
|-----------:|------|---|------:|--------:|-----|-----|
| 0 | stream | 1 | 873,316 | 433,239 | — | — |
| 0 | stream | 8 | 999,894 | 451,066 | — | — |
| 0 | unary | 1 | 3,257 | 1,618 | 527.4µs | 13.2397ms |
| 0 | unary | 8 | 38,317 | 19,136 | 0s | 6.7452ms |

- stream 模式吞吐稳定在 43–45 万条/s，与 bufconn 的 34 万条/s（100 条/流）同一量级，差异来自 TCP 与客户端实现。
- unary 模式受每次 RPC 的往返与调度影响，单 worker 只有约 1.6 千次/s；8 workers 提升到约 1.9 万次/s。
- 本机 `time.Now` 粒度导致 unary 的 `p50` 出现 `0s`；真实百分位请参考 `EngineLatencyDistribution` 的 QPC 数据。

### 5.10 真实后端（本机实测）

环境：Windows 11 build 26200，前台为空白记事本，按键 `F12`，`-benchtime 100x -count 5`，取中位数。测试二进制生成在仓库根目录，因此 `winputWithInterception` 能找到 `Interception\library\x64\interception.dll`。

| 后端 | ns/op（press+release） | 5 次范围 | 单事件 | submit/s | B/op | allocs/op |
|------|-----------------------:|---------:|-------:|---------:|-----:|----------:|
| `noop`（参考） | 2,256 | — | 1.13µs | 886,000 | 272 | 6 |
| `makc`（SendInput） | 1,930,673 | 1.56–2.05ms | 965µs | 1,036 | 517 | 12 |
| `fakerInput` | 118,395 | 114.8–120.6µs | 59.2µs | 16,894 | 258 | 4 |
| `winputWithInterception` | 6,460 | 6.11–6.75µs | 3.23µs | 309,600 | 288 | 6 |

- **`makc` auto 后端不可用于持续压测。** 本机 auto 选择 `user32!InjectKeyboardInput`，在连续注入几十次后报
  `The data area passed to a system call is too small`（`ERROR_INSUFFICIENT_BUFFER`）；基准因此显式改用 SendInput。
  生产代码仍使用 auto 行为，未在本轮修改。
- **Interception 最快（约 3.2µs/事件）**：直接走内核 filter driver 的 IOCTL，比用户态 SendInput 和 HID `WriteFile` 都短。
- **FakerInput 约 59µs/事件**：主要成本在 HID 报告写入与 UMDF 虚拟设备路径。
- **makc SendInput 约 965µs/事件且波动较大（1.56–2.05ms）**：可能包含 user32 内部节流/调度；这是三者中最慢的路径。
- 相对 noop 引擎成本（1.13µs/事件）：makc 约 856×、fakerInput 约 52×、Interception 约 2.9×。这印证了前面的结论——**真实场景下驱动/注入路径是绝对瓶颈，引擎本身可以忽略**。
- 以上数据衡量的是“驱动调用 + 引擎往返”，不等于按键在窗口中的可见延迟。投递正确性已通过集成测试验证：
  `TestFakerInputIntegration`（API version / Tap / TypeText）与 `TestMakcReleaseMatchesHeldKeyIntegration`（SendInput + `GetAsyncKeyState`）均通过。


---

## 6. 分析

### 6.1 submit 成本拆解

一次 `EnginePress` 的主要步骤：

1. 检查 context（预取消时直接返回，32ns）。
2. 构造 `keyOp`，其中包含调用点产生的 `[]KeyCodes`。
3. 为响应创建 `chan error`（带 1 个缓冲）。
4. 发送到 `ops` channel，等待 worker 唤醒。
5. worker 执行 `apply`（noop 后端）+ 更新 held 状态。
6. 通过响应 channel 返回。

3 次分配 / 136B 中，响应 channel 和调用点 slice 是主要来源；`-benchmem` 对单次 submit 的观测与上述结构一致。

### 6.2 吞吐模型

```
键盘稳态吞吐 ≈ 1 / (后端单事件成本 + submit 开销 1.14µs)
```

- noop：约 88 万次/s（单生产者）。
- 1ms 后端：约 636 ops/s（press+release 对），即约 1.27 千次按键/s。
- 真实驱动通常在几十微秒到几毫秒/事件，引擎开销占比很小。

`dispatchQueueSize` 不改变上述上限，它只决定：

- 突发时最多缓存多少个请求；
- 队列满后提交方阻塞，饱和时延迟 ≈ 队列深度 × 每事件耗时。

默认 1024 足以吸收短突发；如果客户端会一次性灌入大量事件且不能阻塞，才需要增大。

### 6.3 键鼠并行

键盘和鼠标各有独立 worker，但共享 Go 调度器与 channel 唤醒成本。实测并发只比串行快 9%，因此：

- 不要把“双 worker”宣传为 2× 吞吐；
- 对延迟敏感的混合输入，收益主要来自“鼠标不被键盘长任务阻塞”，而不是总吞吐翻倍。

### 6.4 gRPC 成本

- 单次 unary 约 35.7µs，其中引擎只占 1.2µs；protobuf、gRPC 框架、goroutine 调度是主要成本。
- 流式复用可以把每消息成本降到 3µs 左右，接近引擎上限。
- `keyServiceDebug` 适合调试，不适合高频输入；生产客户端应使用 `keyService` 长流。

---

## 7. 已知限制与环境因素

1. **`time.Now` 粒度**：本机约 0.5ms，`cmd/loadtest` 的 unary 百分位会失真（出现 `p50=0s`）。基准内部已改用 QPC。
2. **Windows 定时器粒度**：`noopLatencyMicros < 1000` 会被量化到约 0.5–1ms，不能用于验证微秒级后端延迟。
3. **同进程 bufconn**：gRPC 基准的客户端与服务端共享 GOMAXPROCS；结果反映服务端处理成本，不等同于跨机网络性能。
4. **noop 后端**：不包含驱动/USB/HID 成本；真实后端延迟需用 `-tags realbackend` 或实际游戏/桌面场景验证。
5. **机器差异**：CPU 频率、电源计划、后台负载都会影响绝对值；本报告只用于同机前后对比。
6. **基准串行**：每个包内的基准串行执行以保护 `config.GlobalConfig` 与 logger；跨包采集请使用 `go test -p 1`，否则多个测试二进制会并行争抢 CPU。
7. **makc auto 后端**：Windows 11 build 26200 上 `InjectKeyboardInput` 在连续注入几十次后报 `ERROR_INSUFFICIENT_BUFFER`；真实后端基准改为显式 SendInput。生产 auto 行为未改，长期高频使用建议考虑 fallback。

---

## 8. 可选优化方向（未实施）

以下仅基于测量结果提出，尚未修改生产代码：

1. **复用响应 channel**：每次 submit 新建 channel 是主要分配之一，可用 `sync.Pool` 或每调用方复用的响应通道降低分配与 GC 压力。
2. **单键快速路径**：为单键 press/release 提供不构造 slice 的入口，省掉每次 8B 的调用点分配。
3. **键名小写缓存**：在客户端或服务端缓存小写键名，消除 `strings.ToLower` 的 8B 分配。
4. **批量提交**：如果上层一次产生多个事件，可以提供 batch API，把多次 worker 往返合并为一次；收益取决于调用方能否成批产生事件。
5. **长流优先**：在文档/客户端中默认使用 `keyService` 长流，避免大量 unary 调用。

任何优化都应附带 `benchstat` 的 before/after 数据，并确认关闭/取消/held 状态语义不变。

---

## 9. 附录：原始数据

以下为 `-benchtime 500ms -count 3` 的中位数汇总（`-` 表示未启用/不适用）。

### 9.1 引擎层

| 基准 | ns/op | B/op | allocs/op |
|------|------:|-----:|----------:|
| `EngineSubmitSingle/press_held` | 1140 | 136 | 3 |
| `EngineSubmitSingle/release_unheld` | 1141 | 136 | 3 |
| `EngineSubmitNoop` | 2256 | 272 | 6 |
| `EngineSubmitNoopParallel` | 2677 | 272 | 6 |
| `EngineSubmitQueueSize/q1` | 2298 | 256 | 4 |
| `EngineSubmitQueueSize/q8` | 2285 | 256 | 4 |
| `EngineSubmitQueueSize/q1024` | 2285 | 256 | 4 |
| `EngineSubmitQueueSize/q65536` | 2265 | 256 | 4 |
| `EngineKeyChord/keys1` | 2260 | 256 | 4 |
| `EngineKeyChord/keys2` | 2515 | 256 | 4 |
| `EngineKeyChord/keys4` | 2903 | 256 | 4 |
| `EngineEffectiveKeyRelease/held1/exact_hit` | 17.6 | 0 | 0 |
| `EngineEffectiveKeyRelease/held1/same_physical_scan` | 13.5 | 0 | 0 |
| `EngineEffectiveKeyRelease/held1/miss_scan` | 13.7 | 0 | 0 |
| `EngineEffectiveKeyRelease/held32/exact_hit` | 19.7 | 0 | 0 |
| `EngineEffectiveKeyRelease/held32/same_physical_scan` | 39.1 | 0 | 0 |
| `EngineEffectiveKeyRelease/held32/miss_scan` | 37.3 | 0 | 0 |
| `EngineEffectiveKeyRelease/held128/exact_hit` | 20.0 | 0 | 0 |
| `EngineEffectiveKeyRelease/held128/same_physical_scan` | 93.2 | 0 | 0 |
| `EngineEffectiveKeyRelease/held128/miss_scan` | 96.9 | 0 | 0 |
| `EngineLatencySweep/0us` | 2379 | 256 | 4 |
| `EngineLatencySweep/100us` | 1,254,512 | 752 | 10 |
| `EngineLatencySweep/1000us` | 3,181,220 | 752 | 10 |
| `EngineLatencySweep/2000us` | 4,903,460 | 752 | 10 |
| `EngineLatencySweep/5000us` | 10,896,918 | 753 | 10 |
| `EngineKeyboardParallel` | 3130 | 272 | 6 |
| `EngineMouseParallel` | 1417 | 128 | 2 |
| `EngineMixedWorkload/serial` | 3827 | 400 | 8 |
| `EngineMixedWorkload/concurrent` | 3507 | 400 | 8 |
| `EngineMouseEvents/move_rel` | 1171 | 128 | 2 |
| `EngineMouseEvents/move_abs` | 1125 | 128 | 2 |
| `EngineMouseEvents/button_down_held` | 1185 | 128 | 2 |
| `EngineMouseEvents/button_up_unheld` | 1155 | 128 | 2 |
| `EngineMouseEvents/wheel` | 1125 | 128 | 2 |
| `EngineMouseEvents/hwheel` | 1180 | 128 | 2 |
| `EngineCanceledContext` | 32.5 | 8 | 1 |
| `EngineDrainQueue` | 74,203 | — | — |
| `EngineCloseWithHeldKeys/held1` | 1,813 | — | — |
| `EngineCloseWithHeldKeys/held32` | 2,604 | — | — |
| `EngineLatencyDistribution` | 2505 | 304 | 10 |

### 9.2 服务层与映射层

| 基准 | ns/op | B/op | allocs/op |
|------|------:|-----:|----------:|
| `GRPCDispatch/name` | 1175 | 136 | 3 |
| `GRPCDispatch/rune` | 1260 | 136 | 3 |
| `GRPCDispatch/chord4` | 1381 | 152 | 3 |
| `GRPCUnaryKeyServiceDebug` | 35,671 | 9,182 | 158 |
| `GRPCUnaryParallel` | 8,243 | 9,005 | 144 |
| `GRPCStreamKeyService/msgs1` | 39,865 | 9,658 | 165 |
| `GRPCStreamKeyService/msgs10` | 72,186 | 16,569 | 325 |
| `GRPCStreamKeyService/msgs100` | 299,321 | 84,108 | 1,878 |
| `GRPCMouseService` | 129,085 | 31,127 | 608 |
| `KeyCodesFromName/lower` | 13.2 | 0 | 0 |
| `KeyCodesFromName/mixed_case` | 36.2 | 8 | 1 |
| `KeyCodesFromName/upper` | 34.5 | 8 | 1 |
| `KeyCodesFromName/miss` | 35.8 | 0 | 0 |
| `KeyCodesFromRune/lower_ascii` | 8.7 | 0 | 0 |
| `KeyCodesFromRune/shifted_ascii` | 8.4 | 0 | 0 |
| `KeyCodesFromRune/control_enter` | 15.7 | 0 | 0 |
| `KeyCodesFromRune/unsupported` | 13.2 | 0 | 0 |
| `KeyCodesFromRunes/single` | 16.8 | 0 | 0 |
| `KeyCodesFromRunes/chord3` | 36.9 | 0 | 0 |
| `KeyCodesFromRunes/chord8` | 128.8 | 48 | 1 |
| `KeyCodesFromRunes/miss` | 36.1 | 0 | 0 |
| `KeyCodesFromMakc/hit` | 17.4 | 0 | 0 |
| `KeyCodesFromMakc/miss` | 15.0 | 0 | 0 |
| `KeyCodesFromWinput/hit` | 17.1 | 0 | 0 |
| `KeyCodesFromWinput/miss` | 14.3 | 0 | 0 |
| `SamePhysicalKey/same_code` | 0.56 | 0 | 0 |
| `SamePhysicalKey/same_code_shifted` | 0.59 | 0 | 0 |
| `SamePhysicalKey/different_code` | 0.56 | 0 | 0 |
| `SamePhysicalKey/modifier_only` | 0.61 | 0 | 0 |

### 9.3 真实后端（`-tags realbackend`）

| 后端 | ns/op（press+release） | B/op | allocs/op |
|------|-----------------------:|-----:|----------:|
| `makc`（SendInput） | 1,930,673 | 517 | 12 |
| `fakerInput` | 118,395 | 258 | 4 |
| `winputWithInterception` | 6,460 | 288 | 6 |

> `makc` 的 auto 后端（`InjectKeyboardInput`）在 Windows 11 build 26200 上无法完成持续压测，表中数据来自显式 SendInput。

---

## 10. 复现清单

1. 关闭占用 CPU 的后台程序，保持电源计划一致。
2. 运行 `go test ./...` 确认功能无回归。
3. 按第 3.4 节命令采集数据；如需对比改动，先用 `benchstat` 合并两次结果。
4. 端到端数据用 `bench/e2e.ps1` 采集，CSV 位于 `%TEMP%\gokeymux-bench\run-<时间戳>\results.csv`。
5. 真实后端数据只在手动模式下采集，并记录驱动版本、设备与按键目标窗口。

---

## 11. 相关文件

| 文件 | 说明 |
|------|------|
| `internal/engine/bench_helpers_test.go` | 基准公共工具（配置恢复、日志静音、吞吐/百分位、QPC） |
| `internal/input/keymap_bench_test.go` | 映射层基准 |
| `internal/engine/engines_bench_test.go` | 引擎层基准 |
| `internal/service/grpc_bench_test.go` | 服务层基准（bufconn） |
| `internal/engine/realbackend_bench_test.go` | 真实后端基准（`-tags realbackend`） |
| `bench/e2e.ps1` | 端到端 noop 压测脚本 |
| `README.md` | 快速上手、命令与摘要 |
