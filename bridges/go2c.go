package bridges

/*
#cgo CFLAGS: -I. -I"D:/Application/Python3.12/include"
#cgo LDFLAGS: -L./ -l:libc2py.a -L"D:/Application/Python3.12/libs" -lpython312

#include <Python.h>

//#include <c2py.h>
//#include "memory.h"

#include <stdlib.h>
#include <stdbool.h>
static int wrap_PyFloat_Check(PyObject* o) { return PyFloat_Check(o); }
static int wrap_PyLong_Check(PyObject* o)  { return PyLong_Check(o); }
static int wrap_PyUnicode_Check(PyObject* o) { return PyUnicode_Check(o); }

// 原子 CAS 操作（用来安全上锁，可选但推荐）
static inline void mem_lock(volatile uint32_t *lock) {
    while (__sync_val_compare_and_swap(lock, 0, 1) != 0);
}
static inline void mem_unlock(volatile uint32_t *lock) {
    __sync_lock_test_and_set(lock, 0);
}

typedef enum {
    MEM_WRITE_NONE,    // 不可写
    MEM_WRITE // Python可写
} MemPerm;
typedef struct {
	uint8_t*           data;    // 实际共享大内存（文本/图片/字符串等字节流）
	size_t             size;    // 内存大小，由调用方传参决定
	MemPerm            perm;    // 写权限，分配时固定
	volatile uint32_t  lock;    // 占用标记：0=空闲，1=被使用，解决抢内存冲突
} MemoryShare;
MemoryShare* MemoryShare_Alloc(size_t bytes, MemPerm perm);
void MemoryShare_Free(MemoryShare* mem);
PyObject* MemoryShare_ToPythonMemoryView(MemoryShare* mem);

void Init();
PyObject*  c_Startled_bird(const char * name,bool ldt);
void Go_died();
PyObject* u_func_call(void* vpModule,const char * fn,void *vpArgs);
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

var (
	pythonInitialized bool
	modules           = make(map[string]unsafe.Pointer)
)

// GIL 管理（仅用于不确定是否持有 GIL 的场景，如 goroutine）
func withGIL(f func()) {
	if !pythonInitialized {
		panic("BUG: withGIL called before PY_Init")
	}
	state := C.PyGILState_Ensure()
	defer C.PyGILState_Release(state)
	f()
}

// ==================== 初始化状态检查（公共 API 使用） ====================
func checkInitialized() error {
	if !pythonInitialized {
		return errors.New("Python not initialized. Call PY_Init() first")
	}
	return nil
}

// ==================== Python 初始化/清理 ====================
func PY_Init() {
	if pythonInitialized {
		return
	}

	if C.Py_IsInitialized() == 0 {
		C.Py_Initialize()
		if C.Py_IsInitialized() == 0 {
			panic("Py_Initialize failed!")
		}
	}

	// ✅ 关键：主线程已有 GIL，直接调用
	C.Init()

	pythonInitialized = true
	fmt.Println("✓ Python interpreter initialized")
}

func PY_Close() {
	if !pythonInitialized {
		return
	}

	// ✅ 关键修复：不要用 withGIL！
	// Py_Finalize 会自动清理主线程状态，额外的 PyGILState_Ensure 会干扰清理
	C.Go_died()

	C.Py_Finalize()
	pythonInitialized = false
	fmt.Println("✓ Python interpreter finalized")
}
func PY_Panic() {
	if r := recover(); r != nil {
		fmt.Printf("Panic: %v\n", r)
		if pythonInitialized {
			// 紧急清理：同样不使用 withGIL
			C.Go_died()
			C.Py_Finalize()
		}
		panic(r)
	}
	if pythonInitialized {
		PY_Close() // 正常路径
	}
}

// ==================== 模块注册 ====================
func Register(packageName string) error {
	if err := checkInitialized(); err != nil {
		return err
	}

	var result unsafe.Pointer
	withGIL(func() {
		cName := C.CString(packageName)
		defer C.free(unsafe.Pointer(cName))

		mod := C.c_Startled_bird(cName, true) // 1 = true in C
		if mod != nil {
			result = unsafe.Pointer(mod)
		}
	})

	if result == nil {
		return fmt.Errorf("failed to import module '%s'", packageName)
	}

	modules[packageName] = result
	fmt.Printf("✓ Module '%s' registered\n", packageName)
	return nil
}

// ==================== 参数转换 (Go → Python) ====================
func go2py(args ...interface{}) (*C.PyObject, error) {
	n := len(args)
	tuple := C.PyTuple_New(C.Py_ssize_t(n))
	if tuple == nil {
		return nil, fmt.Errorf("PyTuple_New failed: %s", getPyError())
	}

	for i, arg := range args {
		var obj *C.PyObject

		switch v := arg.(type) {
		case int:
			obj = C.PyLong_FromLong(C.long(v))
		case int64:
			obj = C.PyLong_FromLongLong(C.longlong(v))
		case float64:
			obj = C.PyFloat_FromDouble(C.double(v))
		case string:
			cStr := C.CString(v)
			obj = C.PyUnicode_FromString(cStr)
			C.free(unsafe.Pointer(cStr))
		case uintptr:
			obj = C.MemoryShare_ToPythonMemoryView((*C.MemoryShare)(unsafe.Pointer(v)))
			fmt.Println("Warning: uintptr is being used.")
		default:
			// 清理已创建的对象
			for j := 0; j < i; j++ {
				item := C.PyTuple_GetItem(tuple, C.Py_ssize_t(j))
				if item != nil {
					C.Py_DECREF(item)
				}
			}
			C.Py_DECREF(tuple)
			return nil, fmt.Errorf("unsupported type: %T (only int/int64/float64/string supported)", v)
		}

		if obj == nil {
			for j := 0; j < i; j++ {
				item := C.PyTuple_GetItem(tuple, C.Py_ssize_t(j))
				if item != nil {
					C.Py_DECREF(item)
				}
			}
			C.Py_DECREF(tuple)
			return nil, fmt.Errorf("conversion failed for arg %d: %s", i, getPyError())
		}

		if C.PyTuple_SetItem(tuple, C.Py_ssize_t(i), obj) != 0 {
			C.Py_DECREF(obj)
			for j := 0; j < i; j++ {
				item := C.PyTuple_GetItem(tuple, C.Py_ssize_t(j))
				if item != nil {
					C.Py_DECREF(item)
				}
			}
			C.Py_DECREF(tuple)
			return nil, fmt.Errorf("PyTuple_SetItem failed for arg %d: %s", i, getPyError())
		}
	}

	return tuple, nil
}

// ==================== 错误处理辅助 ====================
func getPyError() string {
	if C.PyErr_Occurred() == nil {
		return ""
	}

	var pType, pValue, pTraceback *C.PyObject
	C.PyErr_Fetch(&pType, &pValue, &pTraceback)
	defer func() {
		if pType != nil {
			C.Py_DECREF(pType)
		}
		if pValue != nil {
			C.Py_DECREF(pValue)
		}
		if pTraceback != nil {
			C.Py_DECREF(pTraceback)
		}
	}()

	if pValue == nil {
		return "unknown Python exception"
	}

	strObj := C.PyObject_Str(pValue)
	if strObj == nil {
		return "failed to stringify exception"
	}
	defer C.Py_DECREF(strObj)

	cStr := C.PyUnicode_AsUTF8(strObj)
	if cStr == nil {
		return "failed to get UTF-8 string"
	}
	return C.GoString(cStr)
}

// ==================== 返回值转换 (Python → Go) ====================
func py2go(obj *C.PyObject) (interface{}, error) {
	if obj == nil {
		errMsg := getPyError()
		if errMsg != "" {
			return nil, fmt.Errorf("Python exception: %s", errMsg)
		}
		return nil, errors.New("received nil PyObject")
	}

	// None
	if obj == C.Py_None {
		return nil, nil
	}

	// int
	if C.wrap_PyLong_Check(obj) != 0 {
		val := C.PyLong_AsLongLong(obj)
		if C.PyErr_Occurred() != nil {
			return nil, fmt.Errorf("int conversion failed: %s", getPyError())
		}
		return int64(val), nil
	}

	// float
	if C.wrap_PyFloat_Check(obj) != 0 {
		val := C.PyFloat_AsDouble(obj)
		if C.PyErr_Occurred() != nil {
			return nil, fmt.Errorf("float conversion failed: %s", getPyError())
		}
		return float64(val), nil
	}

	// string
	if C.wrap_PyUnicode_Check(obj) != 0 {
		cStr := C.PyUnicode_AsUTF8(obj)
		if cStr == nil {
			return nil, fmt.Errorf("string conversion failed: %s", getPyError())
		}
		return C.GoString(cStr), nil
	}

	// 未知类型
	repr := C.PyObject_Repr(obj)
	if repr != nil {
		defer C.Py_DECREF(repr)
		cStr := C.PyUnicode_AsUTF8(repr)
		if cStr != nil {
			return nil, fmt.Errorf("unsupported return type: %s", C.GoString(cStr))
		}
	}
	return nil, errors.New("unsupported return type (only int/float/string/None supported)")
}

// ==================== 函数调用 ====================
func PYCall(moduleName, funcName string, args ...interface{}) (interface{}, error) {
	if err := checkInitialized(); err != nil {
		return nil, err
	}

	mod, exists := modules[moduleName]
	if !exists {
		return nil, fmt.Errorf("module '%s' not registered", moduleName)
	}

	var result interface{}
	var err error

	withGIL(func() {
		pyArgs, convErr := go2py(args...)
		if convErr != nil {
			err = convErr
			return
		}
		defer C.Py_DECREF(pyArgs)

		cFuncName := C.CString(funcName)
		defer C.free(unsafe.Pointer(cFuncName))

		pyResult := C.u_func_call(mod, cFuncName, unsafe.Pointer(pyArgs))
		if pyResult == nil {
			err = fmt.Errorf("function call failed: %s", getPyError())
			return
		}
		defer C.Py_DECREF(pyResult)

		result, err = py2go(pyResult)
	})

	return result, err
}

// ==================== 共享内存 ====================
type MemPerm int

const (
	MEM_WRITE_NONE MemPerm = iota
	MEM_WRITE
)

type SharedMemory struct {
	cMem *C.MemoryShare
}

func NewSharedMemory(sizeKB int32, perm MemPerm) (*SharedMemory, error) {
	if err := checkInitialized(); err != nil {
		return nil, err
	}

	if sizeKB <= 0 {
		return nil, errors.New("size must be positive")
	}

	var cPerm C.MemPerm
	switch perm {
	case MEM_WRITE_NONE:
		cPerm = C.MEM_WRITE_NONE
	case MEM_WRITE:
		cPerm = C.MEM_WRITE
	default:
		return nil, errors.New("invalid permission")
	}

	var mem *C.MemoryShare
	withGIL(func() {
		mem = C.MemoryShare_Alloc(C.size_t(sizeKB*1024), cPerm)
	})

	if mem == nil {
		return nil, errors.New("failed to allocate shared memory")
	}
	fmt.Println("memory")
	return &SharedMemory{cMem: mem}, nil
}

func (sm *SharedMemory) Free() {
	if sm.cMem != nil {
		withGIL(func() {
			C.MemoryShare_Free(sm.cMem)
		})
		sm.cMem = nil
	}
}

func (sm *SharedMemory) Write(data []byte) error {
	if sm.cMem == nil {
		return errors.New("shared memory freed")
	}
	if C.size_t(len(data)) > sm.cMem.size {
		return fmt.Errorf("data too large (%d > %d)", len(data), sm.cMem.size)
	}

	C.mem_lock(&sm.cMem.lock)
	defer C.mem_unlock(&sm.cMem.lock)

	dst := (*[1 << 30]byte)(unsafe.Pointer(sm.cMem.data))[:sm.cMem.size:sm.cMem.size]
	copy(dst, data)
	return nil
}

func (sm *SharedMemory) Ptr() uintptr {
	if sm.cMem == nil {
		return 0
	}
	return uintptr(unsafe.Pointer(sm.cMem))
}
