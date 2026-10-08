package assets

import "embed"

//go:embed default.json
var DefaultConfig embed.FS

//go:embed detectors.json
var Detectors embed.FS

//go:embed recogs.json
var Recogs embed.FS

//go:embed ziku.txt
var Fonts embed.FS

//go:embed hashtable.txt
var HashTable embed.FS

//go:embed 脚本配置.json
var ScriptConfig embed.FS

//go:embed tasks
var Tasks embed.FS

//go:embed img
var Images embed.FS
