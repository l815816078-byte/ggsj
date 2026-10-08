using CardServer.Data;
using CardServer.Models;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Services;

// 后台任务：每分钟检查一次是否跨天/跨月，需要时重置钻石基准。
// （跨天 → 重置今日基准；跨月 → 同时重置当月基准）
public class DiamondResetWorker : BackgroundService
{
    private readonly DiamondStats _stats;

    public DiamondResetWorker(DiamondStats stats)
    {
        _stats = stats;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        // 启动后先等 10 秒，避开服务启动阶段
        try
        {
            await Task.Delay(TimeSpan.FromSeconds(10), stoppingToken);
        }
        catch (TaskCanceledException)
        {
            return;
        }

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                _stats.ResetIfNeeded();
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[DiamondResetWorker] 重置异常: {ex.Message}");
            }

            try
            {
                await Task.Delay(TimeSpan.FromMinutes(1), stoppingToken);
            }
            catch (TaskCanceledException)
            {
                break;
            }
        }
    }
}
