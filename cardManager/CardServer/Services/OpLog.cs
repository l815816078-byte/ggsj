using CardServer.Data;
using CardServer.Models;

namespace CardServer.Services;

// 操作日志写入工具：发卡/禁用/启用/解绑/延期/充额度/创建代理/登录统一留痕
public static class OpLog
{
    public static void Write(AppDbContext db, string actor, string action, string detail)
    {
        db.OperationLogs.Add(new OperationLog
        {
            Time = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
            Actor = actor,
            Action = action,
            Detail = detail
        });
        db.SaveChanges();
    }
}
