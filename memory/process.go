package memory

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/unicode"
)

// Process holds general information about the process,
// as well as embedding the struct ProcPlatAttribs which contains platform-specific data
// such as Windows process handles, linux `/proc` file handles, etc.
type Process struct {
	// Platform independent process details
	PID int
}

// GetPid 获取指定包名Pid
func GetPid(pkgName string) int {
	dir, err := os.Open("/proc")
	if err != nil {
		panic(err)
	}
	defer func(dir *os.File) {
		err := dir.Close()
		if err != nil {

		}
	}(dir)

	names, err := dir.Readdirnames(-1)
	if err != nil {
		panic(err)
	}

	for _, name := range names {
		id, err := strconv.Atoi(name)
		if err != nil {
			continue
		}
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", id))
		if err != nil {
			continue
		}
		if strings.Trim(string(cmdline), "\000") == pkgName {
			return id
		}
	}
	return -1
}

// GetProcessByPID returns the process with the given PID.
func GetProcessByPID(pid int) (Process, error) {
	// Try to open the folder to see if it exists and if we have access to it.
	f, err := os.Open(fmt.Sprintf("/proc/%d", pid))
	defer f.Close()
	if err != nil {
		return Process{}, errors.New(fmt.Sprintf("Error opening /proc/%d", pid))
	}

	return Process{PID: pid}, nil
}

// GetProcessByPkgName returns the process with the given file name.
// If multiple processes have the same filename, the first process
// enumerated by this function is returned.
func GetProcessByPkgName(fileName string) (Process, error) {
	// Open the /proc *nix directory.
	procDir, err := os.Open("/proc")
	defer procDir.Close()
	if err != nil {
		fmt.Println(err)
		return Process{}, errors.New("error on opening /proc")
	}

	// Read in the directory names. Processes are listed
	// as folders within this directory, named by their PID.
	dirNames, err := procDir.Readdirnames(0)
	if err != nil {
		fmt.Println(err)
		return Process{}, errors.New("error on reading dirs from /proc")
	}

	// Enumerate all directories here.
	for _, dirString := range dirNames {

		// Parse the directory name as a int.
		pid, err := strconv.ParseUint(dirString, 10, 64)

		// If it is not a numeric directory name, skip it.
		var v *strconv.NumError
		if errors.As(err, &v) && errors.Is(v.Err, strconv.ErrSyntax) {
			continue
		}

		// Read the target program's cmdline file.
		tmpFileBytes, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			fmt.Println(err)
			return Process{}, errors.New("Error on enumerating dirs from " + fmt.Sprintf("/proc/%d/cmdline", pid))
		}
		// HACK!
		// Stat file [1] has the file name surrounded by ()
		curProcFileName := strings.Trim(string(tmpFileBytes), "\000")
		// Check if this is the process we are looking for.
		if curProcFileName == fileName {
			return Process{PID: int(pid)}, nil
		}
	}

	return Process{}, errors.New("couldn't find a process with the given file name")
}

// GetModuleBase reads an int8.
func (p *Process) GetModuleBase(libName string) (int64, int64) {
	return GetModuleBase(p.PID, libName)
}

// GetModuleBss reads an int8.
func (p *Process) GetModuleBss(libName string) (int64, int64) {
	return GetModuleBss(p.PID, libName)
}

// Helper function to handle read operations generically.
func readValue[T any](pid int, addr int64) (T, error) {
	var value T
	err := Read(pid, addr, &value)
	return value, err
}

// Helper function to handle write operations generically.
func writeValue[T any](pid int, addr int64, value T) error {
	return Write(pid, addr, &value)
}

// ReadMemory reads memory at a given address for a specific type.
func (p *Process) ReadMemory(addr int64, memoryType int) (interface{}, error) {
	switch memoryType {
	case I8:
		return readValue[int8](p.PID, addr)
	case I16:
		return readValue[int16](p.PID, addr)
	case I32:
		return readValue[int32](p.PID, addr)
	case I64:
		return readValue[int64](p.PID, addr)
	case U8:
		return readValue[uint8](p.PID, addr)
	case U16:
		return readValue[uint16](p.PID, addr)
	case U32:
		return readValue[uint32](p.PID, addr)
	case U64:
		return readValue[uint64](p.PID, addr)
	case F32:
		return readValue[float32](p.PID, addr)
	case F64:
		return readValue[float64](p.PID, addr)
	default:
		return nil, fmt.Errorf("invalid memory type: %d", memoryType)
	}
}

// WriteMemory writes a value to memory at a given address.
func (p *Process) WriteMemory(addr int64, value interface{}, memoryType int) error {
	switch memoryType {
	case I8:
		return writeValue(p.PID, addr, value.(int8))
	case I16:
		return writeValue(p.PID, addr, value.(int16))
	case I32:
		return writeValue(p.PID, addr, value.(int32))
	case I64:
		return writeValue(p.PID, addr, value.(int64))
	case U8:
		return writeValue(p.PID, addr, value.(uint8))
	case U16:
		return writeValue(p.PID, addr, value.(uint16))
	case U32:
		return writeValue(p.PID, addr, value.(uint32))
	case U64:
		return writeValue(p.PID, addr, value.(uint64))
	case F32:
		return writeValue(p.PID, addr, value.(float32))
	case F64:
		return writeValue(p.PID, addr, value.(float64))
	default:
		return fmt.Errorf("invalid memory type: %d", memoryType)
	}
}

// MemorySearch searches for a value in memory.
func (p *Process) MemorySearch(value interface{}, memoryType int, memRange int, beginning int64, limit int) ([]int64, error) {
	switch memoryType {
	case I8:
		return Search(p.PID, value.(int8), memRange, beginning, limit)
	case I16:
		return Search(p.PID, value.(int16), memRange, beginning, limit)
	case I32:
		return Search(p.PID, value.(int32), memRange, beginning, limit)
	case I64:
		return Search(p.PID, value.(int64), memRange, beginning, limit)
	case U8:
		return Search(p.PID, value.(uint8), memRange, beginning, limit)
	case U16:
		return Search(p.PID, value.(uint16), memRange, beginning, limit)
	case U32:
		return Search(p.PID, value.(uint32), memRange, beginning, limit)
	case U64:
		return Search(p.PID, value.(uint64), memRange, beginning, limit)
	case F32:
		return Search(p.PID, value.(float32), memRange, beginning, limit)
	case F64:
		return Search(p.PID, value.(float64), memRange, beginning, limit)
	default:
		return nil, fmt.Errorf("invalid memory type: %d", memoryType)
	}
}

// MemoryOffset filters addresses by offset and value.
func (p *Process) MemoryOffset(addresses []int64, value interface{}, offset int, memoryType int) []int64 {
	var results []int64
	for _, addr := range addresses {
		readValue, err := p.ReadMemory(addr+int64(offset), memoryType)
		if err == nil && reflect.DeepEqual(readValue, value) {
			results = append(results, addr)
		}
	}
	return results
}

// ReadBytes reads a slice of bytes.
func (p *Process) ReadBytes(addr int64, size int) ([]byte, error) {
	data := make([]byte, size)
	err := Read(p.PID, addr, &data)
	return data, err
}

// WriteBytes writes a slice of bytes.
func (p *Process) WriteBytes(addr int64, data []byte) error {
	return Write(p.PID, addr, &data)
}

// ReadPointerChain reads a uint32 pointer chain with offsets.
func (p *Process) ReadPointerChain(addr int64, offsets ...int64) (uint32, error) {
	curPtr, err := p.ReadMemory(addr, I32)
	if err != nil {
		return 0, fmt.Errorf("read failed at address 0x%X: %w", addr, err)
	}

	var pointer = int64(curPtr.(int32))
	for _, offset := range offsets {
		curPtr, err = p.ReadMemory(pointer+offset, I32)
		if err != nil {
			return 0, fmt.Errorf("read failed at address 0x%X: %w", addr+offset, err)
		}
	}
	return curPtr.(uint32), nil
}

// Takes a []byte and returns the index of the first 0 byte, or -1 if none.
func cstrlen(data []byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			return i
		}
	}
	return -1
}

// ReadNullTerminatedUTF8String reads a null-termimated UTF8 string.
func (p *Process) ReadNullTerminatedUTF8String(addr int64) (string, error) {
	var outputBuffer []byte
	readSize := 2048
	for {
		// Make a buffer to hold our temporary string data.
		v := make([]byte, readSize)
		err := Read(p.PID, addr+int64(len(outputBuffer)), &v)
		if err != nil {
			// Halve readSize if we got an error on read.
			if readSize > 1 {
				readSize /= 2
				continue
			}

			return "", err
		}

		// Find the null index in our buffer.
		idx := cstrlen(v)
		if idx == -1 {
			// No null-terminator in the buffer.
			outputBuffer = append(outputBuffer, v...)
		} else {
			outputBuffer = v[:idx]
			break
		}
	}
	return string(outputBuffer), nil // Golang will decode as utf8.
}

// Takes a []uint16 and returns the index of the first 0, or -1 if none.
func cwstrlen(data []uint16) int {
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			return i
		}
	}
	return -1
}

// ReadNullTerminatedUTF16String reads a null-termimated UTF16 string.
// Respects BOM, assumes little endianess if no BOM is present.
func (p *Process) ReadNullTerminatedUTF16String(addr int64) (string, error) {
	var outputBuffer []uint16
	readSize := 2048
	for {
		// Make a buffer to hold our temporary string data.
		v := make([]uint16, readSize)
		err := Read(p.PID, addr+int64(len(outputBuffer)), &v)
		if err != nil {
			// Halve readSize if we got an error on read.
			if readSize > 1 {
				readSize /= 2
				continue
			}

			return "", err
		}

		// Find the null index in our buffer.
		idx := cwstrlen(v)
		if idx == -1 {
			// No null-terminator in the buffer.
			outputBuffer = append(outputBuffer, v...)
		} else {
			outputBuffer = v[:idx]
			break
		}
	}

	// Convert the []uint16 -> []byte
	byteBuf := make([]byte, len(outputBuffer)*2)
	for i := 0; i < len(outputBuffer); i++ {
		byteBuf[(i*2)+0] = byte((outputBuffer[i] >> 0) & 0xFF)
		byteBuf[(i*2)+1] = byte((outputBuffer[i] >> 8) & 0xFF)
	}

	// Decode the UTF16 to UTF8
	decoder := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
	utf8Bytes, err := decoder.Bytes(byteBuf)
	if err != nil {
		return "", err
	}

	return string(utf8Bytes), nil
}
