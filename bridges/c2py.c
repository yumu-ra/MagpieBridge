#include <stdio.h>
#include <Python.h>
#include <stdbool.h>
#include "c2py.h"
#include "memory.h"

 *
 */
void Init() {
	Py_Initialize();
}
//请在go实现map管理
/**
 *@param name : package name
 *@param  ldt bool : Live and die together
 */
PyObject*  c_Startled_bird(const char * name,bool ldt) {
	PyObject* pModule = PyImport_ImportModule(name);
	if (!pModule) {
		PyErr_Print();
		printf("Error: Cannot import module '{%s}'\n",name);
		if (ldt==true) {
			Py_FinalizeEx();
			printf("Because the module failed to load and the option 'ldt' was set to 'true', the entire interpreter was terminated.\n");
		}else {
			printf("The module cannot be loaded. Please manually manage the lifecycle of the interpreter.");
		}
	}
	return pModule;
}
/**
 * 手动杀死解释器
 */
void Go_died() {
	Py_FinalizeEx();
	printf("py ded");
}
PyObject* u_func_call(void* vpModule,const char * fn,void *vpArgs) {
	if (!vpModule) {
		PyErr_Print();
		printf("Error: Cannot import module\n");
		Py_FinalizeEx();
	}
	PyObject* pModule=(PyObject*)vpModule;
	// 获取函数 add
	PyObject* pFunc = PyObject_GetAttrString(pModule,fn);//python侧的对应函数名
	if (pFunc && PyCallable_Check(pFunc)) {
		// 构造参数 (3, 5)
		PyObject* pArgs=(PyObject *)vpArgs;
		PyObject* pResult = PyObject_CallObject(pFunc, pArgs);
		if (pResult) {
			return pResult;
		}
		Py_DECREF(pArgs);
	}
	Py_DECREF(pFunc);
	return NULL;
}
