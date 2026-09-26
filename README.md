# GoKeyMux

GoKeyMux 是一个运行在 **Windows** 上的按键注入服务（键盘输入多路复用器）。它通过 gRPC 暴露接口，接收“按键按下 /
释放”请求，并转发到可插拔的输入注入后端。其目标是统一多种按键注入方案（驱动 / 虚拟 HID / 窗口消息）为同一个接口，方便游戏自动化等本机场景。

> ⚠️ **仅限本机使用**：这是一个本地按键映射服务，**不提供 TLS 能力**，也没有任何认证机制。请务必只监听回环地址（如
> `localhost:50051`、`127.0.0.1:50051`），不要绑定 `0.0.0.0`、空主机名或局域网 IP。若监听地址不是回环地址，服务启动时
> 会打印警告。将服务暴露到本机之外会让网络上的其他主机直接驱动本机按键，风险自负。

## 特性

- **gRPC 服务**：提供客户端流式 `keyService` 与一元 `keyServiceDebug` 两个 RPC。
- **可插拔后端**：内置 5 种输入注入后端，通过配置切换。
- **统一按键映射**：一套逻辑按键同时映射三种编码（Windows 虚拟键码 / Interception 扫描码 / USB HID 键码）。
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
    - `winputWithInterception`：安装 Interception 驱动。直接运行
      `Interception\command line installer\install-interception.exe`（不带参数）即可查看帮助信息，
      并按提示完成安装；无需移动或改动 `Interception` 目录。
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

## 配置

首次运行（或删除 `config.json` 后运行）会自动生成默认配置。字段如下：

| 字段                                 | 类型   | 默认值            | 说明                                                   |
|--------------------------------------|--------|-------------------|--------------------------------------------------------|
| `driveName`                          | string | `makc`            | 后端驱动，取值见上方表格                               |
| `gRPCAddress`                        | string | `localhost:50051` | gRPC 监听地址（`tcp4`）；仅限回环地址，非回环地址会触发警告 |
| `logLevel`                           | string | `info`            | 日志等级：`debug` / `info` / `warn` / `error`          |
| `gRPCKeepaliveTime`                  | int    | `2`               | keepalive `Time`，单位秒                               |
| `gRPCKeepaliveTimeOut`               | int    | `1`               | keepalive `Timeout`，单位秒                            |
| `gRPCKeepaliveMaxConnectionIdle`     | int    | `2`               | keepalive `MaxConnectionIdle`，单位秒                  |
| `gRPCEnforcementPolicyMinTime`       | int    | `10`              | EnforcementPolicy `MinTime`，单位秒                    |
| `gRPCEnforcementPermitWithoutStream` | bool   | `true`            | EnforcementPolicy `PermitWithoutStream`                |
| `gRPCServerShutdownTimeout`          | int    | `10`              | 优雅关闭超时，单位秒                                   |
| `winputWindowProcessName`            | string | `Game.exe`        | (`winputWithWindow`) 按进程名查找窗口                  |
| `winputWindowUseStaticIndex`         | bool   | `true`            | (`winputWithWindow`) 是否使用固定窗口索引              |
| `winputWindowIndex`                  | int    | `0`               | (`winputWithWindow`) 窗口索引                          |
| `pprofAddress`                       | string | `""`              | 可选 pprof 监听地址，空表示关闭（如 `localhost:6060`） |
| `noopLatencyMicros`                  | int    | `0`               | (`noop`) 每次按键模拟延迟，单位微秒                    |

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
  "noopLatencyMicros": 0
}
```

## 运行

```powershell
# 启动服务（默认读取同目录 config.json）
.\GoKeyMux.exe
```

启动后即可通过 gRPC 客户端（Go / Python / C# 等任意语言，按 `proto/gokeymux.proto` 生成桩代码）调用。

### 演示客户端 `keycycle`

循环按下并释放 a-z，用于快速验证链路：

```powershell
.\keycycle.exe -addr localhost:50051 -interval 5s
```

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
}
```

- `keyService`： **客户端流式**。客户端持续发送 `keyInput`，服务端逐个派发；流结束时返回 `keyReturn`，其中 `isAllDone` 为
  `true` 表示全程无派发错误。
- `keyServiceDebug`： **一元**，用于单次调试调用。

`keyInput` 字段：

| 字段        | 类型   | 说明                                                  |
|-------------|--------|-------------------------------------------------------|
| `key`       | string | 键名（`isRune=false`）或 rune 字符串（`isRune=true`） |
| `isRune`    | bool   | 是否将 `key` 解释为 rune 字符串                       |
| `isPressed` | bool   | `true` 按下，`false` 释放                             |

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

# 集成测试（需安装 FakerInput 虚拟设备，否则跳过）
go test -tags integration ./...
```

## 持续集成与发布

通过 GitHub Actions 自动构建（见 `.github/workflows/`）。

### CI（`.github/workflows/ci.yml`）

`push` / `pull_request` 到 `master` 时触发，执行 `go vet` + `go test` + 三个二进制的 `go build`，用于每次合并前的快速校验。

### 发布（`.github/workflows/release.yml`）

| 触发方式                 | 行为                                                                           |
|--------------------------|--------------------------------------------------------------------------------|
| 推送 `v*` tag            | 构建 → 打包 zip → 自动创建 GitHub Release 并附带 zip（自动生成 release notes） |
| 手动 `workflow_dispatch` | 构建 → 打包 zip → 仅上传 artifact，不创建 Release                              |

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
├── main.go                        # 入口：加载配置、启动引擎与 gRPC 服务、信号处理
├── config.go / config.json        # 配置结构与加载逻辑（缺失时生成默认配置）
├── engines.go                     # 引擎：按 driveName 分发到具体后端
├── keyMap.go                      # 统一按键映射表（makc / winput / FakerInput 三种编码）
├── grpcService.go                 # gRPC 服务实现、健康检查、keepalive、日志拦截器
├── makcInput.go                   # makc 后端
├── winputWithWindow.go            # winput 窗口消息后端
├── winputWithInterception.go      # winput + Interception 驱动后端
├── FakerInput.go                  # FakerInput 虚拟 HID 设备实现
├── noopInput.go                   # noop 空操作后端（压测用）
├── proto/
│   ├── gokeymux.proto             # 服务协议定义
│   └── gokeymux{,_grpc}.pb.go     # 生成的 protobuf / gRPC 桩代码
├── cmd/
│   ├── loadtest/main.go           # 压测客户端
│   └── keycycle/main.go           # a-z 循环演示客户端
├── .github/workflows/             # GitHub Actions：CI 与发布
│   ├── ci.yml                     # 主分支校验（vet / test / build）
│   └── release.yml                # tag 或手动触发的发布构建
├── Interception/                  # Interception 驱动库与许可证
├── FakerInput_Setup_0.1.1_x64.msi # FakerInput 虚拟设备安装包
├── LICENSE                        # MIT
└── THIRD_PARTY_NOTICES            # 第三方依赖与捆绑组件许可说明
```

## 许可证

本项目基于 [MIT License](LICENSE) 发布。

本项目捆绑/依赖的部分第三方组件有其独立许可，详见 [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES)：

- **Interception**（键盘驱动）：非商业用途遵循 LGPL 3.0，商业用途需另行获取商业许可。
- **FakerInput**：以厂商许可条款为准，重新分发前请确认。
