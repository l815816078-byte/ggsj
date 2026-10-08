package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"
)

func print(v ...interface{}) {
	pc, file, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println(v...)
		return
	}
	file = filepath.Base(file)
	funcName := "unknown"
	if fn := runtime.FuncForPC(pc); fn != nil {
		funcName = fn.Name()
	}
	fmt.Printf("%s:%d %s() ", file, line, funcName)
	fmt.Println(v...)
}

// InitLogger 初始化日志配置
// 参数 showLineNum：true=显示行号，false=隐藏行号
func InitLogger(showLineNum bool) {
	// 1. 定义日志目录和文件名
	baseDir := "/sdcard/Android/data/log/Go/"
	timeStr := time.Now().Format("20060102_150405")
	logFilePath := baseDir + timeStr + ".log"

	// 2. 创建日志目录
	if err := os.MkdirAll(baseDir, 0777); err != nil {
		log.Fatalf("创建日志目录失败：%v，目录：%s", err, baseDir)
	}

	// ===================== 🔥 核心新增：自动清理旧日志 =====================
	// 超过30个文件 → 删除最早的10个
	cleanOldLogs(baseDir, 30, 10)

	// 3. 创建/打开日志文件（追加模式）
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("创建日志文件失败：%v，路径：%s", err, logFilePath)
	}

	// 4. 设置输出到文件
	log.SetOutput(logFile)

	// 5. 配置日志格式
	if showLineNum {
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetFlags(log.Ldate | log.Ltime)
	}

	// 6. 初始化成功日志
	log.Printf("日志初始化成功 | 日志路径：%s | 显示行号：%t", logFilePath, showLineNum)
}

// ===================== 🔥 自动清理旧文件函数 =====================
// dir: 日志目录
// maxFiles: 最多保留多少个文件
// deleteCount: 超出后删除多少个
func cleanOldLogs(dir string, maxFiles, deleteCount int) {
	// 读取目录下所有文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("读取日志目录失败：%v", err)
		return
	}

	// 只收集 .log 文件
	var logFiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".log" {
			logFiles = append(logFiles, entry)
		}
	}

	// 文件数量没超限，直接返回
	if len(logFiles) <= maxFiles {
		return
	}

	// 按文件修改时间排序（最早的排在前面）
	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := logFiles[i].Info()
		infoJ, _ := logFiles[j].Info()
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// 删除最早的 N 个文件
	for i := 0; i < deleteCount && i < len(logFiles); i++ {
		filePath := filepath.Join(dir, logFiles[i].Name())
		if err := os.Remove(filePath); err != nil {
			log.Printf("删除旧日志失败：%s | 错误：%v", filePath, err)
		} else {
			log.Printf("已删除旧日志：%s", filePath)
		}
	}
}
