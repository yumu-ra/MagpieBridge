package main

import (
	"fmt"

	bridge "example.com/bridges"
)

// ==================== 主函数 ====================
func main() {
	defer bridge.PY_Panic()
	bridge.PY_Init()

	if err := bridge.Register("main"); err != nil {
		fmt.Printf("❌ Module registration failed: %v\n", err)
		return
	}

	result, err := bridge.PYCall("main", "add", 1, 2)
	if err != nil {
		fmt.Printf("❌ Function call failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Result: %v (type: %T)\n", result, result)
	mem, err := bridge.NewSharedMemory(32, bridge.MEM_WRITE)
	if err != nil {
		fmt.Printf("❌ Function call failed: %v\n", err)
		return
	}
	defer mem.Free()
	_, err = bridge.PYCall("main", "write_photo", mem.Ptr())
	if err != nil {
		fmt.Printf("❌ Function call failed: %v\n", err)
		return
	}
	_, err = bridge.PYCall("main", "read_photo", mem.Ptr())
	if err != nil {
		fmt.Printf("❌ Function call failed: %v\n", err)
		return
	}

}
