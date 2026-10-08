package wsclient

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"app/core"
)

// 手机端脚本配置文件路径（与项目既有约定一致：sdcard 根目录）
const localConfigPath = "/sdcard/脚本配置.json"

// MergeAndSaveLocalConfig 把中控下发的部分配置（patchJson）逐项合并进手机端现有配置，
// 然后写回本地文件。
//
// 为什么是「逐项合并」而不是「整体替换」：
//   手机端配置里有中控台不显示的字段（如「境界配置」），整体替换会把这些字段弄丢；
//   逐项合并只覆盖中控下发的那些 key，其余保持原值。
func MergeAndSaveLocalConfig(patchJson string) error {
	if patchJson == "" {
		return fmt.Errorf("下发配置为空")
	}

	base := readLocalConfig()

	var patch map[string]interface{}
	if err := json.Unmarshal([]byte(patchJson), &patch); err != nil {
		return fmt.Errorf("下发配置不是合法 JSON: %w", err)
	}

	// 逐项覆盖：patch 的 key 覆盖 base 的同名 key，其余保留
	for k, v := range patch {
		base[k] = v
	}

	out, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化合并结果失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(localConfigPath), 0o755); err != nil {
		// 目录一般已存在，忽略创建失败，交给写文件时报错
		_ = err
	}
	if err := os.WriteFile(localConfigPath, out, 0o644); err != nil {
		return fmt.Errorf("写配置文件失败: %w", err)
	}

	core.LogWithTimef("[中控] 已合并保存到 %s（共 %d 个顶层字段）", localConfigPath, len(base))
	return nil
}

// readLocalConfig 读取手机端现有配置；读不到或解析失败则返回空 map（相当于以 patch 为准新建）
func readLocalConfig() map[string]interface{} {
	data, err := os.ReadFile(localConfigPath)
	if err != nil {
		core.LogWithTimef("[中控] 本地配置不存在(%s)，将以下发内容新建", localConfigPath)
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		core.LogWithTimef("[中控] 本地配置解析失败(%v)，将以下发内容新建", err)
		return map[string]interface{}{}
	}
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}
