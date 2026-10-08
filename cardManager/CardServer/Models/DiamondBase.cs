namespace CardServer.Models;

// 钻石基准实体（SQLite 表 DiamondBases）
// 今日/当月钻石 = 当前钻石 - 基准；基准每天 0 点 / 每月 1 号重置，设备首次上线时 = 当前值
public class DiamondBase
{
    public int Id { get; set; }

    // 设备窗口 ID（每台设备一份基准）
    public string WindowId { get; set; } = "";

    // 今日基准（当天 0 点时的钻石数）
    public long TodayBase { get; set; }

    // 当月基准（当月 1 号 0 点时的钻石数）
    public long MonthBase { get; set; }

    // 基准最后更新时间（Unix 秒）
    public long UpdatedAt { get; set; }
}
