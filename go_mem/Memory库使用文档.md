# Memory Go库使用文档

`Memory` 是 `libmem.so` 的 Go 封装，但使用方式遵循 Go 风格：

- 关键 `./resources/libs/<架构>/libmem.so` 放入自己项目的同目录,并同步资源,编译二进制选择把依赖一并打包
- 部分内核无法使用内核后端,可以使用 `apatch` 或者其他支持 `kpm` 模块的进行加载,代码中初始化内核就可以成功
- 通过 `import "Memory"` 导入
- 通过 `Memory.Open(...)` 或 `Memory.New(...)` 创建 `Client`
- 构建期与运行时都依赖同 ABI 的 `libmem.so`
- 大多数接口通过返回 `error` 暴露失败原因，而不是 Lua 风格的 `nil, error`

## 环境要求

- **权限要求**: root 
- **架构支持**:
  - **64位主机** (`arm64-v8a`, `x86_64`): 全部功能可用
  - **32位主机** (`armeabi-v7a`, `x86`): 基本内存读写可用，**ptrace 远程操作不可用**
- **平台**: Android

### 32位编译限制说明

****以下功能仅在 64 位编译的 `so` 上可用，32 位编译时会返回错误或失败****：

| 函数 | 32位行为 | 说明 |
|------|----------|------|
| `client.Inject` / `client.Injector` | 返回 `error` | SO 注入 |
| `client.RemoteMmap` | 返回 `0, error` | 远程 `mmap` |
| `client.RemoteCallFunction` | 返回 `0, error` | 远程函数调用 |
| `client.SetMemoryPermission` | 返回 `error` | 设置内存权限 |

这些能力依赖 `ptrace` 等远程进程操作，32 位主机侧不兼容。

## 初始化

### 导入包

```go
import "Memory"
```

### 与 Lua 版 `mem` 的主要区别

Lua 版是这样初始化的：

```lua
local Memory = require("mem")
local memory = Memory:new(lib_path)
```

Go 版**不按路径动态加载**，而是通过 `cgo` 在构建期链接 `libmem.so`。因此 Go 侧的初始化重点不是 `lib_path`，而是：

- 找到目标进程 PID
- 创建绑定到该 PID 的 `Client`

### 按包名创建 Client

```go
client, err := Memory.Open("com.example.game")
if err != nil {
    return err
}
```

**参数:**
- `packageName`: 目标应用包名

**返回值:**
- `client`: 绑定到目标进程的 `*Memory.Client`
- `err`: 失败时的错误信息

### 按 PID 创建 Client

```go
client, err := Memory.New(12345)
if err != nil {
    return err
}
```

`NewClient` 仍然保留，作为 `New` 的别名：

```go
client, err := Memory.NewClient(12345)
```

## 进程管理

### 获取进程 PID

```go
pid, err := Memory.GetPID("com.example.game")
if err != nil {
    return err
}
fmt.Println("pid:", pid)
```

**参数:**
- `packageName`: 包名

**返回值:**
- `pid`: 进程 ID
- `err`: 失败时的错误信息

### 模糊搜索进程

```go
items, err := Memory.FindProcesses("com.example")
if err != nil {
    return err
}
for _, item := range items {
    fmt.Println(item.PID, item.Cmdline)
}
```

**参数:**
- `pattern`: 进程名或命令行匹配模式

**返回值:**
- `[]Memory.ProcessInfo`
- `err`

`ProcessInfo` 字段：

- `PID`
- `Cmdline`

### 读取当前 Client 绑定的 PID

```go
fmt.Println(client.PID())
```

## 内存读写操作

### 支持的数据类型

以下类型常量用于 `ReadMemory` / `WriteMemory`：

| Go 常量 | 底层类型 | 说明 |
|---------|----------|------|
| `Memory.TypeI8` | `int8_t` | 8位有符号整数 |
| `Memory.TypeI16` | `int16_t` | 16位有符号整数 |
| `Memory.TypeI32` | `int32_t` | 32位有符号整数 |
| `Memory.TypeI64` | `int64_t` | 64位有符号整数 |
| `Memory.TypeUI8` | `uint8_t` | 8位无符号整数 |
| `Memory.TypeUI16` | `uint16_t` | 16位无符号整数 |
| `Memory.TypeUI32` | `uint32_t` | 32位无符号整数 |
| `Memory.TypeUI64` | `uint64_t` | 64位无符号整数 |
| `Memory.TypeChar` | `char` | 字符 |
| `Memory.TypeUChar` | `unsigned char` | 无符号字符 |
| `Memory.TypeF32` | `float` | 32位浮点数 |
| `Memory.TypeF64` | `double` | 64位浮点数 |
| `Memory.TypeString` | `char*` | C 字符串 |
| `Memory.TypeBinary` | `binary` | 二进制数据 |
| `Memory.TypeUTF16` | `utf16` | UTF-16 字符串 |

### 读取原始字节

```go
buf, err := client.ReadAt(0x12345678, 16)
if err != nil {
    return err
}
fmt.Printf("%x\n", buf)
```

**参数:**
- `address`: 起始地址
- `size`: 读取字节数

**返回值:**
- `[]byte`
- `error`

### 使用 procfs 方式读一次

```go
buf, err := client.ReadAtProcfs(0x12345678, 16)
```

这个接口会临时切到 `procfs` 读后端完成一次读取，再恢复之前的全局后端设置。

### 通用读取接口

```go
value, err := client.ReadMemory(0x12345678, Memory.TypeI32)
name, err := client.ReadMemory(0x12346000, Memory.TypeString, 64)
blob, err := client.ReadMemory(0x12347000, Memory.TypeBinary, 32)
text, err := client.ReadMemory(0x12348000, Memory.TypeUTF16, 16)
```

**参数:**
- `address`: 地址
- `valueType`: 读取类型
- `bufferSize`: 可选参数，字符串 / 二进制 / UTF-16 读取时用于指定长度

**返回值:**
- `any`: 实际类型由 `valueType` 决定
- `error`

### 常用类型化读取接口

```go
v1, err := client.ReadInt32(0x12345678)
v2, err := client.ReadFloat32(0x12346000)
v3, err := client.ReadString(0x12347000, 64)
v4, err := client.ReadUTF16(0x12348000, 16)
v5, err := client.ReadBinary(0x12349000, 32)
```

常用读取函数包括：

- `ReadInt8`
- `ReadInt16`
- `ReadInt32`
- `ReadInt64`
- `ReadUint8`
- `ReadUint16`
- `ReadUint32`
- `ReadUint64`
- `ReadFloat32`
- `ReadFloat64`
- `ReadString`
- `ReadBinary`
- `ReadUTF16`
- `ReadChar`
- `ReadUChar`

### 通用写入接口

```go
err = client.WriteMemory(0x12345678, int32(9999), Memory.TypeI32)
err = client.WriteMemory(0x12346000, float32(3.14), Memory.TypeF32)
err = client.WriteMemory(0x12347000, "player", Memory.TypeString)
err = client.WriteMemory(0x12348000, []byte{0x1c, 0xcc, 0x00, 0xff}, Memory.TypeBinary)
```

**参数:**
- `address`: 地址
- `value`: 要写入的值
- `valueType`: 写入类型

**返回值:**
- `error`

### 常用类型化写入接口

```go
err = client.WriteInt32(0x12345678, 1024)
err = client.WriteFloat32(0x12346000, 3.14)
err = client.WriteString(0x12347000, "Hello World")
err = client.WriteUTF16(0x12348000, "你好")
err = client.WriteBinary(0x12349000, []byte{0x1c, 0xcc, 0x00, 0xff})
```

常用写入函数包括：

- `WriteAt`
- `WriteInt8`
- `WriteInt16`
- `WriteInt32`
- `WriteInt64`
- `WriteUint8`
- `WriteUint16`
- `WriteUint32`
- `WriteUint64`
- `WriteFloat32`
- `WriteFloat64`
- `WriteString`
- `WriteBinary`
- `WriteUTF16`
- `WriteChar`
- `WriteUChar`

## 内存搜索

### 支持的搜索范围常量

| Go 常量 | 底层值 | 描述 |
|---------|--------|------|
| `Memory.RegionJ` | `"j"` | Java 堆 |
| `Memory.RegionPS` | `"ps"` | 私有共享内存 |
| `Memory.RegionXA` | `"xa"` | 匿名可执行内存 |
| `Memory.RegionCD` | `"cd"` | 代码段 |
| `Memory.RegionBad` | `"bad"` | 错误区域 |
| `Memory.RegionAS` | `"as"` | 匿名共享内存 |
| `Memory.RegionCB` | `"cb"` | 代码块内存 |
| `Memory.RegionJH` | `"jh"` | Java 堆内存 |
| `Memory.RegionOther` | `"other"` | 其他区域 |
| `Memory.RegionS` | `"s"` | 共享内存 |
| `Memory.RegionVideo` | `"video"` | 视频内存 |
| `Memory.RegionA` | `"a"` | 匿名内存 |
| `Memory.RegionXS` | `"xs"` | 可执行共享内存 |
| `Memory.RegionCA` | `"ca"` | 代码和匿名内存 |
| `Memory.RegionCH` | `"ch"` | 代码和堆内存 |
| `Memory.RegionAll` | `"all"` | 全部区域 |

### 支持的搜索值类型

搜索构造器使用的是 `SearchType*` 常量，而不是 `Type*` 常量：

| Go 常量 | 底层值 | 说明 |
|---------|--------|------|
| `Memory.SearchTypeI8` | `int8_t` | 8位有符号整数 |
| `Memory.SearchTypeI16` | `int16_t` | 16位有符号整数 |
| `Memory.SearchTypeI32` | `int32_t` | 32位有符号整数 |
| `Memory.SearchTypeI64` | `int64_t` | 64位有符号整数 |
| `Memory.SearchTypeUI8` | `u_int8_t` | 8位无符号整数 |
| `Memory.SearchTypeUI16` | `u_int16_t` | 16位无符号整数 |
| `Memory.SearchTypeUI32` | `uint32_t` | 32位无符号整数 |
| `Memory.SearchTypeUI64` | `uint64_t` | 64位无符号整数 |
| `Memory.SearchTypeChar` | `char` | 字符 |
| `Memory.SearchTypeUChar` | `unsigned char` | 无符号字符 |
| `Memory.SearchTypeF32` | `float` | 32位浮点数 |
| `Memory.SearchTypeF64` | `double` | 64位浮点数 |
| `Memory.SearchTypeString` | `char*` | 字符串 |

当前搜索构造器**不提供** `binary` 和 `utf16` 搜索类型常量。

### 推荐方式：使用 SearchRequest 构造器

```go
request := client.NewSearchRequest(Memory.EqualSearch(Memory.SearchTypeI32, 1234)).
    WithRegions(Memory.RegionA, Memory.RegionOther).
    WithRange(0x1000, 0x2000).
    WithDataFilters(Memory.EqualFilter(Memory.SearchTypeString, 0x10, "player")).
    WithMax(64)

addrs, err := client.SearchFastWithRequest(request)
if err != nil {
    return err
}
fmt.Println(addrs)
```

### 搜索入口函数

Go 版当前提供 3 组搜索入口：

#### 1. 使用 `SearchRequest`

```go
addrs, err := client.SearchWithRequest(request)
addrs, err := client.SearchFastWithRequest(request)
```

- `SearchWithRequest`: 走 legacy 搜索实现
- `SearchFastWithRequest`: 走 fast 搜索实现

#### 2. 直接传 `map[string]any`

```go
addrs, err := client.Search(map[string]any{
    "head": map[string]any{
        "type": "int32_t",
        "data": 1234,
    },
    "types": []string{"a", "other"},
    "max":   32,
})

addrs, err = client.SearchFast(map[string]any{
    "head": map[string]any{
        "type": "int32_t",
        "data": 1234,
    },
    "types": []string{"a"},
})
```

#### 3. 直接传 JSON 字符串

```go
raw := `{"head":{"type":"int32_t","data":1234},"types":["a","other"],"max":32}`

addrs, err := client.SearchJSON(raw)
addrs, err = client.SearchFastJSON(raw)
```

如果你不是在做底层 schema 兼容测试，建议优先使用 `SearchRequest` 构造器。

### SearchRequest 字段说明

`SearchRequest` 会序列化成底层搜索引擎使用的 JSON 结构：

| Go 字段 | JSON 字段 | 必需 | 说明 |
|---------|-----------|------|------|
| `PID` | `pid` | 否 | 进程 ID，使用 `client.NewSearchRequest` 时会自动带上 |
| `Head` | `head` | 是 | 头条件 |
| `Ranges` | `rangs` | 否 | 自定义地址范围 |
| `Types` | `types` | 否 | 预定义区域常量 |
| `Data` | `data` | 否 | 基于头地址的偏移过滤 |
| `Remove` | `Remove` | 否 | 要排除的地址范围 |
| `Nop` | `nop` | 否 | 特殊模式 |
| `Max` | `max` | 否 | 最大匹配数量 |

### SearchRequest 链式方法

常用链式方法如下：

- `WithPID(pid)`
  - 显式指定搜索进程 PID
- `WithRange(start, end)`
  - 追加一个搜索范围
- `WithRanges(ranges...)`
  - 批量追加搜索范围
- `WithRegions(regions...)`
  - 追加预定义区域，如 `Memory.RegionA`
- `WithTypes(types...)`
  - 直接追加底层 `types` 字符串
- `WithDataFilters(filters...)`
  - 追加偏移过滤条件
- `WithRemoveRange(start, end)`
  - 追加一个排除区间
- `WithRemoveRanges(ranges...)`
  - 批量追加排除区间
- `WithNop(enabled)`
  - 设置 `nop` 标记
- `WithMax(max)`
  - 设置最大返回数量
- `JSON()`
  - 序列化为底层搜索 JSON 字符串

另外还有两个构造入口：

- `Memory.NewSearchRequest(head)`
- `client.NewSearchRequest(head)`

其中 `client.NewSearchRequest(head)` 会自动带上当前 `client` 的 PID。

### 搜索头条件

```go
Memory.EqualSearch(Memory.SearchTypeI32, 1234)
Memory.LVSearch(Memory.SearchTypeF32, 1.0)
Memory.HVSearch(Memory.SearchTypeF32, 10.0)
```

三种比较方式对应底层含义：

- `EqualSearch`: 精确匹配，对应底层 `data`
- `LVSearch`: 大于等于，对应底层 `lv`
- `HVSearch`: 小于等于，对应底层 `hv`

### 偏移过滤条件

```go
Memory.EqualFilter(Memory.SearchTypeString, 0x10, "player")
Memory.LVFilter(Memory.SearchTypeI32, 0x4, 100)
Memory.HVFilter(Memory.SearchTypeF32, 0x8, 20.0)
```

### 范围与排除区间

```go
request := client.NewSearchRequest(Memory.EqualSearch(Memory.SearchTypeI32, 1234)).
    WithRange(0x1000, 0x2000).
    WithRemoveRange(0x1500, 0x1600)
```

也可以批量传入：

```go
request := Memory.NewSearchRequest(Memory.EqualSearch(Memory.SearchTypeI32, 1234)).
    WithRanges(
        Memory.MakeSearchRange(0x1000, 0x2000),
        Memory.MakeSearchRange(0x3000, 0x4000),
    )
```

### Fast 与 Legacy 搜索

```go
addrs, err := client.SearchFastWithRequest(request)
addrs, err := client.SearchWithRequest(request)
```

- `SearchFastWithRequest`: 调用 `searchMemoryFast`
- `SearchWithRequest`: 调用 legacy `searchMemory`

### 直接传 JSON 或 map

```go
raw := `{"head":{"type":"int32_t","data":1234},"types":["a","other"],"max":32}`
addrs, err := client.SearchFastJSON(raw)
```

```go
addrs, err := client.Search(map[string]any{
    "head": map[string]any{
        "type": "int32_t",
        "data": 1234,
    },
    "types": []string{"a", "other"},
    "max":   32,
})
```

只有在你明确知道底层 schema 时，才建议直接手写 JSON 或 map。一般优先使用构造器。

## 后端模式与内核初始化

### 设置读后端

可选后端：

- `Memory.BackendSyscall`
- `Memory.BackendKernel`
- `Memory.BackendProcfs`

设置方式：

```go
Memory.SetReadBackend(Memory.BackendSyscall)
client.SetReadBackend(Memory.BackendKernel)
```

**注意:** `SetReadBackend` 是**库级别全局设置**，不是每个 `Client` 各自独立的配置。

### 初始化内核模式

```go
ok := Memory.InitKernel()
if !ok {
    return fmt.Errorf("init kernel failed")
}
client.SetReadBackend(Memory.BackendKernel)
```

也可以通过 `Client` 调用：

```go
ok := client.InitKernel()
```

推荐顺序：

1. 先调用 `InitKernel`
2. 返回 `true` 后再切换到 `BackendKernel`
3. 如果失败，则退回 `BackendSyscall` 或 `BackendProcfs`

## 模块信息

### 获取模块信息

```go
mods, err := client.Modules("libc.so")
if err != nil {
    return err
}
for _, mod := range mods {
    fmt.Printf("start=0x%x size=0x%x perms=%s path=%s\n", mod.Start, mod.Size, mod.Perms, mod.Path)
}
```

`ModuleInfo` 字段：

- `Start`
- `Size`
- `Perms`
- `Path`

## 内存权限管理

### 检查地址是否有效

```go
ok := client.IsMemory(0x12345678, 16)
```

**返回值:**
- `true`: 地址范围有效
- `false`: 地址范围无效

### 获取内存权限

```go
perm, err := client.MemoryPermission(0x12345678)
if err != nil {
    return err
}
fmt.Println(perm)
```

### 设置内存权限

```go
err := client.SetMemoryPermission(0x12345678, "rwxp")
if err != nil {
    return err
}
```

## 高级功能

### 指针链解析

```go
addr, err := client.ResolveChain(0x12345678, "0x10+0x20+0x8")
if err != nil {
    return err
}
value, err := client.ReadInt32(addr)
```

也可以直接传偏移数组：

```go
addr, err := client.ResolveOffsets(0x12345678, 0x10, 0x20, 0x8)
```

`ResolveChain` 默认使用 `+` 分隔，支持十进制和 `0x` 十六进制偏移。

### 进程架构检测

```go
arch, err := client.Architecture()
if err != nil {
    return err
}
fmt.Println(arch)
```

**返回值通常为:**
- `32`
- `64`

### SO 文件注入

```go
err := client.Inject("/data/local/tmp/libhook.so")
if err != nil {
    return err
}
```

`Injector` 是 `Inject` 的别名：

```go
err := client.Injector("/data/local/tmp/libhook.so")
```

### 远程 mmap

```go
prot := Memory.PROT_READ | Memory.PROT_WRITE | Memory.PROT_EXEC
flags := Memory.MAP_PRIVATE | Memory.MAP_ANONYMOUS
remoteBuf, err := client.RemoteMmap(0, 0x1000, prot, flags, -1, 0)
if err != nil {
    return err
}
fmt.Printf("remote buf: 0x%x\n", remoteBuf)
```

### 远程函数调用

```go
ret, err := client.RemoteCallFunction(0x7abc123400, 1, 2, 3)
if err != nil {
    return err
}
fmt.Printf("result = 0x%x\n", ret)
```

调用前请自行保证：

- 函数地址属于目标进程
- 参数顺序正确
- 参数位宽正确
- 调用约定匹配目标函数

### 解析导出函数地址

```go
fn, err := Memory.GetFunctionAddress("/data/local/tmp/libhook.so", "my_exported_symbol")
if err != nil {
    return err
}
fmt.Printf("fn = 0x%x\n", fn)
```

### 导出内存与 SO

```go
err = client.DumpMemory(0x12345678, 0x1000, "/data/local/tmp/block.bin")
err = client.DumpSo("libil2cpp.so", "/data/local/tmp/libil2cpp.dump.so")
```

### Unity 字符串辅助

```go
name, err := client.ReadUnityString(0x12345678)
err = client.WriteUnityString(0x12345678, "new name")
```

按对象实例 + 偏移读写：

```go
name, err := client.ReadUnityStringFromInstance(instance, 0x10)
err = client.WriteUnityStringFromInstance(instance, 0x10, "new name")
```

## 错误处理

Go 版接口大体遵循这两类返回模式：

- `(..., error)`: 读取、搜索、远程操作等大多数接口
- `bool`: `InitKernel`、`IsMemory`

推荐写法：

```go
value, err := client.ReadInt32(addr)
if err != nil {
    return fmt.Errorf("read hp failed: %w", err)
}
```

```go
ok := Memory.InitKernel()
if !ok {
    return fmt.Errorf("init kernel failed")
}
```

## 完整使用示例

```go
package main

import (
    "fmt"

    "Memory"
)

func run() error {
    client, err := Memory.Open("com.example.game")
    if err != nil {
        return err
    }

    client.SetReadBackend(Memory.BackendSyscall)

    mods, err := client.Modules("libil2cpp.so")
    if err != nil {
        return err
    }
    if len(mods) == 0 {
        return fmt.Errorf("module not found")
    }

    base := mods[0].Start
    addr, err := client.ResolveOffsets(base+0x123456, 0x10, 0x20, 0x8)
    if err != nil {
        return err
    }

    value, err := client.ReadInt32(addr)
    if err != nil {
        return err
    }

    request := client.NewSearchRequest(Memory.EqualSearch(Memory.SearchTypeI32, value)).
        WithRegions(Memory.RegionA, Memory.RegionOther).
        WithMax(16)

    matches, err := client.SearchFastWithRequest(request)
    if err != nil {
        return err
    }
    if len(matches) == 0 {
        return fmt.Errorf("no match found")
    }

    if err := client.WriteInt32(matches[0], 9999); err != nil {
        return err
    }

    return nil
}
```

## 注意事项

1. **权限要求**: 必须在 root 环境使用
2. **ABI 一致性**: `libmem.so`、Go 产物、目标部署目录必须使用同一 ABI
3. **共享库依赖**: Go 版依赖 `libmem.so`，运行前必须确保该库先可见或先加载
4. **后端是全局配置**: `SetReadBackend` 会影响整个进程内的 Go 包调用
5. **32位限制**: 注入、远程调用、远程 `mmap`、改权限等功能仅推荐在 64 位环境使用
6. **搜索类型与读写类型不同**: 搜索使用 `SearchType*`，读写使用 `Type*`
7. **内核模式建议先初始化**: `InitKernel()` 成功后再切到 `BackendKernel`

## 常见问题

### Q: 为什么 `Open` 或 `GetPID` 提示失败？

A: 通常是以下原因：

- 包名不对
- 目标进程尚未启动
- 当前权限不足
- 宿主环境不是 root / 最高环境

### Q: 为什么 `RemoteMmap` / `Inject` / `RemoteCallFunction` 失败？

A: 优先检查：

- 是否在 64 位主机环境
- 目标进程与当前注入链路 ABI 是否匹配
- 地址、参数、so 路径是否正确
- 目标函数是否真的存在于目标进程地址空间

### Q: 为什么切到 `BackendKernel` 后读取失败？

A: 先确认：

- `Memory.InitKernel()` 是否返回 `true`
- 当前设备内核是否支持对应模式
- 失败时先退回 `BackendSyscall` 或 `BackendProcfs`

### Q: 文档里为什么没有 Lua 版的 `set_pid`、`getAbi`、`ConvertAbi`？

A: 因为 Go 封装当前公开接口里没有这些 API。Go 版采用 `Client` 绑定 PID 的方式工作，ABI 管理能力目前也没有在 Go 层公开。
