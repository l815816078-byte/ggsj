namespace CardServer.Models;

// 卡密实体（SQLite 表 Cards）
public class Card
{
    public int Id { get; set; }

    // 卡号 FY-XXXX-XXXX-XXXX（数据库唯一索引防重）
    public string Code { get; set; } = "";

    // 有效天数（30/60/90…）
    public int Days { get; set; }

    // 最大并发窗口数（1/3/8）
    public int MaxWindows { get; set; }

    // 状态: unused=未使用 active=已激活 disabled=已禁用
    public string Status { get; set; } = "unused";

    // 首次激活时间（Unix 秒）
    public long ActivateAt { get; set; }

    // 到期时间（Unix 秒）；续费时 = max(now, ExpireAt) + Days*86400
    public long ExpireAt { get; set; }

    // 发卡者用户名，管理员为 "admin"，代理为代理用户名
    public string Owner { get; set; } = "admin";

    // 备注（如"卖给老王"）
    public string Remark { get; set; } = "";

    // 发卡批次 UUID（导出追溯）
    public string BatchId { get; set; } = "";

    // 所属项目（分类）名称，发卡时指定
    public string Project { get; set; } = "";

    // 已绑定设备指纹列表（逗号分隔；方案 B 设备绑定）
    public string BindFPs { get; set; } = "";

    // 连续激活失败次数（防爆破）
    public int FailCount { get; set; }

    // 锁定截止时间（Unix 秒），0 表示未锁定
    public long LockUntil { get; set; }
}
