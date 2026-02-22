//
// Created by yumu-ra on 2026/2/6.
//

#ifndef MEMORY_H
#define MEMORY_H
#include <Python.h>
//内存相关
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
#endif //MEMORY_H