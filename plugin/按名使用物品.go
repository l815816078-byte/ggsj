package plugin

import (
	"app/bagrecog"
	"app/core"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Dasongzi1366/AutoGo/ime"
)

func init() {
	Register(&按名使用物品{})
	initExchangeColorDefaults()
}

// 按名使用物品 插件（供"背包物品使用"任务配置调用）
//
// 用哈希识别代替"每种道具配一条多点找色"：hashtable.txt 里已经是"物品名|hash"，
// 识别完直接得到 名字 + 格子中心坐标，所以想用哪个道具，只在下面的名单里写它的
// 名字就行，不用再去取色配 recogs 预设。
//
// 一次调用只处理【一个】道具（用完就返回），原因：用掉一个后背包会变（数量-1 或
// 整个消失），接着用第二个时坐标可能已经错位。每次进来重新识别，坐标永远是最新的。
// 例外：Mode="检测" 的道具不点格子不使用（背包不会变），同一轮（同一次插件调用）内会把
// 本页能检测的目标全部检测完，不逐项跳回 StartUp 重新识别 —— 省掉"处理一个检测就整页重扫一遍"的浪费。
//
// 返回约定（由调用方 JSON 的"成功跳转/失败跳转"接住）:
//
//	跳转步骤名（如"使用"）→ 点格子选中成功, 按钮点击交回该 JSON 步骤做
//	"ok" → 本轮做了会改变界面/背包的动作（召唤/四强领格/材料开箱/交易行查价等），跳回 StartUp 重新识别（背包可能已变，坐标要重新取）
//	"no" → 名单里没有可处理的道具 / 三页扫完只剩已处理过的检测类目标（检测不改背包，处理完就收尾，不必再回 StartUp 空转一轮）
//
// 名单里的道具一个都不在背包 / 表空 / 没认出东西 → "no"。
type 按名使用物品 struct{}

func (p *按名使用物品) Name() string { return "按名使用物品" }

// 使用名单：加道具只改这里，改完重新编译。
//
//	键控写法, 用不到字段就不用写(没有空位): {Name: "…", Mode: "…"} 或 {Name: "…", Mode: "…", Target: "…"}
//	Name —— 物品名，必须和 assets/hashtable.txt 里的写法【完全一致】（繁体、无空格）。
//	        直接照抄表里每行竖线左边那一段（如 鐵針、武器強化符咒）。
//	        表里没这个物品的哈希 → 识别认不出它 → 名单写了也不会用。
//	Mode —— 处理方式：
//	        "召唤使用" = 点格子选中 → 跳 JSON 步骤点按钮（召唤/开启类消耗品走这个:
//	                  武功召喚券/靈物召喚券/调和之箱/铁针 等）。
//	                  按钮步骤由 Target 指定；Target 不写则跳默认的"使用"步骤
//	                  （跳的步骤必须在任务 JSON 步骤列表里存在，否则引擎报
//	                   "插件跳转未找到"直接停任务）。
//	        "普通使用" = 通用消耗品(如【劍聖】拉希德・潘德拉岡这类直接用掉的道具):
//	                  点格子选中 → 点"使用" → 然后循环 6 秒/间隔 1 秒找"点击关闭"并点掉
//	                  (全程在插件内完成, 不依赖 JSON 按钮步骤)。用完返回 ok 回 StartUp
//	                  重新识别, 道具还在就继续用下一个, 用光了自然识别不到 → no 收尾。
//	        "材料使用" = 材料箱(下級/中級/上級材料箱)批量开箱专用: 点格子后全程在
//	                  插件内完成 —— 点'使用'打开数量选择 → 点'max'选最大数量
//	                  → 再点'使用'确认批量开 → 循环点'点击关闭'直到弹窗点完。
//	                  用完一轮返回 ok 回 StartUp 重新识别（有剩继续开, 开光自动
//	                  轮到名单下一项）。不依赖 JSON 按钮步骤。
//	        "检测" = 不做任何点击/使用，识别到即调用 plugin.ResetTaskCooldown 清除
//	                 Target 指定任务的冷却（JSON"冷却秒"+强制冷却都清），让调度器
//	                 下一轮就能选中它执行。同一次任务运行内同一检测物品只重置一次
//	                 （标记放 ctx.Data，一次 Execute 全程共享）；任务重新执行（新一轮
//	                 StartUp 巡检开始/重新进入该任务）时 ctx.Data 重建 → 自动解除限制,
//	                 下次识别到还能再触发重置。检测不点格子不使用 → 背包不会变，同一轮内把本页能检测的目标全部检测完再走，不返回 ok 逐个跳回 StartUp 重识别；全部处理完且没有别的动作 → 直接 no 收尾。
//	        "四强" = 4強飾品自選箱 专用（唯一的特殊流程道具）：箱子已被哈希识别到，
//	                 直接用识别给的格子坐标 it.X,it.Y 点开 → 内部走完整领取流程：
//	                 点"使用" → 选一个未领过的格子 → "加1" → "确认" → 等"点击关闭"
//	                 （最多30秒/间隔2秒）→ 点掉并标记该格已领。
//	                 不再需要单独的插件步骤，也不会二次识别找箱子。
//	                 4 格全领完后该箱子自动跳过，继续名单后面的道具（如 貢獻度）。
//	        "上架交易行" = 交易行看价专用（例: 其他页的 武神寶庫補充石/銀元寶/金元寶）:
//	                  识别到物品 → 在它格子范围内找"可交易"标记(字库 ziku.txt 有,
//	                  recogs 预设"可交易", 找字区域用格子范围动态覆盖) → 有 = 可交易
//	                  → 点物品弹菜单 → 点"交易行看价"按钮(recogs 预设) 进交易行
//	                  → 等3秒 → 按 recogs 预设"最低价格"(取文类, 截图区域/阈值在配置里)
//	                  读"最低价格"栏, 原文抠数字去千分位逗号保留小数(40,000→40000, 3.56→3.56)
//	                  → 存 ctx.Data (key "上架最低价_物品名[_颜色]", 供后续上架定价用;
//	                  规则配了 Color 就带颜色后缀, 同名不同品质的价格/查价标记互相独立),
//	                  本次任务内同一"物品名+颜色"只查一次价(已查过跳过, 不会误跳另一种品质)。
//	                  查价成功后继续"贩售"流程(先写到'点到物品'为止):
//	                  → 点"贩售"按钮(recogs 预设) → 先点"一键结算"和"一次收回"
//	                  (两个都是一次任务只点一次, 标记放 ctx.Data, 下个任务进来才再点)
//	                  → 按物品所属背包页签分类点页签找它
//	                  (装备→[背包页签装备], 消耗品→[背包页签其他],
//	                   其他→[背包页签其他, 背包页签图鉴], 页签依次点击后查找)
//	                  → 每次点完用 bagrecog.RecognizeRegion(763,187,1256,579) 识别一次(不是循环),
//	                  找到同名物品就点击它 → 接着走"登记"流程(step ⑦~⑫, 见 sellRegister):
//	                  点"max" → 取文"登记数量"(纯整数) → 点"登录价格输入"框(字库+坐标偏移)
//	                  → 输入 (最低价格-1)x登记数量 的整数金额(向下取整, 不要小数;
//	                  算出金额<10则跳过本次上架, 连点两次"关闭"收尾) → 点"确定"
//	                  → 取文"每件贩售金额"与"总贩售"(保留小数) → 校验 每件x数量==总贩售 且
//	                  总贩售==输入金额(0.01 容差) → 对上才点"登记物品"(点成功只关一次"点击关闭",
//	                  找不到按钮则连点两次"点击关闭"收尾), 对不上不点(防贱卖)。
//	                  贩售失败只记日志不重试(价格已拿到); 所有没正确上架的物品(登记失败/金额<10
//	                  跳过/按钮没找到)都会记进本次任务名单"上架登记失败_本次"(见 recordSellFail)。
//	                  不可交易/关键按钮没找到 → 跳过该道具(下轮可重试)。
//	        "装备强化" = 装备强化专用: 识别到装备 → 先确认格子有"可交易"(只强化可交易的,
//	                  不可交易强化完也上不了架, 白费符咒 → 跳过) → 读它当前强化等级
//	                  (配了装备强化等级才算, 低于目标才强化, 达标就跳过交给后面条目;
//	                  读不到等级 = 没强化过 → 需要强化) → 点装备 → 点"强化"(602,546,680,588)
//	                  → 循环: 在(612,174,1262,708)里先按"背包_灰"找材料、找不到再"背包_红",
//	                  找到点选 → 找"当前要强化等级箭头"(85,527,530,566)按偏移(-9,+9,+9,+31)
//	                  取文"当前要强化等级", 等于配置的目标才点"进行强化"(192,498,759,706)
//	                  → 本次强化后会变成的等级>4 再点一次"进行强化"(二次确认) → 循环6秒/间隔1秒点"点击关闭"
//	                  → 材料找不到/等级对不上/按钮没找到就结束 → 收尾点"关闭"退出强化界面。
//	                  前置: 背包里必须识别到 武器強化符咒 或 防具強化符咒(见常量 强化符咒)才做,
//	                  一个符咒都没有就跳过(页签扫描顺序 消耗品→其他→装备, 扫到装备页时已知)。
//	                  处理顺序上 装备强化 优先于所有 Mode, 上架交易行 排最后。
//	Target —— 含义随 Mode 变（用不到就不写）：
//	          "召唤使用" → 点格子后跳转的按钮步骤名；不写 = 默认跳"使用"步骤。
//	          "检测" → 要清除冷却的任务名（与 assets/tasks/*.json 的"名称"一致；
//	                    不写则跳过该物品；任务不存在则重置失败并记日志, 本轮不再重试）。
//	Color —— 可选的品质颜色过滤: []string{"红","蓝"} = 该物品格子必须是红或蓝的品质边框
//	          （在哈希识别给的整格范围内多点找色，预设 背包_灰/绿/蓝/红，相似度 0.9 配在
//	          assets/recogs.json）；只填一个 = 只认这一个颜色，填多个 = 任一命中即可（或关系）；
//	          都不命中 → 跳过该格（同名物品可能有多个品质，会继续找下一个同名的）；
//	          留空 = 不检测颜色，只看名字（与没加这个字段之前完全一致）。
//
// itemUse 单条使用规则（字段说明见上方注释）。
type itemUse struct {
	Name   string
	Mode   string
	Target string
	// Color 物品品质颜色过滤（可选）: 写 {"红","蓝"} = 该物品格子必须是红或蓝的边框才算符合,
	// 两个都配 = 红或蓝任一命中即可(或关系); 一个都没命中 = 不符合条件, 跳过该格。
	// 留空/nil = 不检测颜色(只看名字), 与以前行为一致。
	// 可用值: "灰" "绿" "蓝" "红" → 对应 assets/recogs.json 里的 背包_灰/绿/蓝/红 多点找色预设。
	Color []string
	// 装备强化等级 —— 目标强化等级(1~12; 0/不写 = 不检测当前等级):
	//   Mode="装备强化" → 先读该装备当前的强化等级, 低于这个值才强化; 读不到等级 =
	//                    没强化过(0级) → 需要强化;
	//                    已达到/超过 → 跳过本条(交给名单后面同名的条目, 比如上架交易行)。
	//                    留空/0 = 不读等级, 识别到就强化。
	//   Mode="上架交易行" → 只上架"当前强化等级低于该值"的装备(达到/超过的留着不卖);
	//                    留空/0 = 不检测等级, 与以前行为完全一致。
	// 读等级用 recogs 预设"装备强化等级"(取文字库), 检测范围 = 哈希识别给的整个物品格子。
	装备强化等级 int
}

// registerFullKey / 交易行"登记位已满"状态与检测参数:
// 登记物品后若弹出"无法登录更多", 说明本次登记位已满 → 存 ctx.Data[registerFullKey]=true,
// 本次任务后续所有 上架交易行 条目(含整页自动上架)一律跳过, 不再识别/查价/上架;
// ctx.Data 是"一次任务执行共享、任务重跑时重建"的, 所以下次任务重新进来会自动恢复。
const registerFullKey = "交易行_无法登录更多"

// 上架交易行的最低角色等级（交易行开放等级）：
// Mode="上架交易行" 的道具统一按这个门槛, 不够就不查价不贩售, 名单里不用逐条配等级。
// 角色等级读 core.GetLevel(300 秒缓存, 不会每轮重复截图); 识别不到等级 → 不确定就不做, 跳过。
// 要改门槛只改这一个常量; 传 0 = 不做等级限制。
const exchangeMinLevel = 30

// 交易行(上架交易行)总开关 —— 不再用 main.go 的硬编码变量, 改为运行时检测结果:
// 由【检测交易行开启】插件(背包物品使用任务的一个步骤)进商店实际检测"购买完成",
// 命中后调用 IsExchangeOpened() 返回 true, 这里才处理交易行物品。
// 未检测到开启 → 名单里 Mode="上架交易行" 的道具一律跳过, 不查价不贩售, 也不走等级检查。
// 状态在 plugin/检测交易行开启.go(进程内全局, 并发安全)。

// 物品品质颜色 → recogs.json 里的多点找色预设名(相似度 0.9 配在预设里, 插件不写死阈值)
var colorPreset = map[string]string{
	"灰": "背包_灰",
	"绿": "背包_绿",
	"蓝": "背包_蓝",
	"红": "背包_红",
}

// ---------- 装备强化(Mode="装备强化") ----------

const (
	// 强化界面里找强化材料(符咒)的范围: 先按"背包_灰"找, 找不到再按"背包_红"找
	enhanceMatX1, enhanceMatY1, enhanceMatX2, enhanceMatY2 = 612, 174, 1262, 708
	// 材料格子边框 → 格子中间的偏移: "背包_灰/背包_红"多点找色命中在物品格子的边框上,
	// 边框不是可点击区域, 直接点边框会点空 → 点之前坐标各 +30/+18 偏到格子中间再点
	enhanceMatItemOffX, enhanceMatItemOffY = 30, 18
	// 点"进行强化"后循环检测"点击关闭"的窗口(秒)与间隔(秒)
	enhanceCloseWaitSec     = 6
	enhanceCloseIntervalSec = 1
	// 读到的"本次强化后会变成的等级" > 该值 → 点完"进行强化"要再点一次(游戏的高等级二次确认)
	enhanceConfirmAboveLevel = 3
	// 普通使用(Mode="普通使用") 点完"使用"后循环检测"点击关闭"的总时长(秒)与间隔(毫秒)
	normalCloseWaitSec    = 6
	normalCloseIntervalMs = 2000
	// 单次调用里"反复强化"的总时长保底(秒): 到点就收尾退出, 不再循环。
	// 用时间不用次数 —— 强化有失败率, 冲到目标要多少把不确定, 按次数卡会提前掐断。
	// 材料找不到 / 等级已达标 / 按钮没找到 都会提前退出, 不必等到超时。
	enhanceTimeoutSec = 60
	// 装备强化等级 的合法范围, 写超出范围的值 → 该项跳过并记日志
	enhanceLevelMin, enhanceLevelMax = 1, 12
	// "当前要强化等级箭头"命中坐标 → 等级框的偏移量 (x1,y1,x2,y2 各自叠加)
	enhanceLvOffX1, enhanceLvOffY1, enhanceLvOffX2, enhanceLvOffY2 = -10, 7, 8, 31
	// 交易行"购买清单"页里带强化等级装备: "+N"标记(找字, 命中=标记中心) →
	// 该行"最低价格"栏的偏移量 (x1,y1,x2,y2 各自叠加), 即价格栏 =
	// (X+578, Y-57)-(X+753, Y+13); 只有这一档位需要按行找, 不带等级的普通物品
	// 仍用"最低价格"预设默认区域(878,225,1080,303)
	marketPriceOffX1, marketPriceOffY1, marketPriceOffX2, marketPriceOffY2 = 578, -57, 753, 13
)

// 行情列表里带强化等级的装备只会出现这三个档位(recogs.json 里有同名找字预设, 区域 241,214,334,654);
// 配置的装备强化等级不在表里 → 行情里没有对应等级的行, 没法按行读价格, 直接放弃本轮
var marketLevelPreset = map[int]string{
	6:  "+6",
	8:  "+8",
	10: "+10",
}

// 强化符咒: 只有背包里(一般是消耗品页)识别到这些物品时, 装备页的 装备强化 才会执行。
// 页签扫描顺序是 消耗品→其他→装备, 所以扫到装备页时"有没有符咒"已经知道了。
var 强化符咒 = []string{"武器強化符咒", "防具強化符咒"}

// enhanceTalismanKey ctx.Data 里"本次任务运行内在背包里见过强化符咒"的标记 key
const enhanceTalismanKey = "背包有强化符咒"

// 装备强化数量上限 —— 全局统一配置(只认 assets/脚本配置.json 的 "装备强化数量上限.值", 改完重新编译才生效):
// 管【所有装备】的"装备强化"(不是每条名单规则单独配):
//
//	背包里"正在强化"的装备(每件装备格子上的"强化加号"标记)达到多少件,
//	就不再对任何装备开新的强化 —— 所有 Mode="装备强化" 的条目都会跳过, 交给名单后面的条目(如上架交易行)。
//	进入装备强化流程前, 先用 recogs 预设"强化加号"(字库找字"+", 区域=背包整体, 最多找 24 个)扫一遍背包,
//	数出当前已在强化的装备件数; 找到的个数 >= 这个数 → 不开新强化。
//	基于实时识别、每次任务调用重新统计, 反映"同一时刻背包里有多少件装备在强化"。
//	值 >0 = 生效; 0 = 不限(保持原行为)。
//	(Go 里不再有写死的默认值, 全部走 scriptEnhanceLimit())

// countEnhancePlus 统计当前背包里"正在强化/待强化"的装备数量:
// 靠识别每件装备格子上的"强化加号"标记(字库找字 "+")计数, 区域用背包整体范围, 预设最多找 24 个。
// 返回找到的个数; 识别不到预设/识别失败/没找到 → 返回 0(调用方据此放行, 不会因检测异常误拦强化)。
func (p *按名使用物品) countEnhancePlus() int {
	res := core.ExecAll("强化加号")
	if res == nil {
		return 0
	}
	return len(res)
}

// 背包页签分组使用名单: 一块 = 一个页签, 块顺序 = 扫描顺序(消耗品→其他→装备)。
// recogKey 是该页签在 assets/recogs.json 里的识别引用(插件点页签用), 一般不用动。
// 加道具: 把 {Name, Mode, Target} 加进对应页签块的 Items 即可, 插件只在那一页找它。
// 需要限定品质时再加 Color(如 Color: []string{"红", "蓝"} = 红或蓝都算, 都不匹配就跳过该格)。
// Colors —— (可选)本标签(Tab)级默认颜色数组: 块内没写 Color 的上架交易行物品自动继承它,
//
//	实现"全局默认 + 单条覆盖"。单条 Color 写了就用单条的(不受全局影响); 单条 Color 空才继承本默认值。
//	例: 整块默认只认 红/蓝, 个别物品想只认红就写 Color:[]string{"红"} 覆盖。
type itemTab struct {
	Tab      string
	recogKey string
	Colors   []string // 本标签下 上架交易行 物品的默认颜色过滤(全局默认); 单条 Color 不为空时覆盖它
	AutoList bool     // 整页自动上架: 本页识别到的、未被名单认领、匹配 Colors(空=不筛)、且可交易的物品, 自动走 上架交易行(查价→贩售→登记)
	Exclude  []string // 自动上架黑名单(可选): 名字在里面的物品永不自动上架(只拦 AutoList, 不影响名单显式条目)
	Items    []itemUse
}

// itemTabList 背包页签名单 —— 【只认 assets/脚本配置.json 的"背包页签名单"】, Go 里不再有默认名单。
// 加/删道具、改模式、改颜色、开关自动上架, 全在那份 JSON 里改（字段说明见 JSON 里的 "_说明"）。
var itemTabList = scriptItemTabs()

// 贩售流程里, 按物品所属背包页签分类(Tab)依次点哪些页签找它(点完一个识别一次, 找不到点下一个)。
// 贩售/上架界面的页签条与背包页签位置一致(596,109,1232,190), 但分类不同: 消耗品类物品不单列,
// 与其他一起在"其他"页签找; "其他"类(元宝/补充石等)可能在"其他"或"图鉴"页, 两个都要依次试。
var sellFindTabs = map[string][]string{
	"装备":   {"背包页签装备"},
	"消耗品": {"背包页签其他"},
	"其他":   {"背包页签其他", "背包页签图鉴"},
}

// sellFailListKey ctx.Data 里"上架登记失败"名单的 key:
// 值类型 []string(物品名, 去重); Data 一次任务执行共享, 任务重跑重建时自动清空, 每次任务从0累计。
const sellFailListKey = "上架登记失败_本次"

// 4強飾品自選箱 的领取格子（点击范围 左上x,y + 右下x,y；used 标记是否已领过，每次只领未领的）。
// 由 Mode="四强" 的道具使用；格子布局是这个箱子特有的，别的自选箱格子不同要另配。
// 注: 旧插件 自动提升.go 里已有 boxSlot/boxSlots/useBox，这里用 boxCell/boxCells 命名避免冲突。
type boxCell struct {
	x1, y1, x2, y2 int
	used           bool
}

var boxCells = []boxCell{
	{428, 231, 551, 270, false},
	{729, 230, 881, 269, false},
	{400, 311, 572, 358, false},
	{711, 313, 880, 361, false},
}

// 背包三个页签的名单按 itemTabList 块顺序巡检(消耗品→其他→装备)。
// 当前停留页记在 ctx.Data, 下轮巡检从上次页接着找; 本页没货自动点下一页签切过去,
// 所有页都没有可处理道具才返回 no。任务重开后 Data 重建 → 从第一页(消耗品)起。
func (p *按名使用物品) Exec(ctx *Context) (string, error) {
	if IsStopped() {
		return "no", nil
	}
	hasWork := false
	for _, g := range itemTabList {
		if len(g.Items) > 0 || g.AutoList {
			hasWork = true
			break
		}
	}
	if !hasWork {
		core.LogWithTimef("[按名使用物品] 名单为空且无 AutoList 标签 → 返回 no")
		return "no", nil
	}

	start := 0
	if cur, ok := ctx.Data["背包当前页"].(int); ok && cur >= 0 && cur < len(itemTabList) {
		start = cur
	} else {
		// 首次进入(或背包重开): 先切到第一块页签(消耗品)定起点
		core.LogWithTimef("[按名使用物品] 首次进入, 先切到背包【消耗品】页签")
		if !p.switchBagTab(itemTabList[0].recogKey) {
			return "no", nil
		}
	}

	// 从当前页开始按块顺序找第一个有目标道具的页
	for i := 0; i < len(itemTabList); i++ {
		if IsStopped() {
			return "no", nil
		}
		idx := (start + i) % len(itemTabList)
		if i > 0 {
			if !p.switchBagTab(itemTabList[idx].recogKey) {
				return "no", nil
			}
		}
		ctx.Data["背包当前页"] = idx

		// 识别当前页背包（纯识别，只读不改）
		items := bagrecog.Recognize()
		if len(items) == 0 {
			core.LogWithTimef("[按名使用物品] 页签【%s】没识别到物品 → 切下一页", itemTabList[idx].Tab)
			continue
		}

		// 该页在它自己的 Items 名单里找目标处理:
		// handled=true → 本轮做了会改变界面/背包的动作(跳转按钮步骤/回 StartUp 重识别);
		// lightDone=true → 只处理了"检测"类(不点格子不使用, 背包没变) → 无需重识别, 继续扫剩余页;
		// 都没有 → 本页无货, 切下一页。
		if next, handled, lightDone := p.tryUseOne(items, itemTabList[idx].Items, itemTabList[idx], ctx); handled {
			return next, nil
		} else if lightDone {
			core.LogWithTimef("[按名使用物品] 页签【%s】检测类目标已全部处理(不消耗物品, 无需重识别) → 继续扫剩余页", itemTabList[idx].Tab)
			continue
		}
		core.LogWithTimef("[按名使用物品] 页签【%s】没有可处理的道具 → 切下一页", itemTabList[idx].Tab)
	}

	core.LogWithTimef("[按名使用物品] 消耗品/其他/装备 三页都没有可处理的道具 → 返回 no")
	return "no", nil
}

// switchBagTab 点击背包分类页签(装备/消耗品/其他)切页, 点击后等约1.5~2秒让列表刷新。
// 点不到页签(背包没开/界面不对) → 返回 false, 调用方收尾。
func (p *按名使用物品) switchBagTab(tabName string) bool {
	r := core.Exec(tabName)
	if !r.Found {
		core.LogWithTimef("[按名使用物品] 点背包页签【%s】失败(背包没开?) → 返回 no", tabName)
		return false
	}
	core.LogWithTimef("[按名使用物品] 点击背包页签【%s】(%d,%d)", tabName, r.X, r.Y)
	core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+5, r.Y+5)
	core.RandomSleep(1500, 2000)
	return true
}

// wantColorTag 把规则的 Color 集合拼成 ctx.Data key 的颜色后缀:
// 同一名字不同颜色(品质)的物品在交易行是不同条目(价格各自独立, 查价/上架也互不影响),
// "已查交易行_"/"上架最低价_" 这类 key 都要带上颜色, 否则一种品质查完价/上完架,
// 另一种品质的同名格子会被误跳过(漏上架)或读到串了的价格;
// Color 留空(不检测颜色) → 无后缀, 与没加颜色之前的行为完全一致。
func wantColorTag(want itemUse) string {
	if len(want.Color) == 0 {
		return ""
	}
	return "_" + strings.Join(want.Color, "")
}

// itemColorOK 物品品质颜色过滤: 在【该物品的完整格子范围】内多点找色。
//
//	want.Color 为空 → 不检测, 直接 true(只看名字, 与加这个字段之前的行为完全一致)
//	want.Color = {"红","蓝"} → 红、蓝任一命中即 true(或关系)
//	一个都没命中 → false: 不符合条件, 调用方跳过该格(同名物品可能还有别的品质)
//
// 检测区域用哈希识别给的完整外接框 it.X1,it.Y1-it.X2,it.Y2(覆盖整格含品质边框),
// 预设名/多点色串/相似度(0.9)全在 assets/recogs.json 的 背包_灰/绿/蓝/红 里, 插件不写死。
func itemColorOK(it bagrecog.Item, want itemUse) bool {
	if len(want.Color) == 0 {
		return true
	}
	for _, c := range want.Color {
		preset := colorPreset[c]
		if preset == "" {
			core.LogWithTimef("[按名使用物品] %s Color 配了未知颜色 %q(只能填 灰/绿/蓝/红) → 该项跳过", it.Name, c)
			continue
		}
		r := core.Exec(preset, it.X1, it.Y1, it.X2, it.Y2)
		if r.Found {
			core.LogWithTimef("[按名使用物品] %s 格子(%d,%d)-(%d,%d) 颜色【%s】命中 → 符合",
				it.Name, it.X1, it.Y1, it.X2, it.Y2, c)
			return true
		}
	}
	core.LogWithTimef("[按名使用物品] %s 格子(%d,%d)-(%d,%d) 颜色 %v 都没命中 → 不符合, 跳过该格",
		it.Name, it.X1, it.Y1, it.X2, it.Y2, want.Color)
	return false
}

// initExchangeColorDefaults 把标签级默认颜色 Colors 回填到本块内没写 Color 的上架交易行物品,
// 实现"全局该上上、单独也上、单独覆盖全局不被影响":
//   - 单条 itemUse.Color 不为空 → 保持原样(覆盖标签级, 不受全局影响)
//   - 单条 itemUse.Color 为空   → 继承所在 Tab.Colors(全局默认生效)
//
// 合并后若颜色数 > 1 则排序, 保证 wantColorTag 的颜色后缀顺序一致(避免同名同色因顺序不同产生两份价 key)。
// 仅在 init 调用一次; 之后 item.Color 已是最终生效颜色, itemColorOK/wantColorTag/上架流程都不用改。
func initExchangeColorDefaults() {
	for ti := range itemTabList {
		// 标签级默认颜色也排序, 保证 AutoList 走出来的 上架最低价_ key 后缀顺序一致
		if len(itemTabList[ti].Colors) > 1 {
			sort.Strings(itemTabList[ti].Colors)
		}
		tabColors := itemTabList[ti].Colors
		if len(tabColors) == 0 {
			continue
		}
		for ii := range itemTabList[ti].Items {
			it := &itemTabList[ti].Items[ii]
			if len(it.Color) == 0 {
				it.Color = append([]string{}, tabColors...)
			}
			if len(it.Color) > 1 {
				sort.Strings(it.Color)
			}
		}
	}
}

// tryUseOne 在当前页识别结果里, 按 wants(该页签块的名单)顺序找第一个
// "该处理且屏幕上确实有"的道具并处理。
// tab 是该页签的配置(itemTab); 上架交易行 mode 用 tab.Tab 决定贩售时点哪些页签找物品,
// 标签级 AutoList/Colors 用于整页自动上架。
// 返回:
//
//	handled=true → 本轮做了会改变界面/背包的动作, next 是跳转目标(ok 回巡检 / 按钮步骤名如"使用")
//	handled=false + lightDone=true → 本页只处理了"检测"类目标(不消耗物品, 背包没变,
//	    调用方无需重识别本页, 继续扫剩余页; 全部页扫完没有别的动作就 no 收尾)
//	handled=false + lightDone=false → 本页没有可处理的道具(调用方切下一页)
func (p *按名使用物品) tryUseOne(items []bagrecog.Item, wants []itemUse, tab itemTab, ctx *Context) (next string, handled bool, lightDone bool) {
	// 先记本页有没有强化符咒(装备强化的前置材料): 页签按 消耗品→其他→装备 扫,
	// 扫到装备页时"背包里有没有符咒"已经知道了 → 一个符咒都没有就不做 装备强化。
	markEnhanceTalisman(items, ctx)

	// 装备强化数量上限(全局): 本页只识别一次背包里的"强化加号"(正在强化的装备),
	// 缓存起来供下面 装备强化 分支复用, 避免每件装备重复扫一遍。
	enhPlusCount := -1

	// 处理顺序: 装备强化 优先 → 其它 Mode(召唤/材料/四强/检测) → 上架交易行 最后。
	// 装备要先把该强的强化完, 再决定卖不卖, 所以 上架交易行 永远排在最后。
	for _, want := range orderWants(wants) {
		// 空名字通配: 只支持 上架交易行 模式 —— 表示该页所有可交易装备都走此规则
		// (装备强化等级门槛由 tryExchange 内部按"当前强化等级==配置值"过滤, 可交易由 useListOnExchange 过滤)。
		// 其它模式不支持空名字(如 装备强化 通配会强化所有装备, 太危险), 跳过该规则。
		if want.Name == "" && want.Mode != "上架交易行" {
			core.LogWithTimef("[按名使用物品] 名单规则 模式=%s 但名字为空 → 空名字通配仅支持 上架交易行, 跳过该规则", want.Mode)
			continue
		}
		for _, it := range items {
			if want.Name != "" && it.Name != want.Name {
				continue
			}
			// 名字对上后再看颜色: 配了 Color 才检测, 该格颜色不符合就跳过
			// (同名物品可能有多个品质, 跳过这个格子继续找下一个同名的)
			if !itemColorOK(it, want) {
				continue
			}

			switch want.Mode {
			case "召唤使用":
				// 召唤/开启类消耗品(武功召喚券/靈物召喚券/调和之箱/铁针):
				// 只负责点格子选中；点按钮的动作交回 JSON 步骤（Target 或默认"使用"）
				core.LogWithTimef("[按名使用物品] 命中 %s (相似度%.4f) 格子中心(%d,%d) → 点击选中, 按钮点击交回 JSON 步骤",
					it.Name, it.Sim, it.X, it.Y)
				core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)

				jump := want.Target
				if jump == "" {
					jump = "使用"
				}
				core.LogWithTimef("[按名使用物品] %s 已选中 → 跳转步骤【%s】", it.Name, jump)
				return jump, true, false

			case "检测":
				if want.Target == "" {
					core.LogWithTimef("[按名使用物品] %s Mode=检测 但 Target 未配置任务名 → 跳过", it.Name)
					continue
				}
				// 同一次任务运行内只重置一次: ctx.Data 在一次 Execute 里全程共享;
				// 任务重新执行(新 Execute)时 Data 重建, 限制自动解除
				markKey := "已重置冷却_" + want.Name
				if done, _ := ctx.Data[markKey].(bool); done {
					core.LogWithTimef("[按名使用物品] %s 本次任务已重置过【%s】冷却 → 跳过", it.Name, want.Target)
					continue
				}
				if ResetTaskCooldown(want.Target) {
					ctx.Data[markKey] = true
					// 检测不点格子不用道具 → 背包不会变, 不需要跳回 StartUp 重新识别:
					// 置 lightDone 后继续扫本页剩余目标, 一次调用把能检测的都检测完
					core.LogWithTimef("[按名使用物品] %s → 已清除任务【%s】冷却(本次任务内不再重复)", it.Name, want.Target)
					lightDone = true
					continue
				}
				// 重置失败(任务名不存在等)也做标记, 防止每轮巡检反复重试刷屏
				ctx.Data[markKey] = true
				core.LogWithTimef("[按名使用物品] %s → 清除【%s】冷却失败(任务不存在?) → 本次任务内不再重试", it.Name, want.Target)
				continue

			case "四强":
				// 4強飾品自選箱：箱子已被哈希识别到, 直接用识别给的格子中心坐标开箱,
				// 内部走完整领取流程(点箱→使用→选未领格→加1→确认→等'点击关闭'→标记)。
				// 不再像旧版那样单独开"四强饰品自选箱"插件步骤、再 core.Exec 二次识别找箱子。
				if IsStopped() {
					return "no", true, false
				}
				if p.useBoxSelfPick(it) {
					// 领到一格 → 返回 ok 跳回 StartUp 重新识别背包, 箱子还在就用下一格
					return "ok", true, false
				}
				// 4 格全领过 → 箱子无法再领, 跳过它继续名单后面的道具(如 貢獻度)
				continue

			case "材料使用":
				// 材料箱(下級/中級/上級材料箱)：点格子 → 使用 → max → 使用 → 循环点'点击关闭',
				// 全流程在插件内完成, 不跳 JSON 按钮步骤。开完一轮返回 ok 回 StartUp 重新识别,
				// 材料箱还在就继续开下一轮, 用光了自然识别不到 → no 收尾。
				if IsStopped() {
					return "no", true, false
				}
				if p.useMaterialBox(it) {
					return "ok", true, false
				}
				// 本轮没开成功(可能弹窗异常/没找到使用按钮) → 跳过, 继续名单后面的道具
				continue

			case "普通使用":
				// 通用消耗品: 点格子 → 点"使用" → 循环6秒/间隔1秒点"点击关闭",
				// 全程在插件内完成, 不跳 JSON 按钮步骤。用完返回 ok 回 StartUp 重新识别。
				if IsStopped() {
					return "no", true, false
				}
				if p.useNormalItem(it) {
					return "ok", true, false
				}
				// 本轮没用成功(没找到'使用'按钮等) → 跳过, 继续名单后面的道具
				continue

			case "装备强化":
				// 装备强化: 要背包里有强化符咒才做; 目标等级已达标 → 跳过, 交给名单后面
				// 同名的条目(如上架交易行)。不再按"同名只强化一次"做标记 —— 同名多件装备
				// (如多个鐵針)逐件都强化; 只用全局"强化加号数量上限"兜底, 防止同时强化太多。
				if IsStopped() {
					return "no", true, false
				}
				// 装备强化数量上限(全局, 管所有装备): 背包里"正在强化"的装备(强化加号标记)
				// 已达上限 → 不再对任何装备开新强化, 直接跳过本条(交给后面的条目, 如上架交易行)。
				// 本页只在第一次遇到装备强化时统计一次(缓存到 enhPlusCount), 后续复用,
				// 不每件装备都全背包扫描一遍。
				if limit := scriptEnhanceLimit(); limit > 0 {
					if enhPlusCount < 0 {
						enhPlusCount = p.countEnhancePlus()
					}
					if enhPlusCount >= limit {
						core.LogWithTimef("[按名使用物品] %s 背包里强化加号%d个 ≥ 全局上限%d → 所有装备暂不开启新强化", it.Name, enhPlusCount, limit)
						continue
					}
				}
				if !enhanceLevelValid(want) {
					continue
				}
				if !hasEnhanceTalisman(ctx) {
					core.LogWithTimef("[按名使用物品] %s 想装备强化, 但背包里没识别到强化符咒%v → 跳过", it.Name, 强化符咒)
					continue
				}
				if !enhanceNeed(it, want) {
					continue
				}
				// 同名强化流程次数上限 = 脚本"装备强化数量上限"(scriptEnhanceLimit, 0=不限):
				// 防反复失败死循环。计数存 ctx.Data, 新任务重建自动归零
				tryKey := "强化流程_" + want.Name + wantColorTag(want)
				tryN, _ := ctx.Data[tryKey].(int)
				if limit := scriptEnhanceLimit(); limit > 0 && tryN >= limit {
					core.LogWithTimef("[按名使用物品] %s%s 强化流程已尝试%d次≥上限%d → 本次任务不再做装备强化", it.Name, wantColorTag(want), tryN, limit)
					continue
				}
				if p.useEquipEnhance(ctx, it, want) {
					ctx.Data[tryKey] = tryN + 1
					return "ok", true, false
				}
				continue

			case "上架交易行":
				// 总开关/等级/装备强化等级门槛/查价去重/贩售登记 统一走 tryExchange(整页自动上架也复用它)
				if next, ok := p.tryExchange(it, want, tab.Tab, ctx); ok {
					return next, true, false
				}
				continue

			default:
				// 未知 Mode: 防御性跳过, 本插件不处理
				core.LogWithTimef("[按名使用物品] %s Mode=%s 未知处理方式 → 跳过", it.Name, want.Mode)
				continue
			}
		}
	}

	// 整页自动上架(AutoList): 本页识别到的、没被名单认领、匹配 Colors、且可交易的物品,
	// 自动走 上架交易行(查价→贩售→登记)。名单里已显式配置的同名同色物品由上面 for 循环处理, 这里跳过。
	if tab.AutoList {
		for _, it := range items {
			if itemInExclude(it.Name, tab.Exclude) {
				continue
			}
			// 已被名单里某条认领(同名且颜色命中)的物品交给上面 for 循环, 不重复处理
			if itemClaimedByWants(it, wants) {
				continue
			}
			autoWant := itemUse{Name: it.Name, Mode: "上架交易行", Color: tab.Colors}
			// tab.Colors 为空 = 不筛颜色(全部色都上); 非空 = 只认这些色(即"全局颜色"过滤)
			if !itemColorOK(it, autoWant) {
				continue
			}
			core.LogWithTimef("[按名使用物品] 整页自动上架: %s%s 命中(未被名单认领, 走 上架交易行)",
				it.Name, wantColorTag(autoWant))
			if next, ok := p.tryExchange(it, autoWant, tab.Tab, ctx); ok {
				return next, true, false
			}
		}
	}

	// 走到这里 = 本页没有需要"跳转"的动作; 若处理过检测类目标(lightDone) →
	// 调用方继续扫剩余页, 全部页扫完没有别的动作就 no 收尾(检测不改背包, 无需重扫)
	return "", false, lightDone
}

// tryExchange 处理单条 上架交易行 物品的查价+贩售上架(总开关/等级/查价去重/贩售登记 一套)。
// 名单里的 上架交易行 条目与整页自动上架都走这里, 避免两套逻辑。
// 返回 (跳转目标, 是否已处理): handled=true 调用方应返回该 next 跳回 StartUp 重识别;
//
//	handled=false = 跳过(未开启/不可交易/查价失败), 调用方继续下一个物品。
func (p *按名使用物品) tryExchange(it bagrecog.Item, want itemUse, tabName string, ctx *Context) (next string, handled bool) {
	// 本次任务已检测到"无法登录更多"(登记位已满) → 后续一律不再识别/上架, 直到下次任务重进
	if full, _ := ctx.Data[registerFullKey].(bool); full {
		core.LogWithTimef("[按名使用物品] 本次任务已检测到'无法登录更多' → %s 不再识别/上架", it.Name)
		return "", false
	}
	if !IsExchangeOpened() {
		core.LogWithTimef("[按名使用物品] %s Mode=上架交易行 但交易行未确认开启(【检测交易行开启】插件还没检测到) → 跳过", it.Name)
		return "", false
	}
	// 装备强化等级门槛(可选): 配了才检测 —— 只上架"当前强化等级【正好等于】该值"的装备,
	// 高于或低于都不卖(例: 配 3 → 只卖 +3 的, +2 和 +4 都跳过)。
	// 读不到等级 → 不确定, 跳过不上架(防贱卖/误卖)。
	if want.装备强化等级 > 0 {
		cur, ok := readEnhanceLevel(it)
		if !ok {
			core.LogWithTimef("[按名使用物品] %s 配了装备强化等级%d 但读不到当前强化等级 → 跳过不上架",
				it.Name, want.装备强化等级)
			return "", false
		}
		if cur != want.装备强化等级 {
			core.LogWithTimef("[按名使用物品] %s 当前强化等级%d != 配置%d → 不是要卖的等级, 不上架",
				it.Name, cur, want.装备强化等级)
			return "", false
		}
		core.LogWithTimef("[按名使用物品] %s 当前强化等级%d == 配置%d → 可以上架", it.Name, cur, want.装备强化等级)
	}
	if IsStopped() {
		return "no", true
	}
	markKey := "已查交易行_" + want.Name + it.Name + wantColorTag(want)
	if done, _ := ctx.Data[markKey].(bool); done {
		core.LogWithTimef("[按名使用物品] %s%s 本次任务已查过交易行 → 跳过", it.Name, wantColorTag(want))
		return "", false
	}
	if p.useListOnExchange(it, want, tabName, ctx) {
		ctx.Data[markKey] = true
		return "ok", true
	}
	return "", false
}

// itemClaimedByWants 判断已识别物品 it 是否已被名单 wants 中某条认领(同名且颜色命中)。
// 用于整页自动上架时跳过已被显式配置的物品, 避免双处理。
func itemClaimedByWants(it bagrecog.Item, wants []itemUse) bool {
	for _, w := range wants {
		if w.Name != it.Name {
			continue
		}
		if itemColorOK(it, w) {
			return true
		}
	}
	return false
}

// itemInExclude 名字是否在自动上架黑名单里(只拦 AutoList, 不影响名单显式条目)。
func itemInExclude(name string, exclude []string) bool {
	for _, e := range exclude {
		if e == name {
			return true
		}
	}
	return false
}

// orderWants 名单处理顺序: 装备强化 → 其它 Mode(召唤/材料/四强/检测) → 上架交易行(最后)。
// 装备要先强化完再决定卖不卖, 所以 上架交易行 永远排在所有条目之后。
func orderWants(wants []itemUse) []itemUse {
	ordered := make([]itemUse, 0, len(wants))
	for _, w := range wants {
		if w.Mode == "装备强化" {
			ordered = append(ordered, w)
		}
	}
	for _, w := range wants {
		if w.Mode != "装备强化" && w.Mode != "上架交易行" {
			ordered = append(ordered, w)
		}
	}
	for _, w := range wants {
		if w.Mode == "上架交易行" {
			ordered = append(ordered, w)
		}
	}
	return ordered
}

// markEnhanceTalisman 本页识别结果里若有强化符咒 → 在 ctx.Data 打标记(本次任务运行内有效)。
func markEnhanceTalisman(items []bagrecog.Item, ctx *Context) {
	for _, it := range items {
		for _, n := range 强化符咒 {
			if it.Name != n {
				continue
			}
			if done, _ := ctx.Data[enhanceTalismanKey].(bool); !done {
				core.LogWithTimef("[按名使用物品] 背包里识别到强化符咒'%s' → 本次任务 装备强化 可用", it.Name)
			}
			ctx.Data[enhanceTalismanKey] = true
			return
		}
	}
}

// hasEnhanceTalisman 本次任务运行内是否在背包里见过强化符咒
func hasEnhanceTalisman(ctx *Context) bool {
	ok, _ := ctx.Data[enhanceTalismanKey].(bool)
	return ok
}

// enhanceLevelValid 装备强化等级 配了就必须在 1~12 之间, 超范围则该条跳过(记日志)
func enhanceLevelValid(want itemUse) bool {
	lv := want.装备强化等级
	if lv == 0 {
		return true
	}
	if lv < enhanceLevelMin || lv > enhanceLevelMax {
		core.LogWithTimef("[按名使用物品] %s 装备强化等级=%d 超出%d~%d → 跳过该项",
			want.Name, lv, enhanceLevelMin, enhanceLevelMax)
		return false
	}
	return true
}

// readEnhanceLevel 在物品整格范围内取文读它【当前的强化等级】
// (预设"装备强化等级", 取文字库; 检测范围 = 哈希识别给的整个格子 it.X1,Y1-it.X2,Y2)。
func readEnhanceLevel(it bagrecog.Item) (int, bool) {
	r := core.Exec("装备强化等级", it.X1, it.Y1, it.X2, it.Y2)
	if !r.Found || r.Word == "" {
		return 0, false
	}
	lv, ok := parseIntText(r.Word)
	if !ok {
		core.LogWithTimef("[按名使用物品] %s 强化等级原文=%q 抠不出数字 → 读不到", it.Name, r.Word)
		return 0, false
	}
	return lv, true
}

// enhanceNeed 该装备是否还需要强化: 没配目标等级 → 需要; 配了 → 当前等级低于目标才需要。
// 读不到当前等级 = 装备还没强化过(0级/没有强化等级显示) → 需要强化, 进强化界面再判。
func enhanceNeed(it bagrecog.Item, want itemUse) bool {
	if want.装备强化等级 <= 0 {
		return true
	}
	cur, ok := readEnhanceLevel(it)
	if !ok {
		core.LogWithTimef("[按名使用物品] %s 读不到当前强化等级(当没强化过/0级) → 需要强化", it.Name)
		return true
	}
	if cur >= want.装备强化等级 {
		core.LogWithTimef("[按名使用物品] %s 当前强化等级%d 已达目标%d → 不强化(交给后面的条目)",
			it.Name, cur, want.装备强化等级)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 当前强化等级%d < 目标%d → 需要强化", it.Name, cur, want.装备强化等级)
	return true
}

// readTargetEnhanceLevel 读强化界面上"本次强化后会变成几级":
//
//	先找箭头"当前要强化等级箭头"(85,527,530,566, 阈值0.9) → 命中坐标按
//	(-9,+9,+9,+31) 偏移出一个小等级框 → 在该框里取文"当前要强化等级"。
//
// 【数字含义，别搞混】：读到的不是"装备当前等级"，而是"点下去成功后会变成的等级"
// （界面上形如 "+1 > +2"，读的是箭头右边那个）。
//   - 装备现在 +1 → 读到 2
//   - 强化成功变 +2 → 下次读到 3；失败还是 +1 → 下次仍读到 2
//
// 因此"已强化到目标 T 级"的判定是 读到 >= T+1（例: 目标 6 → 读到 7 即达标）。
func (p *按名使用物品) readTargetEnhanceLevel() (int, bool) {
	a := core.Exec("当前要强化等级箭头")
	if !a.Found {
		core.LogWithTimef("[按名使用物品] 没找到'当前要强化等级箭头' → 读不到当前要强化等级")
		return 0, false
	}
	rx1, ry1 := a.X+enhanceLvOffX1, a.Y+enhanceLvOffY1
	rx2, ry2 := a.X+enhanceLvOffX2, a.Y+enhanceLvOffY2
	r := core.Exec("当前要强化等级", rx1, ry1, rx2, ry2)
	if !r.Found || r.Word == "" {
		core.LogWithTimef("[按名使用物品] 等级框(%d,%d)-(%d,%d) 取文'当前要强化等级'失败 → 读不到",
			rx1, ry1, rx2, ry2)
		return 0, false
	}
	lv, ok := parseIntText(r.Word)
	if !ok {
		core.LogWithTimef("[按名使用物品] '当前要强化等级'原文=%q 抠不出数字 → 读不到", r.Word)
		return 0, false
	}
	core.LogWithTimef("[按名使用物品] 箭头(%d,%d) → 等级框(%d,%d)-(%d,%d) 取文=%q → 当前要强化等级=%d",
		a.X, a.Y, rx1, ry1, rx2, ry2, r.Word, lv)
	return lv, true
}

// useEquipEnhance 装备强化完整流程:
//
//	① 先确认该物品可交易(格子范围内找"可交易", 不可交易不强化, 防白费符咒)
//	② 点装备格子 → 点"强化"(602,546,680,588) 打开强化界面
//	③ 每轮: 在强化界面(612,174,1262,708)里先按"背包_灰"找材料, 找不到再按"背包_红"找
//	   → 找到就点击选中; 两个都找不到 = 材料用完 → 结束
//	④ 找"当前要强化等级箭头" → 偏移出等级框 → 取文得到"本次会变成几级"
//	   → 读到 >= 目标+1 (说明已经强化到目标了) → 结束; 否则点"进行强化"(192,498,759,706)
//	   强化可能失败(等级不变 → 下次读到的数不变), 所以靠循环反复点, 不是点一次就完
//	⑤ 读到的"本次强化后等级" > 4 → 再点一次"进行强化"(二次确认)
//	⑥ 循环检测"点击关闭"(6秒/间隔1秒) → 点到就退出本轮, 继续下一轮强化
//	⑦ 收尾点"关闭"退出强化界面
//
// 循环终止条件(全部只是"这件装备到此为止", 都不影响后续扫描):
// 等级达标 / 材料用完 / 按钮找不到 / 6秒没等到结算弹窗 / 满 enhanceTimeoutSec 秒。
//
// 返回 true = 进过强化界面并已收尾(达标、超时、异常都算"处理完这件"):
//
//	调用方标记该装备本次任务已强化过 → 返回 ok 跳回 StartUp 重新识别 → 继续扫其他物品。
//	所以失败/超时不会中断整个任务, 只跳过这一件。
//
// false = 不可交易(没强化) / 连"强化"按钮都没找到(界面不对, 根本没进强化界面);
// 调用方对 false 同样标记 enhKey("本次任务已处理过该装备") → continue,
// 所以不可交易的装备本轮任务内不会再尝试(下次任务重新计时可再判断)。
func (p *按名使用物品) useEquipEnhance(ctx *Context, it bagrecog.Item, want itemUse) bool {
	core.LogWithTimef("[按名使用物品] %s (相似度%.4f) 格子中心(%d,%d) → 点装备走 装备强化(目标等级%d)",
		it.Name, it.Sim, it.X, it.Y, want.装备强化等级)

	// ① 先确认可交易: 强化要消耗符咒, 不可交易的装备强化完也上不了架 → 白费材料, 不强化
	// (与上架交易行同一套: 物品整格范围里找"可交易", recogs 预设, 字库 ziku.txt 里有)
	t := core.Exec("可交易", it.X1, it.Y1, it.X2, it.Y2)
	if !t.Found {
		core.LogWithTimef("[按名使用物品] %s 格子(%d,%d)-(%d,%d)内没找到'可交易' → 不可交易, 不强化(跳过)",
			it.Name, it.X1, it.Y1, it.X2, it.Y2)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 检测到'可交易'(相似度%.4f) → 可强化, 点装备 (%d,%d)",
		it.Name, t.Sim, it.X, it.Y)

	// ② 点装备 → 点"强化"
	core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)
	core.RandomSleep(800, 1500)

	e := core.Exec("强化")
	if !e.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到'强化'按钮 → 本轮放弃装备强化", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] 点击'强化' (%d,%d) sim=%.2f", e.X, e.Y, e.Sim)
	core.RandomClickInAreaWithDelay(e.X, e.Y, e.X+5, e.Y+5)
	core.RandomSleep(800, 1500)

	//  选材料: 先找灰, 找不到再找红
	f2 := core.Exec("背包_灰", enhanceMatX1, enhanceMatY1, enhanceMatX2, enhanceMatY2)
	f1 := core.Exec("背包_红", enhanceMatX1, enhanceMatY1, enhanceMatX2, enhanceMatY2)
	mat := f2
	if !mat.Found {
		mat = f1
	}
	if !mat.Found {
		// 灰/红材料都没找到 = 强化符咒用光了 → 把"背包有强化符咒"标记清掉,
		// 之后 hasEnhanceTalisman 就是"没看见强化符咒", 本次任务运行内所有 装备强化 都被拦住, 不再白跑
		core.LogWithTimef("[按名使用物品] 强化界面(%d,%d)-(%d,%d) 灰/红材料都没找到 → 强化符咒用完, 结束强化",
			enhanceMatX1, enhanceMatY1, enhanceMatX2, enhanceMatY2)
		ctx.Data[enhanceTalismanKey] = false
		core.LogWithTimef("[按名使用物品] 符咒用完 → 标记'背包有强化符咒'=false, 本次运行内不再尝试任何装备强化")
		return false
	}
	core.LogWithTimef("[按名使用物品] 选中强化材料 (%d,%d) sim=%.2f → 偏移到格子中间(%d,%d) 点击",
		mat.X, mat.Y, mat.Sim, mat.X+enhanceMatItemOffX, mat.Y+enhanceMatItemOffY)
	// 命中坐标在格子边框上, 直接点边框点不到 → 先加偏移 30,18 偏到格子中间再点(见常量注释)
	core.RandomClickInAreaWithDelay(mat.X+enhanceMatItemOffX, mat.Y+enhanceMatItemOffY,
		mat.X+enhanceMatItemOffX+5, mat.Y+enhanceMatItemOffY+5)

	// ③~⑥ 反复强化, 直到 材料找不到 / 等级已达标 / 按钮没找到 / 超时(enhanceTimeoutSec)
	enhStart := time.Now()
	enhDeadline := enhStart.Add(time.Duration(enhanceTimeoutSec) * time.Second)
	for round := 1; ; round++ {
		if IsStopped() {
			break
		}
		if time.Now().After(enhDeadline) {
			// 注意: 超时只是"这件装备本轮不再强化", 不是任务失败 ——
			// 下面照常收尾(关强化界面)并返回 true, 调用方跳回 StartUp 继续扫其他物品
			core.LogWithTimef("[按名使用物品] 装备强化 已进行%v(上限%v) → 超时收尾(本轮共点%d次强化); 仅本件装备本轮到此为止, 继续扫其他物品",
				time.Since(enhStart).Round(time.Second),
				time.Duration(enhanceTimeoutSec)*time.Second, round-1)
			break
		}
		core.LogWithTimef("[按名使用物品] 装备强化 第%d轮(剩余%v)", round,
			time.Until(enhDeadline).Round(time.Second))

		// ③ 读"本次强化后会变成几级"(强化界面的数字含义: 不是当前等级!)
		//    - 读到 N = 点下去成功后会变成 N 级(失败则不变, 还是显示 N)
		//    - 所以"装备已强化到目标 T" ⇔ 读到 T+1
		//      (例: 目标 6 → 读到 7 说明已经到 6 了, 该停了)
		//    没配目标等级 → 不读不判, 直接强化
		//    nextLv 提到本层作用域: ④ 的二次确认也要用它判断
		nextLv := 0 // 本次强化后会变成的等级; 没配目标等级时保持 0(二次确认不会触发)
		if want.装备强化等级 > 0 {
			var ok bool
			nextLv, ok = p.readTargetEnhanceLevel()
			if !ok {
				core.LogWithTimef("[按名使用物品] 读不到'当前要强化等级' → 不点强化, 结束")
				break
			}
			doneLv := want.装备强化等级 + 1 // 读到这个值 = 已经强化到目标了
			if nextLv >= doneLv {
				core.LogWithTimef("[按名使用物品] 本次将强化到%d级, 目标%d级已达成(读到%d≥%d) → 结束强化",
					nextLv, want.装备强化等级, nextLv, doneLv)
				break
			}
			core.LogWithTimef("[按名使用物品] 本次将强化到%d级(目标%d, 还差) → 点'进行强化'",
				nextLv, want.装备强化等级)
		}

		// ④ 点"进行强化"; 二次确认按"本次强化后会变成几级"(读到的 nextLv)判断, 不是目标等级:
		//    nextLv > enhanceConfirmAboveLevel → 再点一次"进行强化"(游戏的高等级二次确认)
		if !clickRecog("进行强化") {
			core.LogWithTimef("[按名使用物品] 没找到'进行强化' → 结束强化")
			break
		}
		if nextLv > enhanceConfirmAboveLevel {
			core.LogWithTimef("[按名使用物品] 本次将强化到%d级 > %d → 再点一次'进行强化'(二次确认)",
				nextLv, enhanceConfirmAboveLevel)
			clickRecog("进行强化")
			core.RandomSleep(3500, 5000) //这里必须等。直接点没用。要看强化动画
			// ⑤ 等强化结果弹窗"点击关闭"(6秒/间隔1秒) → 点到就本轮结束、进入下一轮
			cc := WaitFind(enhanceCloseWaitSec, enhanceCloseIntervalSec, "点击关闭")
			if cc == nil {
				core.LogWithTimef("[按名使用物品] %d秒内没等到'点击关闭' → 结束强化", enhanceCloseWaitSec)
				break
			}
			core.LogWithTimef("[按名使用物品] 点击'点击关闭' (%d,%d)", cc.X, cc.Y)
			core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
		}

	}

	// ⑥ 收尾: 点"关闭"退出强化界面
	clickRecog("关闭")
	return true
}

// clickRecog 检测 recogs 预设, 命中就点一下(不命中只记日志), 返回是否点到。
func clickRecog(name string) bool {
	r := core.Exec(name)
	if !r.Found {
		core.LogWithTimef("[按名使用物品] 未检测到'%s' → 不点", name)
		return false
	}
	core.LogWithTimef("[按名使用物品] 检测到'%s' (%d,%d) sim=%.2f → 点击", name, r.X, r.Y, r.Sim)
	core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+5, r.Y+5)
	return true
}

// useBoxSelfPick 处理 4強飾品自選箱（原"四强饰品自选箱"插件的流程，原样并入本插件）：
//
//	不二次识别箱子位置 —— it.X,it.Y 就是哈希识别给的格子中心, 直接点开 →
//	点"使用" → 找一个未领过的格子点选 → 点"加1" → 点"确认"
//	→ 等"点击关闭"(先等2秒再检测, 最多30秒) → 点掉并标记该格已领。
//
// 返回 true = 本次正常领了一格(调用方返回 ok, 跳回 StartUp 开新一轮继续用下一格)；
// false = 4 格全领过 / 已停止(调用方继续名单后面的道具)。
func (p *按名使用物品) useBoxSelfPick(it bagrecog.Item) bool {
	// ① 点开箱子（坐标来自哈希识别, 同原插件 core.Exec("四强饰品自选箱") 的位置）
	core.LogWithTimef("[按名使用物品] %s (相似度%.4f) 格子中心(%d,%d) → 点箱子走自选箱流程",
		it.Name, it.Sim, it.X, it.Y)
	core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)

	// ② 使用
	u := core.Exec("使用")
	core.LogWithTimef("[按名使用物品] 检测'使用' 命中=%v", u.Found)
	if u.Found {
		core.LogWithTimef("[按名使用物品] 点击'使用' (%d,%d)", u.X, u.Y)
		core.RandomClickInAreaWithDelay(u.X, u.Y, u.X+5, u.Y+5)
	}

	// ③ 找一个尚未领取过的格子
	idx := -1
	for i, c := range boxCells {
		if !c.used {
			idx = i
			break
		}
	}
	if idx < 0 {
		core.LogWithTimef("[按名使用物品] %s 所有格子都已领取过 → 无法再领", it.Name)
		return false
	}
	c := boxCells[idx]
	core.LogWithTimef("[按名使用物品] %s 点击格子 %d/%d 范围(%d,%d)-(%d,%d)",
		it.Name, idx+1, len(boxCells), c.x1, c.y1, c.x2, c.y2)
	core.RandomClickInAreaWithDelay(c.x1, c.y1, c.x2, c.y2)

	// ④ 加1
	a := core.Exec("加1")
	core.LogWithTimef("[按名使用物品] 检测'加1' 命中=%v", a.Found)
	if a.Found {
		core.LogWithTimef("[按名使用物品] 点击'加1' (%d,%d)", a.X, a.Y)
		core.RandomClickInAreaWithDelay(a.X, a.Y, a.X+5, a.Y+5)
	}

	// ⑤ 确认（找不到确认也按"本次已处理"返回, 与原插件语义一致）
	cf := core.Exec("确认")
	core.LogWithTimef("[按名使用物品] 检测'确认' 命中=%v", cf.Found)
	if !cf.Found {
		core.LogWithTimef("[按名使用物品] %s 未检测到'确认' → 本次结束(不标记该格)", it.Name)
		return true
	}
	core.LogWithTimef("[按名使用物品] 点击'确认' (%d,%d)", cf.X, cf.Y)
	core.RandomClickInAreaWithDelay(cf.X, cf.Y, cf.X+5, cf.Y+5)

	// ⑥ 等"点击关闭"出现(每轮先等2秒再检测, 最多30秒) → 关闭成功才算该格领取成功
	cc := WaitFind(30, 2, "点击关闭")
	if cc == nil {
		core.LogWithTimef("[按名使用物品] %s 等待'点击关闭'超时, 格子 %d/%d 未标记(下次可重领)",
			it.Name, idx+1, len(boxCells))
		return true
	}
	core.LogWithTimef("[按名使用物品] 点击'点击关闭' (%d,%d)", cc.X, cc.Y)
	core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
	boxCells[idx].used = true
	core.LogWithTimef("[按名使用物品] %s 格子 %d/%d 领取成功, 已标记用过", it.Name, idx+1, len(boxCells))
	return true
}

// useMaterialBox 处理 材料箱(下級/中級/上級材料箱)，全流程在插件内完成：
//
//	点格子(哈希给的坐标) → 点"使用" → 点"max"(选最大数量) → 再点"使用"确认批量开
//	→ 循环检测"点击关闭"(开箱结果弹窗可能连续弹多个)逐个点掉,
//	  直到连续约10秒不再出现 → 全部点完, 本轮结束。
//
// 返回 true = 本轮开箱流程走完(调用方返回 ok, 跳回 StartUp 重新识别背包, 箱子有剩继续)；
// false = 中途关键按钮没找到 / 已停止(调用方跳过, 继续名单后面的道具)。
func (p *按名使用物品) useMaterialBox(it bagrecog.Item) bool {
	// ① 点开材料箱（坐标来自哈希识别）
	core.LogWithTimef("[按名使用物品] %s (相似度%.4f) 格子中心(%d,%d) → 点格子走材料箱流程",
		it.Name, it.Sim, it.X, it.Y)
	core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)

	// ② 点"使用"
	u := core.Exec("使用")
	if !u.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到'使用'按钮 → 本轮失败(跳过)", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 点击'使用' (%d,%d)", it.Name, u.X, u.Y)
	core.RandomClickInAreaWithDelay(u.X, u.Y, u.X+5, u.Y+5)

	// ③ 点"max"选最大数量
	m := core.Exec("max")
	if !m.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到'max'按钮 → 本轮失败(跳过)", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 点击'max' (%d,%d)", it.Name, m.X, m.Y)
	core.RandomClickInAreaWithDelay(m.X, m.Y, m.X+5, m.Y+5)

	// ④ 再点"使用"确认批量开箱
	u2 := core.Exec("使用")
	if !u2.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到确认'使用'按钮 → 本轮失败(跳过)", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 再点'使用' (%d,%d) 确认批量开箱", it.Name, u2.X, u2.Y)
	core.RandomClickInAreaWithDelay(u2.X, u2.Y, u2.X+5, u2.Y+5)

	// ⑤ 循环检测"点击关闭": 开箱结果弹窗可能连续出现多个, 点掉一个再找下一个,
	//    连续 5 次(每次间隔约2秒 = 约10秒)没再出现 → 全部点完, 本轮结束。
	notFound := 0
	for {
		if IsStopped() {
			return false
		}
		cc := core.Exec("点击关闭")
		if cc.Found {
			core.LogWithTimef("[按名使用物品] %s 点击'点击关闭' (%d,%d)", it.Name, cc.X, cc.Y)
			core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
			notFound = 0
			continue
		}
		notFound++
		if notFound >= 5 {
			core.LogWithTimef("[按名使用物品] %s 连续约10秒没有'点击关闭' → 弹窗点完, 本轮结束", it.Name)
			return true
		}
		core.RandomSleep(2000, 2000)
	}
}

// useNormalItem 普通使用(Mode="普通使用"): 点格子选中 → 点"使用" →
// 循环 6 秒(间隔 1 秒)找"点击关闭"并点掉, 全程在插件内完成。
// 返回 true = 本轮已使用(调用方跳回 StartUp 重新识别); false = 没找到"使用"按钮, 本轮失败。
func (p *按名使用物品) useNormalItem(it bagrecog.Item) bool {
	// ① 点开物品（坐标来自哈希识别）
	core.LogWithTimef("[按名使用物品] %s (相似度%.4f) 格子中心(%d,%d) → 点格子走普通使用流程",
		it.Name, it.Sim, it.X, it.Y)
	core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)

	// ② 点"使用"
	u := core.Exec("使用")
	if !u.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到'使用'按钮 → 本轮失败(跳过)", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 点击'使用' (%d,%d)", it.Name, u.X, u.Y)
	core.RandomClickInAreaWithDelay(u.X, u.Y, u.X+5, u.Y+5)

	// ③ 循环检测"点击关闭": 6 秒内每 1 秒找一次, 找到就点掉(可能有连续弹窗), 到点结束本轮
	deadline := time.Now().Add(normalCloseWaitSec * time.Second)
	for round := 1; time.Now().Before(deadline); round++ {
		if IsStopped() {
			return false
		}
		cc := core.Exec("点击关闭")
		if cc.Found {
			core.LogWithTimef("[按名使用物品] %s 第%d次循环: 点击'点击关闭' (%d,%d)", it.Name, round, cc.X, cc.Y)
			core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
		} else {
			core.LogWithTimef("[按名使用物品] %s 第%d次循环: 没找到'点击关闭' → 等%dms", it.Name, round, normalCloseIntervalMs)
		}
		core.RandomSleep(normalCloseIntervalMs, normalCloseIntervalMs)
	}
	core.LogWithTimef("[按名使用物品] %s 普通使用流程结束(循环%d秒) → 返回 ok 重新识别", it.Name, normalCloseWaitSec)
	return true
}

// useListOnExchange 处理 Mode="上架交易行"（例: 其他页的 武神寶庫補充石/銀元寶/金元寶）。
// tab 是该物品所属背包页签分类(消耗品/其他/装备), 决定贩售流程里点哪些页签找它。
//
//	查价流程:
//	① 在物品的完整格子范围(it.X1,it.Y1)-(it.X2,it.Y2), 哈希识别返回的物品外接框,
//	   覆盖整格含右上角)找"可交易"标记(recogs 预设, 字库 ziku.txt 有"可交易")
//	   → 没有 = 不可交易, 直接放弃
//	② 点物品(格子中心 it.X,it.Y) → 弹物品菜单 → 找"交易行看价"按钮(recogs 预设) → 点进交易行
//	③ 等交易行加载
//	④ 读"最低价格"栏取文: 原文抠数字去千分位逗号(40,000→40000), 小数保留(3.56→3.56),
//	   存 ctx.Data (key: "上架最低价_"+物品名+颜色后缀, 例 "上架最低价_元寶_蓝";
//	   规则没配 Color 则无后缀), 供后续上架定价用
//	   配置了 装备强化等级 的装备(带强化等级的行情行位置随等级变化): 不读默认区域 ——
//	   先点"购买清单" → 在等级列(区域 241,214,334,654)找当前等级的"+N"标记 →
//	   命中坐标按 (578,-57,753,13) 偏移出该行价格栏再取文(见 readMarketLevelPrice);
//	   没配等级的普通物品直接按"最低价格"预设默认区域(878,225,1080,303)取文
//	⑤⑥ 查价成功后调 sellFindOnExchange 继续贩售流程(点贩售→按 tab 点页签→识别→点物品)。
//
// 返回 true = 查价成功(变量已存, 贩售流程只记日志不重试);
// false = 不可交易 / 关键按钮没找到 / 没读到价格 (调用方跳过该道具, 不标记, 下轮巡检可再试)。
// want 是这条使用规则(取它的 Color 给贩售面板那一步做颜色过滤), tab 是物品所属背包页签分类。
func (p *按名使用物品) useListOnExchange(it bagrecog.Item, want itemUse, tab string, ctx *Context) bool {
	// ① 完整格子范围找"可交易": 该标记在物品格子右上角, 而 it.X1,it.Y1,it.X2,it.Y2
	//    是哈希识别返回的物品外接框完整全屏坐标, 直接用整格范围覆盖右上角,
	//    不需要像以前那样拿中心点 ±57 估范围再夹边界
	t := core.Exec("可交易", it.X1, it.Y1, it.X2, it.Y2)
	if !t.Found {
		core.LogWithTimef("[按名使用物品] %s 格子(%d,%d)-(%d,%d)内没找到'可交易' → 判定不可交易, 跳过",
			it.Name, it.X1, it.Y1, it.X2, it.Y2)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 检测到'可交易'(相似度%.4f) → 可上架, 点击物品 (%d,%d)",
		it.Name, t.Sim, it.X, it.Y)
	core.RandomClickInAreaWithDelay(it.X, it.Y, it.X+5, it.Y+5)

	// ② 点"交易行看价"按钮(点物品后弹出的菜单/面板上的按钮)
	v := core.Exec("交易行看价")
	if !v.Found {
		core.LogWithTimef("[按名使用物品] %s 点物品后没找到'交易行看价'按钮(菜单没弹出来?) → 本轮放弃, 下轮再试", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] 点击'交易行看价' (%d,%d)", v.X, v.Y)
	core.RandomClickInAreaWithDelay(v.X, v.Y, v.X+5, v.Y+5)

	// ③ 等交易行加载 3 秒
	core.RandomSleep(3000, 5000)

	// ④ 读"最低价格"栏 → 存变量
	// 带强化等级的装备(配置了 装备强化等级, 如 +6/+8/+10): 行情列表里价格栏随等级行变化,
	// 默认区域只盖住最上面一行, 直接读会读串档位 —— 走 readMarketLevelPrice:
	// 点"购买清单" → 找当前等级的"+N"标记 → 按偏移取该行价格栏再取文;
	// 不带等级的普通物品维持原样, 按"最低价格"预设默认区域(878,225,1080,303)取文。
	var r *core.RecogResult
	if want.装备强化等级 > 0 {
		r = p.readMarketLevelPrice(it, want)
	} else {
		r = core.Exec("最低价格")
	}
	if !r.Found {
		core.LogWithTimef("[按名使用物品] %s 交易行界面没读到'最低价格'内容(recogs 预设'最低价格') → 本轮放弃", it.Name)
		return false
	}
	// 原文可能是 "3.56" / "40,000" 等: 抠出价格数字并去千分位逗号(40,000→40000),
	// 小数原样保留(3.56→3.56); 没抽到数字按没读到价格处理
	price := parsePriceText(r.Word)
	if price == "" {
		core.LogWithTimef("[按名使用物品] %s '最低价格'原文 %q 里没抽到数字 → 本轮放弃", it.Name, r.Word)
		return false
	}
	// 价格 key 带颜色后缀: 同名不同品质(颜色)的价格可能不同, 各存各的, 防止后面登记时串价
	key := "上架最低价_" + it.Name + wantColorTag(want)
	ctx.Data[key] = price
	core.LogWithTimef("[按名使用物品] %s%s 最低价格栏取文: 原文 %q → 规范化 %q → 已存变量 %s",
		it.Name, wantColorTag(want), r.Word, price, key)

	// ⑤⑥ 价格已拿到, 继续贩售流程(点"贩售"→按分类点页签→识别→点物品)。
	// 这里只记日志不改变返回值: 贩售失败不重试(查价标记已由调用方设上, 下轮不再重复查)
	p.sellFindOnExchange(it, want, tab, ctx)

	return true
}

// readMarketLevelPrice 读"带强化等级装备"在交易行行情列表里该等级档位的最低价格:
//
//	带强化等级的装备行情行位置随等级变化, "最低价格"预设的默认区域(878,225,1080,303)
//	只盖住列表最上面一行, 直接读会读到别的档位 —— 所以流程不一样:
//	① 先点"购买清单"(recogs 预设"购买清单", 找字 sim0.7, 区域 1077,236,1237,307)
//	   → 切到带强化等级装备的行情列表页
//	② 在等级列(recogs 预设"+6"/"+8"/"+10", 找字, 区域 241,214,334,654)找
//	   当前装备强化等级对应的"+N"标记 —— 行情列表里带强化等级的装备只会出现
//	   +6/+8/+10 三档(档位不在表里 = 行情里没有对应等级的行, 放弃本轮);
//	   命中坐标(返回中间坐标 = 标记中心)就落在该等级档位的行上
//	③ 命中坐标按 (578,-57,753,13) 偏移出该行"最低价格"栏:
//	   价格栏 = (X+578, Y-57)-(X+753, Y+13), 用 recogs 预设"最低价格"(取文类)
//	   覆盖该区域取文返回
//
// 返回 !Found = 没找到购买清单 / 档位不在 +6/+8/+10 / 该等级行没出现 / 价格栏取文失败;
// 调用方按原"没读到价格"逻辑放弃本轮, 下轮巡检再试。
func (p *按名使用物品) readMarketLevelPrice(it bagrecog.Item, want itemUse) *core.RecogResult {
	// ① 点"购买清单"切到行情列表页
	b := core.Exec("购买清单")
	if !b.Found {
		core.LogWithTimef("[按名使用物品] %s 强化装备查价: 交易行界面里没找到'购买清单'(recogs 预设'购买清单') → 本轮放弃",
			it.Name)
		return &core.RecogResult{}
	}
	core.LogWithTimef("[按名使用物品] %s 强化装备查价: 点击'购买清单'(%d,%d) sim=%.4f", it.Name, b.X, b.Y, b.Sim)
	core.RandomClickInAreaWithDelay(b.X, b.Y, b.X+5, b.Y+5)
	core.RandomSleep(1500, 2000) // 等列表切到购买清单并刷新

	// ② 找当前强化等级对应的行标记(行情列表里只有 +6/+8/+10 三档)
	preset := marketLevelPreset[want.装备强化等级]
	if preset == "" {
		core.LogWithTimef("[按名使用物品] %s 强化装备查价: 配置的装备强化等级%d 不在行情列表出现的档位(+6/+8/+10)里 → 本轮放弃",
			it.Name, want.装备强化等级)
		return &core.RecogResult{}
	}
	m := core.Exec(preset)
	if !m.Found {
		core.LogWithTimef("[按名使用物品] %s 强化装备查价: '购买清单'页里没找到'%s'(对应等级%d)的行 → 本轮放弃",
			it.Name, preset, want.装备强化等级)
		return &core.RecogResult{}
	}

	// ③ 命中坐标(标记中心)按偏移量叠出该行"最低价格"栏, 用"最低价格"预设覆盖该区域取文
	rx1, ry1, rx2, ry2 := m.X+marketPriceOffX1, m.Y+marketPriceOffY1, m.X+marketPriceOffX2, m.Y+marketPriceOffY2
	core.LogWithTimef("[按名使用物品] %s 强化装备查价: 找到'%s'(%d,%d) → 该行价格栏区域(%d,%d)-(%d,%d) 用'最低价格'预设取文",
		it.Name, preset, m.X, m.Y, rx1, ry1, rx2, ry2)
	return core.Exec("最低价格", rx1, ry1, rx2, ry2)
}

// sellFindOnExchange 贩售流程（Mode="上架交易行"查价成功后的后续, 先写到"点到物品"为止）:
//
//	⑤ 点"贩售"按钮(recogs 预设"贩售", 截图区域 200,66,290,100 配在 assets/recogs.json)
//	⑤-2 进贩售页先点"一键结算"(recogs 预设, 截图区域 602,638,730,687) —— 一次任务只点一次
//	⑤-2b 点完结算循环检测"点击关闭"(WaitFind 6秒超时/间隔1秒) → 找到即点掉并停止循环,
//	      等不到就当没弹窗继续(不卡流程)
//	⑤-3 再点"一次收回"(recogs 预设, 截图区域 29,634,157,684) —— 一次任务只点一次
//	     (两个都用 ctx.Data 标记, 一次 Execute 全程共享; 任务重新执行时 Data 重建
//	      → 下次进任务才会再各点一下。同一次任务里重复进贩售页不再点, 防点到卖出东西)
//	⑥ 按物品所属背包页签分类 tab 依次点页签找它: 点一个页签(recogs 预设, 页签条区域
//	   596,109,1232,190) → 等列表刷新 → 用 bagrecog.RecognizeRegion(763,187,1256,579)
//	   识别该区域物品 → 找到 名字 == it.Name 且 颜色符合 want.Color(与背包同一套过滤)
//	   的就点击它(格子中心) → 调 sellRegister 走登记流程
//	   (max → 登记数量 → 登录价格输入 → 确定 → 金额校验 → 登记物品);
//	   找不到就点下一个候选页签(如"其他"类要依次试 背包页签其他 → 背包页签图鉴)。
//
// 纯后处理: 价格已经存进 ctx.Data, 这里任何失败都只记日志不重试, 不影响主流程返回。
func (p *按名使用物品) sellFindOnExchange(it bagrecog.Item, want itemUse, tab string, ctx *Context) {
	// ⑤ 点"贩售"进上架/出售界面
	s := core.Exec("贩售")
	if !s.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到'贩售'按钮(当前不在交易行界面?) → 贩售流程跳过", it.Name)
		p.recordSellFail(ctx, it, want)
		return
	}
	core.LogWithTimef("[按名使用物品] 点击'贩售' (%d,%d)", s.X, s.Y)
	core.RandomClickInAreaWithDelay(s.X, s.Y, s.X+5, s.Y+5)
	core.RandomSleep(1500, 2000)
	if p.sellClickOnce(ctx, "一次收回", true) {
		// 结算会弹结算结果窗: 循环检测"点击关闭"(最多6秒, 每轮约1秒) → 找到就点掉并停止循环;
		// 6秒内一直没出现(比如这次没东西可结算) → 记日志继续走后面的流程, 不卡住。
		cc := WaitFind(6, 1, "点击关闭")
		if cc == nil || !cc.Found {
			core.LogWithTimef("[按名使用物品] 点'一键结算'后 6 秒内没等到'点击关闭' → 当成没有结算弹窗, 继续")
		} else {
			core.LogWithTimef("[按名使用物品] 结算弹窗'点击关闭' (%d,%d) → 点掉", cc.X, cc.Y)
			core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
		}
	}

	// ⑤-2/⑤-3 进贩售页先收"一键结算"/"一次收回": 两个都是一次任务只点一次
	// (标记放 ctx.Data, 同一次任务里后面再进贩售页直接跳过; 下个任务进来才再点一下)
	if p.sellClickOnce(ctx, "一键结算", true) {
		// 结算会弹结算结果窗: 循环检测"点击关闭"(最多6秒, 每轮约1秒) → 找到就点掉并停止循环;
		// 6秒内一直没出现(比如这次没东西可结算) → 记日志继续走后面的流程, 不卡住。
		cc := WaitFind(6, 1, "点击关闭")
		if cc == nil || !cc.Found {
			core.LogWithTimef("[按名使用物品] 点'一键结算'后 6 秒内没等到'点击关闭' → 当成没有结算弹窗, 继续")
		} else {
			core.LogWithTimef("[按名使用物品] 结算弹窗'点击关闭' (%d,%d) → 点掉", cc.X, cc.Y)
			core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
		}
	}

	tabs := sellFindTabs[tab]
	if len(tabs) == 0 {
		core.LogWithTimef("[按名使用物品] %s 分类 %q 没有配置贩售查找页签 → 贩售流程跳过", it.Name, tab)
		p.recordSellFail(ctx, it, want)
		return
	}
	// ⑥ 按分类依次点页签找物品
	for _, tabName := range tabs {
		if IsStopped() {
			return
		}
		tb := core.Exec(tabName)
		if !tb.Found {
			core.LogWithTimef("[按名使用物品] 贩售页没找到页签【%s】 → 试下一个", tabName)
			continue
		}
		core.LogWithTimef("[按名使用物品] 点击贩售页签【%s】(%d,%d)", tabName, tb.X, tb.Y)
		core.RandomClickInAreaWithDelay(tb.X, tb.Y, tb.X+5, tb.Y+5)
		core.RandomSleep(1500, 2000)

		// 该页签下识别物品区域(763,187,1256,579，上交交易行背包区域), 找同名目标物品。
		// 该面板物品比背包整体小 1~2px, 归一化后哈希仍有微小差异, 阈值放宽到 0.89
		// (背包默认 0.9 在这儿会卡掉一部分), 用带阈值参数的接口。
		found := bagrecog.RecognizeRegionSim(763, 187, 1256, 579, 0.85)
		hit := false
		for _, fi := range found {
			// 名字对上还不够: 贩售面板里同名不同品质的物品会一起列出来
			// (如 閃耀雕工之石 有红有蓝), 用同一条规则的 Color 再过滤一次, 防止点错品质
			if fi.Name == it.Name && itemColorOK(fi, want) {
				core.LogWithTimef("[按名使用物品] 贩售页签【%s】找到 %s (相似度%.4f) 格子(%d,%d) → 点击",
					tabName, fi.Name, fi.Sim, fi.X, fi.Y)
				core.RandomClickInAreaWithDelay(fi.X, fi.Y, fi.X+5, fi.Y+5)
				hit = true
				break
			}
		}
		if hit {
			core.LogWithTimef("[按名使用物品] %s%s 已在贩售界面点选 → 走登记流程",
				it.Name, wantColorTag(want))
			p.sellRegister(ctx, it, want)
			return
		}
		core.LogWithTimef("[按名使用物品] 贩售页签【%s】里没有 %s → 试下一个页签", tabName, it.Name)
	}
	// 候选页签都试过也没有该物品 → 没正确上架, 记录(价格已存, 下次任务会再试)
	core.LogWithTimef("[按名使用物品] %s 候选页签都没找到(分类 %q) → 贩售流程结束, 价格已存", it.Name, tab)
	p.recordSellFail(ctx, it, want)
}

// sellRegister 贩售登记流程（sellFindOnExchange 点到贩售物品之后的那串操作）:
//
//	⑦ 点"max"(recogs 已有预设) —— 选最大可登记数量
//	⑧ 取"登记数量"(recogs 预设"登记数量", 取文类, 区域 733,177,806,258)
//	   —— 面板上这个是纯整数, 没有小数点
//	⑨ 点"登录价格输入"(recogs 预设, 字库 399,262,489,302 + 坐标偏移 213,48)
//	   → 系统输入框弹出 → 输入 最低价格 * 登记数量 - 1 的整数金额
//	   (该框填的是总价; 金额不要小数, 一律向下取整 —— 宁可少要一块也不多标)
//	   算出金额 < 10 → 不值得上架 → 跳过本次登记(失败收尾统一在 defer 点两次"关闭")
//	   → 点"确定"(recogs 已有预设)
//	⑩ 取"每件贩售金额"(取文 663,379,933,413) 和 "总贩售"(取文 403,417,934,459)
//	   —— 两栏都可能带小数点, 必须按保留小数的数字解析(不能把所有数字拼一起)
//	⑪ 校验金额: 每件贩售金额 * 登记数量 == 总贩售 且 总贩售 == ⑨ 输入的整数金额
//	   (面板可能带小数, 留 0.01 容差) → 对得上才点"登记物品"(recogs 预设, 字库 563,537,731,589);
//	     对不上只记日志, 绝不点 —— 防止标错价把东西贱卖。
//	⑫ 点完"登记物品"后: 按钮找到了并点成功 → 登记成功弹窗只点一次"点击关闭";
//	   没找到按钮(残留弹窗) → 连点两次"点击关闭"收尾。
//
// 返回 true = 金额校验通过且已点"登记物品"; 其余情况一律 false。
// 任何一步失败都只记日志不重试(后处理流程, 不影响主流程)。
// want 是这条使用规则: 登记定价要用它取"该名字+该颜色"查好的价格(useListOnExchange 存的 key 带颜色),
// 防止同名不同品质的物品读到串了的价格。
//
// 失败记录: 返回值是 false(没正确上架: 某步失败 / 金额<10跳过 / 登记物品按钮没找到)时,
// defer 统一把该物品(名+颜色)记进 ctx.Data 的"上架登记失败_本次"名单(去重)并打一条日志,
// 跑完可从日志看到本次有哪些物品没上架成功; 成功(return true)不记录。
func (p *按名使用物品) sellRegister(ctx *Context, it bagrecog.Item, want itemUse) (listed bool) {
	// defer: 拦截所有 return false —— 失败/跳过统一记录; return true 不记录。
	// 半路失败退出(已在贩售界面): 一律用"关闭"预设关掉交易行/贩售界面,
	// 防止界面残留卡住后续流程。失败路径统一在 defer 点两次"关闭"收尾, 这里 WaitFind
	// 会找不到(界面已关) → 自动跳过, 不会多点。
	defer func() {
		// 收尾关闭: 成功点1次, 失败点2次(失败还要记失败名单)
		n := 1
		if !listed {
			p.recordSellFail(ctx, it, want)
			n = 2
		}
		for i := 0; i < n; i++ {
			cc := WaitFind(6, 1, "关闭")
			if cc == nil || !cc.Found {
				core.LogWithTimef("[按名使用物品] %s 收尾第%d次没等到'关闭'(无残留弹窗) → 结束", it.Name, i+1)
				return
			}
			core.LogWithTimef("[按名使用物品] %s 点击'关闭' (%d,%d) 收尾 %d/%d", it.Name, cc.X, cc.Y, i+1, n)
			core.RandomClickInAreaWithDelay(cc.X, cc.Y, cc.X+5, cc.Y+5)
		}
	}()

	// ⑦ max：选最大数量
	m := core.Exec("max")
	if !m.Found {
		core.LogWithTimef("[按名使用物品] %s 贩售面板没找到'max' → 放弃本次登记", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] 点击'max' (%d,%d)", m.X, m.Y)
	core.RandomClickInAreaWithDelay(m.X, m.Y, m.X+5, m.Y+5, 5000)

	// ⑧ 登记数量（纯整数）
	qn := core.Exec("登记数量")
	if !qn.Found || qn.Word == "" {
		core.LogWithTimef("[按名使用物品] %s 取文'登记数量'失败 → 放弃本次登记", it.Name)
		return false
	}
	qty, ok := parseIntText(qn.Word)
	if !ok || qty <= 0 {
		core.LogWithTimef("[按名使用物品] %s '登记数量'取文=%q 抠不出正整数 → 放弃本次登记", it.Name, qn.Word)
		return false
	}

	// ⑨ 算要输入的金额 = 最低价格 * 登记数量, 向下取整成整数
	//    ("登录价格输入"填的是总价 = 每件卖最低价 x 数量;
	//    金额输入要整数不要小数 —— 宁可少要一块也不多标;
	//    面板上的"每件贩售金额/总贩售"取文仍保留小数(见⑩), 这里只是输入取整)
	//    单价(最低价格) >= 10 块 → 减 1(比行情最低价低一块, 更易卖出);
	//    单价 < 10 块 → 不减 1(用户要求: 便宜物品不减价)。
	// 取价 key 带颜色(useListOnExchange 存价时按同一条规则写入, key 一致)
	priceStr, _ := ctx.Data["上架最低价_"+it.Name+wantColorTag(want)].(string)
	price, ok := parseFloatText(priceStr)
	if !ok || price <= 0.1 {
		core.LogWithTimef("[按名使用物品] %s 最低价格=%q 无效(没查过价/≤0.1) → 放弃本次登记", it.Name, priceStr)
		return false
	}
	var raw float64
	minusNote := "不减1(单价<=10)"
	if price > 10 {
		raw = price*float64(qty) - 1 // 单价 > 10 块 → 减1(比行情低一块好卖)
		minusNote = "减1"
	} else {
		raw = price * float64(qty) // 单价 <= 10 块 → 不减1
	}
	inputVal := int(math.Floor(raw)) // 输入要整数 → 向下取整
	if inputVal < 10 {
		// 要输入的总价 < 10(含<=0) → 不值得上架 → 跳过本次登记(失败收尾由 defer 点两次"关闭")
		core.LogWithTimef("[按名使用物品] %s 输入金额 %d (最低价%v x 数量%d %s = %v) < 10 → 跳过本次上架",
			it.Name, inputVal, price, qty, minusNote, raw)
		return false
	}

	pi := core.Exec("登录价格输入")
	if !pi.Found {
		core.LogWithTimef("[按名使用物品] %s 没找到'登录价格输入'文本框 → 放弃本次登记", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 登记数量=%d, 最低价=%v → 输入金额 %d (最低价%v x 数量%d %s = %v 向下取整)",
		it.Name, qty, price, inputVal, price, qty, minusNote, raw)
	core.LogWithTimef("[按名使用物品] 点击'登录价格输入' (%d,%d) → 输入 %d", pi.X, pi.Y, inputVal)
	core.RandomClickInAreaWithDelay(pi.X, pi.Y, pi.X+5, pi.Y+5)
	ime.InputText(strconv.Itoa(inputVal))

	cf := core.Exec("确定")
	if !cf.Found {
		core.LogWithTimef("[按名使用物品] %s 输入框没找到'确定' → 放弃本次登记", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] 点击'确定' (%d,%d)", cf.X, cf.Y)
	core.RandomClickInAreaWithDelay(cf.X, cf.Y, cf.X+5, cf.Y+5, 6000)

	// ⑩ 读回面板上的两个金额（可能带小数点, 如 3.56 —— 必须用保留小数的解析,
	//    不能 parseIntText: 它会把所有数字拼一起, "3.56" 会被抠成 356)
	pu := core.Exec("每件贩售金额")
	to := core.Exec("总贩售")
	perUnit, okPer := parseFloatText(pu.Word)
	total, okTotal := parseFloatText(to.Word)
	if !okPer || !okTotal {
		core.LogWithTimef("[按名使用物品] %s 金额取文失败 每件=%q 总计=%q → 不点登记物品(防卖错价)",
			it.Name, pu.Word, to.Word)
		return false
	}

	// ⑪ 校验：面板"总贩售" == 我们输入的金额(inputVal) == 每件贩售金额 x 数量
	//    （例: 我们输入 500, 面板每件125 x 数量4 = 总贩售500）
	//    面板金额可能带小数(如 500.00), 浮点有精度误差 → 全用 0.01 容差比
	if math.Abs(total-float64(inputVal)) > 1 || math.Abs(perUnit*float64(qty)-total) > 1 {
		core.LogWithTimef("[按名使用物品] %s 金额对不上: 每件%v x 数量%d = %v, 面板总贩售=%v, 我们输入=%d → 不点登记物品",
			it.Name, perUnit, qty, perUnit*float64(qty), total, inputVal)
		return false
	}

	rg := core.Exec("登记物品")
	if !rg.Found {
		// 没找到"登记物品"按钮(界面已是结束态/残留弹窗) → 失败收尾由 defer 点两次"关闭"
		core.LogWithTimef("[按名使用物品] %s 金额校验通过但没找到'登记物品'按钮 → 失败收尾", it.Name)
		return false
	}
	core.LogWithTimef("[按名使用物品] %s 金额校验通过(每件%v x %d = 总贩售%v) → 点击'登记物品' (%d,%d)",
		it.Name, perUnit, qty, total, rg.X, rg.Y)
	core.RandomClickInAreaWithDelay(rg.X, rg.Y, rg.X+5, rg.Y+5, 0)

	// 登记物品之后 → 循环检测"无法登录更多"(recogs 预设, 最多6秒, 间隔1秒, 检测到即停)
	// 检测到 = 登记位已满 → 标记 ctx.Data: 本次任务后续一律不再识别/上架(下次任务重进自动恢复)
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if IsStopped() {
			break
		}
		if rf := core.Exec("无法登录更多"); rf.Found {
			ctx.Data[registerFullKey] = true
			core.LogWithTimef("[按名使用物品] 检测到'无法登录更多' (%d,%d) → 登记位已满, 本次任务后续不再识别/上架", rf.X, rf.Y)
			break
		}
		core.RandomSleep(1000, 1000)
	}

	// ⑫ 登记成功: 收尾关闭交给 defer(成功路径点一次"关闭")
	return true
}

// recordSellFail 记录"没正确上架"的物品(登记失败 / 金额<10跳过 / 贩售按钮或物品没找到):
// 记录条目 = 物品名 + 颜色后缀(同名不同品质各自记录, 去重时不会漏掉另一种品质);
// want.Color 留空时后缀为空, 与之前一致。名单去重追加进 ctx.Data 并打一条日志;
// Data 一次任务执行共享、任务重跑时重建, 所以每次任务都会从 0 累计, 跑完可从日志看到完整名单。
func (p *按名使用物品) recordSellFail(ctx *Context, it bagrecog.Item, want itemUse) {
	entry := it.Name + wantColorTag(want)
	list, _ := ctx.Data[sellFailListKey].([]string)
	for _, n := range list {
		if n == entry {
			return // 本次任务已记过, 不重复
		}
	}
	list = append(list, entry)
	ctx.Data[sellFailListKey] = list
	core.LogWithTimef("[按名使用物品] 记录: %s 本次未正确上架(登记失败/跳过), 本次任务累计未上架: %v",
		entry, list)
}

// parseIntText 从 OCR 原文里抠整数（登记数量/贩售金额面板都是纯整数，没有小数点）:
// 丢掉所有非数字字符（含千分位逗号、单位、空格）后取剩下的数字串；
// 抠不出来返回 false。
func parseIntText(text string) (int, bool) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, text)
	if digits == "" {
		return 0, false
	}
	v, err := strconv.Atoi(digits)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseFloatText 从 OCR 原文里抠价格数字并解析成 float（最低价格可能是 3.56 这种小数）:
// 复用 priceNumRe 抠第一段数字（含小数），再去千分位逗号后 ParseFloat。
func parseFloatText(text string) (float64, bool) {
	m := priceNumRe.FindString(text)
	if m == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// sellClickOnce 在贩售页点一个字库按钮(按钮的识别文字/区域/阈值全配在 assets/recogs.json,
// 这里只用 preset 名字)。once=true = 同一次任务运行内只点一次:
// 标记放 ctx.Data(一次 Execute 全程共享), 任务重新执行时 Data 重建 → 下次进任务才会再点一下。
// 找不到按钮(灰按钮/界面不在贩售页)只记日志跳过, 返回 false —— 后处理流程不重试。
func (p *按名使用物品) sellClickOnce(ctx *Context, preset string, once bool) bool {
	markKey := "贩售已点_" + preset
	if once {
		if done, _ := ctx.Data[markKey].(bool); done {
			core.LogWithTimef("[按名使用物品] %s 本次任务已点过 → 跳过", preset)
			return false
		}
	}
	r := core.Exec(preset)
	if !r.Found {
		core.LogWithTimef("[按名使用物品] 贩售页没找到%s(按钮不可用/不在贩售页?) → 跳过", preset)
		return false
	}
	core.LogWithTimef("[按名使用物品] 点击%s (%d,%d)", preset, r.X, r.Y)
	core.RandomClickInAreaWithDelay(r.X, r.Y, r.X+5, r.Y+5, 0)
	if once {
		ctx.Data[markKey] = true
	}
	return true
}

// priceNumRe 匹配价格数字: 千分位(40,000 / 1,234.56) 或 普通整数/小数(3.56 / 40000)
var priceNumRe = regexp.MustCompile(`\d{1,3}(?:,\d{3})+(?:\.\d+)?|\d+(?:\.\d+)?`)

// parsePriceText 从 OCR 取文原文里抠出价格数字并规范化:
//   - 整数/小数原样保留: 3.56 → 3.56, 40000 → 40000
//   - 千分位逗号去掉: 40,000 → 40000, 1,234.56 → 1234.56
//   - 原文没数字 → 返回空串
func parsePriceText(text string) string {
	return strings.ReplaceAll(priceNumRe.FindString(text), ",", "")
}
