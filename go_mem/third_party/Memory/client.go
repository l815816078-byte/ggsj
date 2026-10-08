package Memory

/*
#include <stdlib.h>
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

typedef int pid_t;

typedef struct {
	char *code;
	void *result;
} call_info;

typedef struct {
	char *cmdline;
	pid_t pid;
} pid_info;

typedef struct {
	int length;
	pid_info *pid_list;
} pid_result;

typedef struct {
	unsigned long long start;
	size_t size;
	char *perms;
	char *path;
} module_info;

typedef struct {
	int length;
	module_info *module_list;
} module_result;

typedef struct {
	unsigned long long address;
} search_info;

typedef struct {
	int length;
	search_info *address_list;
} search_result;

bool isMemory(unsigned long long address, pid_t pid, size_t size);
bool readMemory(pid_t pid, unsigned long long address, void *buffer, size_t size);
bool writeMemory(pid_t pid, unsigned long long address, const void *buffer, size_t size);
int getPid(const char *packageName);
pid_result *getPidX(const char *pattern);
module_result *getModuleInfo(pid_t pid, const char *moduleName);
void free_pid_result(pid_result *result);
void free_module_result(module_result *result);
char *getMemoryPermission(pid_t pid, unsigned long long address);
int setMemoryPermission(pid_t pid, unsigned long long address, const char *perm);
int getProcessArchitecture(pid_t pid);
char *injector(pid_t pid, const char *soPath);
unsigned long long remoteMmap(pid_t pid, unsigned long long addr, size_t length, int prot, int flags, int fd, long long offset);
call_info remoteCallFunction(pid_t pid, unsigned long long func_addr, unsigned long long *args, int arg_count);
search_result *searchMemory(const char *data);
search_result *searchMemoryFast(const char *data);
void freeSearchResult(search_result *result);
unsigned long long getFunctionAddress(const char *soPath, const char *functionName);
bool dumpSo(pid_t pid, const char *soName, const char *filePath);
bool dumpMemory(pid_t pid, unsigned long long address, size_t size, const char *filePath);
bool init_kerenl(void);
char *read_unity_string_from_instance(pid_t pid, unsigned long long instance, unsigned long long offset);
char *read_unity_string(int pid, unsigned long long string_address);
bool write_unity_string(pid_t pid, unsigned long long string_address, const char *new_string);
bool write_unity_string_from_instance(pid_t pid, unsigned long long instance, unsigned long long offset, const char *new_string);
void setReadBackend(int backend);
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"
)

type ReadBackend int

const (
	BackendSyscall ReadBackend = 0
	BackendKernel  ReadBackend = 1
	BackendProcfs  ReadBackend = 2
)

var (
	readBackendMu      sync.Mutex
	currentReadBackend = BackendSyscall
)

type ValueType string

const (
	TypeI8     ValueType = "int8_t"
	TypeI16    ValueType = "int16_t"
	TypeI32    ValueType = "int32_t"
	TypeI64    ValueType = "int64_t"
	TypeUI8    ValueType = "uint8_t"
	TypeUI16   ValueType = "uint16_t"
	TypeUI32   ValueType = "uint32_t"
	TypeUI64   ValueType = "uint64_t"
	TypeChar   ValueType = "char"
	TypeUChar  ValueType = "unsigned char"
	TypeF32    ValueType = "float"
	TypeF64    ValueType = "double"
	TypeString ValueType = "char*"
	TypeBinary ValueType = "binary"
	TypeUTF16  ValueType = "utf16"
)

const (
	DefaultStringReadSize = 64
	DefaultBinaryReadSize = 16
	DefaultUTF16Chars     = 16
	userSpace48BitMask    = uint64(0xFFFFFFFFFFFF)
)

const (
	PROT_NONE           = 0x0
	PROT_READ           = 0x1
	PROT_WRITE          = 0x2
	PROT_EXEC           = 0x4
	PROT_SEM            = 0x8
	PROT_GROWSDOWN      = 0x01000000
	PROT_GROWSUP        = 0x02000000
	MAP_FILE            = 0x0
	MAP_SHARED          = 0x01
	MAP_PRIVATE         = 0x02
	MAP_SHARED_VALIDATE = 0x03
	MAP_TYPE            = 0x0f
	MAP_FIXED           = 0x10
	MAP_ANONYMOUS       = 0x20
	MAP_POPULATE        = 0x008000
	MAP_NONBLOCK        = 0x010000
	MAP_STACK           = 0x020000
	MAP_HUGETLB         = 0x040000
	MAP_SYNC            = 0x080000
	MAP_FIXED_NOREPLACE = 0x100000
	MAP_UNINITIALIZED   = 0x4000000
	MLOCK_ONFAULT       = 0x01
	MS_ASYNC            = 1
	MS_INVALIDATE       = 2
	MS_SYNC             = 4
	MADV_NORMAL         = 0
	MADV_RANDOM         = 1
	MADV_SEQUENTIAL     = 2
	MADV_WILLNEED       = 3
	MADV_DONTNEED       = 4
	MADV_FREE           = 8
	MADV_REMOVE         = 9
	MADV_DONTFORK       = 10
	MADV_DOFORK         = 11
	MADV_MERGEABLE      = 12
	MADV_UNMERGEABLE    = 13
	MADV_HUGEPAGE       = 14
	MADV_NOHUGEPAGE     = 15
	MADV_DONTDUMP       = 16
	MADV_DODUMP         = 17
	MADV_WIPEONFORK     = 18
	MADV_KEEPONFORK     = 19
	MADV_COLD           = 20
	MADV_PAGEOUT        = 21
	MADV_POPULATE_READ  = 22
	MADV_POPULATE_WRITE = 23
	MADV_HWPOISON       = 100
	MADV_SOFT_OFFLINE   = 101
	PKEY_DISABLE_ACCESS = 0x1
	PKEY_DISABLE_WRITE  = 0x2
	PKEY_ACCESS_MASK    = PKEY_DISABLE_ACCESS | PKEY_DISABLE_WRITE
	MREMAP_MAYMOVE      = 1
	MREMAP_FIXED        = 2
	MREMAP_DONTUNMAP    = 4
	OVERCOMMIT_GUESS    = 0
	OVERCOMMIT_ALWAYS   = 1
	OVERCOMMIT_NEVER    = 2
)

type ProcessInfo struct {
	PID     int
	Cmdline string
}

type ModuleInfo struct {
	Start uint64
	Size  uint64
	Perms string
	Path  string
}

type Client struct {
	pid int
}

func New(pid int) (*Client, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid: %d", pid)
	}
	return &Client{pid: pid}, nil
}

func NewClient(pid int) (*Client, error) {
	return New(pid)
}

func Open(packageName string) (*Client, error) {
	pid, err := GetPID(packageName)
	if err != nil {
		return nil, err
	}
	return New(pid)
}

func GetPID(packageName string) (int, error) {
	if packageName == "" {
		return 0, errors.New("package name is empty")
	}
	cPackageName := C.CString(packageName)
	defer C.free(unsafe.Pointer(cPackageName))

	pid := int(C.getPid(cPackageName))
	if pid <= 0 {
		return 0, fmt.Errorf("getPid failed for %q", packageName)
	}
	return pid, nil
}

func FindProcesses(pattern string) ([]ProcessInfo, error) {
	if pattern == "" {
		return nil, errors.New("pattern is empty")
	}
	cPattern := C.CString(pattern)
	defer C.free(unsafe.Pointer(cPattern))

	result := C.getPidX(cPattern)
	if result == nil {
		return nil, fmt.Errorf("getPidX failed for %q", pattern)
	}
	defer C.free_pid_result(result)

	items := unsafe.Slice(result.pid_list, int(result.length))
	out := make([]ProcessInfo, 0, len(items))
	for _, item := range items {
		out = append(out, ProcessInfo{
			PID:     int(item.pid),
			Cmdline: goString(item.cmdline),
		})
	}
	return out, nil
}

func SetReadBackend(backend ReadBackend) {
	readBackendMu.Lock()
	defer readBackendMu.Unlock()
	setReadBackendLocked(backend)
}

func (c *Client) PID() int {
	return c.pid
}

func (c *Client) SetReadBackend(backend ReadBackend) {
	SetReadBackend(backend)
}

func (c *Client) ReadAt(address uint64, size int) ([]byte, error) {
	return c.readWith(address, size, false)
}

func (c *Client) ReadAtProcfs(address uint64, size int) ([]byte, error) {
	return c.readWith(address, size, true)
}

func (c *Client) ReadMemory(address uint64, valueType ValueType, bufferSize ...int) (any, error) {
	size := optionalSize(bufferSize)
	switch valueType {
	case TypeString:
		return c.ReadString(address, useDefaultSize(size, DefaultStringReadSize))
	case TypeBinary:
		return c.ReadBinary(address, useDefaultSize(size, DefaultBinaryReadSize))
	case TypeUTF16:
		return c.ReadUTF16(address, useDefaultSize(size, DefaultUTF16Chars))
	case TypeI8, TypeChar:
		return c.ReadInt8(address)
	case TypeI16:
		return c.ReadInt16(address)
	case TypeI32:
		return c.ReadInt32(address)
	case TypeI64:
		return c.ReadInt64(address)
	case TypeUI8, TypeUChar:
		return c.ReadUint8(address)
	case TypeUI16:
		return c.ReadUint16(address)
	case TypeUI32:
		return c.ReadUint32(address)
	case TypeUI64:
		return c.ReadUint64(address)
	case TypeF32:
		return c.ReadFloat32(address)
	case TypeF64:
		return c.ReadFloat64(address)
	default:
		return nil, fmt.Errorf("unsupported read type: %q", valueType)
	}
}

func (c *Client) ReadString(address uint64, bufferSize ...int) (string, error) {
	size := useDefaultSize(optionalSize(bufferSize), DefaultStringReadSize)
	buf, err := c.ReadAt(address, size)
	if err != nil {
		return "", err
	}
	return trimCString(buf), nil
}

func (c *Client) ReadBinary(address uint64, bufferSize ...int) ([]byte, error) {
	size := useDefaultSize(optionalSize(bufferSize), DefaultBinaryReadSize)
	return c.ReadAt(address, size)
}

func (c *Client) ReadUTF16(address uint64, chars ...int) (string, error) {
	count := useDefaultSize(optionalSize(chars), DefaultUTF16Chars)
	buf, err := c.ReadAt(address, count*2)
	if err != nil {
		return "", err
	}
	return decodeUTF16String(buf), nil
}

func (c *Client) ReadInt8(address uint64) (int8, error) {
	buf, err := c.ReadAt(address, 1)
	if err != nil {
		return 0, err
	}
	return int8(buf[0]), nil
}

func (c *Client) ReadInt16(address uint64) (int16, error) {
	buf, err := c.ReadAt(address, 2)
	if err != nil {
		return 0, err
	}
	return int16(binary.LittleEndian.Uint16(buf)), nil
}

func (c *Client) ReadInt32(address uint64) (int32, error) {
	buf, err := c.ReadAt(address, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

func (c *Client) ReadInt64(address uint64) (int64, error) {
	buf, err := c.ReadAt(address, 8)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(buf)), nil
}

func (c *Client) ReadUint8(address uint64) (uint8, error) {
	buf, err := c.ReadAt(address, 1)
	if err != nil {
		return 0, err
	}
	return buf[0], nil
}

func (c *Client) ReadUint16(address uint64) (uint16, error) {
	buf, err := c.ReadAt(address, 2)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(buf), nil
}

func (c *Client) ReadUint32(address uint64) (uint32, error) {
	buf, err := c.ReadAt(address, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}

func (c *Client) ReadUint64(address uint64) (uint64, error) {
	buf, err := c.ReadAt(address, 8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf), nil
}

func (c *Client) ReadFloat32(address uint64) (float32, error) {
	bits, err := c.ReadUint32(address)
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(bits), nil
}

func (c *Client) ReadFloat64(address uint64) (float64, error) {
	bits, err := c.ReadUint64(address)
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

func (c *Client) ReadChar(address uint64) (int8, error) {
	return c.ReadInt8(address)
}

func (c *Client) ReadUChar(address uint64) (uint8, error) {
	return c.ReadUint8(address)
}

func (c *Client) WriteAt(address uint64, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if !cBool(C.writeMemory(C.pid_t(c.pid), C.ulonglong(address), unsafe.Pointer(&data[0]), C.size_t(len(data)))) {
		return fmt.Errorf("writeMemory failed pid=%d addr=0x%x size=%d", c.pid, address, len(data))
	}
	return nil
}

func (c *Client) WriteMemory(address uint64, value any, valueType ValueType) error {
	switch valueType {
	case TypeString:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("value for %q must be string", valueType)
		}
		return c.WriteString(address, text)
	case TypeBinary:
		data, ok := value.([]byte)
		if !ok {
			return fmt.Errorf("value for %q must be []byte", valueType)
		}
		return c.WriteBinary(address, data)
	case TypeUTF16:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("value for %q must be string", valueType)
		}
		return c.WriteUTF16(address, text)
	case TypeI8, TypeChar:
		v, ok := value.(int8)
		if !ok {
			return fmt.Errorf("value for %q must be int8", valueType)
		}
		return c.WriteInt8(address, v)
	case TypeI16:
		v, ok := value.(int16)
		if !ok {
			return fmt.Errorf("value for %q must be int16", valueType)
		}
		return c.WriteInt16(address, v)
	case TypeI32:
		v, ok := value.(int32)
		if !ok {
			return fmt.Errorf("value for %q must be int32", valueType)
		}
		return c.WriteInt32(address, v)
	case TypeI64:
		v, ok := value.(int64)
		if !ok {
			return fmt.Errorf("value for %q must be int64", valueType)
		}
		return c.WriteInt64(address, v)
	case TypeUI8, TypeUChar:
		v, ok := value.(uint8)
		if !ok {
			return fmt.Errorf("value for %q must be uint8", valueType)
		}
		return c.WriteUint8(address, v)
	case TypeUI16:
		v, ok := value.(uint16)
		if !ok {
			return fmt.Errorf("value for %q must be uint16", valueType)
		}
		return c.WriteUint16(address, v)
	case TypeUI32:
		v, ok := value.(uint32)
		if !ok {
			return fmt.Errorf("value for %q must be uint32", valueType)
		}
		return c.WriteUint32(address, v)
	case TypeUI64:
		v, ok := value.(uint64)
		if !ok {
			return fmt.Errorf("value for %q must be uint64", valueType)
		}
		return c.WriteUint64(address, v)
	case TypeF32:
		v, ok := value.(float32)
		if !ok {
			return fmt.Errorf("value for %q must be float32", valueType)
		}
		return c.WriteFloat32(address, v)
	case TypeF64:
		v, ok := value.(float64)
		if !ok {
			return fmt.Errorf("value for %q must be float64", valueType)
		}
		return c.WriteFloat64(address, v)
	default:
		return fmt.Errorf("unsupported write type: %q", valueType)
	}
}

func (c *Client) WriteString(address uint64, value string) error {
	data := append([]byte(value), 0)
	return c.WriteAt(address, data)
}

func (c *Client) WriteBinary(address uint64, data []byte) error {
	return c.WriteAt(address, data)
}

func (c *Client) WriteUTF16(address uint64, value string) error {
	return c.WriteAt(address, encodeUTF16String(value))
}

func (c *Client) WriteInt8(address uint64, value int8) error {
	return c.WriteAt(address, []byte{byte(value)})
}

func (c *Client) WriteInt16(address uint64, value int16) error {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(value))
	return c.WriteAt(address, buf)
}

func (c *Client) WriteInt32(address uint64, value int32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(value))
	return c.WriteAt(address, buf)
}

func (c *Client) WriteInt64(address uint64, value int64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(value))
	return c.WriteAt(address, buf)
}

func (c *Client) WriteUint8(address uint64, value uint8) error {
	return c.WriteAt(address, []byte{value})
}

func (c *Client) WriteUint16(address uint64, value uint16) error {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, value)
	return c.WriteAt(address, buf)
}

func (c *Client) WriteUint32(address uint64, value uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, value)
	return c.WriteAt(address, buf)
}

func (c *Client) WriteUint64(address uint64, value uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, value)
	return c.WriteAt(address, buf)
}

func (c *Client) WriteFloat32(address uint64, value float32) error {
	return c.WriteUint32(address, math.Float32bits(value))
}

func (c *Client) WriteFloat64(address uint64, value float64) error {
	return c.WriteUint64(address, math.Float64bits(value))
}

func (c *Client) WriteChar(address uint64, value int8) error {
	return c.WriteInt8(address, value)
}

func (c *Client) WriteUChar(address uint64, value uint8) error {
	return c.WriteUint8(address, value)
}

func (c *Client) IsMemory(address uint64, size uint64) bool {
	return cBool(C.isMemory(C.ulonglong(address), C.pid_t(c.pid), C.size_t(size)))
}

func (c *Client) Architecture() (int, error) {
	arch := int(C.getProcessArchitecture(C.pid_t(c.pid)))
	if arch <= 0 {
		return 0, fmt.Errorf("getProcessArchitecture failed for pid=%d", c.pid)
	}
	return arch, nil
}

func (c *Client) MemoryPermission(address uint64) (string, error) {
	perm := C.getMemoryPermission(C.pid_t(c.pid), C.ulonglong(address))
	if perm == nil {
		return "", fmt.Errorf("getMemoryPermission failed pid=%d addr=0x%x", c.pid, address)
	}
	defer C.free(unsafe.Pointer(perm))
	return C.GoString(perm), nil
}

func (c *Client) SetMemoryPermission(address uint64, perm string) error {
	if perm == "" {
		return errors.New("permission is empty")
	}
	cPerm := C.CString(perm)
	defer C.free(unsafe.Pointer(cPerm))

	code := int(C.setMemoryPermission(C.pid_t(c.pid), C.ulonglong(address), cPerm))
	if code != 0 {
		return fmt.Errorf("setMemoryPermission failed pid=%d addr=0x%x perm=%q code=%d", c.pid, address, perm, code)
	}
	return nil
}

func (c *Client) Modules(moduleName string) ([]ModuleInfo, error) {
	cModuleName := C.CString(moduleName)
	defer C.free(unsafe.Pointer(cModuleName))

	result := C.getModuleInfo(C.pid_t(c.pid), cModuleName)
	if result == nil {
		return nil, fmt.Errorf("getModuleInfo failed pid=%d module=%q", c.pid, moduleName)
	}
	defer C.free_module_result(result)

	items := unsafe.Slice(result.module_list, int(result.length))
	out := make([]ModuleInfo, 0, len(items))
	for _, item := range items {
		out = append(out, ModuleInfo{
			Start: uint64(item.start),
			Size:  uint64(item.size),
			Perms: goString(item.perms),
			Path:  goString(item.path),
		})
	}
	return out, nil
}

func (c *Client) Search(request map[string]any) ([]uint64, error) {
	return c.searchMap(request, false)
}

func (c *Client) SearchFast(request map[string]any) ([]uint64, error) {
	return c.searchMap(request, true)
}

func (c *Client) SearchJSON(raw string) ([]uint64, error) {
	return c.searchJSON(raw, false)
}

func (c *Client) SearchFastJSON(raw string) ([]uint64, error) {
	return c.searchJSON(raw, true)
}

func (c *Client) DumpMemory(address uint64, size uint64, filePath string) error {
	if filePath == "" {
		return errors.New("file path is empty")
	}
	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cFilePath))

	if !cBool(C.dumpMemory(C.pid_t(c.pid), C.ulonglong(address), C.size_t(size), cFilePath)) {
		return fmt.Errorf("dumpMemory failed pid=%d addr=0x%x size=%d path=%q", c.pid, address, size, filePath)
	}
	return nil
}

func (c *Client) DumpSo(soName string, filePath string) error {
	if soName == "" {
		return errors.New("so name is empty")
	}
	if filePath == "" {
		return errors.New("file path is empty")
	}
	cSoName := C.CString(soName)
	defer C.free(unsafe.Pointer(cSoName))
	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cFilePath))

	if !cBool(C.dumpSo(C.pid_t(c.pid), cSoName, cFilePath)) {
		return fmt.Errorf("dumpSo failed pid=%d so=%q path=%q", c.pid, soName, filePath)
	}
	return nil
}

func (c *Client) Inject(soPath string) error {
	if soPath == "" {
		return errors.New("so path is empty")
	}
	cSoPath := C.CString(soPath)
	defer C.free(unsafe.Pointer(cSoPath))

	code := C.injector(C.pid_t(c.pid), cSoPath)
	if code == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(code))
	return errors.New(C.GoString(code))
}

func (c *Client) Injector(soPath string) error {
	return c.Inject(soPath)
}

func (c *Client) RemoteMmap(address uint64, size uint64, prot int, flags int, fd int, offset int64) (uint64, error) {
	result := uint64(C.remoteMmap(
		C.pid_t(c.pid),
		C.ulonglong(address),
		C.size_t(size),
		C.int(prot),
		C.int(flags),
		C.int(fd),
		C.longlong(offset),
	))
	if result == 0 {
		return 0, fmt.Errorf("remoteMmap failed pid=%d addr=0x%x size=%d prot=0x%x flags=0x%x fd=%d offset=%d", c.pid, address, size, prot, flags, fd, offset)
	}
	return result, nil
}

func (c *Client) RemoteCallFunction(functionAddress uint64, args ...uint64) (uint64, error) {
	var cArgs *C.ulonglong
	if len(args) > 0 {
		bufSize := C.size_t(len(args)) * C.size_t(unsafe.Sizeof(C.ulonglong(0)))
		ptr := C.malloc(bufSize)
		if ptr == nil {
			return 0, errors.New("malloc failed for remote call arguments")
		}
		defer C.free(ptr)

		items := unsafe.Slice((*C.ulonglong)(ptr), len(args))
		for i, arg := range args {
			items[i] = C.ulonglong(arg)
		}
		cArgs = (*C.ulonglong)(ptr)
	}

	result := C.remoteCallFunction(C.pid_t(c.pid), C.ulonglong(functionAddress), cArgs, C.int(len(args)))
	if result.code != nil {
		message := C.GoString(result.code)
		C.free(unsafe.Pointer(result.code))
		return 0, errors.New(message)
	}
	return uint64(uintptr(result.result)), nil
}

func InitKernel() bool {
	return cBool(C.init_kerenl())
}

func (c *Client) InitKernel() bool {
	return InitKernel()
}

func (c *Client) ReadUnityString(address uint64) (string, error) {
	result := C.read_unity_string(C.int(c.pid), C.ulonglong(address))
	if result == nil {
		return "", fmt.Errorf("read_unity_string failed pid=%d addr=0x%x", c.pid, address)
	}
	defer C.free(unsafe.Pointer(result))
	return C.GoString(result), nil
}

func (c *Client) ReadUnityStringFromInstance(instance uint64, offset uint64) (string, error) {
	result := C.read_unity_string_from_instance(C.pid_t(c.pid), C.ulonglong(instance), C.ulonglong(offset))
	if result == nil {
		return "", fmt.Errorf("read_unity_string_from_instance failed pid=%d instance=0x%x offset=0x%x", c.pid, instance, offset)
	}
	defer C.free(unsafe.Pointer(result))
	return C.GoString(result), nil
}

func (c *Client) WriteUnityString(address uint64, value string) error {
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	if !cBool(C.write_unity_string(C.pid_t(c.pid), C.ulonglong(address), cValue)) {
		return fmt.Errorf("write_unity_string failed pid=%d addr=0x%x", c.pid, address)
	}
	return nil
}

func (c *Client) WriteUnityStringFromInstance(instance uint64, offset uint64, value string) error {
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	if !cBool(C.write_unity_string_from_instance(C.pid_t(c.pid), C.ulonglong(instance), C.ulonglong(offset), cValue)) {
		return fmt.Errorf("write_unity_string_from_instance failed pid=%d instance=0x%x offset=0x%x", c.pid, instance, offset)
	}
	return nil
}

func GetFunctionAddress(soPath string, functionName string) (uint64, error) {
	if soPath == "" {
		return 0, errors.New("so path is empty")
	}
	if functionName == "" {
		return 0, errors.New("function name is empty")
	}
	cSoPath := C.CString(soPath)
	defer C.free(unsafe.Pointer(cSoPath))
	cFunctionName := C.CString(functionName)
	defer C.free(unsafe.Pointer(cFunctionName))

	address := uint64(C.getFunctionAddress(cSoPath, cFunctionName))
	if address == 0 {
		return 0, fmt.Errorf("getFunctionAddress failed so=%q function=%q", soPath, functionName)
	}
	return address, nil
}

func (c *Client) ResolveChain(address uint64, chain string, separator ...string) (uint64, error) {
	sep := "+"
	if len(separator) > 0 && separator[0] != "" {
		sep = separator[0]
	}
	chain = strings.TrimSpace(chain)
	if chain == "" {
		return address, nil
	}

	parts := strings.Split(chain, sep)
	offsets := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		offset, err := strconv.ParseInt(part, 0, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid chain offset %q: %w", part, err)
		}
		offsets = append(offsets, offset)
	}
	return c.ResolveOffsets(address, offsets...)
}

func (c *Client) ResolveOffsets(address uint64, offsets ...int64) (uint64, error) {
	if len(offsets) == 0 {
		return address, nil
	}

	pointerSize, err := c.pointerSize()
	if err != nil {
		return 0, err
	}

	for i := 0; i < len(offsets)-1; i++ {
		current := addSignedOffset(address, offsets[i])
		buf, err := c.ReadAt(current, pointerSize)
		if err != nil {
			return 0, err
		}
		if pointerSize == 4 {
			address = uint64(binary.LittleEndian.Uint32(buf))
		} else {
			address = normalizeUserSpacePointer(binary.LittleEndian.Uint64(buf))
		}
	}

	return addSignedOffset(address, offsets[len(offsets)-1]), nil
}

func (c *Client) readWith(address uint64, size int, procfs bool) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid size: %d", size)
	}
	if size == 0 {
		return []byte{}, nil
	}

	buf := make([]byte, size)
	ptr := unsafe.Pointer(&buf[0])
	name := "readMemory"
	var ok bool
	if procfs {
		name = "readMemoryProcfs"
		readBackendMu.Lock()
		previousBackend := currentReadBackend
		setReadBackendLocked(BackendProcfs)
		ok = cBool(C.readMemory(C.pid_t(c.pid), C.ulonglong(address), ptr, C.size_t(size)))
		setReadBackendLocked(previousBackend)
		readBackendMu.Unlock()
	} else {
		ok = cBool(C.readMemory(C.pid_t(c.pid), C.ulonglong(address), ptr, C.size_t(size)))
	}
	if !ok {
		return nil, fmt.Errorf("%s failed pid=%d addr=0x%x size=%d", name, c.pid, address, size)
	}
	return buf, nil
}

func setReadBackendLocked(backend ReadBackend) {
	C.setReadBackend(C.int(backend))
	currentReadBackend = backend
}

func (c *Client) searchMap(request map[string]any, fast bool) ([]uint64, error) {
	if request == nil {
		return nil, errors.New("search request is nil")
	}
	payload := make(map[string]any, len(request)+1)
	for key, value := range request {
		payload[key] = value
	}
	payload["pid"] = c.pid

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.searchRaw(string(data), fast)
}

func (c *Client) searchJSON(raw string, fast bool) ([]uint64, error) {
	if raw == "" {
		return nil, errors.New("search json is empty")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return c.searchMap(payload, fast)
}

func (c *Client) searchRaw(raw string, fast bool) ([]uint64, error) {
	cRaw := C.CString(raw)
	defer C.free(unsafe.Pointer(cRaw))

	var result *C.search_result
	if fast {
		result = C.searchMemoryFast(cRaw)
	} else {
		result = C.searchMemory(cRaw)
	}
	if result == nil {
		name := "searchMemory"
		if fast {
			name = "searchMemoryFast"
		}
		return nil, fmt.Errorf("%s returned nil for pid=%d", name, c.pid)
	}
	defer C.freeSearchResult(result)

	items := unsafe.Slice(result.address_list, int(result.length))
	out := make([]uint64, len(items))
	for index, item := range items {
		out[index] = uint64(item.address)
	}
	return out, nil
}

func cBool(value C.bool) bool {
	return value != C.bool(false)
}

func goString(value *C.char) string {
	if value == nil {
		return ""
	}
	return C.GoString(value)
}

func optionalSize(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func useDefaultSize(size int, fallback int) int {
	if size > 0 {
		return size
	}
	return fallback
}

func trimCString(buf []byte) string {
	if idx := bytes.IndexByte(buf, 0); idx >= 0 {
		buf = buf[:idx]
	}
	return string(buf)
}

func decodeUTF16String(buf []byte) string {
	if len(buf)%2 != 0 {
		buf = buf[:len(buf)-1]
	}
	words := make([]uint16, 0, len(buf)/2)
	for i := 0; i+1 < len(buf); i += 2 {
		value := binary.LittleEndian.Uint16(buf[i:])
		if value == 0 {
			break
		}
		words = append(words, value)
	}
	return string(utf16.Decode(words))
}

func encodeUTF16String(value string) []byte {
	encoded := utf16.Encode([]rune(value))
	buf := make([]byte, len(encoded)*2)
	for i, code := range encoded {
		binary.LittleEndian.PutUint16(buf[i*2:], code)
	}
	return buf
}

func addSignedOffset(address uint64, offset int64) uint64 {
	if offset < 0 {
		return address - uint64(-offset)
	}
	return address + uint64(offset)
}

func normalizeUserSpacePointer(address uint64) uint64 {
	if address > userSpace48BitMask {
		return address & userSpace48BitMask
	}
	return address
}

func (c *Client) pointerSize() (int, error) {
	arch, err := c.Architecture()
	if err != nil {
		return 0, err
	}
	if arch == 32 {
		return 4, nil
	}
	return 8, nil
}
