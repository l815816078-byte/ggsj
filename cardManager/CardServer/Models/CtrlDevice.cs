namespace CardServer.Models;

// 中控设备实体（SQLite 表 CtrlDevices）
// 一台设备 = 一个脚本窗口（WindowId 每次进程启动随机生成）
// 字段除身份信息外均为「最近一次上报值」，离线时保留旧值供中控台显示（置灰）
public class CtrlDevice
{
    public int Id { get; set; }

    // 脚本窗口 ID（设备端每次启动随机生成，与 WindowTable 的 windowId 同一概念）
    public string WindowId { get; set; } = "";

    // 所属卡密
    public string CardCode { get; set; } = "";

    // 设备号（启动界面输入，同一卡密内唯一）
    public string DeviceNo { get; set; } = "";

    // ── 角色数据（识别缺失时为空/0，见方案"识别缺失"说明）──
    // 区服
    public string Server { get; set; } = "";
    // 角色名
    public string Role { get; set; } = "";
    // 职业
    public string Job { get; set; } = "";
    // 等级
    public int Level { get; set; }
    // 战力
    public int Power { get; set; }
    // 金币
    public long Gold { get; set; }
    // 钻石（当前值）
    public long Diamond { get; set; }
    // 今日钻石（当前 - 今日基准，负数记 0）
    public long TodayDiamond { get; set; }
    // 当月钻石（当前 - 当月基准，负数记 0）
    public long MonthDiamond { get; set; }
    // 背包剩余格数
    public int BagFree { get; set; }

    // 当前使用的配置方案名（服务端记账）
    public string SchemeName { get; set; } = "";

    // 状态：running=运行中 paused=暂停 stopped=停止 offline=离线
    public string State { get; set; } = "offline";

    // 最近一次心跳/上报时间（Unix 秒）
    public long LastSeen { get; set; }
}
