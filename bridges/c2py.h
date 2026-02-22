//coder yumu_Ra
#ifndef MAIN_H
#define MAIN_H
#include <stdbool.h>   // 包含 bool 类型的定义
#include <stdio.h>     // 包含 printf 等标准库（若用到）
#include <stdlib.h>
#include <Python.h>
#include <memory.h>
void Init();
PyObject*  c_Startled_bird(const char * name,bool ldt);
void Go_died();
PyObject* u_func_call(void* vpModule,const char * fn,void *vpArgs);
#endif //MAIN_H