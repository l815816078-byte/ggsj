using System.Text.Json;
using System.Text.Json.Nodes;
using CardServer.Data;
using CardServer.Models;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Services;

// 中控配置服务：配置方案的增删改查 + 逐项合并 + 挂机点池
public class CtrlConfigService
{
    private readonly IServiceScopeFactory _scopeFactory;

    public CtrlConfigService(IServiceScopeFactory scopeFactory)
    {
        _scopeFactory = scopeFactory;
    }

    // ── 配置方案 ──

    // 列出某卡的所有方案
    public List<CtrlConfig> ListSchemes(string cardCode)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        return db.CtrlConfigs.Where(c => c.CardCode == cardCode)
            .OrderBy(c => c.SchemeName)
            .ToList();
    }

    // 取单个方案
    public CtrlConfig? GetScheme(string cardCode, string schemeName)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        return db.CtrlConfigs.FirstOrDefault(c => c.CardCode == cardCode && c.SchemeName == schemeName);
    }

    // 保存方案（不存在则新建，存在则覆盖）
    public CtrlConfig SaveScheme(string cardCode, string schemeName, string configJson, int hangPointId)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var row = db.CtrlConfigs.FirstOrDefault(c => c.CardCode == cardCode && c.SchemeName == schemeName);
        if (row == null)
        {
            row = new CtrlConfig { CardCode = cardCode, SchemeName = schemeName };
            db.CtrlConfigs.Add(row);
        }
        row.ConfigJson = string.IsNullOrWhiteSpace(configJson) ? "{}" : configJson;
        row.HangPointId = hangPointId;
        row.UpdatedAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        db.SaveChanges();
        return row;
    }

    // 删除方案
    public bool DeleteScheme(string cardCode, string schemeName)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var row = db.CtrlConfigs.FirstOrDefault(c => c.CardCode == cardCode && c.SchemeName == schemeName);
        if (row == null)
        {
            return false;
        }
        db.CtrlConfigs.Remove(row);
        db.SaveChanges();
        return true;
    }

    // ── 逐项合并 ──
    // 把下发的配置（只含中控可见字段）合并进手机端现有配置：
    //   手机端未涉及的字段（境界配置等）保留原值，不整体替换。
    // baseJson = 手机端当前配置，patchJson = 下发的部分配置
    public static string Merge(string baseJson, string patchJson)
    {
        var baseObj = ParseObject(baseJson);
        var patchObj = ParseObject(patchJson);
        foreach (var kv in patchObj)
        {
            baseObj[kv.Key] = kv.Value?.DeepClone();
        }
        return baseObj.ToJsonString(new JsonSerializerOptions
        {
            // 保留中文 key，输出缩进便于人工查看
            Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
            WriteIndented = true
        });
    }

    private static JsonObject ParseObject(string json)
    {
        if (string.IsNullOrWhiteSpace(json))
        {
            return new JsonObject();
        }
        try
        {
            var node = JsonNode.Parse(json);
            return node as JsonObject ?? new JsonObject();
        }
        catch
        {
            return new JsonObject();
        }
    }

    // ── 挂机点池 ──

    public List<HangPoint> ListHangPoints(bool onlyEnabled = true)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var q = db.HangPoints.AsQueryable();
        if (onlyEnabled)
        {
            q = q.Where(h => h.Enabled);
        }
        return q.OrderBy(h => h.Id).ToList();
    }

    // 挂机点不存在则创建（中控台可维护）
    public HangPoint EnsureHangPoint(string name, string mapName, int minLevel, int minPower, int switchTimes, bool enabled)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var row = db.HangPoints.FirstOrDefault(h => h.Name == name);
        if (row == null)
        {
            row = new HangPoint { Name = name };
            db.HangPoints.Add(row);
        }
        row.MapName = mapName;
        row.MinLevel = minLevel;
        row.MinPower = minPower;
        row.SwitchTimes = switchTimes;
        row.Enabled = enabled;
        db.SaveChanges();
        return row;
    }

    public bool DeleteHangPoint(int id)
    {
        using var scope = _scopeFactory.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var row = db.HangPoints.FirstOrDefault(h => h.Id == id);
        if (row == null)
        {
            return false;
        }
        db.HangPoints.Remove(row);
        db.SaveChanges();
        return true;
    }
}
