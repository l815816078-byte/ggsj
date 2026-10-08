using CardServer.Data;
using CardServer.Models;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Services;

// 今日/当月钻石统计（A 净增方案）：
//   今日钻石 = 当前钻石 - 今日基准
//   当月钻石 = 当前钻石 - 当月基准
//   基准：设备首次上报时 = 当前值；每天 0 点重置今日基准；每月 1 号重置当月基准。
//   负数统一记 0（防止换号/回档导致负值）。
public class DiamondStats
{
    private readonly IServiceScopeFactory _scopeFactory;

    public DiamondStats(IServiceScopeFactory scopeFactory)
    {
        _scopeFactory = scopeFactory;
    }

    // 计算并回填设备的今日/当月钻石；首次见到的设备先建基准
    // 返回 (今日钻石, 当月钻石)
    public (long today, long month) Apply(string windowId, long currentDiamond)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();

        var baseRow = db.DiamondBases.FirstOrDefault(b => b.WindowId == windowId);
        var now = DateTimeOffset.UtcNow.ToUnixTimeSeconds();

        if (baseRow == null)
        {
            // 首次上线：今日基准 = 当月基准 = 当前值
            baseRow = new DiamondBase
            {
                WindowId = windowId,
                TodayBase = currentDiamond,
                MonthBase = currentDiamond,
                UpdatedAt = now
            };
            db.DiamondBases.Add(baseRow);
            db.SaveChanges();
            return (0, 0);
        }

        long today = currentDiamond - baseRow.TodayBase;
        long month = currentDiamond - baseRow.MonthBase;
        return (today < 0 ? 0 : today, month < 0 ? 0 : month);
    }

    // 定时重置：每天 0 点把所有设备的今日基准设为「当前钻石」；
    // 每月 1 号额外把当月基准也设为「当前钻石」。
    // 由后台任务每分钟检查一次，跨天/跨月时执行一次。
    public void ResetIfNeeded()
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var now = DateTimeOffset.Now;

        // 今日基准的"有效期"判断：用基准行的 UpdatedAt 是否早于今天 0 点
        // 注：DateTime 没有 Offset 属性，用 DateTimeOffset 取本地时区偏移
        var todayStart = new DateTimeOffset(now.Date, now.Offset).ToUnixTimeSeconds();
        var monthStart = new DateTimeOffset(new DateTime(now.Year, now.Month, 1), now.Offset).ToUnixTimeSeconds();

        var bases = db.DiamondBases.ToList();
        if (bases.Count == 0)
        {
            return;
        }

        // 需要重置的设备：查它当前钻石（取 CtrlDevices 的最新值；查不到则跳过）
        var devices = db.CtrlDevices.ToDictionary(d => d.WindowId, d => d.Diamond);
        bool changed = false;

        foreach (var b in bases)
        {
            if (b.UpdatedAt >= todayStart)
            {
                continue; // 今天已经重置过
            }

            // 用设备当前钻石作为新基准；设备不存在时用旧基准（避免归零）
            long cur = devices.TryGetValue(b.WindowId, out var d) ? d : b.TodayBase;

            b.TodayBase = cur;
            if (b.UpdatedAt < monthStart)
            {
                // 跨月：当月基准也重置
                b.MonthBase = cur;
            }
            b.UpdatedAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
            changed = true;
            Console.WriteLine($"[DiamondStats] 重置基准: win={b.WindowId} today={cur} month={b.MonthBase}");
        }

        if (changed)
        {
            db.SaveChanges();
        }
    }
}
