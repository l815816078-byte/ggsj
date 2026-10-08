module app

go 1.25.0

require (
	Memory v0.0.0
	github.com/Dasongzi1366/AutoGo v0.0.0-20250213122744-570df3683c6b
	github.com/gorilla/websocket v1.5.3
	golang.org/x/text v0.35.0
)

replace github.com/Dasongzi1366/AutoGo => ./AutoGo

// 内存读写库（libmem.so 的 CGO 封装）
// 直接指向 go_mem 子工程里的包，避免拷贝出第二份；
// libmem.so 由该包内 link_android_*.go 按相对路径 ../../resources/libs/<arch> 找到。
replace Memory => ./go_mem/third_party/Memory

// 解决genproto版本冲突
replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20240814211410-ddb44dafa142
