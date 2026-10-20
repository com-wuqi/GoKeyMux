# GoKeyMux

GoKeyMux 是一个运行在 **Windows** 上的输入注入服务（键盘 / 鼠标输入多路复用器）。它通过 gRPC 暴露接口，接收“按键按下 /
释放”与鼠标事件请求，并转发到可插拔的输入注入后端。其目标是统一多种输入注入方案（驱动 / 虚拟 HID /
窗口消息）为同一个接口，方便游戏自动化等本机场景。

> [!WARNING]
> **仅限本机使用 (Local Use Only)**  
> 这是一个本地按键映射服务， **不提供 TLS 能力**，也没有任何认证机制。请务必只监听回环地址（如 `localhost:50051`、
`127.0.0.1:50051`）， **绝对不要**绑定 `0.0.0.0`、空主机名或局域网
IP。若监听地址不是回环地址，服务启动时会打印警告。将服务暴露到本机之外会让网络上的其他主机直接驱动本机按键，风险自负。

> [!CAUTION]
> **反作弊与合规使用声明 (Anti-Cheat & Compliance Disclaimer)**  
> **GoKeyMux** 是一个通用的本地按键注入与多路复用服务。其设计初衷是为 **自动化测试、开发者调试**以及合法的本地输入映射场景提供统一的技术方案。
>
> 为了维护良好的技术生态并遵守相关法律法规，请在使用本项目前仔细阅读以下声明：
>
> 1. **禁止用于作弊或违规行为**：本项目 **绝不鼓励、支持或容忍**
     将本软件用于任何形式的游戏作弊、破坏在线游戏公平性，或违反任何第三方软件/游戏的服务条款（ToS）及最终用户许可协议（EULA）的行为。
> 2. **反作弊系统兼容性与风险提示**：本项目提供的某些输入后端（如 `winputWithInterception` 内核级驱动或 `fakerInput` 虚拟
     HID 设备）可能会被现代反作弊系统（如 Easy Anti-Cheat, BattlEye, Vanguard 等）或安全软件标记为可疑行为。本项目
     **不提供、也永远不会提供**任何绕过、禁用或对抗反作弊系统的功能（即不提供任何形式的“免杀”支持）。若因使用本软件导致您的游戏账号被封禁、限制或产生其他后果，
     **风险与责任由您自行承担**。
> 3. **免责声明**：本项目按“原样”提供，不提供任何明示或暗示的保证。作者及贡献者不对因使用或滥用本软件而导致的任何直接、间接、偶然的损害（包括但不限于数据丢失、账号封禁、系统不稳定或法律纠纷）承担任何责任。
> 4. **推荐的合法使用场景**：本地软件的自动化 UI 测试、开发者在受控环境下的输入协议调试。
>
> *继续使用、构建或分发本软件，即表示您已阅读、理解并同意遵守上述声明。如果您打算将本软件用于任何违规用途，请立即停止使用。*

## 特性

- **gRPC 服务**：提供客户端流式 `keyService`、`MouseService` 与一元 `keyServiceDebug` 三个 RPC。
- **可插拔后端**：内置 5 种输入注入后端，通过配置切换。
- **统一按键映射**：一套逻辑按键同时映射三种编码（Windows 虚拟键码 / Interception 扫描码 / USB HID 键码）。
- **安全退出**：正常退出时自动释放所有仍按下的键盘键与鼠标按钮，避免残留卡键。
- **服务治理**：健康检查、keepalive 策略、请求日志拦截器、pprof 性能分析。
- **配置驱动**：运行期通过 `config.json` 配置（缺失时自动生成默认配置）。
- **配套工具**：`loadtest` 压测客户端与 `keycycle` 演示客户端。

## 输入注入后端

通过 `config.json` 中的 `driveName` 选择：

| driveName                | 实现方式                                               | 依赖 / 说明                                         |
|--------------------------|--------------------------------------------------------|-----------------------------------------------------|
| `makc`                   | [aiwaki/makc](https://github.com/aiwaki/makc) 键盘注入 | 无需额外驱动（默认后端）                            |
| `winputWithWindow`       | [rpdg/winput](https://github.com/rpdg/winput) 窗口消息 | 按进程名查找目标窗口发送按键；需要目标进程在运行    |
| `winputWithInterception` | winput + Interception 驱动                             | 内核级注入，抗检测较好；需安装 Interception 驱动    |
| `fakerInput`             | FakerInput 虚拟 HID 设备                               | 自行枚举并写入 HID 报告；需安装 FakerInput 虚拟设备 |
| `noop`                   | 空操作                                                 | 不注入任何按键，仅用于压测服务层；可配置模拟延迟    |

## 先决条件

- **操作系统**：Windows（仅支持 x86 / x64，ARM64 不支持）。
- **Go**：1.27 或更高版本（见 `go.mod`）。
- **按后端可选依赖**：
    - `winputWithInterception`：安装 Interception 驱动。以 **管理员身份**打开 cmd，运行
      `Interception\command line installer\install-interception.exe /install` 即可完成安装
      （`/uninstall` 卸载；不带参数运行可查看帮助信息）；无需移动或改动 `Interception` 目录。
    - `fakerInput`：安装 FakerInput 虚拟 HID 设备，运行
      `FakerInput_Setup_0.1.1_x64.msi`。
    - `winputWithWindow`：需要目标进程在运行（默认查找进程名 `Game.exe`）。
    - `makc` / `noop`：无需额外依赖。

## 构建

```powershell
# 服务端
go build -o GoKeyMux.exe .

# 演示客户端：按固定间隔循环按下/释放 a-z
go build -o keycycle.exe ./cmd/keycycle

# 压测客户端
go build -o loadtest.exe ./cmd/loadtest
```

> 说明：
> - 仓库通过 `.gitignore` 忽略了编译产物（`*.exe`）与 `config.json`，未提交二进制文件，请从源码自行构建（见上方命令）。
> - `winputWithInterception` 后端按可执行文件所在目录定位 `Interception/library/...` 下的 DLL；在仓库根目录直接构建运行即可，无需移动
    `Interception` 目录。

### 构建时注入版本信息

`GoKeyMux.exe` 支持在构建时通过 `-ldflags` 注入版本号（git tag）、打包时间（UTC）与提交哈希，并在启动日志中打印。推荐使用仓库根目录的构建脚本：

```powershell
.\build.ps1
```

脚本会自动计算并注入：

- `Version`：`git describe --tags --always --dirty`（如 `v0.1.0`、`v0.1.0-2-g6262409`）
- `Commit`：`git rev-parse --short HEAD`
- `BuildTime`：UTC 时间戳（`yyyy-MM-ddTHH:mm:ssZ`）

手动构建也可自行拼接 `-ldflags`（注入目标见 `version/version.go` 顶部注释）。未注入时，提交哈希与提交时间会自动回退到 Go
工具链内嵌的 VCS 信息（版本号显示为 `dev`）。

在 GoLand 中可在 Run/Debug Configurations → `go build GoKeyMux` → Go tool arguments 里配置，例如
`-ldflags "-X GoKeyMux/version.Version=dev"`。

## 配置

首次运行（或删除 `config.json` 后运行）会自动生成默认配置。字段如下：

| 字段                                 | 类型   | 默认值            | 说明                                                        |
|--------------------------------------|--------|-------------------|-------------------------------------------------------------|
| `driveName`                          | string | `makc`            | 后端驱动，取值见上方表格                                    |
| `gRPCAddress`                        | string | `localhost:50051` | gRPC 监听地址（`tcp4`）；仅限回环地址，非回环地址会触发警告 |
| `logLevel`                           | string | `info`            | 日志等级：`debug` / `info` / `warn` / `error`               |
| `gRPCKeepaliveTime`                  | int    | `2`               | keepalive `Time`，单位秒                                    |
| `gRPCKeepaliveTimeOut`               | int    | `1`               | keepalive `Timeout`，单位秒                                 |
| `gRPCKeepaliveMaxConnectionIdle`     | int    | `2`               | keepalive `MaxConnectionIdle`，单位秒                       |
| `gRPCEnforcementPolicyMinTime`       | int    | `10`              | EnforcementPolicy `MinTime`，单位秒                         |
| `gRPCEnforcementPermitWithoutStream` | bool   | `true`            | EnforcementPolicy `PermitWithoutStream`                     |
| `gRPCServerShutdownTimeout`          | int    | `10`              | 优雅关闭超时，单位秒                                        |
| `winputWindowProcessName`            | string | `Game.exe`        | (`winputWithWindow`) 按进程名查找窗口                       |
| `winputWindowUseStaticIndex`         | bool   | `true`            | (`winputWithWindow`) 是否使用固定窗口索引                   |
| `winputWindowIndex`                  | int    | `0`               | (`winputWithWindow`) 窗口索引                               |
| `pprofAddress`                       | string | `""`              | 可选 pprof 监听地址，空表示关闭（如 `localhost:6060`）      |
| `noopLatencyMicros`                  | int    | `0`               | (`noop`) 每次按键模拟延迟，单位微秒                         |
| `dispatchQueueSize`                  | int    | `1024`            | 按键派发队列缓冲大小；满时发送方 FIFO 阻塞                  |

示例：

```json
{
  "driveName": "winputWithInterception",
  "gRPCAddress": "localhost:50051",
  "logLevel": "debug",
  "gRPCKeepaliveTime": 2,
  "gRPCKeepaliveTimeOut": 1,
  "gRPCKeepaliveMaxConnectionIdle": 2,
  "gRPCEnforcementPolicyMinTime": 10,
  "gRPCEnforcementPermitWithoutStream": true,
  "gRPCServerShutdownTimeout": 10,
  "winputWindowProcessName": "Game.exe",
  "winputWindowUseStaticIndex": true,
  "winputWindowIndex": 0,
  "pprofAddress": "",
  "noopLatencyMicros": 0,
  "dispatchQueueSize": 1024
}
```

## 运行

```powershell
# 启动服务（默认读取同目录 config.json）
.\GoKeyMux.exe
```

启动后即可通过 gRPC 客户端（Go / Python / C# 等任意语言，按 `proto/gokeymux.proto` 生成桩代码）调用。

> 正常退出（Ctrl+C / SIGTERM / 服务停止）会自动释放所有仍按下的键盘键和鼠标按钮。强制结束进程（任务管理器 /
> `taskkill`）或直接关闭控制台窗口不会执行清理，可能残留卡键。

### 演示客户端 `keycycle`

依次执行三组测试，用于快速验证链路：

1. **a-z 循环**：按 `-cycles` 轮数循环按下并释放 a-z。
2. **命名键测试**：逐个按下/释放 `-keys` 指定的键名（`isRune=false`）。
3. **鼠标测试**：发送一组鼠标事件（相对移动、左/中/右键点击、滚轮、绝对移动）。

```powershell
.\keycycle.exe -addr localhost:50051 -interval 5s -cycles 5 `
  -keys "space,enter,tab,escape,backspace,up,down,left,right" -keyDelay 300ms -mouseDelay 300ms
```

常用参数：

| 参数          | 默认值                                                | 说明                                           |
|---------------|-------------------------------------------------------|------------------------------------------------|
| `-addr`       | `localhost:50051`                                     | 服务端地址                                     |
| `-interval`   | `5s`                                                  | a-z 每轮循环之间的间隔                         |
| `-cycles`     | `5`                                                   | a-z 循环轮数                                   |
| `-keys`       | `space,enter,tab,escape,backspace,up,down,left,right` | 命名键测试的逗号分隔键名列表（`isRune=false`） |
| `-keyDelay`   | `300ms`                                               | 命名键事件之间的延时                           |
| `-mouseDelay` | `300ms`                                               | 鼠标事件之间的延时                             |

### 压测客户端 `loadtest`

```powershell
.\loadtest.exe -addr localhost:50051 -mode stream -c 10 -n 100000
```

常用参数：

| 参数          | 默认值            | 说明                                         |
|---------------|-------------------|----------------------------------------------|
| `-addr`       | `localhost:50051` | 服务端地址                                   |
| `-mode`       | `stream`          | `stream` / `unary` / `both`                  |
| `-c`          | `10`              | 并发 worker 数                               |
| `-n`          | `100000`          | 每个 worker 的消息/请求数（`-d > 0` 时忽略） |
| `-d`          | `0`               | 每个 worker 的持续时长；`> 0` 时覆盖 `-n`    |
| `-key`        | `space`           | 键名或 rune 字符串                           |
| `-isRune`     | `false`           | 将 `-key` 作为 rune 字符串处理               |
| `-isPressed`  | `true`            | 按下（`true`）或释放（`false`）              |
| `-warmup`     | `0`               | 测量前的预热时长                             |
| `-cpuprofile` | `""`              | 输出 CPU profile 文件                        |
| `-memprofile` | `""`              | 输出堆 profile 文件                          |
| `-pprof`      | `""`              | 客户端 pprof 监听地址（如 `localhost:6061`） |

## gRPC 接口

服务定义见 `proto/gokeymux.proto`（`package GoKeyMux`，服务名 `rpcKeyService`，健康检查服务名为 `GoKeyMux.rpcKeyService`）：

```proto
service rpcKeyService {
  rpc keyService (stream keyInput) returns (keyReturn);
  rpc keyServiceDebug (keyInputDebug) returns (keyReturnDebug);
  rpc MouseService (stream MouseEvent) returns (MouseReturn);
}
```

- `keyService`： **客户端流式**。客户端持续发送 `keyInput`，服务端逐个派发；流结束时返回 `keyReturn`，其中 `isAllDone` 为
  `true` 表示全程无派发错误。
- `keyServiceDebug`： **一元**，用于单次调试调用。
- `MouseService`： **客户端流式**。客户端持续发送 `MouseEvent`，服务端逐个派发；流结束时返回 `MouseReturn`，其中
  `isAllDone` 为 `true` 表示全程无派发错误。

`keyInput` 字段：

| 字段        | 类型   | 说明                                                  |
|-------------|--------|-------------------------------------------------------|
| `key`       | string | 键名（`isRune=false`）或 rune 字符串（`isRune=true`） |
| `isRune`    | bool   | 是否将 `key` 解释为 rune 字符串                       |
| `isPressed` | bool   | `true` 按下，`false` 释放                             |

`MouseEvent` 字段：

| 字段           | 类型  | 说明                                                                             |
|----------------|-------|----------------------------------------------------------------------------------|
| `type`         | enum  | 事件类型，取值见下表                                                             |
| `x`            | int32 | `MOVE` 时为相对移动像素增量；`ABS_MOVE` 时为归一化绝对坐标 0~10000；其余类型为 0 |
| `y`            | int32 | 同上，Y 轴                                                                       |
| `scroll_delta` | int32 | 滚轮增量，仅 `SCROLL` 有效；正数向上/向前，负数向下/向后                         |

`MouseEvent.type` 取值：

| 取值          | 说明                                     |
|---------------|------------------------------------------|
| `MOVE`        | 相对移动（`x`/`y` 为像素增量）           |
| `ABS_MOVE`    | 绝对移动（`x`/`y` 为归一化坐标 0~10000） |
| `LEFT_DOWN`   | 左键按下                                 |
| `LEFT_UP`     | 左键抬起                                 |
| `RIGHT_DOWN`  | 右键按下                                 |
| `RIGHT_UP`    | 右键抬起                                 |
| `MIDDLE_DOWN` | 中键按下                                 |
| `MIDDLE_UP`   | 中键抬起                                 |
| `SCROLL`      | 滚轮（使用 `scroll_delta`）              |
| `UNKNOWN`     | 未定义，忽略                             |

> 注意：鼠标注入仅 `makc`、`fakerInput`、`noop` 后端支持；`winputWithWindow` 与 `winputWithInterception`
> 后端不支持鼠标，会返回 "mouse not implemented" 错误。

## 按键与 rune 约定

- **键名模式**（`isRune=false`，大小写不敏感）：`enter`/`return`、`escape`/`esc`、`backspace`、`tab`、`space`、`capslock`、`f1`…
  `f12`、`up`/`down`/`left`/`right`、`home`/`end`、`pageup`/`pgup`/`pagedown`/`pgdn`、`insert`/`ins`、`delete`/`del`、
  `numlock`、`scrolllock`，以及修饰键 `ctrl`/`shift`/`alt`/`gui`（含 `lctrl`/`rctrl`/`lshift`/`rshift`/`lalt`/`ralt` 等左右变体，
  `gui`/`win`/`windows`/`super`/`meta` 等价）。
- **rune 模式**（`isRune=true`）：`key` 中的每个 rune 映射为一个逻辑键； **多个 rune 表示同时按下的组合键**（chord）。支持可打印
  ASCII 字母、数字、符号，以及控制字符 ` `、`\n`/`\r`（Enter）、`\t`（Tab）、`\b`（Backspace）、`\x1b`（Escape）。

> 注意：不同后端对键的表示能力不同。例如 `gui`/`win` 键在 `winputWithInterception` 后端不可表示，会返回错误；`fakerInput`
> 通过报告中的修饰键标志位（而非键码）表示修饰键。
>
> **已知限制**：rune 模式中需要 Shift 的大写字母与符号（如 `A`、`!`、`@`、`?`）依赖 `KeyCodes.Modifiers`（修饰键标志位），
> 该字段目前仅由 `fakerInput` 后端应用；`makc` 与 `winput` 后端会忽略它、只发送未加 Shift 的基键，因此这些字符在这两种
> 后端下会输出不正确（例如 `A` 会变成 `a`）。此问题暂不修复，请在使用 `makc`/`winput` 后端时避免依赖 Shift 修饰。

## 测试

```powershell
# 单元测试（键码映射、FakerInput 报告组装等，无需真实设备）
go test ./...

# 集成测试（FakerInput / makc，需对应设备或驱动，否则跳过）
go test -tags integration ./...

# 按包运行
go test ./internal/input
go test ./internal/engine
```

## 性能基准

基准分为四层：映射层、引擎层、服务层和端到端；另有默认不编译的真实后端基准。除最后一层外均**不会注入真实输入**。完整的方法、原始数据与分析见 [benchmark.md](benchmark.md)。

| 层级 | 文件 | 覆盖内容 |
|------|------|----------|
| 映射层 | `internal/input/keymap_bench_test.go` | `KeyCodesFromName/Rune/Runes`、makc/winput 查表、`SamePhysicalKey` |
| 引擎层 | `internal/engine/engines_bench_test.go` | submit 往返、队列容量、chord、held 状态、取消、drain、关闭、键鼠并发、延迟分布 |
| 服务层 | `internal/service/grpc_bench_test.go` | bufconn 上的 unary / client-stream / mouse-stream，以及进程内 `dispatch` |
| 端到端 | `bench/e2e.ps1` | 真实 TCP 回环 + `cmd/loadtest`（noop 后端），输出 CSV 与吞吐/百分位 |
| 真实后端 | `internal/engine/realbackend_bench_test.go` | makc / fakerInput / winput 的真实注入延迟（会产生真实按键，默认不编译） |

### 运行方式

```powershell
# 全量快速跑（约 40s；-p 1 串行执行各包，避免多个基准二进制互相干扰）
go test -p 1 -run XXX -bench . -benchmem -benchtime 200ms -count 1 ./...

# 稳定数据（约 5-6 分钟，README 中的参考值即由此得到）
go test -p 1 -run XXX -bench . -benchmem -benchtime 500ms -count 3 ./... | Tee-Object bench.txt

# 只跑某一层
go test -run XXX -bench "BenchmarkEngine" -benchmem ./internal/engine
go test -run XXX -bench "BenchmarkGRPC" -benchmem ./internal/service
go test -run XXX -bench "BenchmarkKeyCodes|BenchmarkSamePhysicalKey" -benchmem ./internal/input

# 观察并发争用（键盘只有单个 worker，增加 CPU 不会线性加速）
go test -run XXX -bench "BenchmarkEngineSubmit" -cpu 1,4,16 -count 5 ./internal/engine

# 与改动前对比（可选，不引入仓库依赖）
go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
```

> 基准会修改 `config.GlobalConfig` 并替换默认 `slog` logger，每个包内按顺序执行（没有 `b.Parallel`）。跨包运行时请加 `-p 1`，否则 Go 会并行启动多个包的测试二进制，互相干扰。

### 参考结果

采集环境：Windows / amd64、i7-13620H（10 核 16 线程）、16GB、Go 1.27.1、`GOMAXPROCS=16`、`-benchtime 500ms -count 3`（取中位数）。绝对值随机器和电源策略变化，**只用于同机前后对比**。

引擎层（noop 后端，`ns/op` 为一次迭代；press+release 迭代包含 2 次 submit）：

| 基准 | ns/op | B/op | allocs/op | 说明 |
|------|-------|------|-----------|------|
| `EngineSubmitSingle/press_held` | 1140 | 136 | 3 | 单次 submit ≈ 1.14µs，约 88 万次/s |
| `EngineSubmitNoop` | 2256 | 272 | 6 | press+release，约 44 万对/s |
| `EngineSubmitQueueSize/q1` | 2298 | 256 | 4 | 队列 1 |
| `EngineSubmitQueueSize/q8` | 2285 | 256 | 4 | 队列 8 |
| `EngineSubmitQueueSize/q1024` | 2285 | 256 | 4 | 默认队列 1024 |
| `EngineSubmitQueueSize/q65536` | 2265 | 256 | 4 | 队列 65536 |
| `EngineKeyChord/keys1` | 2260 | 256 | 4 | 单键 |
| `EngineKeyChord/keys4` | 2903 | 256 | 4 | 四键 chord |
| `EngineEffectiveKeyRelease/held128/exact_hit` | 20 | 0 | 0 | held-set 精确命中 |
| `EngineEffectiveKeyRelease/held128/miss_scan` | 97 | 0 | 0 | held-set 全量扫描 |
| `EngineCanceledContext` | 32 | 8 | 1 | 预取消 context 快速失败 |
| `EngineMouseEvents/move_rel` | 1171 | 128 | 2 | 单次鼠标事件 |
| `EngineKeyboardParallel` | 3130 / 66.5 万 ops/s | 272 | 6 | 16 生产者竞争单 worker |
| `EngineMouseParallel` | 1417 / 70.7 万 ops/s | 128 | 2 | 鼠标独立 worker |
| `EngineMixedWorkload/serial` | 3827 / 78.4 万 ops/s | 400 | 8 | 键鼠串行执行 |
| `EngineMixedWorkload/concurrent` | 3507 / 85.5 万 ops/s | 400 | 8 | 键鼠并发执行（+9%） |
| `EngineDrainQueue` | 74203 / 1380 万 ops/s | — | — | 一次排空 1024 条队列 |
| `EngineCloseWithHeldKeys/held1` | 1813 | — | — | 关闭并释放 1 个 held 键 |
| `EngineCloseWithHeldKeys/held32` | 2604 | — | — | 关闭并释放 32 个 held 键 |

> `EngineSubmitSingle` 按真实调用方式每次传入新的 key slice；`benchPressRelease` 复用了同一个 slice，因此 `QueueSize`/`KeyChord` 的分配比单次 submit 少一次（8B）。真实 gRPC 路径每次 `dispatch` 也会新建 slice，与 `GRPCDispatch` 的 3 allocs/op 一致。

后端延迟扫描（`noopLatencyMicros`，每次迭代 = press+release）：

| 设置 | ns/op | ops/s | 备注 |
|------|-------|-------|------|
| 0 | 2379 | 85.5 万 | 纯引擎开销 |
| 100µs | 1254512 | 1609 | 受 Windows 定时器粒度影响，实际约 0.6ms/次 |
| 1ms | 3181220 | 636 | 接近 1/(1ms) 上限 |
| 2ms | 4903460 | 412 | 线性 |
| 5ms | 10896918 | 185 | 线性 |

延迟分布（`EngineLatencyDistribution`，QPC 逐次采样 press+release 往返）：

| p50 | p90 | p99 | max |
|-----|-----|-----|-----|
| 1.4µs | 4.5µs | 8.0µs | 377µs |

服务层（bufconn，客户端与服务端同进程，包含 protobuf 与 gRPC 框架）：

| 基准 | ns/op | B/op | allocs/op | 换算 |
|------|-------|------|-----------|------|
| `GRPCDispatch/name` | 1175 | 136 | 3 | 约 85 万次/s |
| `GRPCDispatch/chord4` | 1381 | 152 | 3 | 约 290 万键/s |
| `GRPCUnaryKeyServiceDebug` | 35671 | 9182 | 158 | 约 2.8 万次/s |
| `GRPCUnaryParallel` | 8243 | 9005 | 144 | 约 12.1 万次/s |
| `GRPCStreamKeyService/msgs1` | 39865 | 9658 | 165 | 约 2.5 万次/s |
| `GRPCStreamKeyService/msgs10` | 72186 | 16569 | 325 | 约 14.2 万条消息/s |
| `GRPCStreamKeyService/msgs100` | 299321 | 84108 | 1878 | 约 34.3 万条消息/s |
| `GRPCMouseService`（32 事件/流） | 129085 | 31127 | 608 | 约 24.8 万事件/s |

映射层（单次调用）：

| 基准 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| `KeyCodesFromName/lower` | 13.2 | 0 | 0 |
| `KeyCodesFromName/mixed_case` | 36.2 | 8 | 1 |
| `KeyCodesFromRune/lower_ascii` | 8.7 | 0 | 0 |
| `KeyCodesFromRunes/chord3` | 36.9 | 0 | 0 |
| `KeyCodesFromRunes/chord8` | 128.8 | 48 | 1 |
| `KeyCodesFromMakc/hit` | 17.4 | 0 | 0 |
| `KeyCodesFromWinput/hit` | 17.1 | 0 | 0 |
| `SamePhysicalKey/same_code` | 0.56 | 0 | 0 |

### 结论

1. **键盘吞吐由单 worker 决定。** 单次 submit 约 1.14µs，press+release 约 2.26µs；并发提交不会提高吞吐，16 个生产者时反而降到约 66.5 万 ops/s（单生产者约 88 万 ops/s），因为所有事件仍要在同一个 worker 上串行注入。
2. **`dispatchQueueSize` 不影响稳态吞吐。** 队列从 1 到 65536，ns/op 基本不变（2265–2298）；它只决定突发能缓存多少事件，以及饱和时请求的排队延迟。默认 1024 足以吸收短突发。
3. **每次 submit 有 3 次分配（136B）。** 主要来自调用点的 `[]KeyCodes` 和每次新建的响应 channel；这是后续优化（对象池/复用响应通道）的首要目标。
4. **held 键的释放是 O(1) 或 O(n)。** 精确命中约 20ns；修饰键不一致时回退到线性扫描，128 个 held 键约 97ns。正常“按下什么就释放什么”的客户端不会走扫描分支。
5. **键鼠 worker 独立，但并发收益有限。** 并发执行相对串行只快约 9%（78.4 万 → 85.5 万 ops/s），瓶颈更多来自 submit 的 goroutine 唤醒与 Go 调度，而不是后端串行化；不要期待 2× 加速。
6. **gRPC 是主要固定成本。** bufconn 下 unary 约 35.7µs/次，单条流的建流成本也在 40µs 量级；批量发送时每消息成本显著下降（100 条/流时约 3µs/条）。对高频按键，优先使用长流而不是大量 unary 调用。
7. **大小写规范化会产生分配。** `KeyCodesFromName("Escape")` 的 `strings.ToLower` 每个调用分配 8B；高频路径尽量传小写键名，或读取端做小写缓存。
8. **关闭路径可控。** 排空 1024 条队列约 74µs，关闭并释放 32 个 held 键约 2.6µs，不会成为退出瓶颈。

### 端到端压测脚本

`bench/e2e.ps1` 会编译 `GoKeyMux.exe` 与 `loadtest.exe` 到临时目录，生成 noop 配置并启动服务，然后按矩阵跑 loopback TCP 压测：

```powershell
pwsh ./bench/e2e.ps1 -Port 50071 -Duration 20s `
  -Concurrency 1,4,16,64 -NoopLatencyMicros 0,50
```

- 产物：`%TEMP%\gokeymux-bench\run-<时间戳>\results.csv`、服务端日志与两个 exe。
- `-Duration` 是每个 worker 的持续时间；脚本额外加 2s warmup。
- 每个 `noopLatencyMicros` 值都会重启一次服务端。
- noop 后端不做真实注入，运行期间可以正常使用机器。

参考输出（2s/worker，`-Concurrency 1,8`）：

| latency_us | mode | c | total | tps/rps | p50 | p99 |
|-----------:|------|---|------:|--------:|-----|-----|
| 0 | stream | 1 | 873316 | 433239 | — | — |
| 0 | stream | 8 | 999894 | 451066 | — | — |
| 0 | unary | 1 | 3257 | 1618 | 527.4µs | 13.2397ms |
| 0 | unary | 8 | 38317 | 19136 | 0s | 6.7452ms |

> `unary` 模式的百分位来自 `cmd/loadtest` 的 `time.Now`。在部分 Windows 环境（如 RDP/虚拟机）`time.Now` 粒度约 0.5ms，会出现 `p50=0s`；此时请以 `stream` 的吞吐或基准中的 QPC 延迟分布为准。

### 真实后端基准（手动）

默认构建不包含，且必须显式设置环境变量才会注入真实按键：

```powershell
$env:GOKEYMUX_REAL_BENCH = "1"

# 直接运行：makc / fakerInput；winput 会因测试二进制在临时目录而 Skip
go test -tags realbackend -run XXX -bench BenchmarkRealBackend -benchtime 100x -count 3 ./internal/engine

# 包含 winputWithInterception：把测试二进制生成到仓库根目录（同目录已有 Interception\）
go test -c -o .\GoKeyMux.realbench.test.exe -tags realbackend ./internal/engine
.\GoKeyMux.realbench.test.exe --% -test.run XXX -test.bench BenchmarkRealBackendPressRelease -test.benchtime 100x -test.count 3
```

- **会把 F12 真正发送到当前前台窗口**。运行前请聚焦一个空白记事本等无害窗口，不要聚焦终端、IDE 或浏览器。
- 驱动/设备不可用时该子基准自动 `Skip`。
- `winputWithInterception` 从**测试二进制所在目录**查找 `Interception/library/...`；直接用 `go test` 时测试二进制在 Go 构建缓存中，因此会跳过。用上面的 `go test -c` 在仓库根目录生成测试 exe 即可，跑完记得删除。
- 真实延迟取决于驱动和 USB/HID 处理，预期比 noop 慢 1–3 个数量级，适合用来确认“引擎开销相对于驱动开销可以忽略”。
- **`makc` 子基准强制使用 SendInput。** 在 Windows 11 build 26200 上，auto 选中的 `user32!InjectKeyboardInput` 在连续注入几十次后会报
  `The data area passed to a system call is too small`，无法完成持续压测；生产代码的 auto 行为未改动，仅基准内改用 SendInput。
- 下表中的 `winputWithInterception` 数据使用 `go test -c` 在仓库根目录生成的测试二进制测得（同目录有 `Interception\`）；直接 `go test` 会因找不到 DLL 而跳过该后端。

本机实测（Windows 11 26200 / i7-13620H，F12，`-benchtime 100x -count 5`，中位数）：

| 后端 | ns/op（press+release） | 单事件 | submit/s | 相对 noop |
|------|-----------------------:|-------:|---------:|----------:|
| `noop`（参考） | 2,256 | 1.13µs | 886,000 | 1× |
| `makc`（SendInput） | 1,930,673 | 965µs | 1,036 | 约 856× |
| `fakerInput` | 118,395 | 59.2µs | 16,894 | 约 52× |
| `winputWithInterception` | 6,460 | 3.23µs | 309,600 | 约 2.9× |

### 注意事项

- 基准固定使用 noop 后端；`makc`/`fakerInput` 的引擎层成本需通过真实后端基准或端到端压测确认。
- 本机 `time.Now` 粒度较粗，因此延迟分布基准改用 Windows QPC（`QueryPerformanceCounter`），单次采样仍包含两次 QPC 读取的几十纳秒开销。
- Windows 定时器粒度会让小于 1ms 的 `noopLatencyMicros` 被量化到约 0.5–1ms，延迟扫描请以 ≥1ms 的数据为准。
- `EngineDrainQueue` 和 `EngineCloseWithHeldKeys` 的准备工作在计时器停止时完成，但 `-benchmem` 仍会统计这些准备工作分配的内存，忽略这两个用例的 `B/op` 即可。
- 数据仅供参考；比较改动时请使用同一台机器、同一电源计划，并用 `benchstat` 对比 `-count` 多次的结果。

## 持续集成与发布

通过 GitHub Actions 自动构建（见 `.github/workflows/`）。

### CI（`.github/workflows/ci.yml`）

`push` / `pull_request` 到 `master` 时触发，执行 `go vet` + `go test` + 三个二进制的 `go build`，用于每次合并前的快速校验。

### 发布（`.github/workflows/release.yml`）

| 触发方式                                           | 行为                                                     |
|----------------------------------------------------|----------------------------------------------------------|
| 推送 `v*` tag（如 `v0.2.0`）                       | 构建 → 打包 zip → 创建正式 Release 并设为 Latest         |
| 推送带 SemVer 预发布后缀的 tag（如 `v0.2.0-rc.1`） | 同上，但 Release 标记为 **Pre-release**，不会成为 Latest |
| 手动 `workflow_dispatch`                           | 构建 → 打包 zip → 仅上传 artifact，不创建 Release        |

预发布 tag 需符合 `vX.Y.Z-suffix` 形式（如 `v0.2.0-beta.2`、`v0.2.0-rc.1`），`alpha` / `beta` / `rc`
等后缀均可；tag 构建时 `version.Version` 会原样注入 tag 名，因此预发布包在启动日志中可直接辨认。

同时构建 **x64（amd64）** 与 **x86（386）** 两个平台，各产出一个 zip：

- `GoKeyMux-windows-amd64.zip`
- `GoKeyMux-windows-386.zip`

每个 zip 均为扁平结构（解压后 exe 与 `Interception/`、`proto/` 同级，可直接运行）：

```
GoKeyMux-windows-amd64.zip
├── GoKeyMux.exe
├── keycycle.exe
├── loadtest.exe
├── FakerInput_Setup_0.1.1_x64.msi   # 仅 amd64 包
├── Interception/      # 运行时驱动库与许可证
└── proto/             # gRPC 协议定义（供其他语言生成客户端桩）

GoKeyMux-windows-386.zip
├── GoKeyMux.exe
├── keycycle.exe
├── loadtest.exe
├── Interception/      # 运行时驱动库与许可证
└── proto/             # gRPC 协议定义（供其他语言生成客户端桩）
```

> 提示：
> - `GoKeyMux.exe` 与 `Interception/` 同级，是为了让 `winputWithInterception` 后端按可执行文件目录定位
>   `Interception/library/x64/interception.dll`（386 版对应 `x86/`），解压后无需移动文件即可运行。
> - `FakerInput_Setup_0.1.1_x64.msi` 为 x64 驱动，仅随 amd64 包分发。

## 项目结构

```
.
├── main.go                        # 入口：加载配置、启动 engine/service、信号处理
├── config.json                    # 运行时配置（缺失时自动生成）
├── internal/
│   ├── config/config.go           # 配置结构与加载逻辑
│   ├── input/
│   │   ├── keyMap.go              # 统一按键映射表（makc / winput / FakerInput）
│   │   ├── FakerInput.go          # FakerInput 虚拟 HID 设备实现
│   │   ├── makcInput.go           # makc 后端
│   │   ├── noopInput.go           # noop 空操作后端（压测用）
│   │   ├── winputWithWindow.go    # winput 窗口消息后端
│   │   ├── winputWithInterception.go
│   │   └── *_test.go              # 键码/后端单测、集成测试、映射基准
│   ├── engine/
│   │   ├── engine.go              # 引擎：按 driveName 分发到具体后端
│   │   ├── aliases.go             # 类型 / 驱动常量别名
│   │   └── *_test.go              # 引擎单测、基准、集成与真实后端
│   └── service/
│       ├── grpc.go                # gRPC 服务、健康检查、keepalive、拦截器
│       └── *_test.go              # gRPC 基准（bufconn）
├── benchmark.md                   # 完整性能基准报告（方法、数据、结论）
├── bench/e2e.ps1                  # 端到端 gRPC 压测脚本（noop 后端）
├── proto/
│   ├── gokeymux.proto             # 服务协议定义
│   └── gokeymux{,_grpc}.pb.go     # 生成的 protobuf / gRPC 桩代码
├── cmd/
│   ├── loadtest/main.go           # 压测客户端
│   └── keycycle/main.go           # 按键 / 鼠标演示客户端
├── .github/workflows/             # GitHub Actions：CI 与发布
│   ├── ci.yml                     # 主分支校验（vet / test / build）
│   └── release.yml                # tag 或手动触发的发布构建
├── Interception/                  # Interception 驱动库与许可证
├── FakerInput/                    # FakerInput 驱动源码（上游 Ryochan7/FakerInput）
├── FakerInput_Setup_0.1.1_x64.msi # FakerInput 虚拟设备安装包
├── LICENSE                        # MIT
└── THIRD_PARTY_NOTICES            # 第三方依赖与捆绑组件许可说明
```

## 许可证

本项目基于 [MIT License](LICENSE) 发布。

本项目捆绑/依赖的部分第三方组件有其独立许可，详见 [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES)：

- **Interception**（键盘驱动）：非商业用途遵循 LGPL 3.0，商业用途需另行获取商业许可。
- **FakerInput**（驱动源码与安装包）：遵循 MIT 许可（Copyright (c) 2021 Travis Nickles）。
