//
// Created by yumu-ra on 2026/2/6.
//

#include "memory.h"
#include <Python.h>
//=============================================================================
// 固定写权限定义（申请时确定，终身不变）
//=============================================================================
//=============================================================================
// 共享内存结构体
// 所有状态唯一一份，通过指针跨语言共享
//=============================================================================
/*
typedef struct {
	uint8_t*           data;    // 实际共享大内存（文本/图片/字符串等字节流）
	size_t             size;    // 内存大小，由调用方传参决定
	MemPerm            perm;    // 写权限，分配时固定
	volatile uint32_t  lock;    // 占用标记：0=空闲，1=被使用，解决抢内存冲突
} MemoryShare;
*/
//=============================================================================
// 分配 MemoryShare：C 侧 malloc，返回结构体指针
// size：由外部（Go）传入需要的字节数，不写死
// perm：初始写权限，固定不变
//=============================================================================
MemoryShare* MemoryShare_Alloc(size_t bytes, MemPerm perm) {
	if (bytes == 0) {
		return NULL;
	}

	// 分配管理结构体本身
	MemoryShare* mem = (MemoryShare*)malloc(sizeof(MemoryShare));
	if (!mem) {
		return NULL;
	}

	// 分配实际共享数据区（大小由参数决定）
	mem->data = (uint8_t*)malloc(bytes);
	if (!mem->data) {
		free(mem);
		return NULL;
	}

	mem->size = bytes;
	mem->perm = perm;
	mem->lock = 0;  // 初始未锁

	return mem;
}
//=============================================================================
// 释放 MemoryShare
// 仅 Go 调用，Python 无权释放
//=============================================================================
void MemoryShare_Free(MemoryShare* mem) {
	if (!mem) {
		return;
	}
	free(mem->data);
	free(mem);
}
//=============================================================================
// 核心胶水：将 MemoryShare 转为 Python memoryview（零拷贝）
// 根据 mem->perm 自动决定返回只读/可写 memoryview
//=============================================================================

// Rewritten core function: pure C99 compatible + full debug + minimal flags
PyObject* MemoryShare_ToPythonMemoryView(MemoryShare* mem) {
    // ========== Step 1: Core debug (C99 compatible printf only) ==========
    printf("\n[DEBUG] ====== MemoryShare Parameters ======\n");
    printf("[DEBUG] mem pointer: %p\n", (void*)mem);
    if (mem) {
        printf("[DEBUG] mem->data pointer: %p\n", (void*)mem->data);
        printf("[DEBUG] mem->size: %zu bytes\n", mem->size);
        printf("[DEBUG] mem->perm: %d (MEM_WRITE_NONE=0, MEM_WRITE=1)\n", mem->perm);
        printf("[DEBUG] mem->lock: %u\n", mem->lock);
        
        // Test if memory is writable (core validation)
        if (mem->data && mem->size > 0) {
            printf("[DEBUG] Testing memory write: trying to modify first byte...\n");
            uint8_t old_val = mem->data[0];
            mem->data[0] = old_val + 1;  // Write test
            if (mem->data[0] == old_val + 1) {
                printf("[DEBUG] OK Memory is writable! Old val=%u, New val=%u\n", old_val, mem->data[0]);
                mem->data[0] = old_val;  // Restore original value
            } else {
                printf("[DEBUG] FUCK Memory is read-only! Write failed\n");
            }
        }
    }

    // ========== Step 2: Basic validation ==========
    if (!mem) {
        PyErr_SetString(PyExc_ValueError, "MemoryShare is null");
        return NULL;
    }
    if (!mem->data || mem->size == 0) {
        PyErr_SetString(PyExc_ValueError, "MemoryShare data invalid");
        return NULL;
    }

    // ========== Step 3: Flags - minimal compatible scheme ==========
    int flags;
    if (mem->perm == MEM_WRITE) {
        // Writable: only use PyBUF_WRITE (compatible with all Python versions)
        flags = PyBUF_WRITE;
        printf("[DEBUG] OK Selected writable flags: PyBUF_WRITE (0x%x)\n", flags);
    } else {
        // Read-only: only use PyBUF_READ
        flags = PyBUF_READ;
        printf("[DEBUG] FUCK Selected read-only flags: PyBUF_READ (0x%x)\n", flags);
    }

    // ========== Step 4: Create memoryview (add null check) ==========
    printf("[DEBUG] Calling PyMemoryView_FromMemory(data=%p, size=%zu, flags=0x%x)\n", (void*)mem->data, mem->size, flags);
    PyObject* mv = PyMemoryView_FromMemory((char*)mem->data, (Py_ssize_t)mem->size, flags);
    
    if (!mv) {
        // Fixed: PyErr_SetString only takes 2 parameters, debug info via printf
        PyErr_SetString(PyExc_MemoryError, "Failed to create memoryview");
        printf("[DEBUG] FUCK PyMemoryView_FromMemory returned NULL! flags=0x%x\n", flags);
        return NULL;
    }

    // ========== Step 5: Verify memoryview permission ==========
    PyObject* readonly_attr = PyObject_GetAttrString(mv, "readonly");
    if (readonly_attr) {
        int is_readonly = PyObject_IsTrue(readonly_attr);
        printf("[DEBUG] memoryview.readonly: %s\n", is_readonly ? "True (read-only)" : "False (writable)");
        Py_DECREF(readonly_attr);
    }

    // ========== Step 6: Reference count ==========
    Py_INCREF(mv);
    printf("[DEBUG] ====== Function end ======\n");

    return mv;
}
/*
//精简版
PyObject* MemoryShare_ToPythonMemoryView(MemoryShare* mem) {
    // ========== Step 2: Basic validation ==========
    if (!mem) {
        PyErr_SetString(PyExc_ValueError, "MemoryShare is null");
        return NULL;
    }
    if (!mem->data || mem->size == 0) {
        PyErr_SetString(PyExc_ValueError, "MemoryShare data invalid");
        return NULL;
    }

    // ========== Step 3: Flags - minimal compatible scheme ==========
    int flags;
    if (mem->perm == MEM_WRITE) {
        // Writable: only use PyBUF_WRITE (compatible with all Python versions)
        flags = PyBUF_WRITE;
    } else {
        // Read-only: only use PyBUF_READ
        flags = PyBUF_READ;
    }

    // ========== Step 4: Create memoryview (add null check) ==========
    PyObject* mv = PyMemoryView_FromMemory((char*)mem->data, (Py_ssize_t)mem->size, flags);
    
    if (!mv) {
        // Fixed: PyErr_SetString only takes 2 parameters, debug info via printf
        PyErr_SetString(PyExc_MemoryError, "Failed to create memoryview");
        return NULL;
    }

    // ========== Step 6: Reference count ==========
    Py_INCREF(mv);

    return mv;
}*/
