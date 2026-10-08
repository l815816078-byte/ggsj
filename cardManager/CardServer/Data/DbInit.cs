using CardServer.Models;
using CardServer.Services;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Data;

// 数据库初始化：WAL 模式 + 建表 + 种子管理员账号
public static class DbInit
{
    public static void Initialize(IServiceProvider services, IConfiguration config)
    {
        using var scope = services.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();

        // 先建库建表，再开 WAL（WAL 是数据库级 PRAGMA，须在库存在后设置）
        db.Database.EnsureCreated();
        db.Database.ExecuteSqlRaw("PRAGMA journal_mode=WAL;");

        // ── 旧库兼容：EnsureCreated 只在新库上建全表，老库缺的表/列需手动补 ──
        db.Database.ExecuteSqlRaw("""
            CREATE TABLE IF NOT EXISTS "Projects" (
                "Id" INTEGER NOT NULL CONSTRAINT "PK_Projects" PRIMARY KEY AUTOINCREMENT,
                "Name" TEXT NOT NULL,
                "CreatedAt" INTEGER NOT NULL
            );
            CREATE UNIQUE INDEX IF NOT EXISTS "IX_Projects_Name" ON "Projects" ("Name");
            """);

        // ── 中控相关新表（老库补建）──
        db.Database.ExecuteSqlRaw("""
            CREATE TABLE IF NOT EXISTS "CtrlDevices" (
                "Id" INTEGER NOT NULL CONSTRAINT "PK_CtrlDevices" PRIMARY KEY AUTOINCREMENT,
                "WindowId" TEXT NOT NULL,
                "CardCode" TEXT NOT NULL,
                "DeviceNo" TEXT NOT NULL,
                "Server" TEXT NOT NULL,
                "Role" TEXT NOT NULL,
                "Job" TEXT NOT NULL,
                "Level" INTEGER NOT NULL,
                "Power" INTEGER NOT NULL,
                "Gold" INTEGER NOT NULL,
                "Diamond" INTEGER NOT NULL,
                "TodayDiamond" INTEGER NOT NULL,
                "MonthDiamond" INTEGER NOT NULL,
                "BagFree" INTEGER NOT NULL,
                "SchemeName" TEXT NOT NULL,
                "State" TEXT NOT NULL,
                "LastSeen" INTEGER NOT NULL
            );
            CREATE INDEX IF NOT EXISTS "IX_CtrlDevices_WindowId" ON "CtrlDevices" ("WindowId");
            CREATE INDEX IF NOT EXISTS "IX_CtrlDevices_CardCode" ON "CtrlDevices" ("CardCode");

            CREATE TABLE IF NOT EXISTS "CtrlConfigs" (
                "Id" INTEGER NOT NULL CONSTRAINT "PK_CtrlConfigs" PRIMARY KEY AUTOINCREMENT,
                "CardCode" TEXT NOT NULL,
                "SchemeName" TEXT NOT NULL,
                "ConfigJson" TEXT NOT NULL,
                "HangPointId" INTEGER NOT NULL,
                "UpdatedAt" INTEGER NOT NULL
            );
            CREATE UNIQUE INDEX IF NOT EXISTS "IX_CtrlConfigs_CardCode_SchemeName" ON "CtrlConfigs" ("CardCode", "SchemeName");

            CREATE TABLE IF NOT EXISTS "DiamondBases" (
                "Id" INTEGER NOT NULL CONSTRAINT "PK_DiamondBases" PRIMARY KEY AUTOINCREMENT,
                "WindowId" TEXT NOT NULL,
                "TodayBase" INTEGER NOT NULL,
                "MonthBase" INTEGER NOT NULL,
                "UpdatedAt" INTEGER NOT NULL
            );
            CREATE UNIQUE INDEX IF NOT EXISTS "IX_DiamondBases_WindowId" ON "DiamondBases" ("WindowId");

            CREATE TABLE IF NOT EXISTS "HangPoints" (
                "Id" INTEGER NOT NULL CONSTRAINT "PK_HangPoints" PRIMARY KEY AUTOINCREMENT,
                "Name" TEXT NOT NULL,
                "MapName" TEXT NOT NULL,
                "MinLevel" INTEGER NOT NULL,
                "MinPower" INTEGER NOT NULL,
                "SwitchTimes" INTEGER NOT NULL,
                "Enabled" INTEGER NOT NULL
            );
            CREATE UNIQUE INDEX IF NOT EXISTS "IX_HangPoints_Name" ON "HangPoints" ("Name");
            """);

        // ── 中控设备表：清理历史重复行 + 强制重建唯一索引 ──
        // 背景：设备号在同一卡密下唯一，是设备身份。但早期版本建索引时库中已有重复行，
        // "CREATE UNIQUE INDEX IF NOT EXISTS" 会因唯一冲突失败并被静默吞掉，索引此后
        // 一直不存在 → 无约束 → 同一设备号被写出多行（中控台看到三行"1"）。
        // 这里先按「卡密 + 设备号」去重（保留 Id 最大 = 最新的一行），再无条件重建索引，
        // 保证每个部署实例上都真正带上唯一约束。
        db.Database.ExecuteSqlRaw("""
            DELETE FROM "CtrlDevices"
            WHERE "Id" NOT IN (
                SELECT MAX("Id") FROM "CtrlDevices" GROUP BY "CardCode", "DeviceNo"
            );
            DROP INDEX IF EXISTS "IX_CtrlDevices_CardCode_DeviceNo";
            CREATE UNIQUE INDEX IF NOT EXISTS "IX_CtrlDevices_CardCode_DeviceNo"
                ON "CtrlDevices" ("CardCode", "DeviceNo");
            """);

        var cardCols = db.Database
            .SqlQueryRaw<string>("SELECT name FROM pragma_table_info('Cards')")
            .ToList();
        if (!cardCols.Contains("Project"))
            db.Database.ExecuteSqlRaw("ALTER TABLE \"Cards\" ADD COLUMN \"Project\" TEXT NOT NULL DEFAULT ''");
        if (!cardCols.Contains("BindFPs"))
            db.Database.ExecuteSqlRaw("ALTER TABLE \"Cards\" ADD COLUMN \"BindFPs\" TEXT NOT NULL DEFAULT ''");

        // 种子管理员账号（不存在才创建）
        var adminUser = config["CardServer:AdminUsername"] ?? "admin";
        var adminPass = config["CardServer:AdminPassword"] ?? "admin888";
        if (!db.Agents.Any(a => a.Username == adminUser))
        {
            var salt = PasswordHasher.GenerateSalt();
            db.Agents.Add(new Agent
            {
                Username = adminUser,
                Salt = salt,
                PassHash = PasswordHasher.Hash(adminPass, salt),
                Role = "admin",
                Quota = -1, // -1 表示不限额度
                CreatedAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds(),
                Parent = "root"
            });
            db.SaveChanges();
            Console.WriteLine($"[CardServer] 已创建管理员账号: {adminUser}");
        }
    }
}
