using CardServer.Models;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Data;

public class AppDbContext : DbContext
{
    public AppDbContext(DbContextOptions<AppDbContext> options) : base(options) { }

    public DbSet<Card> Cards => Set<Card>();
    public DbSet<Agent> Agents => Set<Agent>();
    public DbSet<OperationLog> OperationLogs => Set<OperationLog>();
    public DbSet<Project> Projects => Set<Project>();

    // ── 中控相关 ──
    public DbSet<CtrlDevice> CtrlDevices => Set<CtrlDevice>();
    public DbSet<CtrlConfig> CtrlConfigs => Set<CtrlConfig>();
    public DbSet<DiamondBase> DiamondBases => Set<DiamondBase>();
    public DbSet<HangPoint> HangPoints => Set<HangPoint>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        // 卡号唯一索引：防止同一卡密入库两次
        modelBuilder.Entity<Card>()
            .HasIndex(c => c.Code)
            .IsUnique();

        // 用户名唯一索引
        modelBuilder.Entity<Agent>()
            .HasIndex(a => a.Username)
            .IsUnique();

        // 项目名称唯一索引
        modelBuilder.Entity<Project>()
            .HasIndex(p => p.Name)
            .IsUnique();

        // 中控设备：按「卡密 + 设备号」唯一（同一台设备重启脚本不新增行，只更新 WindowId/状态）
        // 注：WindowId 每次进程启动随机生成，不能作为唯一键，否则重启就会多出一行"离线"残留。
        modelBuilder.Entity<CtrlDevice>()
            .HasIndex(d => new { d.CardCode, d.DeviceNo })
            .IsUnique();
        // 中控设备：按卡密查询
        modelBuilder.Entity<CtrlDevice>()
            .HasIndex(d => d.CardCode);

        // 中控配置：同卡同方案唯一
        modelBuilder.Entity<CtrlConfig>()
            .HasIndex(c => new { c.CardCode, c.SchemeName })
            .IsUnique();

        // 钻石基准：每设备一份
        modelBuilder.Entity<DiamondBase>()
            .HasIndex(b => b.WindowId)
            .IsUnique();

        // 挂机点：名称唯一
        modelBuilder.Entity<HangPoint>()
            .HasIndex(h => h.Name)
            .IsUnique();
    }
}
