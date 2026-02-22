# Magpie Bridge 跨语言框架开发文档
本文档针对**Python 开发人员**和**Go 开发人员**分别说明在 Magpie Bridge 框架下的开发规范、核心职责、接口约束及调试方法，确保跨语言协作高效且符合框架设计逻辑。

## 一、Python 开发人员指南
### 1. 核心职责
Python 开发人员主要负责**业务逻辑实现**（如数据处理、图片解析等），需遵循框架对函数入参、返回值、内存操作的约束，无需关注底层跨语言调用细节。

### 2. 开发规范
#### 2.1 基础函数开发（Go 调用普通 Python 函数）
##### 约束规则
- 函数参数：仅支持 `int`/`float`/`str` 基础类型，或 `memoryview`（共享内存场景）；
- 返回值：仅支持 `int`/`float`/`str`/`None`，复杂类型需自行序列化为字符串（如 JSON）；
- 模块命名：`.py` 文件名需与 Go 端 `Register` 注册的模块名一致（如 `main.py` 对应模块名 `main`）。

##### 开发示例（基础函数）
```python
# 文件名：main.py（必须与 Go 端注册的模块名一致）
def add(a: int, b: int) -> int:
    """
    Go 调用的基础加法函数
    :param a: 整数参数（Go 端传入 int/int64 都会转为 Python int）
    :param b: 整数参数
    :return: 加法结果（仅返回 int/float/str/None）
    """
    return a + b

def format_data(data: str) -> str:
    """
    字符串处理函数
    :param data: Go 端传入的字符串
    :return: 格式化后的字符串
    """
    return f"processed: {data.upper()}"
```

#### 2.2 共享内存函数开发（零拷贝操作大内存）
##### 核心约束
- 入参类型：必须接收 `memoryview` 对象（由 Go 端分配并传递）；
- 内存权限：写入操作前必须校验 `memoryview.readonly` 属性，避免权限错误；
- 边界校验：所有内存读写操作必须做长度校验，防止越界；
- 禁止操作：**严禁释放共享内存**（内存释放仅由 Go 端负责）。

##### 开发示例（共享内存操作）
```python
# 文件名：main.py
def write_photo(mem_view: memoryview) -> None:
    """
    向 Go 分配的共享内存写入数据（零拷贝）
    :param mem_view: Go 传递的共享内存视图
    :raises TypeError: 入参非 memoryview 类型
    :raises PermissionError: 内存只读
    :raises IndexError: 写入范围越界
    """
    # 1. 必做：类型校验
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"入参类型错误，预期 memoryview，实际 {type(mem_view)}")
    
    # 2. 必做：权限校验
    if mem_view.readonly:
        raise PermissionError("共享内存为只读，禁止写入")
    
    # 3. 必做：边界校验
    new_data = b"hello from python"
    write_start = 10
    write_end = write_start + len(new_data)
    if write_end > len(mem_view):
        raise IndexError(f"写入越界：内存长度 {len(mem_view)}，写入结束位置 {write_end}")
    
    # 4. 安全写入
    mem_view[write_start:write_end] = new_data

def read_photo(mem_view: memoryview) -> None:
    """
    从共享内存读取数据并保存为文件
    :param mem_view: 共享内存视图
    """
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"入参类型错误，预期 memoryview，实际 {type(mem_view)}")
    
    # 读取全部内存并写入文件
    photo_data = bytes(mem_view)
    with open("received_photo.jpg", "wb") as f:
        f.write(photo_data)

def get_mem_info(mem_view: memoryview) -> tuple[int, bool]:
    """
    获取共享内存信息（演示返回值规范）
    :return: (内存总长度, 是否只读)
    """
    if not isinstance(mem_view, memoryview):
        raise TypeError(f"入参类型错误，预期 memoryview，实际 {type(mem_view)}")
    return len(mem_view), mem_view.readonly
```

### 3. 调试与问题排查
#### 3.1 常见错误及修复
| 错误类型 | 现象 | 修复方法 |
|----------|------|----------|
| `TypeError: Expected memoryview type` | Go 调用时抛出该异常 | 检查 Go 端是否传递了正确的共享内存指针，或 Python 函数入参是否误接收其他类型 |
| `PermissionError: Memory view is read-only` | 写入内存时报错 | 告知 Go 开发人员，调用 `NewSharedMemory` 时需指定 `MEM_WRITE` 权限 |
| `IndexError: Write range out of bounds` | 写入内存越界 | 增加长度校验，或告知 Go 开发人员扩大共享内存分配大小 |
| `Go 端提示“function call failed”` | Python 函数执行异常 | 在 Python 函数内增加 `print` 日志，或通过 Go 端的 `getPyError()` 查看具体异常信息 |

#### 3.2 调试技巧
- 在函数入口/关键步骤添加 `print` 日志，Go 端会输出 Python 的 `stdout`；
- 利用 `memoryview` 的 `len()`/`readonly` 属性打印内存信息，确认入参合法性；
- 测试单个函数时，可编写独立的 Python 测试脚本（模拟 `memoryview` 入参）：
  ```python
  # 测试脚本：test_mem.py
  if __name__ == "__main__":
      # 模拟 Go 分配的共享内存
      test_data = bytearray(32 * 1024)  # 32KB
      mem_view = memoryview(test_data)
      
      # 测试写入函数
      write_photo(mem_view)
      print("写入后前 20 字节：", mem_view[:20].tobytes())
      
      # 测试读取函数
      read_photo(mem_view)
  ```

### 4. 交付规范
- Python 代码需放在 Go 项目根目录（与 `main.go` 同级），文件名与 Go 端注册的模块名一致；
- 函数注释需明确入参类型、返回值类型、异常类型；
- 所有对外函数（供 Go 调用的）需避免依赖第三方库（如需依赖，需告知 Go 开发人员安装对应库）。

## 二、Go 开发人员指南
### 1. 核心职责
Go 开发人员负责**框架初始化、模块注册、跨语言调用编排、共享内存分配/释放、异常处理**，需确保 Python 解释器生命周期管理和内存安全。

### 2. 开发前准备
#### 2.1 环境配置
- 确认 `go.mod` 配置正确（已映射 `example.com/bridges` 到本地 `./bridges` 目录）：
  ```go
  module main

  go 1.25.0

  replace example.com/bridges => ./bridges

  require example.com/bridges v0.0.0-00010101000000-000000000000
  ```
- 编译 C 胶水层为静态库（需替换 Python 路径为本地实际路径）：
  ```bash
  # 进入 bridges 目录
  cd bridges
  # 编译 c2py.c + memory.c 为目标文件
  gcc -o c2py.o -I D:\Application\Python3.12\include -c ./c2py.c
  gcc -o memory.o -I D:\Application\Python3.12\include -c ./memory.c
  # 打包为静态库
  ar rcs libc2py.a c2py.o memory.o
  ```
- 确认 Python 开发库已安装（需包含 `Python.h` 头文件和 `python312.lib` 库文件）。

#### 2.2 核心依赖导入
```go
package main

import (
	"fmt"
	// 导入框架桥接层（已通过 go.mod 映射）
	bridge "example.com/bridges"
)
```

### 3. 开发规范
#### 3.1 Python 解释器生命周期管理
##### 必做操作
- 初始化：所有调用前必须执行 `bridge.PY_Init()`；
- 异常清理：通过 `defer bridge.PY_Panic()` 确保 panic 时清理解释器；
- 禁止重复初始化：框架已做幂等处理，但仍建议只调用一次 `PY_Init()`。

##### 示例代码
```go
func main() {
	// 必做：异常时自动清理 Python 解释器
	defer bridge.PY_Panic()
	
	// 必做：初始化 Python 解释器
	bridge.PY_Init()

	// 业务逻辑...
}
```

#### 3.2 调用 Python 普通函数
##### 步骤
1. 注册 Python 模块（对应 `.py` 文件名）；
2. 调用 `bridge.PYCall` 执行 Python 函数；
3. 处理返回值和异常。

##### 示例代码
```go
func main() {
	defer bridge.PY_Panic()
	bridge.PY_Init()

	// 1. 注册模块（对应 main.py）
	if err := bridge.Register("main"); err != nil {
		fmt.Printf("❌ 模块注册失败: %v\n", err)
		return
	}

	// 2. 调用 Python 函数（main.add，参数 1、2）
	result, err := bridge.PYCall("main", "add", 1, 2)
	if err != nil {
		fmt.Printf("❌ 函数调用失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 调用结果: %v (类型: %T)\n", result, result) // 输出：3 (int64)

	// 3. 调用字符串处理函数
	strResult, err := bridge.PYCall("main", "format_data", "test data")
	if err != nil {
		fmt.Printf("❌ 函数调用失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 字符串处理结果: %v\n", strResult) // 输出：processed: TEST DATA
}
```

#### 3.3 共享内存管理（零拷贝交互）
##### 核心约束
- 内存分配：通过 `bridge.NewSharedMemory` 分配，单位为 KB，需指定权限（`MEM_WRITE_NONE`/`MEM_WRITE`）；
- 内存释放：必须通过 `defer mem.Free()` 确保释放，禁止 Python 端释放；
- 指针传递：通过 `mem.Ptr()` 获取内存指针，传递给 Python 函数（自动转为 `memoryview`）；
- 并发安全：框架内置内存锁，无需额外加锁，但避免 goroutine 并发写入同一内存块。

##### 示例代码
```go
func main() {
	defer bridge.PY_Panic()
	bridge.PY_Init()

	// 注册模块
	if err := bridge.Register("main"); err != nil {
		fmt.Printf("❌ 模块注册失败: %v\n", err)
		return
	}

	// 1. 分配 32KB 可写共享内存（关键：指定 MEM_WRITE 权限）
	mem, err := bridge.NewSharedMemory(32, bridge.MEM_WRITE)
	if err != nil {
		fmt.Printf("❌ 内存分配失败: %v\n", err)
		return
	}
	defer mem.Free() // 必做：确保内存释放

	// 2. Go 端向共享内存写入初始数据
	initData := []byte("initial photo data from go")
	if err := mem.Write(initData); err != nil {
		fmt.Printf("❌ 内存写入失败: %v\n", err)
		return
	}

	// 3. 调用 Python 函数写入内存（传递内存指针）
	_, err = bridge.PYCall("main", "write_photo", mem.Ptr())
	if err != nil {
		fmt.Printf("❌ Python 写入内存失败: %v\n", err)
		return
	}

	// 4. 调用 Python 函数读取内存并保存为文件
	_, err = bridge.PYCall("main", "read_photo", mem.Ptr())
	if err != nil {
		fmt.Printf("❌ Python 读取内存失败: %v\n", err)
		return
	}

	// 5. 调用 Python 函数获取内存信息
	memInfo, err := bridge.PYCall("main", "get_mem_info", mem.Ptr())
	if err != nil {
		fmt.Printf("❌ 获取内存信息失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 共享内存信息: %v\n", memInfo) // 输出：(32768, false)
}
```

### 4. 核心 API 说明
| API 分类 | 函数/类型 | 作用 | 使用注意事项 |
|----------|-----------|------|--------------|
| 解释器管理 | `bridge.PY_Init()` | 初始化 Python 解释器 | 仅需调用一次，重复调用无副作用 |
| | `bridge.PY_Panic()` | 异常清理 | 必须通过 `defer` 调用，捕获 panic 并清理 |
| 模块管理 | `bridge.Register(moduleName string) error` | 注册 Python 模块 | `moduleName` 必须与 `.py` 文件名一致 |
| 函数调用 | `bridge.PYCall(moduleName, funcName string, args ...interface{}) (interface{}, error)` | 调用 Python 函数 | 参数仅支持 int/int64/float64/string/uintptr（内存指针） |
| 共享内存 | `bridge.NewSharedMemory(sizeKB int32, perm MemPerm) (*SharedMemory, error)` | 分配共享内存 | `sizeKB` 为内存大小（KB），`perm` 指定读写权限 |
| | `(*SharedMemory).Free()` | 释放共享内存 | 必须调用，避免内存泄漏 |
| | `(*SharedMemory).Write(data []byte) error` | Go 写入共享内存 | 数据长度不能超过内存大小 |
| | `(*SharedMemory).Ptr() uintptr` | 获取内存指针 | 传递给 Python 函数的唯一方式 |

### 5. 调试与问题排查
#### 5.1 常见错误及修复
| 错误现象 | 原因 | 修复方法 |
|----------|------|----------|
| `Python not initialized` | 未调用 `bridge.PY_Init()` 或调用顺序错误 | 确保 `PY_Init()` 在所有调用前执行 |
| `module 'main' not registered` | 未调用 `Register` 或模块名错误 | 检查 `Register` 调用是否成功，模块名与 `.py` 文件名一致 |
| `failed to allocate shared memory` | 内存分配失败 | 检查 `sizeKB` 是否为正数，或系统内存是否充足 |
| `Memory view is read-only` | 共享内存权限为只读但 Python 尝试写入 | 调用 `NewSharedMemory` 时指定 `bridge.MEM_WRITE` |
| `unsupported type` | 传递了框架不支持的参数类型 | 仅传递 int/int64/float64/string/uintptr 类型 |

#### 5.2 调试技巧
- 框架内置 `printf` 调试日志（C 层），运行时会输出共享内存的指针、大小、权限等信息；
- 通过 `getPyError()` （框架内部）获取 Python 端的具体异常信息；
- 测试单个功能时，先注释非核心逻辑，逐步验证解释器初始化、模块注册、函数调用、内存操作。

### 6. 交付规范
- 确保 `bridges` 目录下的 `libc2py.a` 静态库为最新编译版本；
- Go 代码中必须包含 `defer bridge.PY_Panic()` 和 `defer mem.Free()`，避免资源泄漏；
- 异常信息需明确（包含错误类型和上下文），便于 Python 开发人员定位问题；
- 共享内存分配大小需根据业务场景合理设置（避免过大浪费内存，过小导致越界）。

## 三、跨语言协作注意事项
1. **类型对齐**：Go 传递的 `int`/`int64` 会转为 Python `int`，`float64` 转为 Python `float`，`string` 转为 Python `str`；
2. **异常传递**：Python 函数抛出的异常会被 Go 端捕获并转为 `error`，需在 Go 端处理或打印；
3. **版本兼容**：Python 版本需为 3.12（与 C 胶水层编译路径一致），Go 版本需 ≥1.25.0；
4. **路径配置**：C 编译时的 Python 路径需与 Go 端 `go2c.go` 中的 `#cgo` 路径一致。