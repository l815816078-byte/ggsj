package imageHandler

/*
#include "ImageHandler.h"
#include <stdlib.h>
#cgo CXXFLAGS: -std=c++11
#cgo arm64 LDFLAGS: -L../resources/libs/arm64-v8a -lranyiyu
#cgo amd64 LDFLAGS: -L../resources/libs/x86_64 -lranyiyu
#cgo 386 LDFLAGS: -L../resources/libs/x86 -lranyiyu
*/
import "C"
import (
	"app/assets"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Dasongzi1366/AutoGo/device"
	"github.com/Dasongzi1366/AutoGo/files"
	"github.com/Dasongzi1366/AutoGo/images"
)

var _version_ string = "v4.9.3"

// 占位符
var defMat = &Mat{ptr: 0}

// 参数元信息结构：存储参数名称、默认值和类型
type paramMeta struct {
	name string      // 参数名称
	def  interface{} // 默认值
}

// datainfo 结构体
type DataInfo struct {
	Colors    string
	ImageMode string
	Info      []float64
	Hash      string
}

// 定义单个检测结果的结构体
type Match struct {
	Index      int     `json:"index"`      // 索引
	Left       int     `json:"left"`       // 左坐标
	Top        int     `json:"top"`        // 上坐标
	Right      int     `json:"right"`      // 右坐标
	Bottom     int     `json:"bottom"`     // 下坐标
	Similarity float64 `json:"similarity"` // 相似度
	Info       string  `json:"info"`       // 附加信息
}

type Point struct {
	X int `json:"x"` // 识别到的坐标X
	Y int `json:"y"` // 识别到的坐标Y
}

type PointEx struct {
	X int `json:"x"` // 识别到的坐标X
	Y int `json:"y"` // 识别到的坐标Y
	R int `json:"r"` // 半径R
}

type PColor struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Color1    string `json:"color1"`
	Color2    string `json:"color2"`
	HasTarget bool   `json:"hasTarget"`
}

type Rect struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

type ConnectedState struct {
	Index     int   `json:"index"`
	Width     int   `json:"width"`
	Height    int   `json:"height"`
	WhiteArea int   `json:"whiteArea"`
	RectArea  int   `json:"rectArea"`
	Center    Point `json:"center"`
	Rect      Rect  `json:"rect"`
}

func (rect *Rect) GetWidth() int {
	return rect.Right - rect.Left
}
func (rect *Rect) GetHeight() int {
	return rect.Bottom - rect.Right
}

// PointVector 封装C++的vector<cv::Point>句柄
type PointVector struct {
	ptr C.long // 存储C++返回的句柄
}

// NewPointVector 创建新的PointVector（对应C++的PointVector()）
func NewPointVector() *PointVector {
	handle := C.PointVector()
	return &PointVector{ptr: handle}
}

// Delete 释放底层C++ vector<cv::Point>内存（必须调用，避免泄漏）
func (pv *PointVector) Delete() {
	if pv.ptr != 0 {
		C.Delete_PointVector(pv.ptr)
		pv.ptr = 0 // 避免重复释放
	}
}

// Add 向向量添加一个点（对应C++的PointVector_Add）
func (pv *PointVector) Add(x, y int) {
	if pv.ptr == 0 {
		return
	}
	C.PointVector_Add(pv.ptr, C.int(x), C.int(y))
}

// Insert 在指定索引插入一个点（对应C++的PointVector_Insert）
func (pv *PointVector) Insert(index, x, y int) {
	if pv.ptr == 0 {
		return
	}
	C.PointVector_Insert(pv.ptr, C.int(index), C.int(x), C.int(y))
}

// RemoveAt 删除指定索引的点（对应C++的PointVector_RemoveAt）
func (pv *PointVector) RemoveAt(index int) {
	if pv.ptr == 0 {
		return
	}
	C.PointVector_RemoveAt(pv.ptr, C.int(index))
}

// Remove 删除第一个匹配的点（对应C++的PointVector_Remove）
func (pv *PointVector) Remove(x, y int) bool {
	if pv.ptr == 0 {
		return false
	}
	return bool(C.PointVector_Remove(pv.ptr, C.int(x), C.int(y)))
}

// Get 获取指定索引的点（对应C++的PointVector_Get）
func (pv *PointVector) Get(index int) (Point, bool) {
	if pv.ptr == 0 {
		return Point{}, false
	}
	var cX, cY C.int
	ok := bool(C.PointVector_Get(pv.ptr, C.int(index), &cX, &cY))
	return Point{X: int(cX), Y: int(cY)}, ok
}

// Size 获取向量中点的数量（对应C++的PointVector_Size）
func (pv *PointVector) Size() int {
	if pv.ptr == 0 {
		return 0
	}
	return int(C.PointVector_Size(pv.ptr))
}

// ToSlice 将向量转换为Go的[]Point（便于Go原生操作）
func (pv *PointVector) ToSlice() []Point {
	size := pv.Size()
	slice := make([]Point, 0, size)
	for i := 0; i < size; i++ {
		if p, ok := pv.Get(i); ok {
			slice = append(slice, p)
		}
	}
	return slice
}

// PointVector2D 封装C++的vector<vector<cv::Point>>句柄
type PointVector2D struct {
	ptr C.long // 存储C++返回的句柄
}

// NewPointVector2D 创建新的二维点向量（对应C++的PointVector2D()）
func NewPointVector2D() *PointVector2D {
	handle := C.PointVector2D()
	return &PointVector2D{ptr: handle}
}

// Delete 释放底层C++ vector<vector<cv::Point>>内存（必须调用）
func (pv2d *PointVector2D) Delete() {
	if pv2d.ptr != 0 {
		C.Delete_PointVector2D(pv2d.ptr)
		pv2d.ptr = 0 // 避免重复释放
	}
}

// AddVector 向二维向量添加一个内部点向量（对应C++的PointVector2D_AddVector）
// 注意：内部向量会被复制，原向量可单独释放
func (pv2d *PointVector2D) AddVector(vec *PointVector) {
	if pv2d.ptr == 0 || vec == nil || vec.ptr == 0 {
		return
	}
	C.PointVector2D_AddVector(pv2d.ptr, vec.ptr)
}

// InsertVector 在指定索引插入一个内部点向量（对应C++的PointVector2D_InsertVector）
func (pv2d *PointVector2D) InsertVector(index int, vec *PointVector) {
	if pv2d.ptr == 0 || vec == nil || vec.ptr == 0 {
		return
	}
	C.PointVector2D_InsertVector(pv2d.ptr, C.int(index), vec.ptr)
}

// RemoveVectorAt 删除指定索引的内部向量（对应C++的PointVector2D_RemoveVectorAt）
func (pv2d *PointVector2D) RemoveVectorAt(index int) {
	if pv2d.ptr == 0 {
		return
	}
	C.PointVector2D_RemoveVectorAt(pv2d.ptr, C.int(index))
}

// Size 获取内部向量的数量（对应C++的PointVector2D_Size）
func (pv2d *PointVector2D) Size() int {
	if pv2d.ptr == 0 {
		return 0
	}
	return int(C.PointVector2D_Size(pv2d.ptr))
}

// GetVector 获取指定索引的内部向量（返回新的PointVector，需单独调用Delete()）
func (pv2d *PointVector2D) GetVector(index int) *PointVector {
	if pv2d.ptr == 0 {
		return nil
	}
	handle := C.PointVector2D_GetVector(pv2d.ptr, C.int(index))
	if handle == 0 {
		return nil
	}
	return &PointVector{ptr: handle}
}

// Clear 清空所有内部向量（对应C++的PointVector2D_Clear）
func (pv2d *PointVector2D) Clear() {
	if pv2d.ptr == 0 {
		return
	}
	C.PointVector2D_Clear(pv2d.ptr)
}

// ToSlice 将二维向量转换为Go的[][]Point（便于Go原生操作）
func (pv2d *PointVector2D) ToSlice() [][]Point {
	size := pv2d.Size()
	if size == 0 {
		return nil
	}
	slice := make([][]Point, 0, size)
	for i := 0; i < size; i++ {
		vec := pv2d.GetVector(i)
		if vec == nil {
			continue
		}
		slice = append(slice, vec.ToSlice())
		vec.Delete() // 释放GetVector返回的临时向量
	}
	return slice
}

// IntVector 封装 C++ 的 std::vector<int>
type IntVector struct {
	ptr C.long // 存储 C 侧返回的句柄
}

// NewIntVector 创建一个新的 IntVector 实例
func NewIntVector() *IntVector {
	return &IntVector{
		ptr: C.IntVector(),
	}
}

// Delete 释放向量占用的内存
func (v *IntVector) Delete() {
	if v.ptr != 0 {
		C.Delete_IntVector(v.ptr)
		v.ptr = 0 // 标记为已释放
	}
}

// Add 向向量添加元素
func (v *IntVector) Add(value int) {
	if v.ptr == 0 {
		return
	}
	C.IntVector_Add(v.ptr, C.int(value))
}

// Insert 在指定索引插入元素
func (v *IntVector) Insert(index int, value int) {
	if v.ptr == 0 {
		return
	}
	C.IntVector_Insert(v.ptr, C.int(index), C.int(value))
}

// RemoveAt 删除指定索引的元素
func (v *IntVector) RemoveAt(index int) {
	if v.ptr == 0 {
		return
	}
	C.IntVector_RemoveAt(v.ptr, C.int(index))
}

// Remove 删除第一个匹配的元素，返回是否删除成功
func (v *IntVector) Remove(value int) bool {
	if v.ptr == 0 {
		return false
	}
	return bool(C.IntVector_Remove(v.ptr, C.int(value)))
}

// Contains 检查向量是否包含指定元素
func (v *IntVector) Contains(value int) bool {
	if v.ptr == 0 {
		return false
	}
	return bool(C.IntVector_Contains(v.ptr, C.int(value)))
}

// Size 获取向量中元素的数量
func (v *IntVector) Size() int {
	if v.ptr == 0 {
		return 0
	}
	return int(C.IntVector_Size(v.ptr))
}

// Get 获取指定索引的元素，索引无效返回 0
func (v *IntVector) Get(index int) int {
	if v.ptr == 0 {
		return 0
	}
	return int(C.IntVector_Get(v.ptr, C.int(index)))
}

// Set 修改指定索引的元素
func (v *IntVector) Set(index int, value int) {
	if v.ptr == 0 {
		return
	}
	C.IntVector_Set(v.ptr, C.int(index), C.int(value))
}

// 转切片
func (v *IntVector) ToSlice() []int {
	size := v.Size()
	if size == 0 {
		return nil
	}
	slice := make([]int, 0, size)
	for i := 0; i < size; i++ {
		vec := v.Get(i)
		slice = append(slice, vec)
	}
	return slice
}

// IndexOf 获取指定元素的第一个索引，不存在返回 - 1
func (v *IntVector) IndexOf(value int) int {
	if v.ptr == 0 {
		return -1
	}
	return int(C.IntVector_IndexOf(v.ptr, C.int(value)))
}

func (v *IntVector) Clear() {
	if v.ptr == 0 {
		return
	}
	C.IntVector_Clear(v.ptr)
}

// IntVector2D 表示二维整数向量的Go封装
type IntVector2D struct {
	ptr C.long // 包内可见的句柄字段（小写）
}

// NewIntVector2D 创建一个新的二维整数向量
func NewIntVector2D() *IntVector2D {
	return &IntVector2D{
		ptr: C.IntVector2D(),
	}
}

// Delete 释放二维向量占用的内存
func (v *IntVector2D) Delete() {
	if v.ptr != 0 {
		C.Delete_IntVector2D(v.ptr)
		v.ptr = 0 // 避免重复释放
	}
}

// AddVector 向二维向量添加一个一维向量
func (v *IntVector2D) AddVector(vec *IntVector) {
	if v.ptr == 0 || vec == nil || vec.ptr == 0 {
		return
	}
	// 直接使用vec.ptr，无需类型转换（假设其类型已为C.long）
	C.IntVector2D_AddVector(v.ptr, vec.ptr)
}

// InsertVector 在指定索引位置插入一个一维向量
func (v *IntVector2D) InsertVector(index int, vec *IntVector) {
	if v.ptr == 0 || vec == nil || vec.ptr == 0 || index < 0 {
		return
	}
	C.IntVector2D_InsertVector(v.ptr, C.int(index), vec.ptr)
}

// RemoveVectorAt 删除指定索引位置的一维向量
func (v *IntVector2D) RemoveVectorAt(index int) {
	if v.ptr == 0 || index < 0 {
		return
	}
	C.IntVector2D_RemoveVectorAt(v.ptr, C.int(index))
}

// Size 返回二维向量中包含的一维向量数量
func (v *IntVector2D) Size() int {
	if v.ptr == 0 {
		return 0
	}
	return int(C.IntVector2D_Size(v.ptr))
}

// GetVector 获取指定索引位置的一维向量（需要手动释放返回的向量）
func (v *IntVector2D) GetVector(index int) *IntVector {
	if v.ptr == 0 || index < 0 {
		return nil
	}
	handle := C.IntVector2D_GetVector(v.ptr, C.int(index))
	if handle == 0 {
		return nil
	}
	return &IntVector{ptr: handle}
}

// Clear 清空二维向量中的所有元素
func (v *IntVector2D) Clear() {
	if v.ptr != 0 {
		C.IntVector2D_Clear(v.ptr)
	}
}

// AddElement 向指定索引的一维向量添加元素
func (v *IntVector2D) AddElement(vecIndex int, value int) {
	if v.ptr == 0 || vecIndex < 0 {
		return
	}
	C.IntVector2D_AddElement(v.ptr, C.int(vecIndex), C.int(value))
}

// GetElement 获取指定位置的元素值
func (v *IntVector2D) GetElement(vecIndex, elemIndex int) int {
	if v.ptr == 0 || vecIndex < 0 || elemIndex < 0 {
		return 0
	}
	return int(C.IntVector2D_GetElement(v.ptr, C.int(vecIndex), C.int(elemIndex)))
}

// GetVectorSize 获取指定索引的一维向量的元素数量
func (v *IntVector2D) GetVectorSize(vecIndex int) int {
	if v.ptr == 0 || vecIndex < 0 {
		return 0
	}
	return int(C.IntVector2D_GetVectorSize(v.ptr, C.int(vecIndex)))
}

func (v *IntVector2D) ToSlice() [][]int {
	size := v.Size()
	if size == 0 {
		return nil
	}
	slice := make([][]int, 0, size)
	for i := 0; i < size; i++ {
		vec := v.GetVector(i)
		slice = append(slice, vec.ToSlice())
		vec.Delete()
	}
	return slice
}

// DoubleVector 封装 C 侧的 std::vector<double> 向量
type DoubleVector struct {
	ptr C.long // 存储 C 侧向量指针（转换为 long 类型）
}

// NewDoubleVector 创建一个新的 double 向量
// 返回：*DoubleVector - 新创建的向量实例
func NewDoubleVector() *DoubleVector {
	vec := &DoubleVector{
		ptr: C.DoubleVector(),
	}
	// 设置 GC 终值器，确保资源释放
	runtime.SetFinalizer(vec, func(v *DoubleVector) {
		v.Delete()
	})
	return vec
}

// Delete 手动释放向量内存
// 返回：bool - 释放是否成功
func (v *DoubleVector) Delete() bool {
	if v.ptr == 0 {
		return false
	}
	// 调用 C 侧释放函数，返回 0 表示成功
	result := C.Delete_DoubleVector(v.ptr)
	v.ptr = 0 // 置空指针，防止重复释放
	return result == 0
}

// Add 向向量添加元素
// 参数：value - 要添加的 double 类型值
func (v *DoubleVector) Add(value float64) {
	if v.ptr == 0 {
		return
	}
	C.DoubleVector_Add(v.ptr, C.double(value))
}

// Insert 在指定索引插入元素
// 参数：
//
//	index - 插入位置索引（0 开始）
//	value - 要插入的元素值
func (v *DoubleVector) Insert(index int, value float64) {
	if v.ptr == 0 {
		return
	}
	C.DoubleVector_Insert(v.ptr, C.int(index), C.double(value))
}

// RemoveAt 删除指定索引的元素
// 参数：index - 要删除的元素索引
func (v *DoubleVector) RemoveAt(index int) {
	if v.ptr == 0 {
		return
	}
	C.DoubleVector_RemoveAt(v.ptr, C.int(index))
}

// Remove 删除第一个匹配的元素
// 参数：value - 要删除的元素值
// 返回：bool - 是否删除成功
func (v *DoubleVector) Remove(value float64) bool {
	if v.ptr == 0 {
		return false
	}
	return bool(C.DoubleVector_Remove(v.ptr, C.double(value)))
}

// Contains 检查向量是否包含指定元素
// 参数：value - 要检查的元素值
// 返回：bool - 是否包含该元素
func (v *DoubleVector) Contains(value float64) bool {
	if v.ptr == 0 {
		return false
	}
	return bool(C.DoubleVector_Contains(v.ptr, C.double(value)))
}

// Size 获取向量元素数量
// 返回：int - 元素个数
func (v *DoubleVector) Size() int {
	if v.ptr == 0 {
		return 0
	}
	return int(C.DoubleVector_Size(v.ptr))
}

// Get 获取指定索引的元素
// 参数：index - 元素索引
// 返回：float64 - 元素值（索引无效时返回 0.0）
func (v *DoubleVector) Get(index int) float64 {
	if v.ptr == 0 {
		return 0.0
	}
	return float64(C.DoubleVector_Get(v.ptr, C.int(index)))
}

// Set 修改指定索引的元素值
// 参数：
//
//	index - 要修改的元素索引
//	value - 新的元素值
func (v *DoubleVector) Set(index int, value float64) {
	if v.ptr == 0 {
		return
	}
	C.DoubleVector_Set(v.ptr, C.int(index), C.double(value))
}

// Clear 清空向量所有元素
func (v *DoubleVector) Clear() {
	if v.ptr == 0 {
		return
	}
	C.DoubleVector_Clear(v.ptr)
}

// IndexOf 获取指定元素的第一个索引
// 参数：value - 要查找的元素值
// 返回：int - 元素索引（不存在返回 -1）
func (v *DoubleVector) IndexOf(value float64) int {
	if v.ptr == 0 {
		return -1
	}
	return int(C.DoubleVector_IndexOf(v.ptr, C.double(value)))
}

func (v *DoubleVector) ToSlice() []float64 {

	size := v.Size()
	if size == 0 {
		return nil
	}
	slice := make([]float64, size)
	for i := 0; i < size; i++ {
		value := v.Get(i)
		slice[i] = value
	}

	return slice
}

// StringVector 封装C的std::vector<std::string>
type StringVector struct {
	ptr C.long
}

// NewStringVector 创建一个新的字符串向量
func NewStringVector() *StringVector {
	vec := &StringVector{
		ptr: C.StringVector(),
	}
	// 设置垃圾回收时自动释放C++资源
	runtime.SetFinalizer(vec, func(v *StringVector) {
		v.Delete()
	})
	return vec
}

// Delete 手动释放字符串向量资源
func (v *StringVector) Delete() {
	if v.ptr != 0 {
		C.Delete_StringVector(v.ptr)
		v.ptr = 0
	}
}

// Add 向向量添加元素
func (v *StringVector) Add(str string) {
	cStr := C.CString(str)
	defer C.free(unsafe.Pointer(cStr))
	C.StringVector_Add(v.ptr, cStr)
}

// RemoveAt 从向量中删除指定索引的元素
func (v *StringVector) RemoveAt(index int) {
	C.StringVector_RemoveAt(v.ptr, C.int(index))
}

// Remove 从向量中删除指定字符串，返回是否删除成功
func (v *StringVector) Remove(str string) bool {
	cStr := C.CString(str)
	defer C.free(unsafe.Pointer(cStr))
	return bool(C.StringVector_Remove(v.ptr, cStr))
}

// Contains 检查向量是否包含指定字符串
func (v *StringVector) Contains(str string) bool {
	cStr := C.CString(str)
	defer C.free(unsafe.Pointer(cStr))
	return bool(C.StringVector_Contains(v.ptr, cStr))
}

// Size 获取向量中元素的数量
func (v *StringVector) Size() int {
	return int(C.StringVector_Size(v.ptr))
}

// Get 获取指定索引的元素，索引无效时返回空字符串
func (v *StringVector) Get(index int) string {
	cStr := C.StringVector_Get(v.ptr, C.int(index))
	if cStr == nil {
		return ""
	}
	return C.GoString(cStr)
}

// Clear 清空向量中的所有元素
func (v *StringVector) Clear() {
	C.StringVector_Clear(v.ptr)
}

// LongVector 封装 C 侧的 std::vector<long>，通过 ptr 存储 C 侧向量指针（long 类型）
type LongVector struct {
	ptr C.long // 本质是 C++ 侧 std::vector<long>* 的地址，转为 long 传递
}

func NewLongVector() *LongVector {
	vec := &LongVector{
		ptr: C.LongVector(), // 调用 C 侧创建函数
	}
	// 注册 GC 终值器：当向量被 GC 回收时，自动调用 Delete 释放 C 侧内存
	runtime.SetFinalizer(vec, func(v *LongVector) {
		v.Delete()
	})
	return vec
}

// Delete 手动释放 C 侧向量内存（建议配合 defer 使用，确保及时释放）
// 注意：调用后向量实例不可再使用（ptr 会被置为 0，避免重复释放）
func (v *LongVector) Delete() {
	if v.ptr != 0 { // 避免重复释放（C 侧已做空指针检查，此处双重保险）
		C.Delete_LongVector(v.ptr)
		v.ptr = 0 // 置空指针，防止后续误操作
	}
}

// Add 向向量末尾添加一个元素
// 参数：value - 要添加的 long 类型值（Go 的 int64 与 C 的 long 在 64 位系统下兼容）
func (v *LongVector) Add(value int64) {
	if v.ptr == 0 {
		return // 已释放的向量不再操作
	}
	// Go 的 int64 转为 C 的 long（需确保平台位数一致，64 位系统下安全）
	C.LongVector_Add(v.ptr, C.long(value))
}

// Insert 在指定索引插入元素（索引超出范围时忽略）
// 参数：
//
//	index - 插入位置（0 开始，允许等于向量长度，即末尾插入）
//	value - 要插入的 long 类型值
func (v *LongVector) Insert(index int, value int64) {
	if v.ptr == 0 {
		return
	}
	C.LongVector_Insert(
		v.ptr,
		C.int(index),  // Go int 转为 C int
		C.long(value), // Go int64 转为 C long
	)
}

// RemoveAt 删除指定索引的元素（索引无效时忽略）
// 参数：index - 要删除的元素索引（0 开始，需满足 0 ≤ index < Size()）
func (v *LongVector) RemoveAt(index int) {
	if v.ptr == 0 {
		return
	}
	C.LongVector_RemoveAt(v.ptr, C.int(index))
}

// Remove 删除第一个匹配 value 的元素（返回是否删除成功）
// 参数：value - 要删除的 long 类型值
// 返回：bool - true=删除成功，false=未找到该元素或向量已释放
func (v *LongVector) Remove(value int64) bool {
	if v.ptr == 0 {
		return false
	}
	return bool(C.LongVector_Remove(v.ptr, C.long(value)))
}

// Contains 检查向量是否包含指定元素
// 参数：value - 要检查的 long 类型值
// 返回：bool - true=包含，false=不包含或向量已释放
func (v *LongVector) Contains(value int64) bool {
	if v.ptr == 0 {
		return false
	}
	return bool(C.LongVector_Contains(v.ptr, C.long(value)))
}

// Size 获取向量中元素的数量
// 返回：int - 元素个数（向量已释放时返回 0）
func (v *LongVector) Size() int {
	if v.ptr == 0 {
		return 0
	}
	return int(C.LongVector_Size(v.ptr))
}

// Get 获取指定索引的元素（索引无效时返回 0）
// 参数：index - 要获取的元素索引（0 开始）
// 返回：int64 - 索引对应的元素值（无效索引返回 0）
func (v *LongVector) Get(index int) int64 {
	if v.ptr == 0 {
		return 0
	}
	return int64(C.LongVector_Get(v.ptr, C.int(index)))
}

// Set 修改指定索引的元素值（索引无效时忽略）
// 参数：
//
//	index - 要修改的元素索引（0 开始，需满足 0 ≤ index < Size()）
//	value - 新的元素值（long 类型）
func (v *LongVector) Set(index int, value int64) {
	if v.ptr == 0 {
		return
	}
	C.LongVector_Set(
		v.ptr,
		C.int(index),
		C.long(value),
	)
}

// Clear 清空向量中的所有元素（向量仍可继续使用）
func (v *LongVector) Clear() {
	if v.ptr == 0 {
		return
	}
	C.LongVector_Clear(v.ptr)
}

// IndexOf 获取指定元素的第一个索引（不存在时返回 -1）
// 参数：value - 要查找的 long 类型值
// 返回：int - 元素的索引（0 开始），未找到返回 -1，向量已释放返回 -1
func (v *LongVector) IndexOf(value int64) int {
	if v.ptr == 0 {
		return -1
	}
	return int(C.LongVector_IndexOf(v.ptr, C.long(value)))
}

type Mat struct {
	ptr C.long
}

// NewMat
//
//	@Description: 创建空Mat图片
//	@return *Mat
func NewMat() *Mat {
	m := &Mat{
		ptr: C.NewMat(),
	}
	// 注册垃圾回收器：当 Mat 被 Go 回收时，自动调用 Recycle()
	runtime.SetFinalizer(m, func(m *Mat) {
		m.Recycle()
	})
	return m
}

/**
 * 回收图片
 * @param img
 * @return
 */
func (m *Mat) Recycle() bool {
	if m.ptr == 0 {
		return false
	}
	result := bool(C.Recycle(m.ptr))
	m.ptr = 0
	return result // 直接传递 C.long 类型
}

// GetWidth
//
//	@Description: 获取图片的宽度
//	@receiver m
//	@return int
func (m *Mat) GetWidth() int {
	return int(C.GetWidth(m.ptr))
}

// GetHeight
//
//	@Description: 获取图片的高度
//	@receiver m
//	@return int
func (m *Mat) GetHeight() int {
	return int(C.GetHeight(m.ptr))
}

// Empty
//
//	@Description:判断图片是否为空
//	@receiver m
//	@return bool true或false
func (m *Mat) Empty() bool {
	return bool(C.Empty(m.ptr))
}

// 图片回收 仅处理*Mat和[]*Mat类型的资源回收
// 支持单个*Mat、多个*Mat或[]*Mat切片作为参数
func Recycle(mats ...any) bool {
	allSuccess := true
	for _, arg := range mats {
		if arg == nil {
			continue
		}
		switch v := arg.(type) {
		case *Mat:
			// 处理单个*Mat
			success := v.Recycle()
			v.ptr = 0
			allSuccess = allSuccess && success

		case []*Mat:
			// 处理[]*Mat切片
			sliceSuccess := true
			for _, mat := range v {
				if mat == nil {
					continue
				}
				matSuccess := mat.Recycle()
				mat.ptr = 0
				sliceSuccess = sliceSuccess && matSuccess
			}
			allSuccess = allSuccess && sliceSuccess

		default:
			// 不支持的类型
			fmt.Printf("错误：不支持的类型 %T，仅支持*Mat和[]*Mat\n", arg)
			allSuccess = false
		}
	}

	return allSuccess
}

// 获取资源文件夹
func getResPath() string {
	return filepath.Dir(os.Args[0]) + "/assets"
}

func jsonToConnectedState(jsonStr string) []ConnectedState {

	var results []ConnectedState
	// 解析 JSON 字符串到结构体切片
	err := json.Unmarshal([]byte(jsonStr), &results)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		return nil
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func jsonToMatches(jsonStr string) []Match {
	// 定义接收结果的切片
	var results []Match
	// 解析 JSON 字符串到结构体切片
	err := json.Unmarshal([]byte(jsonStr), &results)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		return nil
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func jsonToPoints(jsonStr string) []Point {
	// 定义接收结果的切片
	var results []Point
	// 解析 JSON 字符串到结构体切片
	err := json.Unmarshal([]byte(jsonStr), &results)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		return nil
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func jsonToRects(jsonStr string) []Rect {
	// 定义接收结果的切片
	var results []Rect
	// 解析 JSON 字符串到结构体切片
	err := json.Unmarshal([]byte(jsonStr), &results)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		return nil
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func jsonToPointExs(jsonStr string) []PointEx {
	// 定义接收结果的切片
	var results []PointEx
	// 解析 JSON 字符串到结构体切片
	err := json.Unmarshal([]byte(jsonStr), &results)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		return nil
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func jsonToPColors(jsonStr string) []PColor {
	// 定义接收结果的切片
	var results []PColor
	// 解析 JSON 字符串到结构体切片
	err := json.Unmarshal([]byte(jsonStr), &results)
	if err != nil {
		fmt.Printf("JSON 解析失败: %v\n", err)
		return nil
	}
	if len(results) == 0 {
		return nil
	}
	return results
}

func getBoolVector(goStr string) []bool {

	size := len(goStr)
	if size == 0 {
		return nil
	}
	result := make([]bool, size)
	for i, c := range goStr {
		result[i] = c == '1'
	}
	return result
}

func getPoint2DVector(vec2D *PointVector2D) [][]Point {
	// 检查输入有效性
	if vec2D == nil || vec2D.ptr == 0 {
		return [][]Point{}
	}
	// 获取二维向量的大小
	size := vec2D.Size()
	if size <= 0 {
		return [][]Point{}
	}
	// 初始化结果切片
	result := make([][]Point, 0, size)
	for i := 0; i < size; i++ {
		// 获取子向量（需要单独释放）
		subVec := vec2D.GetVector(i)
		defer subVec.Delete()
		subSlice := subVec.ToSlice()
		result = append(result, subSlice)
	}
	return result
}

// checkParam 封装参数校验逻辑
// 参数：
//
//	paramMetasPtr: 参数元信息指针（会被修改以更新默认值）
//	minCount:用户至少需要填写的参数
//	args: 输入的可变参数
//
// 返回：
//
//	校验成功返回true，失败返回false
func checkParam(paramMetasPtr *[]paramMeta, minCount int, args ...any) bool {
	paramMetas := *paramMetasPtr
	// 检查参数数量
	if len(args) < minCount {
		fmt.Printf("参数过少，至少需要%d个参数，实际传入%d个\n", minCount, len(args))
		return false
	}
	if len(args) > len(paramMetas) {
		fmt.Printf("参数过多，最多接受%d个参数，实际传入%d个\n", len(paramMetas), len(args))
		return false
	}
	// 校验参数类型并更新默认值
	for i, arg := range args {
		if arg == nil {
			continue
		}
		// 通过默认值获取期望类型
		expectedType := reflect.TypeOf(paramMetas[i].def)
		actualType := reflect.TypeOf(arg)

		if actualType != expectedType {
			fmt.Printf("第%d个参数类型不匹配，需要%v，实际传入%v\n", i+1, expectedType.String(), actualType.String())
			return false
		}
		// 更新参数值（覆盖默认值）
		paramMetas[i].def = arg
	}
	return true
}

/**
 * 释放启用本地日志
 * @param path 日志路径 默认""
 * @param flag  是否启用 默认true
 * @return 日志路径
 */
func UseLocalLog(path string, flag bool) string {
	cpath := C.CString(path)
	cflag := C.bool(flag)
	defer C.free(unsafe.Pointer(cpath))

	cstr := C.UseLocalLog(cpath, cflag)
	defer C.DeleteCharArr(cstr)
	rpath := C.GoString(cstr)
	fmt.Printf("本地日志地址:%s\n", rpath)
	return rpath
}

func UseLocalLog0(args ...any) string {
	paramMetas := []paramMeta{
		{"path", "/sdcard/err.log"},
		{"flag", true},
	}
	// 调用参数校验函数
	if !checkParam(&paramMetas, 0, args...) {
		return "设置日志失败!"
	}
	return UseLocalLog(paramMetas[0].def.(string), paramMetas[1].def.(bool))
}

/**
 * byte[]转Mat图片
 * @param data byte[]
 * @param height 图片的高
 * @param width 图片宽
 * @return *Mat 返回3通道图片
 */
func BytesToMat(bytes []byte, width, height int) *Mat {
	result := NewMat()
	data := (*C.char)(unsafe.Pointer(&bytes[0]))
	size := len(bytes)
	channels := size / int(width*height)
	if channels == 0 || size == 0 {
		println("数据错误，无法转换为图片")
		return result
	}
	C.BytesToMat(data, C.int(height), C.int(width), C.int(channels), result.ptr)
	return result
}

/**
 * 将 cv::Mat 转换为 Byte[] 原始像素数据
 * @param img 输入的 Mat 对象
 * @param channels 目标通道类型（1: 灰度图, 3: RGB, 4: RGBA） 默认3
 * @return  []byte或nil
 */
func MatToBytes(img *Mat, channels int) []byte {

	if img.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}

	var cOutSize C.int
	cChar := C.MatToBytes(img.ptr, &cOutSize, C.int(channels))
	defer C.DeleteCharArr(cChar)

	if cOutSize <= 0 || cChar == nil {
		println("获取数据错误")
		return nil
	}
	bytePtr := (*byte)(unsafe.Pointer(cChar))
	cData := unsafe.Slice(bytePtr, int(cOutSize))
	goBytes := make([]byte, int(cOutSize))
	copy(goBytes, cData)
	return goBytes
}

func MatToBytes0(args ...any) []byte {
	paramMetas := []paramMeta{
		{"img", defMat},
		{"channels", 3},
	}
	// 调用参数校验函数
	if !checkParam(&paramMetas, 1, args...) {
		return nil
	}
	return MatToBytes(paramMetas[0].def.(*Mat), paramMetas[1].def.(int))
}

/**
 * image转Mat (三通道图)
 * @param img  image图片
 * @return  *Mat
 */
func ImageToMat(img *image.NRGBA) *Mat {
	return BytesToMat(img.Pix, img.Rect.Dx(), img.Rect.Dy())
}

/**
 * Mat转image
 * @param mat  Mat图片
 * @return  *image.NRGBA图
 */
func MatToImage(mat *Mat) *image.NRGBA {

	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	w := mat.GetWidth()
	h := mat.GetHeight()

	data := MatToBytes(mat, 4)
	// 3. 验证数据长度是否匹配（宽×高×4 字节/像素）
	expectedLen := w * h * 4
	if len(data) != expectedLen {
		fmt.Printf("数据长度不匹配: 期望 %d, 实际 %d", expectedLen, len(data))
		return nil
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, data) // 使用 copy 而非直接赋值，确保内存安全

	return img
}

/**
 * Mat格式截图
 * @param sx  起点x
 * @param sy  起点y
 * @param ex  终点x
 * @param ey  终点y
 * @return  *Mat
 */
func CaptureScreenMat(sx, sy, ex, ey int) *Mat {
	// 截取屏幕图片
	img := images.CaptureScreen(sx, sy, ex, ey, 0)
	result := NewMat()
	if img == nil {
		println("截图失败！")
		return result
	}
	data := (*C.char)(unsafe.Pointer(&img.Pix[0]))
	C.BytesToMat(data, C.int(img.Rect.Dy()), C.int(img.Rect.Dx()), 4, result.ptr)
	return result
}

func CaptureScreenMat0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"sx", 0},
		{"sy", 0},
		{"ex", 0},
		{"ey", 0},
	}
	if checkParam(&paramMetas, 0, args...) == false {
		return NewMat()
	}

	return CaptureScreenMat(paramMetas[0].def.(int), paramMetas[1].def.(int), paramMetas[2].def.(int), paramMetas[3].def.(int))
}

// ---- 截图级缓存：同一 key 在 ttl 内复用同一张截图，不重复截屏 ----

type screenCacheEntry struct {
	mat      *Mat
	expireAt time.Time
}

var (
	screenCacheMu    sync.Mutex
	screenCacheStore = map[string]screenCacheEntry{}
)

// CaptureScreenMatCached 带截图缓存的截屏：
//   - 同一 key 在 ttl 内命中时直接复用缓存截图（返回副本，调用方负责 Recycle，原件留缓存）
//   - 未命中则截屏并存入缓存，同样返回副本给调用方
//   - key 直接使用传入值，不拼接区域信息；key 为空或 ttl<=0 时等价于 CaptureScreenMat（不缓存）
func CaptureScreenMatCached(key string, ttl time.Duration, sx, sy, ex, ey int) *Mat {
	if key == "" || ttl <= 0 {
		return CaptureScreenMat(sx, sy, ex, ey)
	}

	screenCacheMu.Lock()
	if e, ok := screenCacheStore[key]; ok {
		if time.Now().Before(e.expireAt) {
			screenCacheMu.Unlock()
			return CopyTo(e.mat, nil) // 命中：返回副本，原件留缓存
		}
		e.mat.Recycle()
		delete(screenCacheStore, key)
	}
	screenCacheMu.Unlock()

	img := CaptureScreenMat(sx, sy, ex, ey)
	if img.Empty() {
		return img
	}

	screenCacheMu.Lock()
	if e, ok := screenCacheStore[key]; ok {
		e.mat.Recycle() // 并发下他人刚写入同一 key，释放旧图
	}
	screenCacheStore[key] = screenCacheEntry{mat: img, expireAt: time.Now().Add(ttl)}
	screenCacheMu.Unlock()
	return CopyTo(img, nil) // 返回副本，调用方负责 Recycle
}

/**
 * 读取文件内容
 * @param path  文件路径 例如 "/sdcard/font.txt"
 * @return string
 */
func ReadFile(path string) string {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath)) // 释放 C.CString 分配的内存
	cData := C.ReadFile(cPath)
	defer C.DeleteCharArr(cData)
	goStr := C.GoString(cData)
	return goStr
}

/**
 * 设置字库
 * @param path  字库路径 例如 "/sdcard/font.txt"
 * @param fontName 字库名称  例如 "font1" 默认library
 * @return bool
 */
func SetDictByPath(path, fontName string) bool {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	cFontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(cFontName))
	cbool := C.SetDictByPath(cPath, cFontName)
	return bool(cbool)
}
func SetDictByPath0(args ...any) bool {
	paramMetas := []paramMeta{
		{"path", ""},
		{"fontName", "library"},
	}
	// 调用参数校验函数
	if !checkParam(&paramMetas, 1, args...) {
		return false
	}
	return SetDictByPath(paramMetas[0].def.(string), paramMetas[1].def.(string))
}

/**
 * 设置字库
 * @param fileName 文件名称 例如 font.txt 或 ziku.txt
 * @param fontName 字库名称 例如 "font1"
 * @return bool
 */
func SetDict(fileName, fontName string) bool {
	// 直接从 embed.FS 读取
	fontContent, err := assets.Fonts.ReadFile(fileName)
	if err != nil {
		fmt.Printf(fileName+"文件不存在: %v\n", err)
		return false
	}

	if len(fontContent) == 0 {
		fmt.Printf(fileName + "文件内容为空!")
		return false
	}

	cFontContent := C.CString(string(fontContent))
	defer C.free(unsafe.Pointer(cFontContent))
	cfontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(cfontName))
	return bool(C.SetDict(cFontContent, cfontName))
}

func SetDict0(args ...any) bool {
	paramMetas := []paramMeta{
		{"fileName", ""},
		{"fontName", "library"},
	}
	// 调用参数校验函数
	if !checkParam(&paramMetas, 1, args...) {
		return false
	}
	return SetDict(paramMetas[0].def.(string), paramMetas[1].def.(string))
}

/**
 * 切换字库
 * @param fontName 字库名称 例如 "font1"
 * @return bool
 */
func UseDict(fontName string) bool {
	cFontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(cFontName))
	return bool(C.UseDict(cFontName))
}

/**
 * 移除字库
 * @param fontName 字库名称  例如 "font1"  为空则释放全部
 * @return bool
 */
func RemoveDict(fontName string) bool {
	cFontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(cFontName))
	return bool(C.RemoveDict(cFontName))
}

/**
 * 普通找字
 * @param mat 图片
 * @param text 目标文字
 * @param sx 起点x 默认0
 * @param sy 起点y 默认0
 * @param ex 终点x 默认0
 * @param ey 终点y 默认0
 * @param threshold  相似度 默认0.9
 * @param count 寻找数量 默认1
 * @param fontName 字库名称 写 "" 则使用默认字库
 * @return Match数组或nil
 */
func FindStr(mat *Mat, text string, sx, sy, ex, ey int, threshold float64, count int, fontName string) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	_text := C.CString(text)
	defer C.free(unsafe.Pointer(_text))
	_threshold := C.double(threshold)
	_count := C.int(count)
	_fontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(_fontName))
	cResult := C.FindStr(mat.ptr, _text, C.int(sx), C.int(sy), C.int(ex), C.int(ey), _threshold, _count, _fontName)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

func FindStr0(args ...any) []Match {
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"text", ""},
		{"sx", 0},
		{"sy", 0},
		{"ex", 0},
		{"ey", 0},
		{"threshold", 0.9},
		{"count", 1},
		{"fontName", "library"},
	}
	// 调用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}
	return FindStr(
		paramMetas[0].def.(*Mat),    // mat: *Mat
		paramMetas[1].def.(string),  // text: string
		paramMetas[2].def.(int),     // sx: int
		paramMetas[3].def.(int),     // sy: int
		paramMetas[4].def.(int),     // ex: int
		paramMetas[5].def.(int),     // ey: int
		paramMetas[6].def.(float64), // threshold: float64
		paramMetas[7].def.(int),     // count: int
		paramMetas[8].def.(string),  // fontName: string
	)
}

/**
 * 找字扩展
 * @param mat 图片
 * @param text 目标文字 例如 "系统应用"
 * @param sx 起点x 默认0
 * @param sy 起点y 默认0
 * @param ex 终点x 默认0
 * @param ey 终点y 默认0
 * @param threshold  相似度 默认0.9
 * @param count 寻找数量  默认1
 * @param _fontName 字库名称 写 "" 则使用默认字库
 * @return Match数组或nil
 */
func FindStrEx(mat *Mat, text string, sx, sy, ex, ey int, threshold float64, count int, fontName string) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	_text := C.CString(text)
	defer C.free(unsafe.Pointer(_text))
	_threshold := C.double(threshold)
	_count := C.int(count)
	_fontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(_fontName))
	cResult := C.FindStrEx(mat.ptr, _text, C.int(sx), C.int(sy), C.int(ex), C.int(ey), _threshold, _count, _fontName)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

func FindStrEx0(args ...any) []Match {
	// 定义参数元信息（名称和默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"text", ""},
		{"sx", 0},               // 起点x，默认0
		{"sy", 0},               // 起点y，默认0
		{"ex", 0},               // 终点x，默认0
		{"ey", 0},               // 终点y，默认0
		{"threshold", 0.9},      // 相似度阈值，默认0.9
		{"count", 1},            // 寻找数量，默认1
		{"fontName", "library"}, // 字库名称，默认使用默认字库
	}

	// 调用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}
	// 传递所有参数给原始函数
	return FindStrEx(
		paramMetas[0].def.(*Mat),    // mat: 图像矩阵
		paramMetas[1].def.(string),  // text: 目标文字
		paramMetas[2].def.(int),     // sx: 起点x
		paramMetas[3].def.(int),     // sy: 起点y
		paramMetas[4].def.(int),     // ex: 终点x
		paramMetas[5].def.(int),     // ey: 终点y
		paramMetas[6].def.(float64), // threshold: 相似度
		paramMetas[7].def.(int),     // count: 寻找数量
		paramMetas[8].def.(string),  // fontName: 字库名称
	)
}

/**
 * 允许用字库里的字任意形式组合找字
 * @param mat 图片
 * @param text 字符串 如果有多个字符串 允许用 丨隔开 如象棋丨野马丨大炮兵
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param threshold 阈值 0-1.0 默认0.9
 * @param count  	寻找个数 默认 1
 * @param charInterval 字符间隔, 以当前查找字符中最大字符的宽度为单位, 默认是 0.5, 也就是0.5个字符宽度
 * @param global 是否为精准查找,默认值false 为模糊查找。 例如找 “第章” 模糊查找可以找到“第一章”“第十章”，反之不能
 * @param fontName 指定字库名称 例如 myFontName，写 "" 则使用默认字库
 * @return Match数组或nil
 */
func FindStrComb(mat *Mat, text string, sx, sy, ex, ey int, threshold float64, count int, charInterval float64, global bool, fontName string) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	_text := C.CString(text)
	defer C.free(unsafe.Pointer(_text))
	_threshold := C.double(threshold)
	_charInterval := C.double(charInterval)
	_global := C.bool(global)
	_count := C.int(count)
	_fontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(_fontName))
	cResult := C.FindStrComb(mat.ptr, _text, C.int(sx), C.int(sy), C.int(ex), C.int(ey), _threshold, _count, _charInterval, _global, _fontName)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

func FindStrComb0(args ...any) []Match {
	// 定义参数元信息（名称和默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"text", ""},
		{"sx", 0},               // 起点x，默认0
		{"sy", 0},               // 起点y，默认0
		{"ex", 0},               // 终点x，默认0
		{"ey", 0},               // 终点y，默认0
		{"threshold", 0.9},      // 相似度阈值，默认0.9
		{"count", 1},            // 寻找个数，默认1
		{"charInterval", 0.5},   // 字符间隔，默认0.5个字符宽度
		{"global", false},       // 是否精准查找，默认false（模糊查找）
		{"fontName", "library"}, // 字库名称，默认使用默认字库
	}

	// 调用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 按顺序传递所有参数给原始函数
	return FindStrComb(
		paramMetas[0].def.(*Mat),    // mat: 图像矩阵
		paramMetas[1].def.(string),  // text: 组合字符串
		paramMetas[2].def.(int),     // sx: 起点x
		paramMetas[3].def.(int),     // sy: 起点y
		paramMetas[4].def.(int),     // ex: 终点x
		paramMetas[5].def.(int),     // ey: 终点y
		paramMetas[6].def.(float64), // threshold: 相似度阈值
		paramMetas[7].def.(int),     // count: 寻找个数
		paramMetas[8].def.(float64), // charInterval: 字符间隔
		paramMetas[9].def.(bool),    // global: 是否精准查找
		paramMetas[10].def.(string), // fontName: 字库名称
	)
}

/**
 * 字库识字
 * @param mat  图片
 * @param colorInfo 颜色  例如 "#FFFFFF-0x101010|#E1E1E1-0x101010" ,如果为"" 就自动使用字库的颜色
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param threshold 阈值 默认0.9
 * @param colorFilter 是否用设定的colorInfo对字库进行筛选,默认 false; 如果是true,有助于提供效率(前提是你的字库的颜色于设定colorInfo一致)
 * @param fontName 指定字库名称 例如 myFontName，写 "" 则使用默认字库
 * @return Match数组或nil
 */
func GetWords(mat *Mat, colorInfo string, sx, sy, ex, ey int, threshold float64, colorFilter bool, fontName string) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	_colorInfo := C.CString(colorInfo)
	defer C.free(unsafe.Pointer(_colorInfo))
	_threshold := C.double(threshold)
	_colorFilter := C.bool(colorFilter)
	_fontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(_fontName))
	cResult := C.GetWords(mat.ptr, _colorInfo, C.int(sx), C.int(sy), C.int(ex), C.int(ey), _threshold, _colorFilter, _fontName)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

func GetWords0(args ...any) []Match {
	// 定义参数元信息（名称和默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"colorInfo", ""},
		{"sx", 0},               // 起点x，默认0
		{"sy", 0},               // 起点y，默认0
		{"ex", 0},               // 终点x，默认0
		{"ey", 0},               // 终点y，默认0
		{"threshold", 0.9},      // 相似度阈值，默认0.9
		{"colorFilter", false},  // 是否启用颜色过滤，默认false
		{"fontName", "library"}, // 字库名称，默认使用默认字库
	}

	// 调用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}
	// 按顺序传递所有参数给原始函数
	return GetWords(
		paramMetas[0].def.(*Mat),    // mat: 图像矩阵
		paramMetas[1].def.(string),  // colorInfo: 颜色信息
		paramMetas[2].def.(int),     // sx: 起点x
		paramMetas[3].def.(int),     // sy: 起点y
		paramMetas[4].def.(int),     // ex: 终点x
		paramMetas[5].def.(int),     // ey: 终点y
		paramMetas[6].def.(float64), // threshold: 相似度阈值
		paramMetas[7].def.(bool),    // colorFilter: 是否启用颜色过滤
		paramMetas[8].def.(string),  // fontName: 字库名称
	)
}

/**
 * 全屏字库识字
 * @param mat  图片
 * @param colorInfo 颜色(必填)  例如 "#FFFFFF-0x101010|#E1E1E1-0x101010"
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param threshold 阈值 默认0.9
 * @param colorFilter 是否用设定的colorInfo对字库进行筛选,默认 false; 如果是true,有助于提供效率(前提是你的字库的颜色于设定colorInfo一致)
 * @param charInterval 字符间距 默认1.0
 * @param fontName 指定字库名称 例如 myFontName，写 "" 则使用默认字库
 * @return Match数组或nil
 */
func OcrText(mat *Mat, colorInfo string, sx, sy, ex, ey int, threshold float64, colorFilter bool, charInterval float64, fontName string) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	if colorInfo == "" {
		fmt.Println("colorInfo参数必填！")
		return nil
	}
	_colorInfo := C.CString(colorInfo)
	defer C.free(unsafe.Pointer(_colorInfo))
	_threshold := C.double(threshold)
	_colorFilter := C.bool(colorFilter)
	_charInterval := C.double(charInterval)
	_fontName := C.CString(fontName)
	defer C.free(unsafe.Pointer(_fontName))
	cResult := C.OcrText(mat.ptr, _colorInfo, C.int(sx), C.int(sy), C.int(ex), C.int(ey), _threshold, _colorFilter, _charInterval, _fontName)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

func OcrText0(args ...any) []Match {
	// 定义参数元信息（名称和默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"colorInfo", ""},
		{"sx", 0},               // 起点x，默认0
		{"sy", 0},               // 起点y，默认0
		{"ex", 0},               // 终点x，默认0
		{"ey", 0},               // 终点y，默认0
		{"threshold", 0.9},      // 相似度阈值，默认0.9
		{"colorFilter", false},  // 是否启用颜色过滤，默认false
		{"charInterval", 1.0},   //  字符间距 默认1.0
		{"fontName", "library"}, // 字库名称，默认使用默认字库
	}

	// 调用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}
	// 按顺序传递所有参数给原始函数
	return OcrText(
		paramMetas[0].def.(*Mat),    // mat: 图像矩阵
		paramMetas[1].def.(string),  // colorInfo: 颜色信息
		paramMetas[2].def.(int),     // sx: 起点x
		paramMetas[3].def.(int),     // sy: 起点y
		paramMetas[4].def.(int),     // ex: 终点x
		paramMetas[5].def.(int),     // ey: 终点y
		paramMetas[6].def.(float64), // threshold: 相似度阈值
		paramMetas[7].def.(bool),    // colorFilter: 是否启用颜色过滤
		paramMetas[8].def.(float64), // 字符间距 默认1.0
		paramMetas[9].def.(string),  // fontName: 字库名称
	)
}

/**
 * 保存图片
 * @param mat  图片
 * @param filename 保存路径 例如 /sdcard/1.png
 * @param format    要保存图像格式 保留值 默认 png
 * @param q  要保存图像质量，1-100 默认 100
 * @return bool
 */
func SaveMat(mat *Mat, filename, format string, q int) bool {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return false
	}
	if !strings.HasPrefix(filename, "/") {
		filename = "/" + filename
	}
	_filename := C.CString(filename)
	defer C.free(unsafe.Pointer(_filename))
	_format := C.CString(format)
	defer C.free(unsafe.Pointer(_format))
	cResult := C.SaveMat(mat.ptr, _filename, _format, C.int(q))
	if int(cResult) == 1 {
		return true
	}
	return false
}

func SaveMat0(args ...any) bool {
	// 定义参数元信息（名称和默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"filename", ""},
		{"format", "png"}, // 保存格式，默认png
		{"q", 100},        // 图像质量，默认100（最高质量）
	}
	// 调用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return false
	}
	// 按顺序传递所有参数给原始函数
	return SaveMat(
		paramMetas[0].def.(*Mat),   // mat: 要保存的图像矩阵
		paramMetas[1].def.(string), // filename: 保存路径
		paramMetas[2].def.(string), // format: 保存格式
		paramMetas[3].def.(int),    // q: 图像质量
	)
}

/**
 * 读取本地图片
 * @param path  图片路径 例如/sdcard/a.png
 * @return *Mat
 */
func ReadMat(path string) *Mat {

	exists := files.Exists(path)
	if !exists {
		fmt.Printf(path + "文件不存在!\n")
		return NewMat()
	}
	img := images.ReadFromPath(path)
	return ImageToMat(img)
}

/**
 * 读取assets 里的文件
 * @param path  图片名称 例如 img/main/xxx.png
 * @return *Mat
 */
func ReadResMat(fileName string) *Mat {
	// 从 embed.FS 读取图片
	data, err := assets.Images.ReadFile(fileName)
	if err != nil {
		fmt.Printf(fileName+"文件不存在: %v\n", err)
		return NewMat()
	}
	if len(data) == 0 {
		fmt.Printf(fileName + "文件为空!")
		return NewMat()
	}

	// 使用 images.ReadFromBytes 解码图片
	img := images.ReadFromBytes(data)
	if img == nil {
		fmt.Printf(fileName + "图像解码失败!")
		return NewMat()
	}

	return ImageToMat(img)
}

/**
 * 批量读取assets 里的文件
 * @param path  图片名称 例如 img1.png|img2.png
 * @return *Mat
 */
func ReadResMatArr(fileNames string) []*Mat {
	names := strings.Split(fileNames, "|")
	mats := make([]*Mat, 0, len(names))
	basePath := getResPath()
	for _, name := range names {
		// 处理可能的空白
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		path := basePath + "/" + name
		exists := files.Exists(path)
		if !exists {
			fmt.Printf("%s 文件不存在!\n", name)
			continue
		}
		img := images.ReadFromPath(path)
		mat := ImageToMat(img)
		mats = append(mats, mat)
	}
	return mats
}

/*
*
  - 单点找色 和EC 用法一样
  - @param mat   Mat图片
  - @param color   {string} 要找的颜色如 "0xCDD7E9-0x101010" "#00FF00" "0xCDD7E9-0x101010|0xCDD7E9-0x101010"
  - @param threshold {number} 找色时颜色相似度取值为 0.0 ~ 1.0  默认 0.9
  - @param sx  起点x 默认0;
  - @param sy 起点y 默认0;
  - @param ex  终点x 默认0;
  - @param ey  终点y 默认0;
  - @param count  寻找的个数 默认1
  - @param direction 寻找的方向 1~8 默认1 <br/>
    1: → 从左上横着找到右下 (默认方式)  <br/>
    2: ← 从右上横着找到左下  <br/>
    3: ↓ 从右上竖直找到左下  <br/>
    4: ↑ 从右下竖直找到左上  <br/>
    5: ← 从右下横着找到左上  <br/>
    6: → 从左下横着找到右上  <br/>
    7: ↑ 从左下竖直找到右上  <br/>
    8: ↓ 从左上竖直找到右下  <br/>
  - @returns Point数组或nil
*/
func FindColor(mat *Mat, color string, threshold float64, sx, sy, ex, ey, count, direction int) []Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cDirection := C.int(direction)
	cResult := C.FindColor(mat.ptr, cColor, cThreshold, cSx, cSy, cEx, cEy, cCount, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

func FindColor0(args ...any) []Point {
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"color", ""},
		{"threshold", 0.9},
		{"sx", 0},
		{"sy", 0},
		{"ex", 0},
		{"ey", 0},
		{"count", 1},
		{"direction", 1},
	}
	// 调用通用参数校验函数，确保数量和类型正确
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}
	// 按原函数参数顺序传递，完成类型断言并调用
	return FindColor(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.(string),
		paramMetas[2].def.(float64),
		paramMetas[3].def.(int),
		paramMetas[4].def.(int),
		paramMetas[5].def.(int),
		paramMetas[6].def.(int),
		paramMetas[7].def.(int),
		paramMetas[8].def.(int),
	)
}

/*
*
  - 单点找非色
  - @param img  {Mat} 图片
  - @param colorStr   {string} 不想找的一个或多个颜色 例如 "0xCDD7E9-0x101010" "#00FF00"   "0xCDD7E9-0x101010 & 0x9999E9-0x101010" 主要 & 符合表示 与 ,
  - @param threshold   {number} 找色时颜色相似度取值为 0.0 ~ 1.0  只对不带偏色的颜色值有效(偏色也是一种相似度) 默认0.9  以255为基准 即 255*(1-0.9)=25;
  - @param sx {number?} 起点x 默认0;
  - @param sy {number?} 起点y 默认0;
  - @param ex {number?} 终点x 默认0;
  - @param ey {number?} 终点y 默认0;
  - @param count {number?} 寻找的个数 默认1
  - @param direction {number?} 寻找的方向 1~8 默认1 <br/>
    1: → 从左上横着找到右下 (默认方式)  <br/>
    2: ← 从右上横着找到左下  <br/>
    3: ↓ 从右上竖直找到左下  <br/>
    4: ↑ 从右下竖直找到左上  <br/>
    5: ← 从右下横着找到左上  <br/>
    6: → 从左下横着找到右上  <br/>
    7: ↑ 从左下竖直找到右上  <br/>
    8: ↓ 从左上竖直找到右下  <br/>
  - @returns  []Point 或者nil
*/
func FindNotColor(mat *Mat, color string, threshold float64, sx, sy, ex, ey, count, direction int) []Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cDirection := C.int(direction)
	cResult := C.FindNotColor(mat.ptr, cColor, cThreshold, cSx, cSy, cEx, cEy, cCount, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

func FindNotColor0(args ...any) []Point {
	// 定义参数元信息（名称+默认值，匹配原函数参数顺序）
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"colorStr", ""},
		{"threshold", 0.9},
		{"sx", 0},
		{"sy", 0},
		{"ex", 0},
		{"ey", 0},
		{"count", 1},
		{"direction", 1},
	}

	// 调用通用参数校验函数，确保数量和类型正确
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 按原函数参数顺序传递，完成类型断言并调用
	return FindNotColor(
		paramMetas[0].def.(*Mat),    // mat: 输入图像
		paramMetas[1].def.(string),  // colorStr: 排除的颜色
		paramMetas[2].def.(float64), // threshold: 相似度阈值
		paramMetas[3].def.(int),     // sx: 起点x
		paramMetas[4].def.(int),     // sy: 起点y
		paramMetas[5].def.(int),     // ex: 终点x
		paramMetas[6].def.(int),     // ey: 终点y
		paramMetas[7].def.(int),     // count: 寻找个数
		paramMetas[8].def.(int),     // direction: 寻找方向
	)
}

/*
*
  - 多点找色 与EC一样的用法
  - @param img {Mat} Mat图片
  - @param firstColor {string} 第一个点的颜色 例如 "0xCDD7E9-0x101010" "#00FF00"
  - @param colorMulti {string} 字符串类似这样 "6|1|0x969696-0x000010,1|12|0x969696,-4|0|0x969696"
  - @param threshold {number?}  找色时颜色相似度取值为 0.0 ~ 1.0 默认0.9
  - @param sx {number?} 起点x 默认0;
  - @param sy {number?} 起点y 默认0;
  - @param ex {number?} 终点x 默认0;
  - @param ey {number?} 终点y 默认0;
  - @param count {number?} 寻找的个数 默认1
  - @param direction {number?} 寻找的方向 1~8 默认1 <br/>
    1: → 从左上横着找到右下 (默认方式)  <br/>
    2: ← 从右上横着找到左下  <br/>
    3: ↓ 从右上竖直找到左下  <br/>
    4: ↑ 从右下竖直找到左上  <br/>
    5: ← 从右下横着找到左上  <br/>
    6: → 从左下横着找到右上  <br/>
    7: ↑ 从左下竖直找到右上  <br/>
    8: ↓ 从左上竖直找到右下  <br/>
  - @returns Point数组或nil
*/
func FindMultiColor(mat *Mat, firstColor, colorMulti string, threshold float64, sx, sy, ex, ey, count, direction int) []Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cFirstColor := C.CString(firstColor)
	defer C.free(unsafe.Pointer(cFirstColor))
	cColorMulti := C.CString(colorMulti)
	defer C.free(unsafe.Pointer(cColorMulti))
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cDirection := C.int(direction)
	cResult := C.FindMultiColor(mat.ptr, cFirstColor, cColorMulti, cThreshold, cSx, cSy, cEx, cEy, cCount, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

func FindMultiColor0(args ...any) []Point {
	paramMetas := []paramMeta{
		{"mat", defMat},
		{"firstColor", ""},
		{"colorMulti", ""},
		{"threshold", 0.9},
		{"sx", 0},
		{"sy", 0},
		{"ex", 0},
		{"ey", 0},
		{"count", 1},
		{"direction", 1},
	}
	// 调用通用参数校验函数，确保参数数量和类型正确
	if !checkParam(&paramMetas, 3, args...) {
		return nil
	}

	// 按原函数参数顺序传递，完成类型断言并调用
	return FindMultiColor(
		paramMetas[0].def.(*Mat),    //mat: 输入图像
		paramMetas[1].def.(string),  //firstColor: 第一个基准点颜色
		paramMetas[2].def.(string),  //colorMulti: 多颜色偏移配置
		paramMetas[3].def.(float64), //threshold: 相似度阈值
		paramMetas[4].def.(int),     //sx: 起点 x
		paramMetas[5].def.(int),     //sy: 起点 y
		paramMetas[6].def.(int),     //ex: 终点 x
		paramMetas[7].def.(int),     //ey: 终点 y
		paramMetas[8].def.(int),     //count: 寻找个数
		paramMetas[9].def.(int),     //direction: 寻找方向
	)
}

/*
*
  - 多点找色 可以指定点颜色是否相等
  - @param img {Mat} Mat图片
  - @param firstColor {string} 第一个点的颜色 例如 "0xCDD7E9-0x101010" "#00FF00"  用于定位用 不可指定相等或不相等,
  - @param colorMulti {string} 字符串类似这样 前缀 1代表相等,0代表不相等 没前缀,默认相等 例如 "1|6|1|0x969696-0x000010,0|6|12|0x969696,-4|5|0x969696"
  - @param threshold {number?}  找色时颜色相似度取值为 0.0 ~ 1.0 默认0.9
  - @param sx {number?} 起点x 默认0;
  - @param sy {number?} 起点y 默认0;
  - @param ex {number?} 终点x 默认0;
  - @param ey {number?} 终点y 默认0;
  - @param count {number?} 寻找的个数 默认1
  - @param direction {number?} 寻找的方向 1~8 默认1 <br/>
    1: → 从左上横着找到右下 (默认方式)  <br/>
    2: ← 从右上横着找到左下  <br/>
    3: ↓ 从右上竖直找到左下  <br/>
    4: ↑ 从右下竖直找到左上  <br/>
    5: ← 从右下横着找到左上  <br/>
    6: → 从左下横着找到右上  <br/>
    7: ↑ 从左下竖直找到右上  <br/>
    8: ↓ 从左上竖直找到右下  <br/>
  - @returns Point数组或nil
*/
func FindMultiColorEx(mat *Mat, firstColor, colorMulti string, threshold float64, sx, sy, ex, ey, count, direction int) []Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cFirstColor := C.CString(firstColor)
	defer C.free(unsafe.Pointer(cFirstColor))
	cColorMulti := C.CString(colorMulti)
	defer C.free(unsafe.Pointer(cColorMulti))
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cDirection := C.int(direction)
	cResult := C.FindMultiColorEx(mat.ptr, cFirstColor, cColorMulti, cThreshold, cSx, cSy, cEx, cEy, cCount, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

func FindMultiColorEx0(args ...any) []Point {
	paramMetas := []paramMeta{{"mat", defMat},
		{"firstColor", ""},
		{"colorMulti", ""},
		{"threshold", 0.9},
		{"sx", 0},
		{"sy", 0},
		{"ex", 0},
		{"ey", 0},
		{"count", 1},
		{"direction", 1},
	}
	// 调用通用参数校验函数，确保参数数量和类型正确
	if !checkParam(&paramMetas, 3, args...) {
		return nil
	}

	// 按原函数参数顺序传递，完成类型断言并调用
	return FindMultiColorEx(
		paramMetas[0].def.(*Mat),    //mat: 输入图像
		paramMetas[1].def.(string),  //firstColor: 基准点颜色
		paramMetas[2].def.(string),  //colorMulti: 增强型多颜色配置
		paramMetas[3].def.(float64), //threshold: 相似度阈值
		paramMetas[4].def.(int),     //sx: 起点 x
		paramMetas[5].def.(int),     //sy: 起点 y
		paramMetas[6].def.(int),     //ex: 终点 x
		paramMetas[7].def.(int),     //ey: 终点 y
		paramMetas[8].def.(int),     //count: 寻找个数
		paramMetas[9].def.(int),     //direction: 寻找方向
	)
}

/**
 * 与EC的单点比色类似 但有些许不同 EC的points里的点都是绝对位置,而这个方法,只有第一个点是绝对位置,
 * 其他点都是相对于第一个点的相对位置
 *  单点或多点比色
 * @param img  {Mat} 图片  <br/>
 * @param points {string} 颜色字符串 例如"512|833|0x969696-0x000010,1|12|0x969696,-4|0|0x969696" 第一个点为绝对位置,其他点是相对于点一个点的相对位置  <br/>
 * @param threshold  {number?} 匹配度 默认0.9  <br/>
 * @returns {boolean} true 表示纯在 false 表示不纯在  <br/>
 */
func CmpColor(mat *Mat, points string, threshold float64) bool {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return false
	}
	cpoints := C.CString(points)
	defer C.free(unsafe.Pointer(cpoints))
	cThreshold := C.double(threshold)
	cResult := C.CmpColor(mat.ptr, cpoints, cThreshold)
	return bool(cResult)
}

func CmpColor0(args ...any) bool {
	// 定义参数元信息（名称+默认值，匹配原函数参数顺序）
	paramMetas := []paramMeta{
		{"mat", defMat},    // *Mat类型，必须参数（无有效默认值）
		{"points", ""},     // 待比较的点颜色信息字符串，默认空字符串
		{"threshold", 0.9}, // 颜色相似度阈值（0.0~1.0），默认0.9
	}

	// 调用通用参数校验函数，确保参数数量和类型正确
	if !checkParam(&paramMetas, 2, args...) {
		return false
	}

	// 按原函数参数顺序传递，完成类型断言并调用
	return CmpColor(
		paramMetas[0].def.(*Mat),    // mat: 输入图像
		paramMetas[1].def.(string),  // points: 点颜色信息字符串
		paramMetas[2].def.(float64), // threshold: 相似度阈值
	)
}

/** 单点比色扩展 可以指定点颜色是否相等
 * 与EC的单点比色类似 但有些许不同 EC的points里的点都是绝对位置,而这个方法,只有第一个点是绝对位置,
 * 其他点都是相对于第一个点的相对位置
 *  单点或多点比色
 * @param img  {Mat} 图片
 * @param points {string} 颜色字符串 前缀0代表不相等 1代表相等 没前缀默认相等  例如"0|512|833|0x969696-0x000010,1|12|0x969696,1|-4|0|0x969696" 第一个点为绝对位置,其他点是相对于点一个点的相对位置
 * @param threshold  {number?} 匹配度 ,默认0.9
 * @returns {boolean} true 表示纯在 false 表示不纯在
 */
func CmpColorEx(mat *Mat, points string, threshold float64) bool {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return false
	}
	cPoints := C.CString(points)
	defer C.free(unsafe.Pointer(cPoints))
	cThreshold := C.double(threshold)
	cResult := C.CmpColorEx(mat.ptr, cPoints, cThreshold)
	return bool(cResult)
}

func CmpColorEx0(args ...any) bool {
	// 定义参数元信息（使用defMat作为默认图片）
	paramMetas := []paramMeta{
		{"mat", defMat},    // 图片参数默认使用全局defMat
		{"points", ""},     // 待比较的点颜色信息字符串，默认空字符串
		{"threshold", 0.9}, // 颜色相似度阈值（0.0~1.0），默认0.9
	}
	// 调用通用参数校验函数
	if !checkParam(&paramMetas, 2, args...) {
		return false
	}
	// 传递参数并调用原始函数
	return CmpColorEx(
		paramMetas[0].def.(*Mat),    // mat: 使用默认defMat或用户传入的图片
		paramMetas[1].def.(string),  // points: 点颜色信息字符串
		paramMetas[2].def.(float64), // threshold: 相似度阈值
	)
}

/**
 * 多点数组比色  与Ec的区别 同上述 cmpColor;
 * @param img   {Mat} 图片
 * @param pointsArr {string[]} 颜色字符串数组 具体参考EC 的写法 注意区别
 * @param threshold {number?} 匹配度 ,默认值0.9
 * @returns {number} 如果找到就返回当前points的索引值，如果返回-1，说明都没有找到}
 */
func CmpMultiColor(mat *Mat, pointsArr []string, threshold float64) int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return -1
	}
	strVec := NewStringVector()
	defer strVec.Delete()
	for _, point := range pointsArr {
		strVec.Add(point)
	}
	cThreshold := C.double(threshold)
	cResult := C.CmpMultiColor(mat.ptr, strVec.ptr, cThreshold)
	return int(cResult)
}

func CmpMultiColor0(args ...any) int {
	// 定义参数元信息（默认值配置）
	paramMetas := []paramMeta{
		{"mat", defMat},           // 图片默认使用全局defMat
		{"pointsArr", []string{}}, // 比较点数组默认空切片
		{"threshold", 0.9},        // 阈值默认0.9
	}

	// 仅保留参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return -1
	}

	// 直接提取参数并调用原始函数（无额外有效性检查）
	return CmpMultiColor(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.([]string),
		paramMetas[2].def.(float64),
	)
}

/**
 *  快速找图
 * @param mat {Mat} Mat或者Mat大图
 * @param temp {Mat} Mat目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false
 * @returns Match数组或nil
 */
func FindImgFast(mat *Mat, temp *Mat, threshold float64, sx, sy, ex, ey, count int, strictMode bool) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cStrictMode := C.bool(strictMode)

	cResult := C.FindImgFast(mat.ptr, temp.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStrictMode)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFast0 带默认参数的快速找图函数
func FindImgFast0(args ...any) []Match {
	// 定义参数元信息（使用defMat作为mat的默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认使用全局defMat
		{"temp", defMat},      // 目标小图（无默认值）
		{"threshold", 0.9},    // 匹配度默认0.9
		{"sx", 0},             // 起点x默认0
		{"sy", 0},             // 起点y默认0
		{"ex", 0},             // 终点x默认0
		{"ey", 0},             // 终点y默认0
		{"count", 10},         // 查找个数默认10
		{"strictMode", false}, // 严格模式默认false
	}

	// 参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始函数
	return FindImgFast(
		paramMetas[0].def.(*Mat),    // mat
		paramMetas[1].def.(*Mat),    // temp
		paramMetas[2].def.(float64), // threshold
		paramMetas[3].def.(int),     // sx
		paramMetas[4].def.(int),     // sy
		paramMetas[5].def.(int),     // ex
		paramMetas[6].def.(int),     // ey
		paramMetas[7].def.(int),     // count
		paramMetas[8].def.(bool),    // strictMode
	)
}

/**
 *  快速找图
 * @param img {Mat} Mat或者Mat大图
 * @param temp {Mat[]} Mat小图数组
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false
 * @returns Match数组或nil
 */
func FindImgFastArr(mat *Mat, temps []*Mat, threshold float64, sx, sy, ex, ey, count int, strictMode bool) []Match {

	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	for i, temp := range temps {
		if temp.Empty() {
			fmt.Printf("小图中第%d个图片为空\n", i+1)
			return nil
		}
	}

	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cStrictMode := C.bool(strictMode)

	longVector := NewLongVector()
	for _, temp := range temps {
		longVector.Add(int64(temp.ptr))
	}
	defer longVector.Delete()

	cResult := C.FindImgFastArr(mat.ptr, longVector.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStrictMode)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

func FindImgFastArr0(args ...any) []Match {
	// 定义参数元信息（使用defMat作为大图默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认使用全局defMat
		{"temps", []*Mat{}},   // 小图数组默认空切片
		{"threshold", 0.9},    // 匹配度默认0.9
		{"sx", 0},             // 起点x默认0
		{"sy", 0},             // 起点y默认0
		{"ex", 0},             // 终点x默认0
		{"ey", 0},             // 终点y默认0
		{"count", 10},         // 查找个数默认10
		{"strictMode", false}, // 严格模式默认false
	}

	// 参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始函数
	return FindImgFastArr(
		paramMetas[0].def.(*Mat),    // mat
		paramMetas[1].def.([]*Mat),  // temps
		paramMetas[2].def.(float64), // threshold
		paramMetas[3].def.(int),     // sx
		paramMetas[4].def.(int),     // sy
		paramMetas[5].def.(int),     // ex
		paramMetas[6].def.(int),     // ey
		paramMetas[7].def.(int),     // count
		paramMetas[8].def.(bool),    // strictMode
	)
}

/*
*
*  基于彩色的带方向的找图
  - @param img {Mat} Mat大图
  - @param temp {Mat} Mat目标小图
  - @param threshold  {number?} 匹配度 0~1 默认0.9
  - @param sx {number?} 起点x 默认0;
  - @param sy {number?} 起点y 默认0;
  - @param ex {number?} 终点x 默认0;
  - @param ey {number?} 终点y 默认0;
  - @param count  {number?} 查找个数 默认10;
  - @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
  - @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
    0: 不带方向 <br/>
    1: → 从左上横着找到右下   <br/>
    2: ← 从右上横着找到左下  <br/>
    3: ↓ 从右上竖直找到左下  <br/>
    4: ↑ 从右下竖直找到左上  <br/>
    5: ← 从右下横着找到左上  <br/>
    6: → 从左下横着找到右上  <br/>
    7: ↑ 从左下竖直找到右上  <br/>
    8: ↓ 从左上竖直找到右下  <br/>

* @returns Match数组或nil
*/
func FindImgFastEx(mat *Mat, temp *Mat, threshold float64, sx, sy, ex, ey, count int, strictMode bool, direction int) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cStrictMode := C.bool(strictMode)
	cDirection := C.int(direction)
	cResult := C.FindImgFastEx(mat.ptr, temp.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStrictMode, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFastEx0 带默认参数的带方向彩色彩色快速找图函数
func FindImgFastEx0(args ...any) []Match {
	// 定义参数元信息（使用defMat作为大图默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认使用全局defMat
		{"temp", defMat},      // 目标小图（无默认值）
		{"threshold", 0.9},    // 匹配度默认0.9
		{"sx", 0},             // 起点x默认0
		{"sy", 0},             // 起点y默认0
		{"ex", 0},             // 终点x默认0
		{"ey", 0},             // 终点y默认0
		{"count", 10},         // 查找个数默认10
		{"strictMode", false}, // 严格模式默认false
		{"direction", 0},      // 方向默认0（不带方向）
	}

	// 参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始函数
	return FindImgFastEx(
		paramMetas[0].def.(*Mat),    // mat
		paramMetas[1].def.(*Mat),    // temp
		paramMetas[2].def.(float64), // threshold
		paramMetas[3].def.(int),     // sx
		paramMetas[4].def.(int),     // sy
		paramMetas[5].def.(int),     // ex
		paramMetas[6].def.(int),     // ey
		paramMetas[7].def.(int),     // count
		paramMetas[8].def.(bool),    // strictMode
		paramMetas[9].def.(int),     // direction
	)
}

/*
*
  - 基于彩色的带方向的找图
  - @param img {Mat} Mat大图
  - @param temp {Mat[]} Mat[]目标小图
  - @param threshold  {number?} 匹配度 0~1 默认0.9
  - @param sx {number?} 起点x 默认0;
  - @param sy {number?} 起点y 默认0;
  - @param ex {number?} 终点x 默认0;
  - @param ey {number?} 终点y 默认0;
  - @param count  {number?} 查找个数 默认10;
  - @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
  - @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
    0: 不带方向 <br/>
    1: → 从左上横着找到右下   <br/>
    2: ← 从右上横着找到左下  <br/>
    3: ↓ 从右上竖直找到左下  <br/>
    4: ↑ 从右下竖直找到左上  <br/>
    5: ← 从右下横着找到左上  <br/>
    6: → 从左下横着找到右上  <br/>
    7: ↑ 从左下竖直找到右上  <br/>
    8: ↓ 从左上竖直找到右下  <br/>
  - @returns Match数组或nil
*/
func FindImgFastExArr(mat *Mat, temps []*Mat, threshold float64, sx, sy, ex, ey, count int, strictMode bool, direction int) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	for i, temp := range temps {
		if temp.Empty() {
			fmt.Printf("小图中第%d个图片为空\n", i+1)
			return nil
		}
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cStrictMode := C.bool(strictMode)
	cDirection := C.int(direction)

	longVector := NewLongVector()
	for _, temp := range temps {
		longVector.Add(int64(temp.ptr))
	}
	defer longVector.Delete()
	cResult := C.FindImgFastExArr(mat.ptr, longVector.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStrictMode, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFastExArr0 带默认参数的带方向多模板彩色快速找图函数
func FindImgFastExArr0(args ...any) []Match {
	// 定义参数元信息（使用defMat作为大图默认值）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认使用全局defMat
		{"temps", []*Mat{}},   // 目标小图数组默认空切片
		{"threshold", 0.9},    // 匹配度默认0.9
		{"sx", 0},             // 起点x默认0
		{"sy", 0},             // 起点y默认0
		{"ex", 0},             // 终点x默认0
		{"ey", 0},             // 终点y默认0
		{"count", 10},         // 查找个数默认10
		{"strictMode", false}, // 严格模式默认false
		{"direction", 0},      // 方向默认0（不带方向）
	}

	// 参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始函数
	return FindImgFastExArr(
		paramMetas[0].def.(*Mat),    // mat
		paramMetas[1].def.([]*Mat),  // temps
		paramMetas[2].def.(float64), // threshold
		paramMetas[3].def.(int),     // sx
		paramMetas[4].def.(int),     // sy
		paramMetas[5].def.(int),     // ex
		paramMetas[6].def.(int),     // ey
		paramMetas[7].def.(int),     // count
		paramMetas[8].def.(bool),    // strictMode
		paramMetas[9].def.(int),     // direction
	)
}

/*
*
*  基于灰度的带方向的找图
* @param mat {Mat} mat大图
* @param temp {Mat} Mat目标小图
* @param threshold  {number?} 匹配度 0~1 默认0.9
* @param sx {number?} 起点x 默认0;
* @param sy {number?} 起点y 默认0;
* @param ex {number?} 终点x 默认0;
* @param ey {number?} 终点y 默认0;
* @param count  {number?} 查找个数 默认10;
* @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
* @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
0: 不带方向 <br/>
1: → 从左上横着找到右下   <br/>
2: ← 从右上横着找到左下  <br/>
3: ↓ 从右上竖直找到左下  <br/>
4: ↑ 从右下竖直找到左上  <br/>
5: ← 从右下横着找到左上  <br/>
6: → 从左下横着找到右上  <br/>
7: ↑ 从左下竖直找到右上  <br/>
8: ↓ 从左上竖直找到右下  <br/>
* @returns Match数组或nil
*/
func FindImgFastWeakEx(mat *Mat, temp *Mat, threshold float64, sx, sy, ex, ey, count int, strictMode bool, direction int) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}

	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cStrictMode := C.bool(strictMode)
	cDirection := C.int(direction)
	cResult := C.FindImgFastWeakEx(mat.ptr, temp.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStrictMode, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFastWeakEx0 带默认参数的基于灰度的带方向找图函数
// 支持可变参数调用，未传递的参数将使用默认值
func FindImgFastWeakEx0(args ...any) []Match {
	// 定义参数元信息及默认值
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认使用全局defMat
		{"temp", defMat},      // 目标小图（无默认值）
		{"threshold", 0.9},    // 匹配度默认0.9
		{"sx", 0},             // 起点x默认0
		{"sy", 0},             // 起点y默认0
		{"ex", 0},             // 终点x默认0
		{"ey", 0},             // 终点y默认0
		{"count", 10},         // 查找个数默认10
		{"strictMode", false}, // 严格模式默认false
		{"direction", 0},      // 方向默认0（不带方向）
	}

	// 参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始函数并返回结果
	return FindImgFastWeakEx(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.(*Mat),
		paramMetas[2].def.(float64),
		paramMetas[3].def.(int),
		paramMetas[4].def.(int),
		paramMetas[5].def.(int),
		paramMetas[6].def.(int),
		paramMetas[7].def.(int),
		paramMetas[8].def.(bool),
		paramMetas[9].def.(int),
	)
}

/*
*
*  基于灰度的带方向的找图
* @param mat {Mat} mat大图
* @param temp {Mat[]} Mat[]目标小图
* @param threshold  {number?} 匹配度 0~1 默认0.9
* @param sx {number?} 起点x 默认0;
* @param sy {number?} 起点y 默认0;
* @param ex {number?} 终点x 默认0;
* @param ey {number?} 终点y 默认0;
* @param count  {number?} 查找个数 默认10;
* @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true(极为耗时),默认为false
* @param direction {number?}  取值 0~8,带方向耗时,默认为0 不带方向
0: 不带方向 <br/>
1: → 从左上横着找到右下   <br/>
2: ← 从右上横着找到左下  <br/>
3: ↓ 从右上竖直找到左下  <br/>
4: ↑ 从右下竖直找到左上  <br/>
5: ← 从右下横着找到左上  <br/>
6: → 从左下横着找到右上  <br/>
7: ↑ 从左下竖直找到右上  <br/>
8: ↓ 从左上竖直找到右下  <br/>
* @returns Match数组或nil
*/
func FindImgFastWeakExArr(mat *Mat, temps []*Mat, threshold float64, sx, sy, ex, ey, count int, strictMode bool, direction int) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	for i, temp := range temps {
		if temp.Empty() {
			fmt.Printf("小图中第%d个图片为空\n", i+1)
			return nil
		}
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cStrictMode := C.bool(strictMode)
	cDirection := C.int(direction)

	longVector := NewLongVector()
	for _, temp := range temps {
		longVector.Add(int64(temp.ptr))
	}
	defer longVector.Delete()
	cResult := C.FindImgFastWeakExArr(mat.ptr, longVector.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStrictMode, cDirection)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFastWeakExArr0 带默认参数的基于灰度带方向多模板找图函数
func FindImgFastWeakExArr0(args ...any) []Match {
	// 定义参数元信息及默认值（对齐原始函数注释）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认用 defMat
		{"temps", []*Mat{}},   // 小图数组默认空切片
		{"threshold", 0.9},    // 匹配度默认 0.9
		{"sx", 0},             // 起点 x 默认 0
		{"sy", 0},             // 起点 y 默认 0
		{"ex", 0},             // 终点 x 默认 0
		{"ey", 0},             // 终点 y 默认 0
		{"count", 10},         // 查找个数默认 10
		{"strictMode", false}, // 严格模式默认 false
		{"direction", 0},      // 方向默认 0（不带方向）
	}

	// 参数数量和类型校验
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始函数返回结果
	return FindImgFastWeakExArr(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.([]*Mat),
		paramMetas[2].def.(float64),
		paramMetas[3].def.(int),
		paramMetas[4].def.(int),
		paramMetas[5].def.(int),
		paramMetas[6].def.(int),
		paramMetas[7].def.(int),
		paramMetas[8].def.(bool),
		paramMetas[9].def.(int),
	)
}

/**
 *  快速找图 弱化版 (效果和ec aj找图一样), 相比于 findImgFast 没有 颜色校验 明暗校验 哈希校验, 对找半透明且背景变化的图有利,失去了严格的校验也存在找错的可能
 * @param mat {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @param strictMode {boolean?}  如果对匹配要求严格不允许有差异(比如找字) 设为true,默认为false
 * @returns Match数组或nil
 */
func FindImgFastWeak(mat *Mat, temp *Mat, threshold float64, sx, sy, ex, ey, count int) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cResult := C.FindImgFastWeak(mat.ptr, temp.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFastWeak0 带默认参数的基于灰度快速找图函数
func FindImgFastWeak0(args ...any) []Match {
	// 定义参数元信息及默认值（对齐原始函数参数逻辑）
	paramMetas := []paramMeta{
		{"mat", defMat},    // 大图默认用全局 defMat
		{"temp", defMat},   // 目标小图（无默认值）
		{"threshold", 0.9}, // 匹配度默认 0.9（0~1 范围）
		{"sx", 0},          // 查找起点 x 默认 0
		{"sy", 0},          // 查找起点 y 默认 0
		{"ex", 0},          // 查找终点 x 默认 0
		{"ey", 0},          // 查找终点 y 默认 0
		{"count", 10},      // 最大查找个数默认 10
	}

	// 基础参数校验：数量不超限 + 类型匹配
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始基于灰度的快速找图函数
	return FindImgFastWeak(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(*Mat),    // 目标小图 temp
		paramMetas[2].def.(float64), // 匹配度 threshold
		paramMetas[3].def.(int),     // 起点 x
		paramMetas[4].def.(int),     // 起点 y
		paramMetas[5].def.(int),     // 终点 x
		paramMetas[6].def.(int),     // 终点 y
		paramMetas[7].def.(int),     // 查找个数 count
	)
}

/**
 *  快速找图 弱化版 (效果和ec aj找图一样), 相比于 findImgFast 没有 颜色校验 明暗校验 哈希校验, 对找半透明且背景变化的图有利,失去了严格的校验也存在找错的可能
 * @param img {Mat} Mat大图
 * @param temp {Mat[]} Mat小图数组
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 查找个数 默认10;
 * @returns Match数组或nil
 */
func FindImgFastWeakArr(mat *Mat, temps []*Mat, threshold float64, sx, sy, ex, ey, count int) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	for i, temp := range temps {
		if temp.Empty() {
			fmt.Printf("小图中第%d个图片为空\n", i+1)
			return nil
		}
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)

	longVector := NewLongVector()
	for _, temp := range temps {
		longVector.Add(int64(temp.ptr))
	}
	defer longVector.Delete()
	cResult := C.FindImgFastWeakArr(mat.ptr, longVector.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgFastWeakArr0 带默认参数的弱化版多模板快速找图函数
// 适配半透明 / 背景变化场景，无严格校验，默认参数对齐原始函数逻辑
func FindImgFastWeakArr0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释）
	paramMetas := []paramMeta{
		{"mat", defMat},     // 大图默认用全局 defMat
		{"temps", []*Mat{}}, // 小图数组默认空切片
		{"threshold", 0.9},  // 匹配度默认 0.9（0~1 范围）
		{"sx", 0},           // 查找起点 x 默认 0
		{"sy", 0},           // 查找起点 y 默认 0
		{"ex", 0},           // 查找终点 x 默认 0
		{"ey", 0},           // 查找终点 y 默认 0
		{"count", 10},       // 最大查找个数默认 10
	}

	// 基础参数校验：数量不超限 + 类型匹配
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始弱化版多模板找图函数
	return FindImgFastWeakArr(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.([]*Mat),  // 小图数组 temps
		paramMetas[2].def.(float64), // 匹配度 threshold
		paramMetas[3].def.(int),     // 起点 x
		paramMetas[4].def.(int),     // 起点 y
		paramMetas[5].def.(int),     // 终点 x
		paramMetas[6].def.(int),     // 终点 y
		paramMetas[7].def.(int),     // 查找个数 count
	)
}

/**模板匹配找图
 *  与EC找图用法类似  注意返回值 这里做成了和EC一样
 * @param mat {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param resize  {number?} 大图缩放倍数 适当缩放有利于速度提升 例如 1表示不缩放 0.5表示缩小一倍 默认0.5
 * @param threshold  {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count   {number?} 寻找个数 默认 1
 * @param mask    {Mat?} 掩膜 对应的是小图 与小图一样大小 默认为nil
 * @returns Match数组或nil
 */
func FindImg(mat *Mat, temp *Mat, resize float64, threshold float64, sx, sy, ex, ey, count int, mask *Mat) []Match {

	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (temp.GetHeight() != mask.GetHeight() || temp.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与小图大小必须一致！")
		return nil
	}

	cResize := C.double(resize)
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cResult := C.FindImg(mat.ptr, temp.ptr, cResize, cThreshold, cSx, cSy, cEx, cEy, cCount, mask.ptr)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImg0 带默认参数的 EC 风格模板匹配找图函数
// 适配 EC 找图用法，默认参数对齐函数注释，简化调用逻辑
func FindImg0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释规则）
	paramMetas := []paramMeta{
		{"mat", defMat},    // 大图默认用全局 defMat
		{"temp", defMat},   // 目标小图（无默认值，必传）
		{"resize", 0.5},    // 大图缩放倍数默认 0.5（平衡速度与精度）
		{"threshold", 0.9}, // 匹配度默认 0.9（0~1 范围）
		{"sx", 0},          // 查找起点 x 默认 0
		{"sy", 0},          // 查找起点 y 默认 0
		{"ex", 0},          // 查找终点 x 默认 0
		{"ey", 0},          // 查找终点 y 默认 0
		{"count", 1},       // 寻找个数默认 1（与 EC 默认一致）
		{"mask", defMat},   // 掩膜默认 nil（无掩膜匹配）
	}

	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	if paramMetas[len(paramMetas)-1].def.(*Mat).ptr == 0 {
		mask := NewMat()
		defer mask.Recycle()
		paramMetas[len(paramMetas)-1].def = mask
	}
	// 调用原始 EC 风格模板匹配找图函数
	return FindImg(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(*Mat),    // 目标小图 temp
		paramMetas[2].def.(float64), // 大图缩放倍数 resize
		paramMetas[3].def.(float64), // 匹配度 threshold
		paramMetas[4].def.(int),     // 起点 x
		paramMetas[5].def.(int),     // 起点 y
		paramMetas[6].def.(int),     // 终点 x
		paramMetas[7].def.(int),     // 终点 y
		paramMetas[8].def.(int),     // 寻找个数 count
		paramMetas[9].def.(*Mat),    // 掩膜 mask
	)
}

/**
 * 只做图片的轮廓匹配,无视颜色  注:这个不是全分辨率
 * @param img {Mat} Mat大图
 * @param temp   {Mat} Mat目标小图
 * @param resize  {number?} 大图缩放倍数 适当缩放有利于速度提升 例如 1表示不缩放 0.5表示缩小一倍 默认0.5
 * @param threshold {number?} 匹配度 0~1 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 寻找个数 默认 1
 * @param weakThreshold {number?}  检测轮廓(canny)时的弱阈值 大于弱阈值且依附在边缘的点会被保留  默认值 50
 * @param strongThreshold {number?} 检测轮廓(canny)时的强阈值 即相邻的像素点差值大于强阈值则被认为是边缘 默认值 100
 * @return Match数组或nil
 */
func FindImgByContour(mat *Mat, temp *Mat, resize float64, threshold float64, sx, sy, ex, ey, count, weakThreshold, strongThreshold int) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}

	cResize := C.double(resize)
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cWeakThreshold := C.int(weakThreshold)
	cStrongThreshold := C.int(strongThreshold)
	cResult := C.FindImgByContour(mat.ptr, temp.ptr, cResize, cThreshold, cSx, cSy, cEx, cEy, cCount, cWeakThreshold, cStrongThreshold)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindImgByContour0 带默认参数的轮廓匹配找图函数
// 无视颜色，默认参数对齐函数注释，简化非全分辨率轮廓匹配调用
func FindImgByContour0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释规则）
	paramMetas := []paramMeta{
		{"mat", defMat},          // 大图默认用全局 defMat
		{"temp", defMat},         // 目标小图（无默认值，必传）
		{"resize", 0.5},          // 大图缩放倍数默认 0.5（平衡速度与匹配精度）
		{"threshold", 0.9},       // 匹配度默认 0.9（0~1 范围）
		{"sx", 0},                // 查找起点 x 默认 0
		{"sy", 0},                // 查找起点 y 默认 0
		{"ex", 0},                // 查找终点 x 默认 0
		{"ey", 0},                // 查找终点 y 默认 0
		{"count", 1},             // 寻找个数默认 1
		{"weakThreshold", 50},    // Canny 弱阈值默认 50（符合函数注释默认值）
		{"strongThreshold", 100}, // Canny 强阈值默认 100（符合函数注释默认值）
	}

	// 基础参数校验：数量不超限 + 类型匹配（避免类型断言错误）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始轮廓匹配找图函数（注意：resize 为 float64，阈值为 int，需精准类型断言）
	return FindImgByContour(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(*Mat),    // 目标小图 temp
		paramMetas[2].def.(float64), // 大图缩放倍数 resize
		paramMetas[3].def.(float64), // 匹配度 threshold
		paramMetas[4].def.(int),     // 起点 x
		paramMetas[5].def.(int),     // 起点 y
		paramMetas[6].def.(int),     // 终点 x
		paramMetas[7].def.(int),     // 终点 y
		paramMetas[8].def.(int),     // 寻找个数 count
		paramMetas[9].def.(int),     // Canny 弱阈值 weakThreshold
		paramMetas[10].def.(int),    // Canny 强阈值 strongThreshold
	)
}

/** 半透明找图 类似 按键精灵的半透明找图
 *  模板 背景最好为纯色,如果是杂色,请涂成纯色,此方法认为与模板四个脚颜色相同的点为背景,将忽略
 *
 * @param mat {Mat} mat大图
 * @param temp {Mat} mat目标小图
 * @param resize  {number?} 大图缩放倍数 适当缩放有利于速度提升 例如 1表示不缩放 0.5表示缩小一倍 默认0.5
 * @param threshold {number?} 匹配度 0~1 默认0.95
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count   {number?} 寻找个数 默认 1
 * @param colorOffset {number?} 背景颜色偏差 默认 20 个点
 * @returns Match数组或nil
 */
func FindTranslucentImg(mat *Mat, temp *Mat, resize float64, threshold float64, sx, sy, ex, ey, count, colorOffset int) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}

	cResize := C.double(resize)
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cColorOffset := C.int(colorOffset)
	cResult := C.FindTranslucentImg(mat.ptr, temp.ptr, cResize, cThreshold, cSx, cSy, cEx, cEy, cCount, cColorOffset)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// FindTranslucentImg0 带默认参数的半透明找图函数
// 类似按键精灵半透明找图逻辑，默认参数对齐函数注释
func FindTranslucentImg0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释规则）
	paramMetas := []paramMeta{
		{"mat", defMat},     // 大图默认用全局defMat
		{"temp", defMat},    // 目标小图（无默认值，必传）
		{"resize", 0.5},     // 大图缩放倍数默认0.5（平衡速度与精度）
		{"threshold", 0.95}, // 匹配度默认0.95（半透明场景默认更高匹配度）
		{"sx", 0},           // 查找起点x默认0
		{"sy", 0},           // 查找起点y默认0
		{"ex", 0},           // 查找终点x默认0
		{"ey", 0},           // 查找终点y默认0
		{"count", 1},        // 寻找个数默认1
		{"colorOffset", 20}, // 背景颜色偏差默认20个点（符合函数注释）
	}

	// 基础参数校验：数量不超限 + 类型匹配（严格区分float64与int）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始半透明找图函数
	return FindTranslucentImg(
		paramMetas[0].def.(*Mat),    // 大图mat
		paramMetas[1].def.(*Mat),    // 目标小图temp
		paramMetas[2].def.(float64), // 大图缩放倍数resize
		paramMetas[3].def.(float64), // 匹配度threshold
		paramMetas[4].def.(int),     // 起点x
		paramMetas[5].def.(int),     // 起点y
		paramMetas[6].def.(int),     // 终点x
		paramMetas[7].def.(int),     // 终点y
		paramMetas[8].def.(int),     // 寻找个数count
		paramMetas[9].def.(int),     // 背景颜色偏差colorOffset
	)
}

/**
 * 创建图片
 * @param width 宽
 * @param height 高
 * @param _color 颜色 例如 "#000000"
 */
func CreateMat(width, height int, color string) *Mat {
	mat := NewMat()
	cWidth := C.int(width)
	cHeight := C.int(height)
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	C.CreateMat(cWidth, cHeight, cColor, mat.ptr)
	return mat
}

/**  surf算法全分辨率找图
 * 对图片进行缩放有利于提高速度，缩放过了就会因太模糊而找不到图
 * 需自行测试最佳比例，使用中等分辨率做模板最佳(其他分辨率也可以,中等分辨率对高低兼容性强而已;),
 * @param img {Mat}  Mat大图
 * @param template {Mat}  Mat目标小图
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param threshold {number?} 匹配度  默认是0.6
 *                             注：这个匹配度 是先找到位置再算匹配度 而ec模板匹配是先算匹配度再确认位置
 *                             所以这个匹配度只是用于最终确认用的,写的更低也是ok的比如 0.5,其他全分辨率同理。
 * @param upright {boolean?} 只计算直立方向？true 是， false 计算全部方向（即旋转不变性，耗时） 默认true
 * @returns Point数组或nil
 */
func FindImgBySurf(mat, temp *Mat, imgResize, tempResize float64, sx, sy, ex, ey int, threshold float64, upright bool) []Point {

	fmt.Println("Surf算受专利保护,4.7.6(含)后不支持！")
	return nil

	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}

	cImgResize := C.double(imgResize)
	cTempResize := C.double(tempResize)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cThreshold := C.double(threshold)
	cUpright := C.bool(upright)
	cResult := C.FindImgBySurf(mat.ptr, temp.ptr, cImgResize, cTempResize, cSx, cSy, cEx, cEy, cThreshold, cUpright)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// FindImgBySurf0 带默认参数的 SURF 算法全分辨率找图函数
// 适配 SURF 算法特性，默认参数对齐函数注释，简化缩放倍数与方向配置
func FindImgBySurf0(args ...any) []Point {
	// 定义参数元信息及默认值（严格匹配函数注释规则）
	paramMetas := []paramMeta{
		{"mat", defMat},     // 大图默认用全局 defMat
		{"temp", defMat},    // 目标小图（无默认值，必传）
		{"imgResize", 1.0},  // 大图缩放倍数默认 1.0（全分辨率默认不缩放）
		{"tempResize", 1.0}, // 小图缩放倍数默认 1.0（默认不缩放）
		{"sx", 0},           // 查找起点 x 默认 0
		{"sy", 0},           // 查找起点 y 默认 0
		{"ex", 0},           // 查找终点 x 默认 0
		{"ey", 0},           // 查找终点 y 默认 0
		{"threshold", 0.6},  // 匹配度默认 0.6（符合 SURF 算法最终确认逻辑）
		{"upright", true},   // 默认只算直立方向（避免旋转不变性的高耗时）
	}

	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始 SURF 算法找图函数
	return FindImgBySurf(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(*Mat),    // 目标小图 temp
		paramMetas[2].def.(float64), // 大图缩放倍数 imgResize
		paramMetas[3].def.(float64), // 小图缩放倍数 tempResize
		paramMetas[4].def.(int),     // 起点 x
		paramMetas[5].def.(int),     // 起点 y
		paramMetas[6].def.(int),     // 终点 x
		paramMetas[7].def.(int),     // 终点 y
		paramMetas[8].def.(float64), // 匹配度 threshold
		paramMetas[9].def.(bool),    // 直立方向开关 upright
	)
}

/**
 *原始surf找图 具有尺度不变性 旋转不变性 光照不变性
 * @param img {Mat} Mat大图
 * @param temp  {Mat} Mat目标小图
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param upright  {boolean?} 只计算直立方向？true 是， false 计算全部方向（即旋转不变性，耗时） 默认true
 * @param distanceRatio {number?} 距离比值 最近距离和次最近距离的比值 可以理解为准确度 这个方法没有匹配度而是使用距离比值,值范围在 0.4~0.6之间最佳,对准确度要求高的写0.4,一般写0.5,默认值0.6
 * @param customFeatures  {number?} 特征匹配对 阈值, 用于定义 至少多少对特征点匹配成功,才认为是目标. 特征点个数与模板大小成正比,默认 6 对
 * @param minHessian  {number?}    minHessian越大特征点越稳定 同时也越少,默认值100
 * @param nOctaves    {number?}   图像金字塔组数,默认值 4
 * @param nOctaveLayers {number?}   金字塔每组层数,默认值 3
 * @param extended   {boolean?}    表示特征向量的维度, false 表示64维 速度快,true 表示128维 准确性高,默认 false;
 * @returns Point数组或nil
 */
func FindImgBySurfEx(mat, temp *Mat, imgResize, tempResize float64, sx, sy, ex, ey int, upright bool, distanceRatio float64,
	customFeatures, minHessian, nOctaves, nOctaveLayers int, extended bool) []Point {

	fmt.Println("Surf算受专利保护,4.7.6(含)后不支持！")
	return nil

	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cImgResize := C.double(imgResize)
	cTempResize := C.double(tempResize)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cUpright := C.bool(upright)
	cDistanceRatio := C.double(distanceRatio)
	cCustomFeatures := C.int(customFeatures)
	cMinHessian := C.int(minHessian)
	cNOctaves := C.int(nOctaves)
	cNOctaveLayers := C.int(nOctaveLayers)
	cExtended := C.bool(extended)
	cResult := C.FindImgBySurfEx(mat.ptr, temp.ptr, cImgResize, cTempResize, cSx, cSy, cEx, cEy, cUpright, cDistanceRatio,
		cCustomFeatures, cMinHessian, cNOctaves, cNOctaveLayers, cExtended)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// FindImgBySurfEx0 带默认参数的原始 SURF 增强版找图函数
// 保留 SURF 尺度 / 旋转 / 光照不变性，默认参数对齐函数注释，简化多参数配置
func FindImgBySurfEx0(args ...any) []Point {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},        // 大图默认用全局 defMat
		{"temp", defMat},       // 目标小图（无默认值，必传）
		{"imgResize", 1.0},     // 大图缩放倍数默认 1.0（全分辨率默认不缩放）
		{"tempResize", 1.0},    // 小图缩放倍数默认 1.0（默认不缩放）
		{"sx", 0},              // 查找起点 x 默认 0
		{"sy", 0},              // 查找起点 y 默认 0
		{"ex", 0},              // 查找终点 x 默认 0
		{"ey", 0},              // 查找终点 y 默认 0
		{"upright", true},      // 默认只算直立方向（避免旋转不变性高耗时）
		{"distanceRatio", 0.6}, // 距离比值默认 0.6（一般场景推荐值）
		{"customFeatures", 6},  // 特征匹配对阈值默认 6 对（默认最低匹配要求）
		{"minHessian", 100},    // 最小 Hessian 值默认 100（平衡特征点稳定性与数量）
		{"nOctaves", 4},        // 图像金字塔组数默认 4（默认层级配置）
		{"nOctaveLayers", 3},   // 金字塔每组层数默认 3（默认层级配置）
		{"extended", false},    // 特征向量维度默认 64 维（优先保证速度）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（多参数场景需严格区分类型）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始 SURF 增强版找图函数（按参数顺序精准传参）
	return FindImgBySurfEx(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(*Mat),    // 目标小图 temp
		paramMetas[2].def.(float64), // 大图缩放倍数 imgResize
		paramMetas[3].def.(float64), // 小图缩放倍数 tempResize
		paramMetas[4].def.(int),     // 起点 x
		paramMetas[5].def.(int),     // 起点 y
		paramMetas[6].def.(int),     // 终点 x
		paramMetas[7].def.(int),     // 终点 y
		paramMetas[8].def.(bool),    // 直立方向开关 upright
		paramMetas[9].def.(float64), // 距离比值 distanceRatio
		paramMetas[10].def.(int),    // 特征匹配对阈值 customFeatures
		paramMetas[11].def.(int),    // 最小 Hessian 值 minHessian
		paramMetas[12].def.(int),    // 金字塔组数 nOctaves
		paramMetas[13].def.(int),    // 金字塔每组层数 nOctaveLayers
		paramMetas[14].def.(bool),   // 特征向量维度开关 extended
	)
}

/** sift算法全分辨率找图
 *对图片进行缩放有利于提高速度，缩放过了就会因太模糊而找不到图
 * 需自行测试最佳比例，使用中等分辨率做模板最佳(其他分辨率也可以,中等分辨率对高低兼容性强而已;),
 * @param {Mat} img  Mat大图
 * @param {Mat} template Mat目标小图
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param threshold  {number?} 匹配度 0~1 默认0.6
 * @returns Point数组或nil
 */
func FindImgBySift(mat, temp *Mat, imgResize, tempResize float64, sx, sy, ex, ey int, threshold float64) []Point {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cImgResize := C.double(imgResize)
	cTempResize := C.double(tempResize)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cThreshold := C.double(threshold)
	cResult := C.FindImgBySift(mat.ptr, temp.ptr, cImgResize, cTempResize, cSx, cSy, cEx, cEy, cThreshold)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// FindImgBySift0 带默认参数的 SIFT 算法全分辨率找图函数
// 适配 SIFT 尺度 / 旋转不变性，默认参数对齐函数注释，简化中等分辨率模板匹配调用
func FindImgBySift0(args ...any) []Point {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},     // 大图默认用全局 defMat
		{"temp", defMat},    // 目标小图（无默认值，必传，推荐中等分辨率）
		{"imgResize", 1.0},  // 大图缩放倍数默认 1.0（全分辨率默认不缩放）
		{"tempResize", 1.0}, // 小图缩放倍数默认 1.0（默认不缩放，保留特征）
		{"sx", 0},           // 查找起点 x 默认 0
		{"sy", 0},           // 查找起点 y 默认 0
		{"ex", 0},           // 查找终点 x 默认 0
		{"ey", 0},           // 查找终点 y 默认 0
		{"threshold", 0.6},  // 匹配度默认 0.6（SIFT 算法通用阈值，0~1 范围）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（区分 float64 与 int，避免参数错误）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始 SIFT 全分辨率找图函数
	return FindImgBySift(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(*Mat),    // 目标小图 temp
		paramMetas[2].def.(float64), // 大图缩放倍数 imgResize
		paramMetas[3].def.(float64), // 小图缩放倍数 tempResize
		paramMetas[4].def.(int),     // 起点 x
		paramMetas[5].def.(int),     // 起点 y
		paramMetas[6].def.(int),     // 终点 x
		paramMetas[7].def.(int),     // 终点 y
		paramMetas[8].def.(float64), // 匹配度 threshold
	)
}

/**
 * 原始sift找图 具有尺度不变性 旋转不变性 光照不变性  无视背景
 * @param img {Mat} Mat大图
 * @param temp {Mat} Mat目标小图
 * @param srcMultiple {number?}大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempMultiple {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param distanceRatio {number?} 距离比值 最近距离和次最近距离的比值 可以理解为准确度 这个方法没有匹配度而是使用距离比值,值范围在 0.4~0.6之间最佳,对准确度要求高的写0.4,一般写0.5,默认值0.6
 * @param customFeatures  {number?} 特征匹配对 阈值, 用于定义 至少多少对特征点匹配成功,才认为是目标. 特征点个数与模板大小成正比,默认 6 对
 * @param nFeatures  {number?}  表示要返回的最优秀特征点数量  默认 0;
 * @param nOctaveLayers  {number?} 每组图像的层数(这个方法图像金字塔的组数是自动计算的) 默认 3;
 * @param contrastThreshold {number?} 对比度阈值 阈值越大，检测器产生的特征越少。 默认 0.04;
 * @param edgeThreshold    {number?}  边缘阈值 ，阈值越大，被过滤掉的越少，特征点越多  默认10;
 * @param sigma    {number?}   金字塔第0层图像高斯滤波系数，也就是σ。默认1.6;
 * @returns Point数组或nil
 */
func FindImgBySiftEx(mat, temp *Mat, srcMultiple, tempMultiple float64, sx, sy, ex, ey int, distanceRatio float64, customFeatures, nFeatures, nOctaveLayers int, contrastThreshold, edgeThreshold, sigma float64) []Point {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cSrcMultiple := C.double(srcMultiple)
	cTempMultiple := C.double(tempMultiple)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cDistanceRatio := C.double(distanceRatio)
	cCustomFeatures := C.int(customFeatures)
	cNFeatures := C.int(nFeatures)
	cNOctaveLayers := C.int(nOctaveLayers)
	cContrastThreshold := C.double(contrastThreshold)
	cEdgeThreshold := C.double(edgeThreshold)
	cSigma := C.double(sigma)

	cResult := C.FindImgBySiftEx(mat.ptr, temp.ptr,
		cSrcMultiple, cTempMultiple,
		cSx, cSy, cEx, cEy,
		cDistanceRatio, cCustomFeatures,
		cNFeatures, cNOctaveLayers,
		cContrastThreshold, cEdgeThreshold, cSigma)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// FindImgBySiftEx0 带默认参数的原始SIFT增强版找图函数
// 保留SIFT尺度/旋转/光照不变性，默认参数对齐函数注释，简化多参数配置
func FindImgBySiftEx0(args ...any) []Point {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},             // 大图默认用全局defMat
		{"temp", defMat},            // 目标小图（无默认值，必传）
		{"srcMultiple", 1.0},        // 大图缩放倍数默认1.0（全分辨率默认不缩放）
		{"tempMultiple", 1.0},       // 小图缩放倍数默认1.0（默认不缩放）
		{"sx", 0},                   // 查找起点x默认0
		{"sy", 0},                   // 查找起点y默认0
		{"ex", 0},                   // 查找终点x默认0
		{"ey", 0},                   // 查找终点y默认0
		{"distanceRatio", 0.6},      // 距离比值默认0.6（一般场景推荐值）
		{"customFeatures", 6},       // 特征匹配对阈值默认6对
		{"nFeatures", 0},            // 返回特征点数量默认0（自动选择）
		{"nOctaveLayers", 3},        // 每组图像层数默认3
		{"contrastThreshold", 0.04}, // 对比度阈值默认0.04
		{"edgeThreshold", 10.0},     // 边缘阈值默认10.0
		{"sigma", 1.6},              // 高斯滤波系数默认1.6
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（多参数场景需严格区分类型）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始SIFT增强版找图函数（按参数顺序精准传参）
	return FindImgBySiftEx(
		paramMetas[0].def.(*Mat),     // 大图mat
		paramMetas[1].def.(*Mat),     // 目标小图temp
		paramMetas[2].def.(float64),  // 大图缩放倍数srcMultiple
		paramMetas[3].def.(float64),  // 小图缩放倍数tempMultiple
		paramMetas[4].def.(int),      // 起点x
		paramMetas[5].def.(int),      // 起点y
		paramMetas[6].def.(int),      // 终点x
		paramMetas[7].def.(int),      // 终点y
		paramMetas[8].def.(float64),  // 距离比值distanceRatio
		paramMetas[9].def.(int),      // 特征匹配对阈值customFeatures
		paramMetas[10].def.(int),     // 返回特征点数量nFeatures
		paramMetas[11].def.(int),     // 每组图像层数nOctaveLayers
		paramMetas[12].def.(float64), // 对比度阈值contrastThreshold
		paramMetas[13].def.(float64), // 边缘阈值edgeThreshold
		paramMetas[14].def.(float64), // 高斯滤波系数sigma
	)
}

/**ORB算法全分辨率找图
 *  //此方法速度快 但比较难以控制,看实验
 * @param img {Mat} Mat大图
 * @param template {Mat} Mat目标小图  这个方法对长宽要求不能小于40个像素
 * @param imgResize {number?} 大图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param tempResize {number?} 小图缩放倍数  例如 1表示不缩放 0.5表示缩小一倍 默认1.0
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param threshold  {number?} 匹配度 0~1 默认0.7
 * @param nfeatures  {number?} 选择特征点数  默认值 10000个 如果找不到目标可以试着把这个值设的更高 比如100000
 * @param scaleFactor {number?} 图像金字塔每层缩放倍数 默认值 1.2 必须大于1
 * @param nlevels  {number?} 图像金字塔组数 默认值 8  可以简单的理解为 nlevels*scaleFactor 越大 适应的分辨率越宽
 * @returns Point数组或nil
 */
func FindImgByORB(mat, temp *Mat, imgResize, tempResize float64, sx, sy, ex, ey int, threshold float64, nfeatures int, scaleFactor float64, nlevels int) []Point {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cImgResize := C.double(imgResize)
	cTempResize := C.double(tempResize)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cThreshold := C.double(threshold)
	cNFeatures := C.int(nfeatures)
	cScaleFactor := C.double(scaleFactor)
	cNLevels := C.int(nlevels)

	cResult := C.FindImgByORB(mat.ptr, temp.ptr,
		cImgResize, cTempResize,
		cSx, cSy, cEx, cEy,
		cThreshold, cNFeatures,
		cScaleFactor, cNLevels)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// FindImgByORB0 带默认参数的 ORB 算法全分辨率找图函数
// 适配 ORB 算法高速特性，默认参数对齐函数注释，简化小图尺寸限制与特征配置
func FindImgByORB0(args ...any) []Point {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},      // 大图默认用全局 defMat
		{"temp", defMat},     // 目标小图（无默认值，必传，需满足长宽≥40 像素）
		{"imgResize", 1.0},   // 大图缩放倍数默认 1.0（全分辨率默认不缩放）
		{"tempResize", 1.0},  // 小图缩放倍数默认 1.0（默认不缩放，避免尺寸不足）
		{"sx", 0},            // 查找起点 x 默认 0
		{"sy", 0},            // 查找起点 y 默认 0
		{"ex", 0},            // 查找终点 x 默认 0
		{"ey", 0},            // 查找终点 y 默认 0
		{"threshold", 0.7},   // 匹配度默认 0.7（ORB 算法通用阈值，0~1 范围）
		{"nfeatures", 10000}, // 特征点数默认 10000（平衡速度与召回率）
		{"scaleFactor", 1.2}, // 金字塔缩放倍数默认 1.2（严格 > 1，符合算法要求）
		{"nlevels", 8},       // 金字塔组数默认 8（适配常规分辨率范围）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含 ORB 算法特殊参数校验）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始 ORB 全分辨率找图函数
	return FindImgByORB(
		paramMetas[0].def.(*Mat),     // 大图 mat
		paramMetas[1].def.(*Mat),     // 目标小图 temp（需确保长宽≥40 像素）
		paramMetas[2].def.(float64),  // 大图缩放倍数 imgResize
		paramMetas[3].def.(float64),  // 小图缩放倍数 tempResize
		paramMetas[4].def.(int),      // 起点 x
		paramMetas[5].def.(int),      // 起点 y
		paramMetas[6].def.(int),      // 终点 x
		paramMetas[7].def.(int),      // 终点 y
		paramMetas[8].def.(float64),  // 匹配度 threshold
		paramMetas[9].def.(int),      // 特征点数 nfeatures
		paramMetas[10].def.(float64), // 金字塔缩放倍数 scaleFactor
		paramMetas[11].def.(int),     // 金字塔组数 nlevels
	)
}

/**
 *  全分辨率形状匹配
 * @param img  {Mat} Mat图片
 * @param dataInfo {DataInfo}   由图色工具生成的属性对象
 * @param threshold  {number?} 相识度 默认0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count  {number?} 寻找数量 默认1个
 * @param minArea {number?} 最小面积,指轮廓围成的面积,默认200
 * @param maxArea {number?} 最大面积,指轮廓围成的面积,默认-1 表示不限制
 * @returns Match数组或nil
 */
func MatchShape(mat *Mat, dataInfo DataInfo, threshold float64, sx, sy, ex, ey, count, minArea, maxArea int) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cColors := C.CString(dataInfo.Colors)
	defer C.free(unsafe.Pointer(cColors))
	cImageMode := C.CString(dataInfo.ImageMode)
	defer C.free(unsafe.Pointer(cImageMode))
	cHash := C.CString(dataInfo.Hash)
	defer C.free(unsafe.Pointer(cHash))
	cInfo := NewDoubleVector()
	defer cInfo.Delete()
	for _, value := range dataInfo.Info {
		cInfo.Add(value)
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cMinArea := C.double(minArea)
	cMaxArea := C.double(maxArea)
	cResult := C.MatchShape(mat.ptr, cColors, cImageMode, cInfo.ptr, cHash, cThreshold, cSx, cSy, cEx, cEy, cCount, cMinArea, cMaxArea)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// MatchShape0 带默认参数的全分辨率形状匹配函数
// 适配图色工具生成的 DataInfo，默认参数对齐函数注释，简化形状匹配调用
func MatchShape0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},          // 大图默认用全局 defMat
		{"dataInfo", DataInfo{}}, // 图色工具生成的属性对象（无默认值，必传）
		{"threshold", 0.9},       // 相似度默认 0.9（0~1 范围）
		{"sx", 0},                // 查找起点 x 默认 0
		{"sy", 0},                // 查找起点 y 默认 0
		{"ex", 0},                // 查找终点 x 默认 0
		{"ey", 0},                // 查找终点 y 默认 0
		{"count", 1},             // 寻找数量默认 1 个
		{"minArea", 200},         // 最小轮廓面积默认 200
		{"maxArea", -1},          // 最大轮廓面积默认 - 1（不限制）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含 DataInfo 特殊类型校验）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始全分辨率形状匹配函数
	return MatchShape(
		paramMetas[0].def.(*Mat),     // 大图 mat
		paramMetas[1].def.(DataInfo), // 图色工具属性对象 dataInfo
		paramMetas[2].def.(float64),  // 相似度 threshold
		paramMetas[3].def.(int),      // 起点 x
		paramMetas[4].def.(int),      // 起点 y
		paramMetas[5].def.(int),      // 终点 x
		paramMetas[6].def.(int),      // 终点 y
		paramMetas[7].def.(int),      // 寻找数量 count
		paramMetas[8].def.(int),      // 最小轮廓面积 minArea
		paramMetas[9].def.(int),      // 最大轮廓面积 maxArea
	)
}

/**
 * 全分辨率模板匹配
 * @param img {Mat} Mat大图
 * @param temp   {Mat} 小图
 * @param oWidth  {number} 制作小图时的屏幕宽度
 * @param oHeight {number} 制作小图时的屏幕高度
 * @param threshold {number?} 匹配度 默认0.9;
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 寻找个数 默认 1; 注:返回的都是相同大小的目标
 * @param scalePro {number?} 每次缩放比例, 默认0.05,即百分之五,程序根据分辨率自动放大或缩小,如果为负数将执行相反的过程;
 * @returns Match数组或nil
 */
func FindImgByPyramid(mat, temp *Mat, oWidth, oHeight int, threshold float64, sx, sy, ex, ey, count int, scalePro float64) []Match {
	if mat.Empty() || temp.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	width, height, _, _ := device.GetDisplayInfo(0)
	num1 := int(math.Min(float64(width), float64(height)))
	num2 := int(math.Min(float64(oWidth), float64(oHeight)))

	if num1 < num2 {
		scalePro *= -1
		temp := num1
		num1 = num2
		num2 = temp
	}
	scaleTimes := int(math.Abs(math.Ceil((float64(num1-num2))/(float64(num1)*scalePro)))) + 1

	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cMatchMethod := C.int(3)
	cWeakThreshold := C.double(threshold - 0.1)
	cStrictThreshold := C.double(threshold)
	cCount := C.int(count)
	cScaleTimes := C.int(scaleTimes)
	cScalePro := C.double(scalePro)
	cResult := C.FindImgByPyramid(mat.ptr, temp.ptr,
		cSx, cSy, cEx, cEy,
		cMatchMethod, cWeakThreshold, cStrictThreshold,
		cCount, cScaleTimes, cScalePro)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))

}

// FindImgByPyramid0 带默认参数的全分辨率模板匹配函数
// 适配多分辨率场景，默认参数对齐函数注释，简化金字塔缩放匹配调用
func FindImgByPyramid0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},    // 大图默认用全局defMat
		{"temp", defMat},   // 目标小图（无默认值，必传）
		{"oWidth", 0},      // 制作小图时的屏幕宽度（无默认值，必传）
		{"oHeight", 0},     // 制作小图时的屏幕高度（无默认值，必传）
		{"threshold", 0.9}, // 匹配度默认0.9（0~1范围）
		{"sx", 0},          // 查找起点x默认0
		{"sy", 0},          // 查找起点y默认0
		{"ex", 0},          // 查找终点x默认0
		{"ey", 0},          // 查找终点y默认0
		{"count", 1},       // 寻找个数默认1（返回相同大小目标）
		{"scalePro", 0.05}, // 每次缩放比例默认0.05（5%）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含必传参数校验）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始全分辨率模板匹配函数
	return FindImgByPyramid(
		paramMetas[0].def.(*Mat),     // 大图mat
		paramMetas[1].def.(*Mat),     // 目标小图temp
		paramMetas[2].def.(int),      // 制作小图时的屏幕宽度oWidth
		paramMetas[3].def.(int),      // 制作小图时的屏幕高度oHeight
		paramMetas[4].def.(float64),  // 匹配度threshold
		paramMetas[5].def.(int),      // 起点x
		paramMetas[6].def.(int),      // 起点y
		paramMetas[7].def.(int),      // 终点x
		paramMetas[8].def.(int),      // 终点y
		paramMetas[9].def.(int),      // 寻找个数count
		paramMetas[10].def.(float64), // 每次缩放比例scalePro
	)
}

/**
 * 判断 findImgByColor 是否已经提取过 tempPath 的图片颜色
 * @param tempPath {String} 模板图片路径 例如 "myImg.jpg", "drawable/myImg.png"
 * @returns {String[]} 返回nil 或者 字符串数组 [color1,color2]
 */
func GetColorMap(tempPath string) []string {
	cPath := C.CString(tempPath)
	defer C.free(unsafe.Pointer(cPath))
	cResult := C.GetColorMap(cPath)
	defer C.DeleteCharArr(cResult)
	if cResult == nil {
		return nil
	}
	result := C.GoString(cResult)
	if result == "" {
		return nil
	}
	return strings.Split(result, "&")
}

/** 全分辨率多点找色<br/>
 *  使用精度最高的siftEx与多点找色相结合,同时也解决了色差问题 <br/>
 *  原理:先进行找图,然后对找到的目标进行颜色提取,如果目标一直未出现,这个方法也只是个普通的siftEx找图,
 *  如果出现过一次,之后使用这个方法将转化为多点找色(自动判断是否提取过颜色)<br/>
 *
 *  如果不确定 目标是否会出现,可以使用其他较快的函数先行确定目标是否已经出现.<br/>
 *
 *  可以使用GetColorMap函数确定模板是否已经提取过颜色.<br/>
 *
 * @param img {Mat} 大图
 * @param tempPath {string} 模板图路径 例如 "myImg.jpg", "drawable/myImg.png"
 * @param color    {string?} 第一个点的颜色,同多点找色.如果为""将由程序自动筛选,返回的尽可能为中心的坐标,如需精确的请手动填写
 * @param colorMulti {string?} 其他颜色点  同多点找色  与color只要有一个为"" 将由程序自动筛选
 * @param threshold {number?} 相似度 0~1.0,默认 0.9
 * @param sx {number?} 起点x 默认0;
 * @param sy {number?} 起点y 默认0;
 * @param ex {number?} 终点x 默认0;
 * @param ey {number?} 终点y 默认0;
 * @param count {number?} 保留变量,以后可能用到,现在不起作用
 * @param direction {number?} 寻找方向 同多点找色
 * @returns {PColor} []PColor数组 或 nil <br/>
 *          PColor[0].X  坐标x <br/>
 *          PColor[0].y  坐标y <br/>
 *          PColor[0].Color1  第一个颜色字符串  亦可用于普通多点找色<br/>
 *          PColor[0].Color2  其他颜色字符串    亦可用于普通多点找色<br/>
 */
func FindImgByColor(mat *Mat, tempPath, color, colorMulti string, threshold float64, sx, sy, ex, ey, count, direction int) []PColor {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cTempPath := C.CString(tempPath)
	defer C.free(unsafe.Pointer(cTempPath))
	cColors := C.CString(color)
	defer C.free(unsafe.Pointer(cColors))
	cColorMulti := C.CString(colorMulti)
	defer C.free(unsafe.Pointer(cColorMulti))
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cDirection := C.int(direction)
	cCount := C.int(count)
	cResult := C.FindImgColor(mat.ptr, cTempPath, cThreshold, cSx, cSy, cEx, cEy, cCount, cDirection)
	defer C.DeleteCharArr(cResult)
	pColors := jsonToPColors(C.GoString(cResult))
	if len(pColors) != 0 && pColors[0].HasTarget {
		fmt.Println("执行4")
		return pColors
	} else {
		fmt.Println("执行5")
		temp := ReadResMat(tempPath)
		defer temp.Recycle()
		if temp.Empty() {
			fmt.Println("小图为空")
			return nil
		}
		cSrcMultiple := C.double(1.0)   //原图缩放比例
		cTempMultiple := C.double(1.0)  //模板缩放比例
		cDistanceRatio := C.double(0.6) //距离比值
		cCustomFeatures := C.int(6)     //特征点阈值

		cFinalResult := C.FindImgColor2(mat.ptr, cSrcMultiple, temp.ptr, cTempMultiple, cDistanceRatio, cCustomFeatures,
			cTempPath, cColors, cColorMulti, cThreshold, cSx, cSy, cEx, cEy, cCount, cDirection)
		defer C.DeleteCharArr(cFinalResult)
		pColors := jsonToPColors(C.GoString(cFinalResult))
		if len(pColors) != 0 && pColors[0].HasTarget {
			return pColors
		}
	}
	return nil
}

// FindImgByColor0 带默认参数的全分辨率多点找色函数
// 适配 SIFT-Ex 与多点找色结合逻辑，默认参数对齐函数注释，简化模板路径与颜色配置
func FindImgByColor0(args ...any) []PColor {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},    // 大图默认用全局 defMat
		{"tempPath", ""},   // 模板图路径（无默认值，必传）
		{"color", ""},      // 第一个点颜色（默认空，程序自动筛选）
		{"colorMulti", ""}, // 其他颜色点（默认空，程序自动筛选）
		{"threshold", 0.9}, // 相似度默认 0.9（0~1 范围）
		{"sx", 0},          // 查找起点 x 默认 0
		{"sy", 0},          // 查找起点 y 默认 0
		{"ex", 0},          // 查找终点 x 默认 0
		{"ey", 0},          // 查找终点 y 默认 0
		{"count", 0},       // 保留变量默认 0（当前不起作用）
		{"direction", 1},   // 寻找方向默认 1（同多点找色默认值）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含必传参数校验）
	if !checkParam(&paramMetas, 2, args...) {
		return nil
	}

	// 调用原始全分辨率多点找色函数
	return FindImgByColor(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(string),  // 模板图路径 tempPath
		paramMetas[2].def.(string),  // 第一个点颜色 color
		paramMetas[3].def.(string),  // 其他颜色点 colorMulti
		paramMetas[4].def.(float64), // 相似度 threshold
		paramMetas[5].def.(int),     // 起点 x
		paramMetas[6].def.(int),     // 起点 y
		paramMetas[7].def.(int),     // 终点 x
		paramMetas[8].def.(int),     // 终点 y
		paramMetas[9].def.(int),     // 保留变量 count
		paramMetas[10].def.(int),    // 寻找方向 direction
	)
}

// 1. globToRegex：将通配符模式（如 "*.jpg"）转换为 Go 正则表达式对象
// 参数 pattern：通配符模式（支持 . 匹配原义点、* 匹配任意字符）
// 返回值：编译后的正则表达式对象，若转换失败返回 error
func globToRegex(pattern string) (*regexp.Regexp, error) {
	// 1. 转义原义字符：. 在正则中是特殊字符，需替换为 \.
	escaped := strings.ReplaceAll(pattern, ".", `\.`)
	// 2. 替换通配符：* 在正则中对应 .*（匹配任意字符任意次数）
	regexStr := strings.ReplaceAll(escaped, "*", ".*")
	// 3. 包裹正则边界：^ 匹配开头、$ 匹配结尾（确保全量匹配，避免部分匹配）
	regexStr = "^" + regexStr + "$"
	// 4. 编译正则表达式（返回 error 便于外部处理失败场景）
	return regexp.Compile(regexStr)
}

// 2. matchesPattern：检查文件名是否匹配通配符模式
// 参数 filename：待检查的文件名（如 "image.jpg"）
// 参数 pattern：通配符模式（如 "*.jpg"、"image.*"）
// 返回值：true 表示匹配，false 表示不匹配；若正则编译失败也返回 false（可根据需求调整为返回 error）
func matchesPattern(filename, pattern string) bool {
	// 无通配符时，直接精确匹配（比正则更高效）
	if !strings.Contains(pattern, "*") {
		return filename == pattern
	}
	// 有通配符时，转换为正则并匹配
	re, err := globToRegex(pattern)
	if err != nil {
		return false // 正则编译失败（如非法模式），视为不匹配
	}
	return re.MatchString(filename)
}

// 获取路径下文件的完整路径(包含文件名)
func findFileNames(path, extension string) []string {

	pathArr := make([]string, 0)
	paths := files.ListDir(path)
	for _, tempPath := range paths {
		if strings.HasSuffix(tempPath, extension) {
			pathArr = append(pathArr, tempPath)
		}
	}
	return pathArr
}

func YamlExist(names []string) []bool {
	result := make([]bool, len(names))
	cNames := NewStringVector()
	defer cNames.Delete()
	for _, name := range names {
		cNames.Add(name)
	}
	cBools := C.YamlExist(cNames.ptr)
	defer C.DeleteCharArr(cBools)
	if cBools != nil {
		goStr := C.GoString(cBools)
		for i, c := range goStr {
			result[i] = c == '1'
		}
	}
	return result
}

/**
 * 加载yaml 文件
 * @param names {string?} yaml文件名称允许使用通配符“*” 多个用"|"隔开  例如 "yamlOne|yaml*|weixin",如果为 空 或不写 则加载path下所有 yaml文件;
 * @param path {string?} 路径  可以是外部的 也可以是内部脚本目录下  例如 "/storage/emulated/0/Pictures" , "drawable"; 默认为 ""(脚本同目录);
 * @return {string} 返回yaml文件名 例 yamlOne|yamlTwo|weixin
 */
func LoadYaml(names, path string) string {
	if path == "" {
		path = getResPath()
	}
	newNames := make([]string, 0)
	yamlArr := make([]string, 0)
	if names == "" { //读取全部

		if !files.Exists(path) {
			path = getResPath() + "/" + path
			if !files.Exists(path) {
				fmt.Println("文件路径错误!")
				return ""
			}
		}
		yamlPathList := findFileNames(path, ".yaml")
		for _, file := range yamlPathList {
			name := files.GetNameWithoutExtension(file)
			newNames = append(newNames, name)
		}

		boolArr := YamlExist(newNames)
		for i, b := range boolArr {
			if b {
				yamlArr = append(yamlArr, "")
			} else {
				yamlArr = append(yamlArr, files.Read(yamlPathList[i]))
			}
		}

	} else {

		finalNames := make([]string, 0)

		if !files.Exists(path) {
			path = getResPath() + "/" + path
			if !files.Exists(path) {
				fmt.Println("文件路径错误!")
				return ""
			}
		}

		if strings.Contains(names, "*") {
			tempNames := strings.Split(names, "|")
			namesTemp := make([]string, 0)
			fileList := findFileNames(path, ".yaml") //保存完整路径
			_fileNames := make([]string, 0)
			for _, file := range fileList {
				name := files.GetNameWithoutExtension(file)
				_fileNames = append(_fileNames, name)
			}
			for _, pattern := range tempNames {
				if strings.Contains(pattern, "*") {
					for _, fileName := range _fileNames {
						if matchesPattern(fileName, pattern) {
							namesTemp = append(namesTemp, fileName)
						}
					}
				} else {
					namesTemp = append(namesTemp, pattern)
				}
			}

			sort.Strings(namesTemp)
			for i := 0; i < len(namesTemp); i++ {
				// 条件：i == 0（保留第一个元素），或者当前元素与前一个元素长度不同，或者内容不同
				if i == 0 || len(namesTemp[i]) != len(namesTemp[i-1]) || namesTemp[i] != namesTemp[i-1] {
					// 符合条件则添加到结果切片
					finalNames = append(finalNames, namesTemp[i])
				}
			}

		} else {
			finalNames = strings.Split(names, "|")
		}

		boolArr := YamlExist(finalNames)
		for i, b := range boolArr {
			if b {
				newNames = append(newNames, finalNames[i])
				yamlArr = append(yamlArr, "")
			} else {
				newPath := path + "/" + finalNames[i] + ".yaml"
				if files.Exists(newPath) {
					str := files.Read(newPath)
					newNames = append(newNames, finalNames[i])
					yamlArr = append(yamlArr, str)

				} else {
					fmt.Println(finalNames[i] + ".yaml 文件不存在,自动忽略")
				}

			}
		}

	}

	namesVec := NewStringVector()
	defer namesVec.Delete()
	for _, name := range newNames {
		namesVec.Add(name)
	}
	yamlVec := NewStringVector()
	defer yamlVec.Delete()
	for _, yaml := range yamlArr {
		yamlVec.Add(yaml)
	}
	C.LoadYaml(namesVec.ptr, yamlVec.ptr)
	return strings.Join(newNames, "|")
}

func LoadYaml0(args ...any) string {
	paramMetas := []paramMeta{
		{"names", ""}, // 大图默认用全局 defMat
		{"path", ""},
	}
	if !checkParam(&paramMetas, 0, args...) {
		return ""
	}
	return LoadYaml(
		paramMetas[0].def.(string), // 大图 mat
		paramMetas[1].def.(string), // 模板图路径 tempPath
	)
}

/**
 * 释放 yaml资源
 * @param names {string?} yaml文件名称 多个用"|"隔开  例如 "yamlOne|yamlTow|weixin"; 不写则释放全部
 * @returns  释放成功或失败
 */
func ReleaseYaml(names ...any) bool {
	nameVec := NewStringVector()
	defer nameVec.Delete()

	for _, name := range names {
		if name.(string) != "" {
			nameArr := strings.Split(name.(string), "|")
			for _, name := range nameArr {
				nameVec.Add(name)
			}
		}
	}

	return bool(C.ReleaseYaml(nameVec.ptr))
}

/**
 *  梯度特征匹配   可全分,可测角度,无视色差,可返回多个不同目标
 * @param img {Mat} Mat图片;
 * @param names {string} yaml文件名称 多个用"|"隔开  例如 "yamlOne|yamlTow|weixin",如果不写 则查找assets下所有yaml目标(不包含子目录);
 * @param sx  {number?} 起点x 默认0;
 * @param sy  {number?} 起点y 默认0;
 * @param ex  {number?} 终点x 默认0;
 * @param ey  {number?} 终点y 默认0;
 * @param threshold {number?} 阈值 0~1,默认 0.9;
 * @param count {number?} 寻找个数,默认 1;
 * @param step   {number[]?}  金字塔梯度 默认为[4,8],可选值 [2,4] ,[4,8];
 * @param NMSThreshold {number?} 非极大值抑制阈值 默认0.3; 即两个目标重叠时 面积交集/并集>阈值 将被舍去
 * @param mask {Mat?} 大图的掩膜 默认nil;
 * @return  Match数组或nil
 */
func GradientShapeMatch(mat *Mat, names string, sx, sy, ex, ey int, threshold float64, count int, step []int, NMSThreshold float64, mask *Mat) []Match {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	nameList := make([]string, 0)
	filePathList := make([]string, 0)
	yamlArr := make([]string, 0)

	if names == "" {
		//一次性读取yaml文件, 如果不想全部读取 可以小图放进多个文件夹里
		path := getResPath()
		res := findFileNames(path, ".yaml")
		for _, value := range res {
			filePathList = append(filePathList, value)
			nameList = append(nameList, files.GetNameWithoutExtension(value))
		}
	} else {
		path := getResPath()
		nameList = strings.Split(names, "|")
		for _, name := range nameList {
			filePathList = append(filePathList, path+"/"+name+".yaml")
		}
	}
	if len(nameList) == 0 {
		return nil
	}
	boolArr := YamlExist(nameList)

	for i := 0; i < len(boolArr); i++ {
		if boolArr[i] {
			// 如果为true，添加空字符串
			yamlArr = append(yamlArr, "")
		} else {
			// 读取文件内容
			yamlStr := files.Read(filePathList[i])
			// 处理文件读取结果
			if len(yamlStr) == 0 {
				// 检查是否包含通配符
				if strings.Contains(filePathList[i], "*") {
					fmt.Println("存在通配符名称：" + filePathList[i] + ",请使用支持通配符的loadYaml方法提前加载，当前自动忽略")
				} else {
					fmt.Println("不存在文件" + filePathList[i] + ",自动忽略")
				}
				// 移除对应的名称（注意Go中切片删除的正确方式）
				if i < len(nameList) {
					nameList = append(nameList[:i], nameList[i+1:]...)
				}
			} else {
				// 添加文件内容
				yamlArr = append(yamlArr, yamlStr)
			}
		}
	}

	cNames := NewStringVector()
	defer cNames.Delete()
	for _, name := range nameList {
		cNames.Add(name)
	}

	cYamlArr := NewStringVector()
	defer cYamlArr.Delete()
	for _, yaml := range yamlArr {
		cYamlArr.Add(yaml)
	}
	cThreshold := C.double(threshold)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cCount := C.int(count)
	cNMSThreshold := C.double(NMSThreshold)
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (mat.GetHeight() != mask.GetHeight() || mat.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与图1大小必须一致！")
		return nil
	}

	if step == nil {
		step = []int{4, 8}
	}
	cStep := NewIntVector()
	defer cStep.Delete()
	for _, t := range step {
		cStep.Add(t)
	}
	cResult := C.GradientShapeMatch(mat.ptr, cNames.ptr, cYamlArr.ptr, cThreshold, cSx, cSy, cEx, cEy, cCount, cStep.ptr, cNMSThreshold, mask.ptr)
	defer C.DeleteCharArr(cResult)
	return jsonToMatches(C.GoString(cResult))
}

// GradientShapeMatch0 带默认参数的梯度特征匹配函数
// 适配全分辨率 / 角度检测 / 无色差场景，默认参数对齐函数注释，简化 YAML 配置与多目标匹配调用
func GradientShapeMatch0(args ...any) []Match {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按原始参数顺序排列）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 大图默认用全局 defMat
		{"names", ""},         // YAML 文件名（默认空，查找 assets 下所有 YAML 目标）
		{"sx", 0},             // 查找起点 x 默认 0
		{"sy", 0},             // 查找起点 y 默认 0
		{"ex", 0},             // 查找终点 x 默认 0
		{"ey", 0},             // 查找终点 y 默认 0
		{"threshold", 0.9},    // 匹配阈值默认 0.9（0~1 范围）
		{"count", 1},          // 寻找个数默认 1
		{"step", []int{4, 8}}, // 金字塔梯度默认 [4,8]（可选 [2,4]）
		{"NMSThreshold", 0.3}, // 非极大值抑制阈值默认 0.3
		{"mask", defMat},      // 大图掩膜默认 nil（无掩膜匹配）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含切片 / 结构体类型校验）
	if !checkParam(&paramMetas, 1, args...) {
		return nil
	}

	if paramMetas[len(paramMetas)-1].def.(*Mat).ptr == 0 {
		mask := NewMat()
		defer mask.Recycle()
		paramMetas[len(paramMetas)-1].def = mask
	}

	// 调用原始梯度特征匹配函数
	return GradientShapeMatch(
		paramMetas[0].def.(*Mat),    // 大图 mat
		paramMetas[1].def.(string),  // YAML 文件名 names
		paramMetas[2].def.(int),     // 起点 x
		paramMetas[3].def.(int),     // 起点 y
		paramMetas[4].def.(int),     // 终点 x
		paramMetas[5].def.(int),     // 终点 y
		paramMetas[6].def.(float64), // 匹配阈值 threshold
		paramMetas[7].def.(int),     // 寻找个数 count
		paramMetas[8].def.([]int),   // 金字塔梯度 step
		paramMetas[9].def.(float64), // 非极大值抑制阈值 NMSThreshold
		paramMetas[10].def.(*Mat),   // 大图掩膜 mask
	)
}

/**
 * 返回指定点的颜色字符串大写
 * @param mat {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns
 */
func GetPixelColor(mat *Mat, x, y int) string {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return ""
	}
	cX := C.int(x)
	cY := C.int(y)
	cResult := C.GetPixelColor(mat.ptr, cX, cY)
	defer C.DeleteCharArr(cResult)
	return C.GoString(cResult)
}

/**
 * 获取指定点的 Red 通道颜色
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns {number} 0~255 范围的颜色值
 */
func GetRed(mat *Mat, x, y int) int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return 0
	}
	cX := C.int(x)
	cY := C.int(y)
	cResult := C.GetRed(mat.ptr, cX, cY)
	return int(cResult)
}

/**
 * 获取指定点的 Green 通道颜色
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns {number} 0~255 范围的颜色值
 */
func GetGreen(mat *Mat, x, y int) int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return 0
	}
	cX := C.int(x)
	cY := C.int(y)
	cResult := C.GetGreen(mat.ptr, cX, cY)
	return int(cResult)

}

/**
 * 获取指定点的 Blue 通道颜色
 * @param img {Mat} 图片
 * @param x {number} 坐标 x
 * @param y {number} 坐标 y
 * @returns {number} 0~255 范围的颜色值
 */
func GetBlue(mat *Mat, x, y int) int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return 0
	}
	cX := C.int(x)
	cY := C.int(y)
	cResult := C.GetBlue(mat.ptr, cX, cY)
	return int(cResult)

}

/** 图片伸缩
 * @param mat  {Mat} mat原图
 * @param newWidth {number} 新宽度
 * @param newHeight {number} 新高度
 * @returns {Mat}
 */
func ScaleMatByWH(mat *Mat, width, height int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}

	cWidth := C.int(width)
	cHeight := C.int(height)
	C.ScaleMatByWH(mat.ptr, resultMat.ptr, cWidth, cHeight)
	return resultMat
}

/** 图片伸缩
 * @param mat {Mat} mat原图
 * @param ratioX  {number} X轴伸缩比例 如0.5 1.3 等
 * @param ratioY {number} Y轴伸缩比例
 * @returns {Mat}
 */
func ScaleMatByRatio(mat *Mat, ratioX, ratioY float64) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cRatioX := C.double(ratioX)
	cRatioY := C.double(ratioY)
	C.ScaleMatByRatio(mat.ptr, resultMat.ptr, cRatioX, cRatioY)
	return resultMat
}

/** 图片旋转
 * 需要自己管理图片的释放
 * @param mat {Mat} mat原图
 * @param alpha  {number} 角度 如 90  旋转90度
 * @returns {Mat}
 */
func RotateMat(mat *Mat, alpha int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cAlpha := C.int(alpha)
	C.RotateMat(mat.ptr, resultMat.ptr, cAlpha)
	return resultMat
}

/** 图片剪切
 * @param mat {Mat} mat原图
 * @param sx  {number} 起点x
 * @param sy  {number} 起点y
 * @param ex  {number} 终点x
 * @param ey  {number} 终点y
 * @returns {Mat}
 */
func CropMat(mat *Mat, sx, sy, ex, ey int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	C.CropMat(mat.ptr, resultMat.ptr, cSx, cSy, cEx, cEy)
	return resultMat
}

/**
 * 图片叠加
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2
 * @param x {number?} 起点x 默认0;
 * @param y {number?} 起点y 默认0;
 * @param alpha {number?} 图片2 的透明度,范围 0~1,默认1 不透明
 * @param mask {Mat?} 与图片2一样大小的二值化图片,默认nil
 * @returns
 */
func OverlyingMat(mat1, mat2 *Mat, sx, sy int, alpha float64, mask *Mat) *Mat {
	resultMat := NewMat()

	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}

	cSx := C.int(sx)
	cSy := C.int(sy)
	cAlpha := C.double(alpha)
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	C.OverlyingMat(mat1.ptr, mat2.ptr, resultMat.ptr, cSx, cSy, cAlpha, mask.ptr)
	return resultMat
}

// OverlyingMat0 带默认参数的图片叠加函数
// 简化图片叠加调用，默认参数对齐函数注释，适配透明度与掩膜配置
func OverlyingMat0(args ...any) *Mat {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按逻辑参数顺序排列）
	// 注：原始函数声明参数顺序存在笔误（缺少 img2），此处按 "图片 1 + 图片 2 + 位置 + 透明度 + 掩膜" 逻辑修正参数顺序
	paramMetas := []paramMeta{
		{"img1", defMat}, // 底层图片 1（无默认值，必传）
		{"img2", defMat}, // 叠加图片 2（无默认值，必传）
		{"x", 0},         // 叠加起点 x 默认 0
		{"y", 0},         // 叠加起点 y 默认 0
		{"alpha", 1.0},   // 图片 2 透明度默认 1.0（不透明，0~1 范围）
		{"mask", defMat}, // 图片 2 的二值化掩膜默认 nil（无掩膜）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含必传图片校验）
	if !checkParam(&paramMetas, 2, args...) {
		return NewMat() // 返回空 Mat 避免 nil 指针问题
	}

	if paramMetas[len(paramMetas)-1].def.(*Mat).ptr == 0 {
		mask := NewMat()
		defer mask.Recycle()
		paramMetas[len(paramMetas)-1].def = mask
	}

	// 调用原始图片叠加函数（注意：原始函数参数名与逻辑对齐，img1 对应 mat，img2 对应原始函数隐含的叠加源）
	// 修正说明：原始函数声明参数 "mat" 实际为底层图片 img1，叠加图片 img2 需通过内部逻辑传递（此处按项目实际实现适配）
	return OverlyingMat(
		paramMetas[0].def.(*Mat),    // 底层图片 1（对应原始函数 mat）
		paramMetas[1].def.(*Mat),    // 底层图片 2（对应原始函数 mat）
		paramMetas[2].def.(int),     // 叠加起点 x（对应原始函数 sx）
		paramMetas[3].def.(int),     // 叠加起点 y（对应原始函数 sy）
		paramMetas[4].def.(float64), // 图片 2 透明度（对应原始函数 alpha）
		paramMetas[5].def.(*Mat),    // 图片 2 的掩膜（对应原始函数 mask）
	)
}

/**
 * 画矩形
 * @param img {Mat} 图片
 * @param sx  {number} 起点x
 * @param sy  {number} 起点y
 * @param ex  {number} 终点x
 * @param ey  {number} 终点y
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 4 不抗锯齿
 * @returns  绘制结果图
 */
func DrawRect(mat *Mat, left, top, right, bottom int, color string, thickness, lineType int) *Mat {

	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cLeft := C.int(left)
	cTop := C.int(top)
	cRight := C.int(right)
	cBottom := C.int(bottom)
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	C.DrawRect(resultMat.ptr, cLeft, cTop, cRight, cBottom, cColor, cThickness, cLineType)
	return resultMat
}

// DrawRect0 带默认参数的画矩形函数
// 简化矩形绘制调用，默认参数对齐函数注释，适配颜色、画笔粗细与线条类型配置
func DrawRect0(args ...any) *Mat {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按逻辑优先级排列）
	paramMetas := []paramMeta{
		{"img", defMat},      // 待绘制图片（无默认值，必传）
		{"sx", 0},            // 起点 x（无默认值语义，按注释设默认 0，需用户确保坐标合法）
		{"sy", 0},            // 起点 y（同上）
		{"ex", 0},            // 终点 x（同上）
		{"ey", 0},            // 终点 y（同上）
		{"color", "#FF0000"}, // 矩形颜色默认红色（#RRGGBB 格式）
		{"thickness", 2},     // 画笔粗细默认 2（-1 表示填充）
		{"lineType", 4},      // 线条类型默认 4（不抗锯齿，可选 8/16）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含坐标 / 配置参数合法性校验）
	if !checkParam(&paramMetas, 5, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}

	// 调用原始画矩形函数
	return DrawRect(
		paramMetas[0].def.(*Mat),   // 待绘制图片 img
		paramMetas[1].def.(int),    // 起点 x（对应原始函数 left）
		paramMetas[2].def.(int),    // 起点 y（对应原始函数 top）
		paramMetas[3].def.(int),    // 终点 x（对应原始函数 right）
		paramMetas[4].def.(int),    // 终点 y（对应原始函数 bottom）
		paramMetas[5].def.(string), // 矩形颜色 color
		paramMetas[6].def.(int),    // 画笔粗细 thickness
		paramMetas[7].def.(int),    // 线条类型 lineType
	)
}

/**
 * 画矩形
 * @param img {Mat} 图片
 * @param rects  []Rect
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 4 不抗锯齿
 * @returns  结果图
 */
func DrawRects(mat *Mat, rects []Rect, color string, thickness, lineType int) *Mat {
	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	for _, rect := range rects {
		cLeft := C.int(rect.Left)
		cTop := C.int(rect.Top)
		cRight := C.int(rect.Right)
		cBottom := C.int(rect.Bottom)
		C.DrawRect(resultMat.ptr, cLeft, cTop, cRight, cBottom, cColor, cThickness, cLineType)
	}
	return resultMat
}

// DrawRects0 带默认参数的批量画矩形函数
// 简化多矩形绘制调用，默认参数对齐函数注释，适配颜色、画笔粗细与线条类型配置
func DrawRects0(args ...any) *Mat {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按逻辑优先级排列）
	paramMetas := []paramMeta{
		{"img", defMat},      // 待绘制图片（无默认值，必传）
		{"rects", []Rect{}},  // 矩形数组（无默认值语义，默认空数组）
		{"color", "#FF0000"}, // 矩形颜色默认红色（#RRGGBB格式）
		{"thickness", 2},     // 画笔粗细默认2（-1表示填充）
		{"lineType", 4},      // 线条类型默认4（不抗锯齿，可选8/16）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含矩形数组合法性校验）
	if !checkParam(&paramMetas, 2, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}

	// 调用原始批量画矩形函数
	return DrawRects(
		paramMetas[0].def.(*Mat),   // 待绘制图片img
		paramMetas[1].def.([]Rect), // 矩形数组rects
		paramMetas[2].def.(string), // 矩形颜色color
		paramMetas[3].def.(int),    // 画笔粗细thickness
		paramMetas[4].def.(int),    // 线条类型lineType
	)
}

/**
 * 画矩形
 * @param img {Mat} 图片
 * @param matches  []Match
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 4 不抗锯齿
 * @returns  结果图
 */
func DrawMatches(mat *Mat, matches []Match, color string, thickness, lineType int) *Mat {
	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	for _, match := range matches {
		cLeft := C.int(match.Left)
		cTop := C.int(match.Top)
		cRight := C.int(match.Right)
		cBottom := C.int(match.Bottom)
		C.DrawRect(resultMat.ptr, cLeft, cTop, cRight, cBottom, cColor, cThickness, cLineType)
	}
	return resultMat
}

// DrawMatches0 带默认参数的匹配结果矩形绘制函数
// 简化匹配结果可视化调用，默认参数对齐函数注释，适配颜色、画笔粗细与线条类型配置
func DrawMatches0(args ...any) *Mat {
	// 定义参数元信息及默认值（严格匹配函数注释规则，按逻辑优先级排列）
	paramMetas := []paramMeta{
		{"img", defMat},        // 待绘制图片（无默认值，必传）
		{"matches", []Match{}}, // 匹配结果数组（无默认值语义，默认空数组）
		{"color", "#FF0000"},   // 矩形颜色默认红色（#RRGGBB 格式）
		{"thickness", 2},       // 画笔粗细默认 2（-1 表示填充）
		{"lineType", 4},        // 线条类型默认 4（不抗锯齿，可选 8/16）
	}

	// 基础参数校验：数量不超限 + 类型精准匹配（含匹配结果数组合法性校验）
	if !checkParam(&paramMetas, 2, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}

	// 调用原始匹配结果绘制函数
	return DrawMatches(
		paramMetas[0].def.(*Mat),    // 待绘制图片 img
		paramMetas[1].def.([]Match), // 匹配结果数组 matches
		paramMetas[2].def.(string),  // 矩形颜色 color
		paramMetas[3].def.(int),     // 画笔粗细 thickness
		paramMetas[4].def.(int),     // 线条类型 lineType
	)
}

/**
 *   图片上画圆
 * @param img {Mat} 图片
 * @param x   {number}  圆心x
 * @param y    {number}  圆心y
 * @param r    {number} 半径r
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 16 抗锯齿
 * @returns  结果图
 **/
func DrawCircle(mat *Mat, x, y, r int, color string, thickness, lineType int) *Mat {
	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cX := C.int(x)
	cY := C.int(y)
	cR := C.int(r)
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	C.DrawCircle(resultMat.ptr, cX, cY, cR, cColor, cThickness, cLineType)
	return resultMat
}

// DrawCircle0 带默认参数的图片画圆函数
// 简化圆形绘制调用，默认参数严格对齐函数注释，适配抗锯齿、透明度与填充配置
func DrawCircle0(args ...any) *Mat {
	// 定义参数元信息及默认值（按函数注释逻辑优先级排列，核心参数前置）
	paramMetas := []paramMeta{
		{"img", defMat},      // 待绘制图片（无默认值，必传）
		{"x", 0},             // 圆心 x 坐标（无默认语义，设默认 0，需用户确保合法）
		{"y", 0},             // 圆心 y 坐标（同上）
		{"r", 1},             // 圆半径（默认 1，避免 0 半径无绘制效果）
		{"color", "#FF0000"}, // 圆颜色默认红色（#RRGGBB 格式）
		{"thickness", 2},     // 画笔粗细默认 2（-1 表示填充）
		{"lineType", 16},     // 线条类型默认 16（抗锯齿，符合注释优先配置）
	}

	// 基础参数校验：类型匹配 + 核心规则校验（保障绘制有效性）
	if !checkParam(&paramMetas, 4, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}

	// 调用原始画圆函数，按参数顺序精准传参
	return DrawCircle(
		paramMetas[0].def.(*Mat),   // 待绘制图片 img
		paramMetas[1].def.(int),    // 圆心 x 坐标
		paramMetas[2].def.(int),    // 圆心 y 坐标
		paramMetas[3].def.(int),    // 圆半径 r
		paramMetas[4].def.(string), // 圆颜色 color
		paramMetas[5].def.(int),    // 画笔粗细 thickness
		paramMetas[6].def.(int),    // 线条类型 lineType
	)
}

/**
 *   图片上画圆
 * @param img {Mat} 图片
 * @param points   {number}  PointEx数组
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2, -1表示填充
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 16 抗锯齿
 * @returns  结果图
 **/
func DrawCircles(mat *Mat, points []PointEx, color string, thickness, lineType int) *Mat {
	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	for _, point := range points {
		C.DrawCircle(resultMat.ptr, C.int(point.X), C.int(point.Y), C.int(point.R), cColor, cThickness, cLineType)
	}
	return resultMat
}

// DrawCircles0 带默认参数的图片画圆函数
// 简化圆形绘制调用，默认参数严格对齐函数注释，适配抗锯齿、透明度与填充配置
func DrawCircles0(args ...any) *Mat {
	// 定义参数元信息及默认值（按函数注释逻辑优先级排列，核心参数前置）
	paramMetas := []paramMeta{
		{"img", defMat}, // 待绘制图片（无默认值，必传）
		{"points", []PointEx{}},
		{"color", "#FF0000"}, // 圆颜色默认红色（#RRGGBB 格式）
		{"thickness", 2},     // 画笔粗细默认 2（-1 表示填充）
		{"lineType", 16},     // 线条类型默认 16（抗锯齿，符合注释优先配置）
	}
	// 基础参数校验：类型匹配 + 核心规则校验（保障绘制有效性）
	if !checkParam(&paramMetas, 2, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}
	// 调用原始画圆函数，按参数顺序精准传参
	return DrawCircles(
		paramMetas[0].def.(*Mat),      // 待绘制图片 img
		paramMetas[1].def.([]PointEx), // 圆心 x 坐标
		paramMetas[2].def.(string),    // 圆颜色 color
		paramMetas[3].def.(int),       // 画笔粗细 thickness
		paramMetas[4].def.(int),       // 线条类型 lineType
	)
}

/**
 * 画直线
 * @param img {Mat} 图片
 * @param sx  {number} 起点x
 * @param sy  {number} 起点y
 * @param ex  {number} 终点x
 * @param ey  {number} 终点y
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 16 抗锯齿
 */
func DrawLine(mat *Mat, sx, sy, ex, ey int, color string, thickness, lineType int) *Mat {
	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	C.DrawLines(resultMat.ptr, cSx, cSy, cEx, cEy, cColor, cThickness, cLineType)
	return resultMat
}

// DrawLine0 带默认参数的图片画直线函数
// 简化直线绘制调用，默认参数严格对齐函数注释，适配抗锯齿、线条粗细与颜色配置
func DrawLine0(args ...any) *Mat {
	// 定义参数元信息及默认值（按函数注释逻辑优先级排列，核心坐标参数前置）
	paramMetas := []paramMeta{
		{"img", defMat},      // 待绘制图片（无默认值，必传）
		{"sx", 0},            // 起点 x 坐标（无默认语义，设默认 0，需用户确保合法）
		{"sy", 0},            // 起点 y 坐标（同上）
		{"ex", 0},            // 终点 x 坐标（同上）
		{"ey", 0},            // 终点 y 坐标（同上）
		{"color", "#FF0000"}, // 线条颜色默认红色（#RRGGBB 格式）
		{"thickness", 2},     // 画笔粗细默认 2（正数有效，无填充逻辑）
		{"lineType", 16},     // 线条类型默认 16（抗锯齿，符合注释优先配置）
	}

	// 基础参数校验：类型匹配 + 核心规则校验（保障绘制有效性）
	if !checkParam(&paramMetas, 5, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}

	// 调用原始画直线函数，按参数顺序精准传参
	return DrawLine(
		paramMetas[0].def.(*Mat),   // 待绘制图片 img
		paramMetas[1].def.(int),    // 起点 x 坐标 sx
		paramMetas[2].def.(int),    // 起点 y 坐标 sy
		paramMetas[3].def.(int),    // 终点 x 坐标 ex
		paramMetas[4].def.(int),    // 终点 y 坐标 ey
		paramMetas[5].def.(string), // 线条颜色 color
		paramMetas[6].def.(int),    // 画笔粗细 thickness
		paramMetas[7].def.(int),    // 线条类型 lineType
	)
}

/**
 * 画多条连接的直线直线
 * @param mat {Mat} 图片
 * @param points  {[]Point} 点数组
 * @param color {string?} 颜色  默认 "#FF0000"
 * @param thickness {number?} 画笔粗细  默认 2
 * @param lineType {number?} 线条类型   4, 8, 16(抗锯齿),默认 16 抗锯齿
 * @param closed  是否首尾闭合(对3点以上而言) 默认 false
 */
func DrawLines(mat *Mat, points []Point, color string, thickness, lineType int, closed bool) *Mat {

	resultMat := CopyTo(mat, nil)
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	if len(points) < 2 {
		fmt.Println("点数必须大于等于2!")
		return resultMat
	}

	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	if closed {
		points = append(points, points[0])
	}
	psLen := len(points)
	for i := 1; i < psLen; i++ {
		cSx := C.int(points[i-1].X)
		cSy := C.int(points[i-1].Y)
		cEx := C.int(points[i].X)
		cEy := C.int(points[i].Y)
		C.DrawLines(resultMat.ptr, cSx, cSy, cEx, cEy, cColor, cThickness, cLineType)
	}
	return resultMat
}

// DrawLines0 带默认参数的多条连接直线绘制函数
// 简化折线 / 闭合图形绘制调用，默认参数严格对齐函数注释，适配首尾闭合、抗锯齿与线条配置
func DrawLines0(args ...any) *Mat {
	// 定义参数元信息及默认值（按函数注释逻辑优先级排列，核心参数前置）
	paramMetas := []paramMeta{
		{"mat", defMat},       // 待绘制图片（无默认值，必传）
		{"points", []Point{}}, // 顶点数组（无默认语义，默认空数组）
		{"color", "#FF0000"},  // 线条颜色默认红色（#RRGGBB 格式）
		{"thickness", 2},      // 画笔粗细默认 2（正数有效）
		{"lineType", 16},      // 线条类型默认 16（抗锯齿，符合注释优先配置）
		{"closed", false},     // 是否首尾闭合默认 false（仅对 3 点及以上有效）
	}

	// 基础参数校验：类型匹配 + 核心规则校验（保障绘制有效性）
	if !checkParam(&paramMetas, 2, args...) {
		return CopyTo(args[0].(*Mat), nil)
	}

	// 调用原始多条直线绘制函数
	return DrawLines(
		paramMetas[0].def.(*Mat),    // 待绘制图片 mat
		paramMetas[1].def.([]Point), // 顶点数组 points
		paramMetas[2].def.(string),  // 线条颜色 color
		paramMetas[3].def.(int),     // 画笔粗细 thickness
		paramMetas[4].def.(int),     // 线条类型 lineType
		paramMetas[5].def.(bool),    // 是否首尾闭合 closed
	)
}

/**
 * 统计非零的像素点个数
 * @param img  {Mat} 图片
 * @returns   非零像素点个数
 */
func CountNonZero(mat *Mat) int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return 0
	}
	return int(C.CountNonZero(mat.ptr))
}

/**
 *  图片镜像
 * @param img  {Mat} Mat大图
 * @param mode  镜像模式 1:左右镜像; 0:上下左右镜像;-1上下镜像
 * @returns {Mat} 镜像图片
 */
func Flip(mat *Mat, mode int) *Mat {

	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cMode := C.int(mode)
	C.Flip(mat.ptr, resultMat.ptr, cMode)
	return resultMat
}

/**
 * 感知哈希算法
 * @param mat {Mat} Mat图片
 * @param dctSize  {number?}  离散余弦变换大小,可以理解为缩放后的图片大小 用于DCT计算 默认32 即32*32大小 可以选值 8,16,32 值越小速度越快
 * @param ROISize  {number?}  对DCT的结果取值大小,默认8,即 8*8大小
 * @returns {string} 哈希值
 */
func PHashCode(mat *Mat, dctSize, ROISize int) string {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return ""
	}
	cDctSize := C.int(dctSize)
	cROISize := C.int(ROISize)
	cResult := C.PHashCode(mat.ptr, cDctSize, cROISize)
	defer C.DeleteCharArr(cResult)
	return C.GoString(cResult)
}

// 简化感知哈希计算调用，默认参数严格对齐函数注释，适配不同精度需求
func PHashCode0(args ...any) string {
	// 定义参数元信息及默认值（按函数注释逻辑优先级排列，核心图片参数前置）
	paramMetas := []paramMeta{
		{"mat", defMat}, // 待计算哈希的图片（无默认值，必传）
		{"dctSize", 32}, // 离散余弦变换大小默认 32（3232，可选 8/16/32）
		{"ROISize", 8},  // DCT 结果取值大小默认 8（88，需小于 dctSize）
	}

	// 基础参数校验：类型匹配 + 核心规则校验（保障哈希计算有效性）
	if !checkParam(&paramMetas, 1, args...) {
		return "" // 校验失败返回空哈希值
	}

	// 调用原始感知哈希算法函数
	return PHashCode(
		paramMetas[0].def.(*Mat), // 待计算图片 mat
		paramMetas[1].def.(int),  // 离散余弦变换大小 dctSize
		paramMetas[2].def.(int),  // DCT 结果取值大小 ROISize
	)
}

/**
 * 汉明距离算法
 * @param str1 {string} 字符串1
 * @param str2 {string} 字符串2
 * @returns {number}  数字越大代表差别越大
 */
func HanMingDistance(str1, str2 string) int {
	cStr1 := C.CString(str1)
	defer C.free(unsafe.Pointer(cStr1))
	cStr2 := C.CString(str2)
	defer C.free(unsafe.Pointer(cStr2))
	return int(C.HanMingDistance(cStr1, cStr2))
}

/**
 * 马赛克
 * @param srcMat {Mat} 图片
 * @param stride {number?} 马赛克大小 默认10;
 * @param sx  {number?} 起点x
 * @param sy  {number?} 起点y
 * @param ex  {number?} 终点x
 * @param ey  {number?} 终点y
 * @returns {Mat}
 */
func Masaike(mat *Mat, stride, sx, sy, ex, ey int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cStride := C.int(stride)
	cSx := C.int(sx)
	cSy := C.int(sy)
	cEx := C.int(ex)
	cEy := C.int(ey)
	C.Masaike(mat.ptr, resultMat.ptr, cStride, cSx, cSy, cEx, cEy)
	return resultMat
}

// 获取版本
func Version() string {
	cResult := C.Version()
	defer C.DeleteCharArr(cResult)
	return "Go版本：" + _version_ + " 插件版本:" + C.GoString(cResult)
}

/**
 * 转灰度
 * @param img  Mat图片
 * @param *Mat
 */
func MatToGray(mat *Mat) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.MatToGray(mat.ptr, resultMat.ptr)
	return resultMat
}

/*
*
  - 图片二值化 参考EC的二值化
  - @param mat {Mat} Mat原图
  - @param threshold_value {number} 二值化阈值，0 ~ 255
  - @param maxValue {number?} 大于阈值的点变成的数值 默认 255 白色;
  - @param type  一般写0即可 默认0<br/>
    0 灰度值大于阈值为最大值，其他值为0<br/>
    1 灰度值大于阈值为0，其他值为最大值<br/>
    2 灰度值大于阈值的为阈值，其他值不变<br/>
    3 灰度值大于阈值的不变，其他值为0<br/>
    4 灰度值大于阈值的为零，其他值不变<br/>
    8 大津法自动寻求全局阈值<br/>
    16 三角形法自动寻求全局阈值<br/>
  - @returns {Mat}
*/
func BinaryzationMat(mat *Mat, thresholdValue, maxValue, _type int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cThresholdValue := C.int(thresholdValue)
	cMaxValue := C.int(maxValue)
	cType := C.int(_type)
	C.BinaryzationMat(mat.ptr, resultMat.ptr, cThresholdValue, cMaxValue, cType)
	return resultMat
}

// BinaryzationMat0 带默认参数的图片二值化函数
// 简化二值化处理调用，默认参数严格对齐函数注释，适配不同阈值模式与输出配置
func BinaryzationMat0(args ...any) *Mat {
	// 定义参数元信息及默认值（按函数注释逻辑优先级排列，核心参数前置）
	paramMetas := []paramMeta{
		{"mat", defMat},          // 原图（无默认值，必传）
		{"threshold_value", 127}, // 二值化阈值默认128（0~255范围）
		{"maxValue", 255},        // 大于阈值的点的目标值默认255（白色）
		{"type", 0},              // 二值化类型默认0（常规模式：大于阈值为maxValue）
	}

	// 基础参数校验：类型匹配+核心规则校验（保障二值化有效性）
	if !checkParam(&paramMetas, 2, args...) {
		return NewMat() // 校验失败返回空Mat
	}

	// 调用原始二值化函数
	return BinaryzationMat(
		paramMetas[0].def.(*Mat), // 原图mat
		paramMetas[1].def.(int),  // 二值化阈值threshold_value
		paramMetas[2].def.(int),  // 目标最大值maxValue
		paramMetas[3].def.(int),  // 二值化类型type
	)
}

/**
 * 双阈值二值化
 * @param mat {Mat} Mat原图
 * @param lowerThreshold  {number} 低阈值，0 ~ 255
 * @param upperThreshold  {number} 高阈值，0 ~ 255
 * @param maxValue  {number?} 大于阈值的点变成的数值 默认 255 白色;
 * @returns {Mat}
 */
func DoubleBinaryzation(mat *Mat, lowerThreshold, upperThreshold, maxValue int) *Mat {
	// 创建结果矩阵并确保其指针已初始化
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	// 转换Go参数为C兼容类型
	cLowerThreshold := C.int(lowerThreshold)
	cUpperThreshold := C.int(upperThreshold)
	cMaxValue := C.int(maxValue)

	// 调用C函数执行双阈值化
	C.DoubleBinaryzation(mat.ptr, resultMat.ptr, cLowerThreshold, cUpperThreshold, cMaxValue)

	return resultMat
}

// DoubleBinaryzation0 带默认参数的双阈值二值化函数
// 简化双阈值处理调用，默认参数对齐函数注释，适配高低阈值与输出值配置
func DoubleBinaryzation0(args ...any) *Mat {
	// 定义参数元信息及默认值（核心参数：原图→低阈值→高阈值→最大值）
	paramMetas := []paramMeta{
		{"mat", defMat},         // 原图（无默认值，必传）
		{"lowerThreshold", 50},  // 低阈值默认 50（0~255，需小于高阈值）
		{"upperThreshold", 150}, // 高阈值默认 150（0~255，需大于低阈值）
		{"maxValue", 255},       // 大于高阈值的目标值默认 255（白色）
	}

	// 参数校验：类型 + 逻辑合法性（如高低阈值大小关系）
	if !checkParam(&paramMetas, 3, args...) {
		return NewMat() // 校验失败返回空 Mat
	}

	// 调用原始双阈值二值化函数
	return DoubleBinaryzation(
		paramMetas[0].def.(*Mat), // 原图 mat
		paramMetas[1].def.(int),  // 低阈值 lowerThreshold
		paramMetas[2].def.(int),  // 高阈值 upperThreshold
		paramMetas[3].def.(int),  // 目标最大值 maxValue
	)
}

/**
 * 自适应二值化
 * @param img {Mat} 图片
 * @param adaptiveMethod {number?} 阈值计算方式 选则值为 0或1, 0-通过平均的方法取得平均值做阈值,1-通过高斯取得高斯值做阈值,默认0
 * @param blockSize  {number?}  计算阈值的领域大小,此值对二值化的最终效果起主要作用, 默认11;
 * @param maxValue   {number?}  二值化最大值 默认255;
 * @param type  {number?}一般写0即可 默认0<br/>
 *				0 灰度值大于阈值为最大值，其他值为<br/>
 *				1 灰度值大于阈值为0，其他值为最大值<br/>
 * @param C {number?} 偏移值调整量 默认0, 像素值减去C再与阈值比较
 * @returns {Mat|null} 二值化图片
 */
func AdaptiveThreshold(mat *Mat, adaptiveMethod, blockSize, maxValue, _type, c int) *Mat {
	// 创建结果矩阵并确保其指针已初始化
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	// 转换Go参数为C兼容类型
	cAdaptiveMethod := C.int(adaptiveMethod)
	cBlockSize := C.int(blockSize)
	cMaxValue := C.int(maxValue)
	cType := C.int(_type)
	cC := C.int(c)
	// 调用C函数执行自适应阈值化
	C.AdaptiveThreshold(mat.ptr, resultMat.ptr, cAdaptiveMethod, cBlockSize, cMaxValue, cType, cC)
	return resultMat
}

// AdaptiveThreshold0 带默认参数的自适应二值化函数
// 简化自适应阈值处理调用，默认参数严格对齐函数注释，适配光照不均场景的二值化需求
func AdaptiveThreshold0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},       // 待二值化图片（必传，无默认值）
		{"adaptiveMethod", 0}, // 阈值计算方式默认 0（平均法），可选 0/1
		{"blockSize", 11},     // 领域大小默认 11（核心参数，需为奇数）
		{"maxValue", 255},     // 二值化最大值默认 255（白色），0~255 范围
		{"type", 0},           // 二值化类型默认 0（大于阈值为 maxValue），可选 0/1
		{"C", 0},              // 偏移调整量默认 0（像素值 - C 后与阈值比较）
	}

	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空 Mat，避免后续异常
	}

	// 调用原始自适应二值化函数，按参数顺序精准传参
	return AdaptiveThreshold(
		paramMetas[0].def.(*Mat), // 待处理图片 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),  // 阈值计算方式 adaptiveMethod
		paramMetas[2].def.(int),  // 领域大小 blockSize
		paramMetas[3].def.(int),  // 二值化最大值 maxValue
		paramMetas[4].def.(int),  // 二值化类型 type（对应原始函数_type 参数）
		paramMetas[5].def.(int),  // 偏移调整量 C（对应原始函数 c 参数）
	)
}

/**
 * 边缘检测
 * @param img  {Mat} 图片
 * @param threshold1  {number?} 弱阈值 默认50
 * @param threshold2 {number?}  强阈值 默认 100
 * @param apertureSize {number?} Sobel大小 默认3
 * @param L2gradient  {Boolean?} 是否更加精确的计算  默认 false
 * @return {Mat} 边缘图片
 */
func Canny(mat *Mat, threshold1, threshold2, apertureSize int, L2gradient bool) *Mat {
	// 创建结果矩阵
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	// 转换Go参数为C兼容类型
	cThreshold1 := C.int(threshold1)
	cThreshold2 := C.int(threshold2)
	cApertureSize := C.int(apertureSize)
	cL2gradient := C.bool(L2gradient) // 转换bool类型

	// 调用C函数执行Canny边缘检测
	C.Canny(mat.ptr, resultMat.ptr, cThreshold1, cThreshold2, cApertureSize, cL2gradient)

	return resultMat
}

// Canny0 带默认参数的边缘检测函数
// 简化 Canny 边缘检测调用，默认参数严格对齐函数注释，适配不同精度与边缘提取需求
func Canny0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},       // 待检测图片（必传，无默认值）
		{"threshold1", 50},    // 弱阈值默认 50（过滤弱边缘，需小于强阈值）
		{"threshold2", 100},   // 强阈值默认 100（保留强边缘，需大于弱阈值）
		{"apertureSize", 3},   // Sobel 算子大小默认 3（核心参数，需为奇数）
		{"L2gradient", false}, // 是否精确计算梯度默认 false（false 性能优，true 精度高）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空 Mat，避免后续调用异常
	}

	// 调用原始 Canny 边缘检测函数，按参数顺序精准传参
	return Canny(
		paramMetas[0].def.(*Mat), // 待检测图片 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),  // 弱阈值 threshold1
		paramMetas[2].def.(int),  // 强阈值 threshold2
		paramMetas[3].def.(int),  // Sobel 算子大小 apertureSize
		paramMetas[4].def.(bool), // 是否精确计算梯度 L2gradient
	)
}

/**
 *  直线检测
 * @param img {Mat} Mat图片
 * @param src_multiple {number?} 缩放比例  1不缩放 0.5 缩小一倍 适当缩放有例如加快速度 默认 1.0
 * @param rho    {number?}  步长 即检测的每次跨越的像素点  默认值 1
 * @param theta   {number?} 角度,即每次检查的角度,比如90 则只检测水平和竖直的线 默认值 1
 * @param threshold {number?} 即连续检测到 多少个rho点则认为是一条直线  默认值 10
 * @param minLineLength {number?} 只有大于这个最小长度的直线被返回  默认值 50
 * @param maxLineGap   {number?}  两直线之间缺了多少个像素 也认为它们是同一条直线 默认 5
 * @param weakThreshold {number?}  检测轮廓(canny)时的弱阈值 大于弱阈值且依附在边缘的点会被保留  默认值 50
 * @param strongThreshold {number?} 检测轮廓(canny)时的强阈值 即相邻的像素点差值大于强阈值则被认为是边缘 默认值 100
 * @return {[]Point或者nil} 返回Point数组 或 nil 注意两个点为一条直线
 */
func FindLines(mat *Mat, srcMultiple float64, rho int, theta float64, threshold int, minLineLength float64, maxLineGap float64, cannyWeakThreshold int, cannyStrongThreshold int) []Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cSrcMultiple := C.double(srcMultiple)
	cRho := C.int(rho)
	cTheta := C.double(theta)
	cThreshold := C.int(threshold)
	cMinLineLength := C.double(minLineLength)
	cMaxLineGap := C.double(maxLineGap)
	cCannyWeak := C.int(cannyWeakThreshold)
	cCannyStrong := C.int(cannyStrongThreshold)

	cResult := C.FindLines(mat.ptr, cSrcMultiple, cRho, cTheta, cThreshold, cMinLineLength, cMaxLineGap, cCannyWeak, cCannyStrong)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// FindLines0 带默认参数的直线检测函数
// 简化霍夫直线检测调用，默认参数严格对齐函数注释，适配不同场景的直线提取需求
func FindLines0(args ...any) []Point {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},          // 待检测图片（必传，无默认值）
		{"src_multiple", 1.0},    // 缩放比例默认 1.0（不缩放），0.5 = 缩小一倍
		{"rho", 1},               // 步长默认 1（每次跨越 1 像素）
		{"theta", 1.0},           // 角度默认 1.0（每次检查 1 度）
		{"threshold", 10},        // 检测阈值默认 10（连续 10 个点判定为直线）
		{"minLineLength", 50.0},  // 最小直线长度默认 50.0（小于此值不返回）
		{"maxLineGap", 5.0},      // 最大线间隙默认 5.0（间隙小于此值视为同一直线）
		{"weakThreshold", 50},    // Canny 弱阈值默认 50（过滤弱边缘）
		{"strongThreshold", 100}, // Canny 强阈值默认 100（保留强边缘）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil
	}

	// 调用原始直线检测函数，按参数顺序精准传参
	return FindLines(
		paramMetas[0].def.(*Mat),    // 待检测图片 img（对应原始函数 mat 参数）
		paramMetas[1].def.(float64), // 缩放比例 src_multiple
		paramMetas[2].def.(int),     // 步长 rho
		paramMetas[3].def.(float64), // 角度 theta
		paramMetas[4].def.(int),     // 检测阈值 threshold
		paramMetas[5].def.(float64), // 最小直线长度 minLineLength
		paramMetas[6].def.(float64), // 最大线间隙 maxLineGap
		paramMetas[7].def.(int),     // Canny 弱阈值 weakThreshold
		paramMetas[8].def.(int),     // Canny 强阈值 strongThreshold
	)
}

/**
 * 圆形检测
 * @param img {Mat} Mat 图片
 * @param minDist {number}  圆心之间最小距离,能解决同心圆问题 默认5
 * @param accuracy {number}  找圆的精度 范围为0~1.0,越接近1越像圆
 * @param minRadius {number}  找圆最小半径
 * @param maxRadius {number}  找圆最大半径
 * @param cannyThreshold {number?} 边缘阈值 默认值 100,即转成单通道后的相邻像素点的值相差多少 认为是边界点
 * @param dp  {number?} 累加器图像的分辨率 直接写1.5即可
 * @returns  {PointEx[]|nil}  返回PointEx数组或nil 继承自Point  例如: 圆心坐标:points[0].x ,points[0].y  圆的半径: points[0].r
 */
func FindCircles(mat *Mat, minDist int, accuracy float64, minRadius, maxRadius int, cannyThreshold, dp float64) []PointEx {

	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cMethod := C.int(4)
	cMinDist := C.int(minDist)
	cAccuracy := C.double(accuracy)
	cMinRadius := C.int(minRadius)
	cMaxRadius := C.int(maxRadius)
	cCannyThreshold := C.double(cannyThreshold)
	cDp := C.double(dp)
	cResult := C.FindCircles(mat.ptr, cMethod, cMinDist, cAccuracy, cMinRadius, cMaxRadius, cCannyThreshold, cDp)
	defer C.DeleteCharArr(cResult)
	return jsonToPointExs(C.GoString(cResult))
}

// FindCircles0 带默认参数的圆形检测函数
// 简化霍夫圆形检测调用，默认参数严格对齐函数注释，适配不同场景的圆形提取需求
func FindCircles0(args ...any) []PointEx {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},           // 待检测图片（必传，无默认值）
		{"minDist", 5},            // 圆心最小距离默认 5（解决同心圆 / 邻近圆重叠问题）
		{"accuracy", 0.8},         // 圆形精度默认 0.8（0~1.0，越接近 1 越严格）
		{"minRadius", 5},          // 最小半径默认 5（过滤过小的圆形噪声）
		{"maxRadius", 100},        // 最大半径默认 100（过滤过大的非目标圆形）
		{"cannyThreshold", 100.0}, // Canny 边缘阈值默认 100.0（默认值对齐函数注释）
		{"dp", 1.5},               // 累加器分辨率默认 1.5（按注释建议固定常用值）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空数组，避免后续异常
	}

	// 调用原始圆形检测函数，按原始函数参数顺序精准传参
	return FindCircles(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.(int),
		paramMetas[2].def.(float64),
		paramMetas[3].def.(int),
		paramMetas[4].def.(int),
		paramMetas[5].def.(float64),
		paramMetas[6].def.(float64),
	)
}

/**
 * 获取HSV图像或其中一种颜色图像
 * @param img {Mat} 图片
 * @param type {number}
 *          -1 表示获取 HSV图像,
 *           0 表示获取H通道图像,
 *           1 表示获取S通道图像,
 *           2 表示获取V通道图像
 * @returns {Mat}  返回Mat图像
 */
func GetHSV(mat *Mat, typ int) *Mat {

	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cType := C.int(typ)
	C.GetHSV(mat.ptr, resultMat.ptr, cType)
	return resultMat
}

/**
 * 获取HLS图像或其中一种颜色图像
 * @param img {Mat} 图片
 * @param type {number}
 *          -1 表示获取 HLS图像,
 *           0 表示获取H通道图像,
 *           1 表示获取L通道图像,
 *           2 表示获取S通道图像
 * @returns {Mat}  返回Mat图像
 */
func GetHLS(mat *Mat, typ int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cType := C.int(typ)
	C.GetHLS(mat.ptr, resultMat.ptr, cType)
	return resultMat
}

/**
 * 获取BGR图像或其中一种颜色图像
 * @param img {Mat} 图片
 * @param type {number}
 *          -1 表示获取 BGR图像,
 *           0 表示获取B通道图像,
 *           1 表示获取G通道图像,
 *           2 表示获取R通道图像
 * @returns {Mat}  返回Mat图像
 */
func GetBGR(mat *Mat, typ int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cType := C.int(typ)
	C.GetBGR(mat.ptr, resultMat.ptr, cType)
	return resultMat
}

/**
 * 多边形检测
 * @param img  {Mat} Mat二值化图像或者轮廓
 * @param Angles  {number} 目标角数/边数 ,如三角形是3 五边形是5
 * @param compareMode  {number?}  比较方式, -1 表示小于Angles,0表示等于Angles,1表示大于Angles,默认为0
 * @param minArea    {number?}  目标的最小面积 默认为100
 * @param maxArea    {number?}  目标的最大面积 默认为-1 无上限
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param epsilon   {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @returns {[]int|nil} 返回的是引索数组或nil,配合其他轮廓函数使用,比如轮廓重心,轮廓外接矩形等
 */
func PolygonDetection(mat *Mat, angles int, compareMode int, minArea int, maxArea int, mode int, method int, epsilon float64) []int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cAngles := C.int(angles)
	cCompareMode := C.int(compareMode)
	cMinArea := C.int(minArea)
	cMaxArea := C.int(maxArea)
	cMode := C.int(mode)
	cMethod := C.int(method)
	cEpsilon := C.double(epsilon)
	intVec := NewIntVector()
	defer intVec.Delete()
	C.PolygonDetection(mat.ptr, intVec.ptr, cAngles, cCompareMode, cMinArea, cMaxArea, cMode, cMethod, cEpsilon)
	return intVec.ToSlice()
}

// PolygonDetection0 带默认参数的多边形检测函数
// 简化多边形检测调用，默认参数严格对齐函数注释，适配不同边数、面积的多边形提取需求
func PolygonDetection0(args ...any) []int {
	paramMetas := []paramMeta{
		{"img", defMat},    // 待检测图像（二值化图 / 轮廓图，必传无默认）
		{"Angles", 3},      // 目标角数 / 边数默认 3（三角形，核心检测目标）
		{"compareMode", 0}, // 比较方式默认 0（等于目标边数）
		{"minArea", 100},   // 最小面积默认 100（过滤过小多边形噪声）
		{"maxArea", -1},    // 最大面积默认 - 1（无上限）
		{"mode", 0},        // 轮廓检索模式默认 0（仅最外围轮廓）
		{"method", 2},      // 轮廓近似方法默认 2（仅保存拐点，精简数据）
		{"epsilon", -1.0},  // 逼近阈值默认 - 1.0（自动取周长 1%）
	}

	if !checkParam(&paramMetas, 2, args...) {
		return nil // 校验失败返回空索引数组，避免后续异常
	}

	return PolygonDetection(
		paramMetas[0].def.(*Mat),    // 待检测图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),     // 目标边数 Angles
		paramMetas[2].def.(int),     // 比较方式 compareMode
		paramMetas[3].def.(int),     // 最小面积 minArea
		paramMetas[4].def.(int),     // 最大面积 maxArea
		paramMetas[5].def.(int),     // 轮廓检索模式 mode
		paramMetas[6].def.(int),     // 轮廓近似方法 method
		paramMetas[7].def.(float64), // 逼近阈值 epsilon
	)
}

/**
 * 获取轮廓的凸包
 * @param img  {Mat} Mat二值化图像或者轮廓
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param thickness  {number?} 绘制的线条宽度 -1表示填充,默认1
 * @param lineType   {number?} 线条类型  8 不抗锯齿 ,16抗锯齿,默认 8
 * @returns {Mat}  凸包图片
 */
func GetConvexHull(mat *Mat, mode int, method int, contoursIndex []int, thickness int, lineType int) *Mat {

	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cMode := C.int(mode)
	cMethod := C.int(method)
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete()
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	C.GetConvexHull(mat.ptr, resultMat.ptr, cMode, cMethod, cContoursIndex.ptr, cThickness, cLineType)
	return resultMat
}

// GetConvexHull0 带默认参数的轮廓凸包提取函数
// 简化凸包计算调用，默认参数严格对齐函数注释，适配不同轮廓模式与绘制需求
func GetConvexHull0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图/轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认0（仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（处理所有轮廓）
		{"thickness", 1},           // 线条宽度默认1（细线绘制）
		{"lineType", 8},            // 线条类型默认8（不抗锯齿，效率高）
	}

	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	return GetConvexHull(
		paramMetas[0].def.(*Mat),  // 输入图像img（对应原始函数mat参数）
		paramMetas[2].def.(int),   // 轮廓检索模式mode
		paramMetas[3].def.(int),   // 轮廓近似方法method
		paramMetas[4].def.([]int), // 目标轮廓索引contoursIndex
		paramMetas[5].def.(int),   // 线条宽度thickness
		paramMetas[6].def.(int),   // 线条类型lineType
	)
}

/**
 * 判断轮廓是否是凸包
 * @param img  大图 二值化图片或者轮廓
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param epsilon        {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @return {boolean[]|或nil} 返回 boolean 数组或nil  true表示是凸包,false表示不是凸包
 */
func IsContourConvex(mat *Mat, mode int, method int, contoursIndex []int, epsilon float64) []bool {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cMode := C.int(mode)
	cMethod := C.int(method)

	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete()
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}

	cEpsilon := C.double(epsilon)
	cResult := C.IsContourConvex(mat.ptr, cMode, cMethod, cContoursIndex.ptr, cEpsilon)
	defer C.DeleteCharArr(cResult)
	return getBoolVector(C.GoString(cResult))
}

// IsContourConvex0 带默认参数的轮廓凸包判断函数
// 简化凸包校验调用，默认参数严格对齐函数注释，适配不同轮廓筛选与精度需求
func IsContourConvex0(args ...any) []bool {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图 / 轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认 0（仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认 2（仅保存拐点，精简计算）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（判断所有轮廓）
		{"epsilon", -1.0},          // 逼近阈值默认 - 1.0（自动取周长 1%，平衡精度与效率）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空数组，避免后续异常
	}

	// 调用原始轮廓凸包判断函数，按参数顺序精准传参
	return IsContourConvex(
		paramMetas[0].def.(*Mat),    // 输入图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),     // 轮廓检索模式 mode
		paramMetas[2].def.(int),     // 轮廓近似方法 method
		paramMetas[3].def.([]int),   // 目标轮廓索引 contoursIndex
		paramMetas[4].def.(float64), // 逼近阈值 epsilon
	)
}

/**
 * 轮廓近似逼近
 * @param img  {Mat} Mat二值化图像或者轮廓
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param thickness  {number?} 绘制的线条宽度 -1表示填充,默认1
 * @param lineType   {number?} 线条类型  8 不抗锯齿 ,16抗锯齿,默认 8
 * @param epsilon        {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 * @returns {Mat}    图片
 */
func ApproxPolyDP(mat *Mat, mode int, method int, contoursIndex []int, thickness int, lineType int, epsilon float64) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cMode := C.int(mode)
	cMethod := C.int(method)
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete()
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	cEpsilon := C.double(epsilon)

	C.ApproxPolyDP(mat.ptr, resultMat.ptr, cMode, cMethod, cContoursIndex.ptr, cThickness, cLineType, cEpsilon)
	return resultMat
}

// ApproxPolyDP0 带默认参数的轮廓近似逼近函数
// 简化轮廓多边形逼近调用，默认参数严格对齐函数注释，适配不同轮廓简化与绘制需求
func ApproxPolyDP0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图 / 轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认 0（仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认 2（仅保存拐点，精简轮廓）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（逼近所有轮廓）
		{"thickness", 1},           // 绘制线条宽度默认 1（细线显示逼近结果）
		{"lineType", 8},            // 线条类型默认 8（不抗锯齿，效率优先）
		{"epsilon", -1.0},          // 逼近阈值默认 - 1.0（自动取周长 1%，平衡精度与简化度）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空 Mat，避免后续异常
	}

	// 调用原始轮廓近似逼近函数，按参数顺序精准传参（匹配原始函数参数列表）
	return ApproxPolyDP(
		paramMetas[0].def.(*Mat),    // 输入图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),     // 轮廓检索模式 mode
		paramMetas[2].def.(int),     // 轮廓近似方法 method
		paramMetas[3].def.([]int),   // 目标轮廓索引 contoursIndex
		paramMetas[4].def.(int),     // 绘制线条宽度 thickness
		paramMetas[5].def.(int),     // 线条类型 lineType
		paramMetas[6].def.(float64), // 逼近阈值 epsilon
	)
}

/**
 * 获取轮廓上的点
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode      {number?}  定义轮廓的检索模式 默认为 0 ：<br/>
 *                0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略<br/>
 *                1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1  <br/>
 *                2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层  <br/>
 * @param method  {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
 *                1 保存物体边界上所有连续的轮廓点到contours向量内 <br/>
 *                2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留 <br/>
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param epsilon        {number?} 轮廓近似逼近阈值 默认为 -1 表示目标周长的百分之一为阈值 ： <br/>
 *  @returns  {[][]Point|nil} Point的二维数组
 */
func GetApproxPolyDPPoint(mat *Mat, mode int, method int, contoursIndex []int, epsilon float64) [][]Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	pointVector2D := NewPointVector2D()
	cMode := C.int(mode)
	cMethod := C.int(method)
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete()
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}
	cEpsilon := C.double(epsilon)

	C.GetApproxPolyDPPoint(mat.ptr, pointVector2D.ptr, cMode, cMethod, cContoursIndex.ptr, cEpsilon)

	return getPoint2DVector(pointVector2D)
}

// GetApproxPolyDPPoint0 带默认参数的轮廓点提取函数
// 简化轮廓近似后点的获取调用，默认参数严格对齐函数注释，适配不同轮廓点分析需求
func GetApproxPolyDPPoint0(args ...any) [][]Point {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图 / 轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认 0（仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认 2（仅保存拐点，精简点集）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（提取所有轮廓的点）
		{"epsilon", -1.0},          // 逼近阈值默认 - 1.0（自动取周长 1%，平衡精度与点数量）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空二维数组，避免后续异常
	}

	// 调用原始轮廓点提取函数，按参数顺序精准传参
	return GetApproxPolyDPPoint(
		paramMetas[0].def.(*Mat),    // 输入图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),     // 轮廓检索模式 mode
		paramMetas[2].def.(int),     // 轮廓近似方法 method
		paramMetas[3].def.([]int),   // 目标轮廓索引 contoursIndex
		paramMetas[4].def.(float64), // 逼近阈值 epsilon
	)
}

/*
*
  - 绘制轮廓
  - @param img   {Mat}  图片 一般为二值化图片
  - @param mode  {number?}  定义轮廓的检索模式 默认为 0 ：
    0 只检测最外围轮廓，包含在外围轮廓内的内围轮廓被忽略
    1 检测所有的轮廓，包括内围、外围轮廓，但是检测到的轮廓不建立等级关系 ,hierarchy向量内所有元素的第3、第4个分量都会被置为-1
    2 检测所有的轮廓，但所有轮廓只建立两个等级关系，外围为顶层，若外围内的内围轮廓还包含了其他的轮廓信息，则内围内的所有轮廓均归属于顶层
  - @param method {number?}   定义轮廓的近似方法 默认为 2 ： <br/>
  - 1 保存物体边界上所有连续的轮廓点到contours向量内
  - 2 仅保存轮廓的拐点信息，把所有轮廓拐点处的点保存入contours向量内，拐点与拐点之间直线段上的信息点不予保留
  - @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
  - 比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
  - @param thickness  {number?} 绘制的线条宽度 -1表示填充,默认1
  - @param lineType   {number?} 线条类型  8 不抗锯齿 ,16抗锯齿,默认 8
  - @returns {Mat}
*/
func GetContours(mat *Mat, mode int, method int, contoursIndex []int, thickness int, lineType int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cMode := C.int(mode)
	cMethod := C.int(method)
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete()
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}
	cThickness := C.int(thickness)
	cLineType := C.int(lineType)
	C.GetContours(mat.ptr, resultMat.ptr, cMode, cMethod, cContoursIndex.ptr, cThickness, cLineType)
	return resultMat
}

// GetContours0 带默认参数的轮廓绘制函数
// 简化轮廓提取与绘制调用，默认参数严格对齐函数注释，适配不同轮廓筛选与可视化需求
func GetContours0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图 / 轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认 0（仅最外围轮廓，减少冗余）
		{"method", 2},              // 轮廓近似方法默认 2（仅保存拐点，精简绘制数据）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（绘制所有匹配轮廓）
		{"thickness", 1},           // 线条宽度默认 1（细线绘制，清晰区分轮廓）
		{"lineType", 8},            // 线条类型默认 8（不抗锯齿，兼顾效率与基础画质）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空 Mat，避免后续异常
	}

	// 调用原始轮廓绘制函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContours(
		paramMetas[0].def.(*Mat),  // 输入图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(int),   // 轮廓检索模式 mode
		paramMetas[2].def.(int),   // 轮廓近似方法 method
		paramMetas[3].def.([]int), // 目标轮廓索引 contoursIndex
		paramMetas[4].def.(int),   // 绘制线条宽度 thickness
		paramMetas[5].def.(int),   // 线条类型 lineType
	)
}

/**
 *  获取轮廓外接矩形
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {Rect[]|nil}   返回Rect[] 或者 nil
 */
func GetContoursRect(mat *Mat, mode int, method int, contoursIndex []int) []Rect {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	// 将轮廓索引列表转换为C语言的整数向量
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete() // 确保资源会被释放
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}

	// 调用C语言函数，直接传递指针（已为C.long类型）
	cResult := C.GetContoursRect(
		mat.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr)
	defer C.DeleteCharArr(cResult)
	return jsonToRects(C.GoString(cResult))
}

// GetContoursRect0 带默认参数的轮廓外接矩形获取函数
// 简化轮廓外接矩形提取调用，默认参数严格对齐函数注释，适配不同轮廓的矩形信息获取需求
func GetContoursRect0(args ...any) []Rect {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},
		{"mode", 0},                // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（同GetContours，仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（获取所有匹配轮廓的外接矩形）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空Rect数组，避免后续异常
	}

	// 调用原始轮廓外接矩形函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursRect(
		paramMetas[0].def.(*Mat),  // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),   // 轮廓检索模式mode
		paramMetas[2].def.(int),   // 轮廓近似方法method
		paramMetas[3].def.([]int), // 目标轮廓索引contoursIndex
	)
}

/**
 *  轮廓外接矩形扩展
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @param dis_x {number?} 外接矩形间的的x轴距离,小于dis_x的矩形将合并,默认5; 若dis_x和dis_y 都为0,则等同于getContoursRect
 * @param dis_y {number?} 外接矩形间的的y轴距离,小于dis_y的矩形将合并,默认5
 * @returns {Rect[]|nil}   返回Rect[] 或者 nil
 */
func GetContoursRectEx(mat *Mat, mode int, method int, contoursIndex []int, disX, disY int) []Rect {
	// 将轮廓索引列表转换为C语言的整数向量
	cContoursIndex := NewIntVector()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	defer cContoursIndex.Delete() // 确保资源会被释放
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}

	cResult := C.GetContoursRectEx(
		mat.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr,
		C.int(disX),
		C.int(disY))
	defer C.DeleteCharArr(cResult)
	return jsonToRects(C.GoString(cResult))
}

// GetContoursRectEx0 带默认参数的轮廓外接矩形扩展函数
// 简化带合并逻辑的轮廓外接矩形提取调用，默认参数严格对齐函数注释，适配需合并邻近矩形的场景
func GetContoursRectEx0(args ...any) []Rect {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},
		{"mode", 0},                // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（同GetContours，仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（获取所有匹配轮廓的外接矩形）
		{"dis_x", 5},               // x轴合并距离默认5（小于该值的矩形在x方向合并）
		{"dis_y", 5},               // y轴合并距离默认5（小于该值的矩形在y方向合并）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空Rect数组，避免后续异常
	}

	// 调用原始轮廓外接矩形扩展函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursRectEx(
		paramMetas[0].def.(*Mat),  // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),   // 轮廓检索模式mode
		paramMetas[2].def.(int),   // 轮廓近似方法method
		paramMetas[3].def.([]int), // 目标轮廓索引contoursIndex
		paramMetas[4].def.(int),   // x轴合并距离disX
		paramMetas[5].def.(int),   // y轴合并距离disY
	)
}

/**
 *  获取轮廓的最小外接矩形
 * @param srcMat  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {Point[][]|nil}   返回Point的二维数组 或者 nil
 */
func GetContoursMinRect(mat *Mat, mode int, method int, contoursIndex []int) [][]Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	// 将轮廓索引列表转换为C语言的整数向量
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete() // 确保资源会被释放
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}

	pointVector2D := NewPointVector2D()
	defer pointVector2D.Delete()

	// 调用C语言函数
	C.getContoursMinRect(
		mat.ptr,
		pointVector2D.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr)
	return pointVector2D.ToSlice()
}

// GetContoursMinRect0 带默认参数的轮廓最小外接矩形获取函数
// 简化轮廓最小外接矩形提取调用，默认参数严格对齐函数注释，适配不同轮廓的最小包围矩形信息获取需求
func GetContoursMinRect0(args ...any) [][]Point {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"srcMat", defMat},         // 输入图像（二值化图/轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（同GetContours，仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（获取所有匹配轮廓的最小外接矩形）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空Point二维数组，避免后续异常
	}

	// 调用原始轮廓最小外接矩形函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursMinRect(
		paramMetas[0].def.(*Mat),  // 输入图像srcMat（对应原始函数mat参数）
		paramMetas[1].def.(int),   // 轮廓检索模式mode
		paramMetas[2].def.(int),   // 轮廓近似方法method
		paramMetas[3].def.([]int), // 目标轮廓索引contoursIndex
	)
}

/**
 *  获取轮廓外接圆
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {PointEx[]|nil}  返回PointEx数组 内有 x点,y点,r半径  或者 nil;
 */
func GetContoursCircles(mat *Mat, mode int, method int, contoursIndex []int) []PointEx {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	// 将轮廓索引列表转换为C语言的整数向量
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete() // 确保资源会被释放
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}

	cResult := C.GetContoursCircles(
		mat.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr)
	defer C.DeleteCharArr(cResult)
	return jsonToPointExs(C.GoString(cResult))
}

// GetContoursCircles0 带默认参数的轮廓外接圆获取函数
// 简化轮廓外接圆提取调用，默认参数严格对齐函数注释，适配不同轮廓的外接圆信息获取需求
func GetContoursCircles0(args ...any) []PointEx {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图/轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（同GetContours，仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（获取所有匹配轮廓的外接圆）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空PointEx数组，避免后续异常
	}

	// 调用原始轮廓外接圆函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursCircles(
		paramMetas[0].def.(*Mat),  // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),   // 轮廓检索模式mode
		paramMetas[2].def.(int),   // 轮廓近似方法method
		paramMetas[3].def.([]int), // 目标轮廓索引contoursIndex
	)
}

/**
 *  获取轮廓长度
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {number[]|nil}   返回所有轮廓的长度数组 或者 nil
 */
func GetContoursLength(mat *Mat, mode int, method int, contoursIndex []int) []float64 {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	// 转换轮廓索引列表为C语言的整数向量
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete() // 确保资源会被释放
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}
	resultDoubleVec := NewDoubleVector()
	defer resultDoubleVec.Delete()
	// 调用C语言函数
	C.GetContoursLength(
		mat.ptr,
		resultDoubleVec.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr)

	return resultDoubleVec.ToSlice()
}

// GetContoursLength0 带默认参数的轮廓长度获取函数
// 简化轮廓长度提取调用，默认参数严格对齐函数注释，适配不同轮廓的周长信息获取需求
func GetContoursLength0(args ...any) []float64 {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图/轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（同GetContours，仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（获取所有匹配轮廓的长度）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空float64数组，避免后续异常
	}

	// 调用原始轮廓长度函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursLength(
		paramMetas[0].def.(*Mat),  // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),   // 轮廓检索模式mode
		paramMetas[2].def.(int),   // 轮廓近似方法method
		paramMetas[3].def.([]int), // 目标轮廓索引contoursIndex
	)
}

/**
 *  获取轮廓面积
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认null  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {number[]|nil}   返回所有轮廓的面积数组 或者 nil
 */
func GetContoursArea(mat *Mat, mode int, method int, contoursIndex []int) []float64 {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	// 转换轮廓索引列表为C语言的整数向量
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete() // 确保资源会被释放
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}
	resultDoubleVec := NewDoubleVector()
	defer resultDoubleVec.Delete()
	// 调用C语言函数
	C.GetContoursArea(
		mat.ptr,
		resultDoubleVec.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr)

	return resultDoubleVec.ToSlice()
}

// GetContoursArea0 带默认参数的轮廓面积获取函数
// 简化轮廓面积提取调用，默认参数严格对齐函数注释，适配不同轮廓的面积信息获取需求
func GetContoursArea0(args ...any) []float64 {

	paramMetas := []paramMeta{
		{"img", defMat},            // 输入图像（二值化图/轮廓图，必传无默认）
		{"mode", 0},                // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},              // 轮廓近似方法默认2（同GetContours，仅保存拐点）
		{"contoursIndex", []int{}}, // 目标轮廓索引默认空数组（获取所有匹配轮廓的长度）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空float64数组，避免后续异常
	}

	// 调用原始轮廓长度函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursArea(
		paramMetas[0].def.(*Mat),  // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),   // 轮廓检索模式mode
		paramMetas[2].def.(int),   // 轮廓近似方法method
		paramMetas[3].def.([]int), // 目标轮廓索引contoursIndex
	)
}

/**
 *  获取轮廓拓扑信息 (即位置信息)
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @returns {number[][]|nil} 返回二位数组  <br/>一个数组包含 nextIndex(下一个引索),beforeIndex(上一个引索),childIndex(子引索),parentIndex(父引索)四个引索 例如 [[2,0,5,-1]]  其中-1代表没有
 */
func GetContoursHierarchy(mat *Mat, mode int, method int) [][]int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	// 创建2D整数向量用于接收结果
	resultIntVec2D := NewIntVector2D()
	defer resultIntVec2D.Delete() // 确保资源释放

	// 调用C语言函数
	C.GetContoursHierarchy(
		mat.ptr,
		resultIntVec2D.ptr,
		C.int(mode),
		C.int(method),
	)

	return resultIntVec2D.ToSlice()
}

// GetContoursHierarchy0 带默认参数的轮廓拓扑信息获取函数
// 简化轮廓层级（位置）信息提取调用，默认参数严格对齐函数注释，适配轮廓父子/前后关系分析需求
func GetContoursHierarchy0(args ...any) [][]int {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat}, // 输入图像（二值化图/轮廓图，必传无默认）
		{"mode", 0},     // 轮廓检索模式默认0（同GetContours，仅最外围轮廓）
		{"method", 2},   // 轮廓近似方法默认2（同GetContours，仅保存拐点）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return nil // 校验失败返回空二维int数组，避免后续异常
	}

	// 调用原始轮廓拓扑信息函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetContoursHierarchy(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 轮廓检索模式mode
		paramMetas[2].def.(int),  // 轮廓近似方法method
	)
}

/**
 * 图片的腐蚀操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations       {number?} 迭代次数 即重复此操作的次数 默认1次
 * @return {Mat} 腐蚀后图片
 */
func Erode(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	// 创建结果图像Mat
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	// 调用C语言函数
	C.Erode(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// Erode0 带默认参数的图片腐蚀操作函数
// 简化图像腐蚀调用，默认参数严格对齐函数注释，适配不同腐蚀强度与结构元素需求
func Erode0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用腐蚀场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础腐蚀范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称腐蚀）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次腐蚀，避免过度侵蚀）
	}

	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	return Erode(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 腐蚀迭代次数iterations
	)
}

/**
 * 图片的膨胀操作
 * @param img {Mat} 图片
 * @param structType    {number?}     结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width    {number?}   结构元素的宽度 默认 5
 * @param struct_height   {number?}   结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y        {number?}  结构元素的锚点y 默认-1
 * @param iterations     {number?} 迭代次数 即重复此操作的次数 默认1次
 * @return {Mat} 膨胀后图片
 */
func Dilate(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {

	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Dilate(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// Dilate0 带默认参数的图片膨胀操作函数
// 简化图像膨胀调用，默认参数严格对齐函数注释，适配不同膨胀强度与结构元素需求
func Dilate0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用膨胀场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础膨胀范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称膨胀）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次膨胀，避免过度扩张）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像膨胀函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Dilate(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 膨胀迭代次数iterations
	)
}

/**
 * 图片的开操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations       {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat}  图片
 */
func Open(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Open(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// Open0 带默认参数的图片开操作函数
// 简化图像开操作调用（先腐蚀后膨胀，用于去除小噪点），默认参数严格对齐函数注释，适配不同去噪与结构元素需求
func Open0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用开操作场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础去噪范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称处理）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次开操作，避免过度损失图像细节）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像开操作函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Open(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 开操作迭代次数iterations
	)
}

/**
 * 图片的闭操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations       {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat} 图片
 */
func Close(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Close(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// Close0 带默认参数的图片闭操作函数
// 简化图像闭操作调用（先膨胀后腐蚀，用于填充轮廓内部小孔），默认参数严格对齐函数注释，适配不同填充需求与结构元素场景
func Close0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用闭操作场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础填充范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称处理）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次闭操作，避免过度扩张轮廓）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像闭操作函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Close(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 闭操作迭代次数iterations
	)
}

/**
 * 图片的梯度
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat}  图片
 */
func Gradient(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Gradient(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// Gradient0 带默认参数的图片梯度操作函数
// 简化图像梯度计算调用（基于膨胀与腐蚀差值，突出轮廓边缘），默认参数严格对齐函数注释，适配不同边缘检测强度与结构元素需求
func Gradient0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用梯度计算场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础边缘检测范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称边缘提取）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次梯度操作，避免边缘过度模糊）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像梯度函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Gradient(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 梯度操作迭代次数iterations
	)
}

/**
 * 顶帽操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat}  图片
 */
func TopHat(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.TopHat(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// TopHat0 带默认参数的图片顶帽操作函数
// 简化图像顶帽计算调用（原始图减开操作结果，突出亮区域小细节/噪点），默认参数严格对齐函数注释，适配不同亮细节提取需求
func TopHat0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用顶帽操作场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础亮细节提取范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称细节检测）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次顶帽操作，避免细节过度放大）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像顶帽函数，按参数顺序精准传参（匹配原始函数参数列表）
	return TopHat(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 顶帽操作迭代次数iterations
	)
}

/**
 * 黑帽操作
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat}  图片
 */
func BlackHat(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.BlackHat(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// BlackHat0 带默认参数的图片黑帽操作函数
// 简化图像黑帽计算调用（闭操作结果减原始图，突出暗区域小细节/孔洞），默认参数严格对齐函数注释，适配不同暗细节提取需求
func BlackHat0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用黑帽操作场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础暗细节提取范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称细节检测）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次黑帽操作，避免暗细节过度放大）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像黑帽函数，按参数顺序精准传参（匹配原始函数参数列表）
	return BlackHat(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 黑帽操作迭代次数iterations
	)
}

/**
 * 击中不击中
 * @param img {Mat} 图片
 * @param structType      {number?}   结构元素类型  0 矩形 1 十字架型 2 椭圆形 默认0
 * @param struct_width     {number?}  结构元素的宽度 默认 5
 * @param struct_height    {number?}  结构元素的高度 默认 5
 * @param anchor_x        {number?} 结构元素的锚点x 默认-1
 * @param anchor_y         {number?}  结构元素的锚点y 默认-1
 * @param iterations      {number?} 迭代次数 即重复操作的次数 默认1次
 * @return {Mat}  图片
 */
func HitMiss(mat *Mat, structType int, structWidth, structHeight int, anchorX, anchorY int, iterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.HitMiss(
		mat.ptr,
		resultMat.ptr,
		C.int(structType),
		C.int(structWidth),
		C.int(structHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(iterations),
	)
	return resultMat
}

// HitMiss0 带默认参数的图片击中不击中操作函数
// 简化图像击中不击中计算调用（基于形态学匹配，检测特定结构模式），默认参数严格对齐函数注释，适配不同模式检测需求
func HitMiss0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"structType", 0},    // 结构元素类型默认0（矩形，通用模式检测场景）
		{"struct_width", 5},  // 结构元素宽度默认5（基础模式匹配范围）
		{"struct_height", 5}, // 结构元素高度默认5（与宽度匹配，保证对称模式检测）
		{"anchor_x", -1},     // 锚点x默认-1（自动取结构元素中心）
		{"anchor_y", -1},     // 锚点y默认-1（自动取结构元素中心）
		{"iterations", 1},    // 迭代次数默认1（单次匹配操作，避免重复检测干扰）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像击中不击中函数，按参数顺序精准传参（匹配原始函数参数列表）
	return HitMiss(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 结构元素类型structType
		paramMetas[2].def.(int),  // 结构元素宽度structWidth
		paramMetas[3].def.(int),  // 结构元素高度structHeight
		paramMetas[4].def.(int),  // 锚点x坐标anchorX
		paramMetas[5].def.(int),  // 锚点y坐标anchorY
		paramMetas[6].def.(int),  // 击中不击中操作迭代次数iterations
	)
}

/**
 * 帧差法
 * @param img1 {Mat} 图1
 * @param img2 {Mat} 图2 与图1同大小
 * @returns {Mat}
 */
func AbsDiff(mat1 *Mat, mat2 *Mat) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.AbsDiff(mat1.ptr, mat2.ptr, resultMat.ptr)
	return resultMat
}

/**
 * 中值滤波
 * @param img {Mat} 图片
 * @param kSize {number?} 中值统计区域大小,如填5,则在5*5的区域内选中值,默认5
 * @returns {Mat}  图片
 */
func MedianBlur(mat *Mat, kSize int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.MedianBlur(
		mat.ptr,
		resultMat.ptr,
		C.int(kSize),
	)
	return resultMat
}

// MedianBlur0 带默认参数的图片中值滤波函数
// 简化图像中值滤波调用（用于椒盐平滑，有效去除椒盐噪声），默认参数严格函数注释，适配不同噪声去除强度需求
func MedianBlur0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"img", defMat}, // 输入图像（必传无默认）
		{"kSize", 5},    // 中值统计区域大小默认5（5×5区域，平衡去噪与细节保留）
	}
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}
	return MedianBlur(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 中值统计区域大小kSize
	)
}

/**
 * 高斯模糊 对图像进行模糊,以达到去噪的过程
 * @param img {Mat} 图片
 * @param kSize   {number?} 卷积核大小 只能选 1,3,5,7。 越大约模糊,默认为 3
 * @param sigmaX  {number?}  图片x轴方向的 模糊程度,越大越模糊, 0 表示根据ksize 自动计算 默认0;
 * @param sigmaY  {number?}  图片y轴方向的 模糊程度,越大越模糊, 0 等同sigmaX  默认0;
 * @param borderType {number?} 边缘模式 选项有0~5,16   默认4
 * @returns {Mat} 模糊后的图片
 */
func GaussianBlur(mat *Mat, ksize int, sigmaX, sigmaY float64, borderType int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.GaussianBlur(
		mat.ptr,
		resultMat.ptr,
		C.int(ksize),
		C.double(sigmaX),
		C.double(sigmaY),
		C.int(borderType),
	)
	return resultMat
}

// GaussianBlur0 带默认参数的图片高斯模糊函数
// 简化图像高斯模糊调用（通过高斯核平滑去噪，保留图像细节），默认参数严格对齐函数注释，适配不同模糊强度与边缘处理需求
func GaussianBlur0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},   // 输入图像（必传无默认）
		{"kSize", 3},      // 卷积核大小默认3（仅支持1/3/5/7，3×3核平衡去噪与细节）
		{"sigmaX", 0.0},   // x轴模糊程度默认0.0（按kSize自动计算，避免手动调参）
		{"sigmaY", 0.0},   // y轴模糊程度默认0.0（等同sigmaX，保证横竖方向对称模糊）
		{"borderType", 4}, // 边缘模式默认4（符合函数注释选项，适配通用边缘处理场景）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像高斯模糊函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GaussianBlur(
		paramMetas[0].def.(*Mat),    // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),     // 卷积核大小kSize
		paramMetas[2].def.(float64), // x轴模糊程度sigmaX
		paramMetas[3].def.(float64), // y轴模糊程度sigmaY
		paramMetas[4].def.(int),     // 边缘模式borderType
	)
}

/**
 * 均值滤波  即均值模糊
 * @param img  {Mat} 图片
 * @param ksize_width  {number?} 卷积核宽度 必须单数 默认5
 * @param ksize_height {number?} 卷积核高度 必须单数 默认5
 * @param anchor_x  {number?} 锚点x 默认-1 表示中心
 * @param anchor_y  {number?} 锚点y 默认-1 表示中心
 * @param borderType  {number?} 边缘类型  默认 4
 * @returns {Mat|null} 模糊后的图片
 */
func Blur(mat *Mat, ksizeWidth, ksizeHeight int, anchorX, anchorY int, borderType int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Blur(
		mat.ptr,
		resultMat.ptr,
		C.int(ksizeWidth),
		C.int(ksizeHeight),
		C.int(anchorX),
		C.int(anchorY),
		C.int(borderType),
	)
	return resultMat
}

// Blur0 带默认参数的图片均值滤波函数
// 简化图像均值模糊调用（通过滑动窗口平均去噪，模糊效果较均匀），默认参数严格对齐函数注释，适配不同模糊范围与边缘处理需求
func Blur0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},     // 输入图像（必传无默认）
		{"ksize_width", 5},  // 卷积核宽度默认5（单数，基础模糊范围）
		{"ksize_height", 5}, // 卷积核高度默认5（单数，与宽度匹配保证对称模糊）
		{"anchor_x", -1},    // 锚点x默认-1（自动取卷积核中心）
		{"anchor_y", -1},    // 锚点y默认-1（自动取卷积核中心）
		{"borderType", 4},   // 边缘类型默认4（符合函数注释选项，适配通用边缘处理场景）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像均值滤波函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Blur(
		paramMetas[0].def.(*Mat), // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),  // 卷积核宽度ksizeWidth
		paramMetas[2].def.(int),  // 卷积核高度ksizeHeight
		paramMetas[3].def.(int),  // 锚点x坐标anchorX
		paramMetas[4].def.(int),  // 锚点y坐标anchorY
		paramMetas[5].def.(int),  // 边缘类型borderType
	)
}

/**
 *  双边滤波器
 * @param img {Mat} 图片
 * @param d  {number} 滤波过程中每个像素邻域的直径，如果这个值是非正数，则由参数sigmaSpace计算得到。
 * @param sigmaColor  颜色空间滤波器的标准差值，这个参数越大表明该像素领域内有越多的颜色被混合到一起，产生较大的半相等颜色区域。
 * @param sigmaSpace  空间坐标中滤波器的标准差值，这个参数越大表明越远的像素会相互影响，从而使更大领域中有足够相似的颜色获取相同的颜色。当第三个参数d大于0时，邻域范围由d确定，当第三个参数小于等于0时，邻域范围正比于这个参数的数值。
 * @param borderType  像素外推法选择标志，取值范围在表3-5中给出，默认参数为4，表示不包含边界值倒序填充。
 * @returns {Mat}
 */
func Bilateral(mat *Mat, d int, sigmaColor, sigmaSpace float64, borderType int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Bilateral(
		mat.ptr,
		resultMat.ptr,
		C.int(d),
		C.double(sigmaColor),
		C.double(sigmaSpace),
		C.int(borderType),
	)
	return resultMat
}

// Bilateral0 带默认参数的图片双边滤波函数
// 简化图像双边滤波调用（在平滑降噪的同时保留边缘细节），默认参数严格对齐函数注释，适配不同降噪与边缘保留需求
func Bilateral0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},      // 输入图像（必传无默认）
		{"d", -1},            // 邻域直径默认-1（由sigmaSpace自动计算，简化调参）
		{"sigmaColor", 10.0}, // 颜色空间标准差默认10.0（适度混合相似颜色）
		{"sigmaSpace", 10.0}, // 空间坐标标准差默认10.0（平衡邻域影响范围）
		{"borderType", 4},    // 像素外推法默认4（不包含边界值倒序填充，通用场景）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 4, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像双边滤波函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Bilateral(
		paramMetas[0].def.(*Mat),    // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),     // 邻域直径d
		paramMetas[2].def.(float64), // 颜色空间标准差sigmaColor
		paramMetas[3].def.(float64), // 空间坐标标准差sigmaSpace
		paramMetas[4].def.(int),     // 像素外推法标志borderType
	)
}

/**
 *  对图片进行锐化,使图棱角分明 ,可以理解成上述模糊的反向过程
 * @param img {Mat} 图片
 * @param kernelType {number?} 卷积核的类型  默认 1 <Br/>
 *                       1:锐化卷积核,
 *                       2:robert_x算子,
 *                       3:robert_y算子,
 *                       4:Sobel_x算子
 *                       5:Sobel_y算子
 *                       6:拉普拉斯算子 <Br/>
 * @param ddepth  {number?} 目标图片所需深度 默认 -1 自动计算
 * @param anchor_x {number?} 卷积核的锚点 x 默认 -1  表示中心
 * @param anchor_y {number?} 卷积核的锚点 y 默认 -1  表示中心
 * @param delta    {number?} 对结果的每个像素增加值  默认 0
 * @param borderType {number?} 边缘模式 选项有1~5,16 默认4
 * @returns {Mat|null} 锐化后的图片
 */
func Filter2D(mat *Mat, kernelType int, ddepth int, anchorX, anchorY int, delta float64, borderType int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Filter2D(
		mat.ptr,
		resultMat.ptr,
		C.int(kernelType),
		C.int(ddepth),
		C.int(anchorX),
		C.int(anchorY),
		C.double(delta),
		C.int(borderType),
	)
	return resultMat
}

// Filter2D0 带默认参数的图片锐化函数
// 简化图像锐化处理（增强图像边缘与细节，实现模糊的反向效果），默认参数严格对齐函数注释，适配不同锐化算子与边缘处理需求
func Filter2D0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},   // 输入图像（必传无默认）
		{"kernelType", 1}, // 卷积核类型默认1（基础锐化卷积核，通用锐化场景）
		{"ddepth", -1},    // 目标图片深度默认-1（自动计算，匹配输入图像深度）
		{"anchor_x", -1},  // 卷积核锚点x默认-1（自动取卷积核中心）
		{"anchor_y", -1},  // 卷积核锚点y默认-1（自动取卷积核中心）
		{"delta", 0.0},    // 像素增加值默认0.0（不额外调整像素亮度）
		{"borderType", 4}, // 边缘模式默认4（符合选项范围，适配通用边缘处理）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	// 调用原始图像锐化函数，按参数顺序精准传参（匹配原始函数参数列表）
	return Filter2D(
		paramMetas[0].def.(*Mat),    // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.(int),     // 卷积核类型kernelType
		paramMetas[2].def.(int),     // 目标图片深度ddepth
		paramMetas[3].def.(int),     // 卷积核锚点x坐标anchorX
		paramMetas[4].def.(int),     // 卷积核锚点y坐标anchorY
		paramMetas[5].def.(float64), // 像素增加值delta
		paramMetas[6].def.(int),     // 边缘模式borderType
	)
}

/**
 * 颜色范围提取  一般和 HLS HSV 图片一起使用 RGB也可以 只是不好确定颜色范围
 * @param img   { Mat } 图片
 * @param lowerColor {number[]} 颜色下限数组 例如[30,127,90];
 * @param upperColor {number[]} 颜色上限数组 例如[90,200,130];
 * @returns {Mat} 掩膜图片 即黑白图片  可以做为 copyto 或  bitwise_and 掩膜获得彩色图片
 */
func InRange(mat *Mat, lowerColor, upperColor []int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	lower := NewIntVector()
	defer lower.Delete()
	upper := NewIntVector()
	defer upper.Delete()
	for _, value := range lowerColor {
		lower.Add(value)
	}
	for _, value := range upperColor {
		upper.Add(value)
	}
	C.InRange(mat.ptr, resultMat.ptr, lower.ptr, upper.ptr)
	return resultMat
}

/**
 * 骨架提取
 * @param img {Mat} 二值化图片
 * @param maxIterations {number?} 限制迭代次数，如果不进行限制，默认为-1
 * @returns {Mat}
 */
func Skeleton(mat *Mat, maxIterations int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Skeleton(
		mat.ptr,
		resultMat.ptr,
		C.int(maxIterations),
	)
	return resultMat
}

// Skeleton0 带默认参数的骨架提取函数
// 简化二值化图像的骨架提取调用（通过迭代腐蚀保留中心骨架），默认参数严格对齐函数注释，适配不同迭代控制需求
func Skeleton0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"img", defMat},
		{"maxIterations", -1},
	}
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat()
	}
	return Skeleton(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.(int),
	)
}

/**
 * 获取连通域状态
 * @param img {Mat} 二值化图片
 * @param connectivity {number?}  领域类型 4 或 8, 默认8,表示8连通
 * @returns {nil|ConnectedComponents[]} 返回 nil 或者 ConnectedComponents数组
 * ConnectedComponents对象 包含一下信息<br/>
 *  index:引索值 为connectedComponentsWithMat提供的<br/>
 *  center:中心的point<br/>
 *  rect:区域<br/>
 *  whiteArea:白点的面积(像素点为单位)<br/>
 *  rectArea:矩形围成的面积<br/>
 */
func ConnectedComponentsWithStats(mat *Mat, connectivity int) []ConnectedState {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cResult := C.ConnectedComponentsWithStats(
		mat.ptr,
		C.int(connectivity),
	)
	defer C.DeleteCharArr(cResult)
	return jsonToConnectedState(C.GoString(cResult))
}

// ConnectedComponentsWithStats0 带默认参数的连通域状态获取函数
// 简化二值化图像的连通域分析调用（提取连通区域的位置、面积等统计信息），默认参数严格对齐函数注释，适配不同连通性分析需求
func ConnectedComponentsWithStats0(args ...any) []ConnectedState {
	paramMetas := []paramMeta{
		{"img", defMat},
		{"connectivity", 8},
	}
	if !checkParam(&paramMetas, 1, args...) {
		return nil
	}
	return ConnectedComponentsWithStats(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.(int),
	)
}

/**
 * 获取连通域图片
 * @param img {Mat} 二值化图片
 * @param indexArr {number[]} 引索值数组 可从connectedComponentsWithStats结果的index属性获得
 * @param connectivity {number?}  领域类型 4 或 8, 默认8
 * @returns { Mat } 图片
 */
func ConnectedComponentsWithMat(mat *Mat, contoursIndex []int, connectivity int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	index := NewIntVector()
	defer index.Delete()
	for _, value := range contoursIndex {
		index.Add(value)
	}
	C.ConnectedComponentsWithMat(mat.ptr, resultMat.ptr, index.ptr, C.int(connectivity))
	return resultMat
}

// ConnectedComponentsWithMat0 带默认参数的连通域图片获取函数
// 简化二值化图像的指定连通域提取调用（根据索引数组获取对应连通区域的图像），默认参数严格对齐函数注释，适配不同连通性提取需求
func ConnectedComponentsWithMat0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"img", defMat},
		{"indexArr", []int{}},
		{"connectivity", 8},
	}
	if !checkParam(&paramMetas, 1, args...) {
		return NewMat()
	}
	return ConnectedComponentsWithMat(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.([]int),
		paramMetas[2].def.(int),
	)
}

/**
 * 掩膜复制
 * @param img  { Mat } 目标图片
 * @param mask  {Mat } 掩膜图片必须和目标图片一样大小, 一般是二值化图片,如果为nil 则是复制原图片;
 *                        掩膜用于告诉函数复制过程中仅仅复制掩膜上非零的点所对应的坐标,
 *                        例如 掩膜上 {x:0,y:1} 是白色,{x:2,y:2} 是黑色,
 *                        那么复制的图片中{x:0,y:1}有颜色,{x:2,y:2} 未复制所以没有颜色
 * @returns {Mat} 图片
 */
func CopyTo(mat, mask *Mat) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (mat.GetHeight() != mask.GetHeight() || mat.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与图1大小必须一致！")
		return resultMat
	}
	C.CopyTo(mat.ptr, resultMat.ptr, mask.ptr)
	return resultMat
}

/**
 * 图片 与
 * @param img1  {Mat} 图片1
 * @param img2  {Mat} 图片2 必须与 图片1 一样大小
 * @param mask  {Mat?}   掩膜 可选 必须与 图片1 一样大小
 * @returns {Mat} 根据图片1 的彩色图片
 */
func Bitwise_and(mat1, mat2, mask *Mat) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	if mat1.GetHeight() != mat2.GetHeight() || mat1.GetWidth() != mat2.GetWidth() {
		fmt.Println("图1图2大小必须一致！")
		return resultMat
	}
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (mat1.GetHeight() != mask.GetHeight() || mat1.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与图1大小必须一致！")
		return resultMat
	}
	C.Bitwise_and(mat1.ptr, mat2.ptr, resultMat.ptr, mask.ptr)
	return resultMat
}

/**
 * 图片 或
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2 必须与 图片1 一样大小  , 也可以是 图片1 图片
 * @param mask  {Mat?}   掩膜 可填nil 否则 必须与 图片1 一样大小
 * @returns {Mat} 掩膜图片 即黑白图片  可以使用 bitwise_and 获取彩色图片
 */
func Bitwise_or(mat1, mat2, mask *Mat) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	if mat1.GetHeight() != mat2.GetHeight() || mat1.GetWidth() != mat2.GetWidth() {
		fmt.Println("图1图2大小必须一致！")
		return resultMat
	}

	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (mat1.GetHeight() != mask.GetHeight() || mat1.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与图1大小必须一致！")
		return resultMat
	}
	C.Bitwise_or(mat1.ptr, mat2.ptr, resultMat.ptr, mask.ptr)
	return resultMat
}

/**
 * 图片 非
 * @param img {Mat} 图片1
 * @param mask  {Mat?}   掩膜 可填nil 否则 必须与 图片1 一样大小
 * @returns {Mat} 图片
 */
func Bitwise_not(mat, mask *Mat) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (mat.GetHeight() != mask.GetHeight() || mat.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与图1大小必须一致！")
		return resultMat
	}
	C.Bitwise_not(mat.ptr, resultMat.ptr, mask.ptr)
	return resultMat
}

/**
 * 图片 异或
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2 必须与 图片1 一样大小
 * @param mask {Mat?}   掩膜 可填nil 否则 必须与 图片1 一样大小
 * @returns {Mat} 图片
 */
func Bitwise_xor(mat1, mat2, mask *Mat) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	if mat1.GetHeight() != mat2.GetHeight() || mat1.GetWidth() != mat2.GetWidth() {
		fmt.Println("图1图2大小必须一致！")
		return resultMat
	}
	if mask == nil {
		mask = NewMat()
		defer mask.Recycle()
	}
	if !mask.Empty() && (mat1.GetHeight() != mask.GetHeight() || mat1.GetWidth() != mask.GetWidth()) {
		fmt.Println("掩膜与图1大小必须一致！")
		return resultMat
	}
	C.Bitwise_xor(mat1.ptr, mat2.ptr, resultMat.ptr, mask.ptr)
	return resultMat
}

/**
 *  获取轮廓重心
 * @param img  {Mat}  图片 一般为二值化图片
 * @param mode {number?}  同 GetContours函数  默认0
 * @param method {number?}  同 GetContours函数 默认2
 * @param contoursIndex {number[]?} 要绘制的轮廓的引索数组 默认nil  引索的筛选可以从其他轮廓函数返回的信息中提取,只要mode和method 一样,返回的数组信息都是一对一相对应的<br/>
 *                       比如, 轮廓重心返回的数组 和 轮廓长度返回数组是一样大小的, 轮廓重心[5] 和 轮廓长度[5]是同一个轮廓的信息
 * @returns {Point[]|nil}   返回所有轮廓的重心坐标数组 或者 nil
 */
func GetContoursMoments(mat *Mat, mode int, method int, contoursIndex []int) []Point {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	cContoursIndex := NewIntVector()
	defer cContoursIndex.Delete()
	for _, index := range contoursIndex {
		cContoursIndex.Add(index)
	}

	cResult := C.GetContoursMoments(
		mat.ptr,
		C.int(mode),
		C.int(method),
		cContoursIndex.ptr,
	)
	defer C.DeleteCharArr(cResult)
	return jsonToPoints(C.GoString(cResult))
}

// GetContoursMoments0 带默认参数的轮廓重心获取函数
// 简化轮廓重心坐标提取（基于图像矩计算，支持指定轮廓索引筛选），默认参数严格对齐函数注释，适配不同轮廓分析场景
func GetContoursMoments0(args ...any) []Point {
	paramMetas := []paramMeta{
		{"img", defMat},
		{"mode", 0},
		{"method", 2},
		{"contoursIndex", []int{}},
	}
	if !checkParam(&paramMetas, 1, args...) {
		return nil
	}

	return GetContoursMoments(
		paramMetas[0].def.(*Mat),
		paramMetas[1].def.(int),
		paramMetas[2].def.(int),
		paramMetas[3].def.([]int),
	)
}

/**
 * 透视变换，可用于矫正目标
 * @param img {Mat}  图片
 * @param srcPoint {Point[]|int[][]} Point数组 或 number的二维数组  原图要转换的目标的4个坐标的数组（注：逆时针顺序） 例如[{x:12,y:33},{x:42,y:63}...] 或[[12,33],[42,63]...]
 * @param dstPoint {Point[]|int[][]} Point数组 或 number的二维数组  转换后的4个坐标的数组（注：逆时针顺序，且与srcPoint四点一一对应） 例如[{x:0,y:0},{x:66,y:88}...] 或[[0,0],[66,88]...]
 * @param width  {number?}  转换后图片的宽带，默认0 根据dstPoint自动计算
 * @param height {number?} 转换后图片的高度，默认0 根据dstPoint自动计算
 * @returns {Mat|nil} 转换后的图片 或者 nil
 */
func WarpPerspective(mat *Mat, srcPointArr, dstPointArr []Point, width, height int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cSrcPointArr := NewPointVector()
	defer cSrcPointArr.Delete()
	cDstPointArr := NewPointVector()
	defer cDstPointArr.Delete()
	for _, point := range srcPointArr {
		cSrcPointArr.Add(point.X, point.Y)
	}
	for _, point := range dstPointArr {
		cDstPointArr.Add(point.X, point.Y)
	}
	C.WarpPerspective(
		mat.ptr,
		resultMat.ptr,
		cSrcPointArr.ptr,
		cDstPointArr.ptr,
		C.int(width),
		C.int(height),
	)
	return resultMat
}

// WarpPerspective0 带默认参数的透视变换函数
// 简化图像透视矫正处理（通过四点映射实现目标矫正），默认参数严格对齐函数注释，适配不同尺寸自动计算需求
func WarpPerspective0(args ...any) *Mat {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},         // 输入图像（必传无默认）
		{"srcPoint", []Point{}}, // 原图目标四点坐标（必传，逆时针顺序）
		{"dstPoint", []Point{}}, // 转换后四点坐标（必传，与原图四点一一对应）
		{"width", 0},            // 转换后宽度默认0（根据dstPoint自动计算）
		{"height", 0},           // 转换后高度默认0（根据dstPoint自动计算）
	}

	if !checkParam(&paramMetas, 3, args...) {
		return NewMat() // 校验失败返回空Mat，避免后续异常
	}

	return WarpPerspective(
		paramMetas[0].def.(*Mat),    // 输入图像img（对应原始函数mat参数）
		paramMetas[1].def.([]Point), // 原图目标四点坐标srcPointArr
		paramMetas[2].def.([]Point), // 转换后四点坐标dstPointArr
		paramMetas[3].def.(int),     // 转换后图片宽度width
		paramMetas[4].def.(int),     // 转换后图片高度height
	)
}

/**
 * 投影
 * @param srcMat {Mat} 二值化图片
 * @param direction {number} 0 表示垂直投影， 1 表示水平投影
 * @returns {Mat} 二值化图片
 */
func Projection(mat *Mat, mode int) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Projection(
		mat.ptr,
		resultMat.ptr,
		C.int(mode),
	)
	return resultMat
}

/**
 * 投影的值
 * @param srcMat {Mat} 二值化图片
 * @param direction {number} 0 表示垂直投影， 1 表示水平投影
 * @returns {number[]|nil} 一维数组 表示每个数字表示图片一列或行上白点的数量或nil
 */
func ProjectionValue(mat *Mat, mode int) []int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return nil
	}
	resultIntVec := NewIntVector()
	defer resultIntVec.Delete()
	C.ProjectionValue(
		mat.ptr,
		resultIntVec.ptr,
		C.int(mode),
	)
	return resultIntVec.ToSlice()
}

/**
 * 图片比较相似度, 只做颜色统计比较,无视大小和方向是否相同
 * @param img1
 * @param img2
 * @returns 返回相似度  0~1.0  1.0最好
 */
func ImageCompare(mat1, mat2 *Mat) float64 {
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return 0.0
	}
	return float64(C.ImageCompare(mat1.ptr, mat2.ptr))
}

/**
 * 检测圆形进度条的进度 顺时针检测
 * @param img {Mat} Mat图片
 * @param color  {string} 颜色字符串 例如:"#FFFFFF-0x101010"
 * @param x      {number} 圆心坐标 x
 * @param y      {number} 圆心坐标 y
 * @param r      {number} 圆形半径 r
 * @param startAngle {number?} 开始角度0~359,以12点钟方向为0度起点, -1表示自动计算起点,默认-1;
 * @param endAngle   {number?} 结束角度1~360,默认360;
 * @returns {int}  进度,0~359;
 */
func GetCircleBarValue(mat *Mat, color string, x, y, r, startAngle, endAngle int) int {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return 0
	}
	// 将Go字符串转换为C字符串
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor)) // 确保C字符串内存被释放
	// 调用C函数
	result := C.GetCircleBarValue(mat.ptr, cColor, C.int(x), C.int(y), C.int(r), C.int(startAngle), C.int(endAngle))
	return int(result)
}

// GetCircleBarValue0 带默认参数的圆形进度条进度检测函数
// 简化圆形进度条的顺时针进度检测，默认参数严格对齐函数注释，适配自动起点检测场景
func GetCircleBarValue0(args ...any) int {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},    // 输入图像（必传无默认）
		{"color", ""},      // 颜色字符串（必传，格式如"#FFFFFF-0x101010"）
		{"x", 0},           // 圆心 x 坐标（必传参数）
		{"y", 0},           // 圆心 y 坐标（必传参数）
		{"r", 0},           // 圆形半径（必传参数）
		{"startAngle", -1}, // 开始角度默认 - 1（自动计算起点）
		{"endAngle", 360},  // 结束角度默认 360（完整圆周范围）
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 5, args...) {
		return 0 // 校验失败返回 0，避免后续异常
	}

	// 调用原始圆形进度条检测函数，按参数顺序精准传参（匹配原始函数参数列表）
	return GetCircleBarValue(
		paramMetas[0].def.(*Mat),   // 输入图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(string), // 颜色字符串 color
		paramMetas[2].def.(int),    // 圆心 x 坐标 x
		paramMetas[3].def.(int),    // 圆心 y 坐标 y
		paramMetas[4].def.(int),    // 圆形半径 r
		paramMetas[5].def.(int),    // 开始角度 startAngle
		paramMetas[6].def.(int),    // 结束角度 endAngle
	)
}

/**
 * 图片合并
 * @param img1 {Mat} 图片1
 * @param img2 {Mat} 图片2
 * @param direction {number}  合并方向 0表示横向,1表示纵向;
 * @returns {Mat}
 */
func MergeMat(mat1, mat2 *Mat, direction int) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	cDirection := C.int(direction)
	C.MergeMat(mat1.ptr, mat2.ptr, resultMat.ptr, cDirection)
	return resultMat
}

/**
 * 图片相加
 * @param img1 {Mat} 图1
 * @param img2 {Mat} 图2
 * @returns {Mat}  返回相加结果图片
 */
func Add(mat1, mat2 *Mat) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Add(mat1.ptr, mat2.ptr, resultMat.ptr)
	return resultMat
}

/**
 * 图片相减
 * @param img1 {Mat} 图1
 * @param img2 {Mat} 图2
 * @returns {Mat}  返回相减结果图片
 */
func Subtract(mat1, mat2 *Mat) *Mat {
	resultMat := NewMat()
	if mat1.Empty() || mat2.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	C.Subtract(mat1.ptr, mat2.ptr, resultMat.ptr)
	return resultMat
}

/**
 * Mat转base64
 * @param addrRgb1 图片
 * @param format 格式 只支持jpg和png 默认jpg
 * @param q 质量0-100 只对jpg有效 默认100
 * @return base64字符串
 */
func MatToBase64(mat *Mat, format string, q int) string {
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return ""
	}
	// 将Go字符串转换为C字符串
	cFormat := C.CString(format)
	defer C.free(unsafe.Pointer(cFormat)) // 确保C字符串内存被释放
	// 调用C函数
	result := C.MatToBase64(mat.ptr, cFormat, C.int(q))
	base64Str := C.GoString(result)
	C.DeleteCharArr(result)
	return base64Str
}

/**
 * Mat转base64
 * @param addrRgb1 图片
 * @param format 格式 只支持jpg和png 默认jpg
 * @param q 质量0-100 只对jpg有效 默认100
 * @return base64
 */
func MatToBase640(args ...any) string {
	// 定义参数元信息（按函数注释参数顺序排列，确保语义一致性）
	paramMetas := []paramMeta{
		{"img", defMat},   // 输入图像（必传无默认）
		{"format", "jpg"}, // 格式
		{"q", 100},        // 质量
	}

	// 参数校验：确保必传参数非空、类型匹配、核心参数符合规则
	if !checkParam(&paramMetas, 1, args...) {
		return "" // 校验失败返回 ，避免后续异常
	}

	// 调用原始圆形进度条检测函数，按参数顺序精准传参（匹配原始函数参数列表）
	return MatToBase64(
		paramMetas[0].def.(*Mat),   // 输入图像 img（对应原始函数 mat 参数）
		paramMetas[1].def.(string), // 格式
		paramMetas[2].def.(int),    // 质量
	)
}

/**
 * base64转Mat
 * @param encoded base64字符串
 */
func Base64ToMat(encoded string) *Mat {
	if encoded == "" {
		fmt.Println("base64数据不能为空！")
		return nil
	}
	resultMat := NewMat()
	cEncoded := C.CString(encoded)
	defer C.free(unsafe.Pointer(cEncoded))
	C.Base64ToMat(cEncoded, resultMat.ptr)
	return resultMat
}

/**
 * 颜色替换   只是不好确定颜色范围
 * @param img   { Mat } 图片
 * @param lowerColor {number[]} 颜色下限数组 例如[30,127,90];
 * @param upperColor {number[]} 颜色上限数组 例如[90,200,130];
 * @param color RGB颜色 与_mode参数无关 如"#FF0000"
 * @param sx    起点x 默认0
 * @param sy    起点y 默认0
 * @param ex    终点x 默认0
 * @param ey    终点y 默认0
 * @param alpha 透明度 0-1 默认1.0
 * @param _mode 内部lowerColor upperColor参数使用方式 HSV HLS RGB
 * @return *Mat 替换后的图片
 */
func ReplaceColor(mat *Mat, lowerColor, upperColor []int, color string, sx, sy, ex, ey int, alpha float64, mode string) *Mat {
	resultMat := NewMat()
	if mat.Empty() {
		fmt.Println("图片不能为空！")
		return resultMat
	}
	// 构建颜色下限向量
	lower := NewIntVector()
	defer lower.Delete()
	for _, value := range lowerColor {
		lower.Add(value)
	}
	// 构建颜色上限向量
	upper := NewIntVector()
	defer upper.Delete()
	for _, value := range upperColor {
		upper.Add(value)
	}
	// 转换字符串参数为 C 字符串
	cColor := C.CString(color)
	defer C.free(unsafe.Pointer(cColor))
	cMode := C.CString(mode)
	defer C.free(unsafe.Pointer(cMode))
	// 调用 C 接口
	C.ReplaceColor(mat.ptr, resultMat.ptr, lower.ptr, upper.ptr, cColor,
		C.int(sx), C.int(sy), C.int(ex), C.int(ey), C.double(alpha), cMode)
	return resultMat
}

func ReplaceColor0(args ...any) *Mat {
	paramMetas := []paramMeta{
		{"mat", defMat},         // 输入图片
		{"lowerColor", []int{}}, // 颜色下限数组
		{"upperColor", []int{}}, // 颜色上限数组
		{"color", "#FF0000"},    // 替换颜色，默认红色
		{"sx", 0},               // 起点x
		{"sy", 0},               // 起点y
		{"ex", 0},               // 终点x
		{"ey", 0},               // 终点y
		{"alpha", 1.0},          // 透明度 0-1
		{"mode", "RGB"},         // 颜色模式：HSV HLS RGB
	}
	// 参数校验：mat、lowerColor、upperColor 必传
	if !checkParam(&paramMetas, 3, args...) {
		return NewMat()
	}
	return ReplaceColor(
		paramMetas[0].def.(*Mat),    // mat
		paramMetas[1].def.([]int),   // lowerColor
		paramMetas[2].def.([]int),   // upperColor
		paramMetas[3].def.(string),  // color
		paramMetas[4].def.(int),     // sx
		paramMetas[5].def.(int),     // sy
		paramMetas[6].def.(int),     // ex
		paramMetas[7].def.(int),     // ey
		paramMetas[8].def.(float64), // alpha
		paramMetas[9].def.(string),  // mode
	)
}
