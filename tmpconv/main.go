package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// 一次性脚本：把 assets/recogs.json 里单串写法的 "多点找色": "..." 改成数组写法 "多点找色": ["..."]
func main() {
	path := "assets/recogs.json"
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("read err:", err)
		return
	}
	src := string(b)
	before := strings.Count(src, `"多点找色": "`)
	re := regexp.MustCompile(`(?m)^(\s*)"多点找色": "(.*?)"(,?)(\r?)$`)
	out := re.ReplaceAllString(src, "$1\"多点找色\": [\n$1  \"$2\"\n$1]$3$4")
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		fmt.Println("write err:", err)
		return
	}
	after := strings.Count(out, `"多点找色": [`)
	fmt.Printf("单串 %d 处 → 数组；转换后数组共 %d 处\n", before, after)
}
