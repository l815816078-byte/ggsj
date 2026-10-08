package memory

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	_ "path/filepath"
	"reflect"
	"strings"
	"unsafe"
)

const (
	MemAuto = iota // 所有内存页
	MemA
	MemCa
	MemCd
	MemCb
	MemJh
	MemJ
	MemS
	MemV
	MemXa
	MemXs
	MemAs
	MemB
	MemO
)

const (
	I8 = iota
	U8
	I16
	U16
	I32
	U32
	I64
	U64
	F32
	F64
)

// GetModuleBase 获取模块的地址
func GetModuleBase(pid int, libName string) (int64, int64) {
	var start, end int64

	fileName := fmt.Sprintf("/proc/%d/maps", pid)
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, libName) && strings.Contains(line, "rw") && strings.Contains(line, "-") {
			_, err := fmt.Sscanf(line, "%x-%x", &start, &end)
			if err != nil {
				fmt.Println("Error parsing line:", err)
				return 0, 0
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return 0, 0
	}

	return start, end
}

// GetModuleBss 获取模块的BSS地址
func GetModuleBss(pid int, libName string) (int64, int64) {
	var start, end int64
	fileName := fmt.Sprintf("/proc/%d/maps", pid)
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return 0, 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, libName) && strings.Contains(line, "[anon:.bss]") {
			_, err := fmt.Sscanf(line, "%x-%*x", &start, &end)
			if err != nil {
				fmt.Println("Error parsing line:", err)
				return 0, 0
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return 0, 0
	}
	return start, end
}

// Read The platform specific read function.
func Read(pid int, addr int64, ptr interface{}) error {
	// Reflection magic!
	v := reflect.ValueOf(ptr)
	dataAddr := getDataAddr(v)
	dataSize := getDataSize(v)

	// Open the file mapped process memory.
	mem, err := os.Open(fmt.Sprintf("/proc/%d/mem", pid))
	defer mem.Close()
	if err != nil {
		return errors.New(fmt.Sprintf("Error opening /proc/%d/mem. Are you root?", pid))
	}

	// Create a buffer and read data into it.
	dataBuf := make([]byte, dataSize)
	n, err := mem.ReadAt(dataBuf, addr)
	if n != int(dataSize) {
		return errors.New(fmt.Sprintf("Tried to read %d bytes, actually read %d bytes\n", dataSize, n))
	} else if err != nil {
		return err
	}

	// Unsafely cast to []byte to copy data into.
	buf := (*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: dataAddr,
		Len:  int(dataSize),
		Cap:  int(dataSize),
	}))
	copy(*buf, dataBuf)
	return nil
}

// Write The platform specific write function.
func Write(pid int, addr int64, ptr interface{}) error {
	// Reflection magic!
	v := reflect.ValueOf(ptr)
	dataAddr := getDataAddr(v)
	dataSize := getDataSize(v)

	// Open the file mapped process memory.
	mem, err := os.OpenFile(fmt.Sprintf("/proc/%d/mem", pid), os.O_WRONLY, 0666)
	defer mem.Close()
	if err != nil {
		return errors.New(fmt.Sprintf("Error opening /proc/%d/mem. Are you root?", pid))
	}

	// Unsafe cast to []byte to copy data from.
	buf := (*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: dataAddr,
		Len:  int(dataSize),
		Cap:  int(dataSize),
	}))

	// Write the data from buf into memory.
	n, err := mem.WriteAt(*buf, addr)
	if n != int(dataSize) {
		return errors.New((fmt.Sprintf("Tried to write %d bytes, actually wrote %d bytes\n", dataSize, n)))
	} else if err != nil {
		return err
	}
	return nil
}

func getMemRange(filePath string, permission string) int {
	// A内存的判断并不完善,有能力的可以自己优化
	if strings.Contains(permission, "rw") && len(filePath) == 0 {
		return MemA
	} else if (strings.Contains(filePath, "/data/app/") && strings.Contains(permission, "r-xp")) || (strings.Contains(permission, "r-xp") && len(filePath) == 0) {
		return MemXa
	} else if strings.Contains(filePath, "/dev/ashmem/") {
		return MemAs
	} else if strings.Contains(filePath, "/system/fonts/") {
		return MemB
	} else if strings.Contains(filePath, "/system/framework/") {
		return MemXs
	} else if filePath == "[anon:libc_malloc]" {
		return MemCa
	} else if strings.Contains(filePath, ":bss") {
		return MemCb
	} else if strings.Contains(filePath, "/data/data/") {
		return MemCd
	} else if strings.Contains(filePath, "[anon:dalvik") {
		return MemJ
	} else if filePath == "[stack]" {
		return MemS
	} else if filePath == "/dev/kgsl-3d0" {
		return MemV
	}
	return MemO
}

func Search[T comparable](pid int, value T, memRange int, beginning int64, limit int) ([]int64, error) {
	// 确定类型的大小
	valueSize := int(reflect.TypeOf(value).Size())
	// 打开 maps 文件
	fileName := fmt.Sprintf("/proc/%d/maps", pid)
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return nil, err
	}
	defer file.Close()

	// 打开内存文件
	memFile := fmt.Sprintf("/proc/%d/mem", pid)
	mem, err := os.Open(memFile)
	if err != nil {
		fmt.Printf("Error opening %s. Are you root?\n", memFile)
		return nil, err
	}
	defer mem.Close()

	// 用切片存储匹配的地址
	var addresses []int64
	// 遍历 maps 文件
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		var start, end, offset int64
		var permission, dev, inode, filename string

		// 解析内存范围
		fmt.Sscanf(line, "%x-%x %s %x %s %s %s", &start, &end, &permission, &offset, &dev, &inode, &filename)

		// 跳过小于起始地址的内存范围
		if end <= beginning {
			continue
		}
		// 调整起始地址
		if start < beginning {
			start = beginning
		}

		// 确定是否处理此段内存
		if (getMemRange(filename, permission) == memRange || memRange == MemAuto) && strings.Contains(permission, "r") {
			dataSize := end - start
			if dataSize == 0 || dataSize > 1<<30 {
				fmt.Printf("Skipping unreasonable memory range: start=0x%x, end=0x%x, size=%d\n", start, end, dataSize)
				continue
			}

			// 使用 mmap 或一次性读取内存数据
			dataBuf := make([]byte, dataSize)
			n, err := mem.ReadAt(dataBuf, int64(start))
			if err != nil && err != io.EOF {
				fmt.Printf("Error reading memory at range 0x%x-0x%x: %v\n", start, end, err)
				continue
			}
			if n < int(dataSize) {
				dataBuf = dataBuf[:n]
			}

			// 避免多次分配切片，通过 unsafe 转换为所需类型切片
			var typedBuf []T
			header := (*reflect.SliceHeader)(unsafe.Pointer(&typedBuf))
			header.Data = uintptr(unsafe.Pointer(&dataBuf[0]))
			header.Len = len(dataBuf) / valueSize
			header.Cap = len(dataBuf) / valueSize

			// 遍历内存查找值
			for i, val := range typedBuf {
				if val == value {
					matchedAddr := start + int64(i*valueSize)
					addresses = append(addresses, matchedAddr)
					// 达到限制后返回结果
					if limit > 0 && len(addresses) >= limit {
						return addresses, nil
					}
				}
			}
		}
	}
	return addresses, nil
}
