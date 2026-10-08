package main

import (
	"Memory"
	"fmt"
)

func main() {
	request := Memory.NewSearchRequest(Memory.EqualSearch(Memory.SearchTypeI32, 1234)).
		WithRegions(Memory.RegionA, Memory.RegionOther).
		WithRange(0x1000, 0x2000).
		WithDataFilters(Memory.EqualFilter(Memory.SearchTypeString, 0x10, "player")).
		WithMax(32)

	raw, err := request.JSON()
	if err != nil {
		fmt.Println("Memory request build failed:", err)
		return
	}

	prot := Memory.PROT_READ | Memory.PROT_WRITE
	flags := Memory.MAP_PRIVATE | Memory.MAP_ANONYMOUS

	fmt.Println("Memory imported successfully")
	fmt.Println("search request:", raw)
	fmt.Printf("remote mmap flags: prot=0x%x flags=0x%x\n", prot, flags)

	packageName := "bin.mt.plus.canary"
	pid, err := Memory.GetPID(packageName)
	if err != nil {
		fmt.Println("get pid failed:", err)
		return
	}

	client, err := Memory.New(pid)
	if err != nil {
		fmt.Println("open client failed:", err)
		return
	}

	fmt.Printf("package=%s pid=%d\n", packageName, client.PID())

	value, err := client.ReadInt32(0x12D05DC4)
	if err != nil {
		fmt.Println("read int32 failed:", err)
		return
	}

	fmt.Printf("int32 @0x%x = %d\n", 0x12D05DC4, value)
	m, _ := client.Modules("libc.so")
	fmt.Println("libc.so modules:", m)

}
