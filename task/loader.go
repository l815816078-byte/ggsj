package task

import (
	"app/core"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Loader 任务加载器
type Loader struct {
	tasks []*RuntimeTask
	idx   map[string]*RuntimeTask // name → task
	dir   string                  // JSON 配置目录（文件系统路径）
	efs   fs.FS                   // 嵌入文件系统（兜底）
}

// NewLoader 创建加载器，dir 为文件系统路径
func NewLoader(dir string) *Loader {
	return &Loader{
		dir: dir,
		idx: make(map[string]*RuntimeTask),
	}
}

// NewEmbeddedLoader 创建带嵌入文件系统的加载器（文件系统优先，嵌入兜底）
func NewEmbeddedLoader(dir string, efs fs.FS) *Loader {
	return &Loader{
		dir: dir,
		efs: efs,
		idx: make(map[string]*RuntimeTask),
	}
}

// LoadAll 扫描目录, 加载所有任务配置
// 优先从文件系统读取，失败则回退到嵌入文件系统
func (l *Loader) LoadAll() ([]*RuntimeTask, error) {
	var configs []TaskConfig

	entries, err := os.ReadDir(l.dir)
	if err != nil {
		// 文件系统不可用，尝试嵌入文件系统
		if l.efs != nil {
			core.LogWithTimef("[Loader] 文件系统不可用，使用嵌入配置")
			configs, err = l.loadFromEmbed()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		// 从文件系统加载
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(l.dir, entry.Name())
			cfg, err := l.loadFile(path)
			if err != nil {
				core.LogWithTimef("[Loader] 加载失败 %s: %v", entry.Name(), err)
				continue
			}
			configs = append(configs, *cfg)
		}
	}

	// 按 level 排序（小→大）
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Level < configs[j].Level
	})

	l.tasks = make([]*RuntimeTask, 0, len(configs))
	l.idx = make(map[string]*RuntimeTask, len(configs))
	for _, cfg := range configs {
		rt := NewRuntimeTask(cfg)
		l.tasks = append(l.tasks, rt)
		l.idx[cfg.Name] = rt
	}

	core.LogWithTimef("[Loader] 加载完成: %d 个任务", len(l.tasks))
	return l.tasks, nil
}

// loadFromEmbed 从嵌入文件系统加载所有任务
func (l *Loader) loadFromEmbed() ([]TaskConfig, error) {
	var configs []TaskConfig
	core.LogWithTimef("[Loader] 扫描嵌入目录: %s", "tasks")

	err := fs.WalkDir(l.efs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		cfg, cfgErr := l.loadEmbedFile(path)
		if cfgErr != nil {
			core.LogWithTimef("[Loader] 加载嵌入失败 %s: %v", d.Name(), cfgErr)
			return nil // 跳过失败的文件
		}
		configs = append(configs, *cfg)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// loadEmbedFile 从嵌入文件系统读取并解析单个 JSON
func (l *Loader) loadEmbedFile(path string) (*TaskConfig, error) {
	data, err := fs.ReadFile(l.efs, path)
	if err != nil {
		return nil, err
	}
	return l.parseConfig(data, path)
}

// loadFile 读取并解析单个 JSON 文件
func (l *Loader) loadFile(path string) (*TaskConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return l.parseConfig(data, path)
}

// parseConfig 解析 JSON 并填充默认值
func (l *Loader) parseConfig(data []byte, source string) (*TaskConfig, error) {
	var cfg TaskConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 校验必填字段
	if cfg.Name == "" {
		cfg.Name = strings.TrimSuffix(filepath.Base(source), ".json")
	}
	if len(cfg.Steps) == 0 {
		return nil, &os.PathError{Op: "parse", Path: source, Err: os.ErrInvalid}
	}

	// 识别预设不再合并到 StepConfig，引擎/插件直接调 core.Exec(name)

	// 设置默认值 & 秒→毫秒转换
	for i := range cfg.Steps {
		s := &cfg.Steps[i]
		if s.Sim <= 0 && len(s.Images) > 0 {
			s.Sim = 0.9
		}
		// 秒 → 毫秒
		s.WaitAfter *= 1000
		if s.ClickDelay != nil {
			*s.ClickDelay *= 1000
		}
		s.Timeout *= 1000
		s.RateLimit *= 1000
		// 有 timeout 的步骤不设默认 WaitAfter（用于循环检测场景）
		if s.Timeout > 0 {
			continue
		}
		// MaxTimes 不设默认值：不配(0) = 不限次数。
		// 原因：默认值 1 会掐断"需要跨跳转反复进入"的步骤(如 主线任务 循环链第二圈)，
		// 因为 ExecutedCounts 是整轮累计的，第 2 次进入就被"超过最大次数"掐断。
		// 不限次 ≠ 原地重复执行：没配"超时秒"的步骤，单次 runNode 内执行完动作后
		// 若"成功跳转"无命中就 break 跳出(engine.go 重试间隔那段 Timeout<=0 分支)，
		// 不会自己转圈。本上限只约束"跨跳转被再次进入"的次数。
		// 真要限制就显式配数字（如 "最大次数": 1 = 整轮只允许执行一次）。
		if s.MaxTimes < 0 {
			s.MaxTimes = 0 // 负数统一按"不限"处理
		}
		if s.WaitAfter == 0 {
			s.WaitAfter = 500 // 默认等 500ms
		}
	}

	return &cfg, nil
}

// Get 按名称获取任务
func (l *Loader) Get(name string) *RuntimeTask {
	return l.idx[name]
}

// All 返回所有任务（按优先级排序）
func (l *Loader) All() []*RuntimeTask {
	return l.tasks
}

// TasksByLevel 按优先级层级筛选
func (l *Loader) TasksByLevel(maxLevel int) []*RuntimeTask {
	var result []*RuntimeTask
	for _, t := range l.tasks {
		if t.config.Level <= maxLevel {
			result = append(result, t)
		}
	}
	return result
}

// PickNext 从待运行任务中选出最合适的
func (l *Loader) PickNext() *RuntimeTask {
	if l == nil {
		return nil
	}
	for _, t := range l.tasks {
		// 跳过手动触发任务（如 登录），它们只在启动/恢复流程中触发
		if !t.Config().Auto {
			continue
		}
		if t.CanRun() && t.State() != TaskRunning {
			return t
		}
	}
	return nil
}

// ResetAll 重置所有任务状态
func (l *Loader) ResetAll() {
	for _, t := range l.tasks {
		t.Reset()
	}
}
