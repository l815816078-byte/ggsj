namespace CardServer.Models;

// 操作日志实体（SQLite 表 OperationLogs）
public class OperationLog
{
    public int Id { get; set; }

    // 操作时间（Unix 秒）
    public long Time { get; set; }

    // 操作者用户名
    public string Actor { get; set; } = "";

    // 动作: 发卡/禁用/启用/解绑/延期/充额度/创建代理/登录/登出
    public string Action { get; set; } = "";

    // 详情（文本或 JSON）
    public string Detail { get; set; } = "";
}
