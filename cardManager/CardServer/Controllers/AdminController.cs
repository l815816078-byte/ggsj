using System.Security.Claims;
using System.Text.Json;
using CardServer.Data;
using CardServer.Models;
using CardServer.Services;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Controllers;

// 管理员 API（需 admin 角色）：发卡/列表/禁用/启用/解绑/延期/日志/代理管理/充额度/在线统计
[ApiController]
[Route("api/admin")]
[Authorize(Roles = "admin")]
public class AdminController : ControllerBase
{
    private readonly AppDbContext _db;
    private readonly WindowTable _windows;
    private readonly string _hotUpdateRoot;

    public AdminController(AppDbContext db, WindowTable windows, IConfiguration config)
    {
        _db = db;
        _windows = windows;
        // 热更目录解析与 HotUpdateEndpoints.cs 完全一致：基于 exe 所在目录
        var dir = config["CardServer:HotUpdateDir"] ?? "hotupdate";
        _hotUpdateRoot = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, dir));
    }

    private string Actor => User.FindFirstValue(ClaimTypes.Name) ?? "unknown";

    public class GenerateRequest
    {
        public int count { get; set; }
        public int days { get; set; }
        public int max_windows { get; set; }
        public string? remark { get; set; }
        public string? project { get; set; }
    }

    public class CodeRequest
    {
        public string? code { get; set; }
    }

    public class RenewRequest
    {
        public string? code { get; set; }
        public int days { get; set; }
    }

    public class CreateAgentRequest
    {
        public string? username { get; set; }
        public string? password { get; set; }
        public int quota { get; set; }
    }

    public class RechargeRequest
    {
        public string? username { get; set; }
        public int quota { get; set; }
    }

    private static long Now() => DateTimeOffset.UtcNow.ToUnixTimeSeconds();

    // ── 仪表盘统计 ──

    [HttpGet("stats")]
    public async Task<IActionResult> Stats()
    {
        var dayStart = Now() - Now() % 86400; // 今日 0 点（UTC）
        var total = await _db.Cards.CountAsync();
        var active = await _db.Cards.CountAsync(c => c.Status == "active");
        var today = await _db.Cards.CountAsync(c => c.ActivateAt >= dayStart);
        var online = _windows.TotalOnline();
        var agents = await _db.Agents.CountAsync(a => a.Role == "agent");
        return Ok(new { total, active, today, online, agents });
    }

    // ── 发卡（返回批次号 + 卡号列表） ──

    [HttpPost("generate")]
    public async Task<IActionResult> Generate([FromBody] GenerateRequest req)
    {
        if (req.count <= 0 || req.count > 5000)
            return Ok(new { code = 1, msg = "数量需在 1-5000 之间" });
        if (req.days <= 0 || req.days > 3650)
            return Ok(new { code = 1, msg = "天数需在 1-3650 之间" });
        if (req.max_windows is < 1 or > 100)
            return Ok(new { code = 1, msg = "窗口数需在 1-100 之间" });
        if (string.IsNullOrWhiteSpace(req.project))
            return Ok(new { code = 1, msg = "请选择项目" });
        var project = req.project.Trim();
        if (!await _db.Projects.AnyAsync(p => p.Name == project))
            return Ok(new { code = 1, msg = "项目不存在，请先在项目管理中创建" });

        var batchId = Guid.NewGuid().ToString("N")[..16];
        var codes = KeyGen.GenerateUnique(code => _db.Cards.Any(c => c.Code == code), req.count);

        foreach (var c in codes)
        {
            _db.Cards.Add(new Card
            {
                Code = c,
                Days = req.days,
                MaxWindows = req.max_windows,
                Status = "unused",
                Owner = Actor,
                Remark = req.remark ?? "",
                Project = project,
                BatchId = batchId
            });
        }
        await _db.SaveChangesAsync();

        OpLog.Write(_db, Actor, "发卡", $"数量:{req.count} 天数:{req.days} 窗口:{req.max_windows} 批次:{batchId}");
        return Ok(new { code = 0, msg = "ok", batch_id = batchId, count = codes.Count, cards = codes });
    }

    // ── 卡密列表（搜索/状态筛选/归属/分页） ──

    [HttpGet("cards")]
    public async Task<IActionResult> Cards(
        string? search = null, string? status = null, string? owner = null,
        int page = 1, int page_size = 20)
    {
        var query = _db.Cards.AsQueryable();
        if (!string.IsNullOrWhiteSpace(search))
        {
            var s = search.Trim().ToUpperInvariant();
            query = query.Where(c => c.Code.Contains(s));
        }
        if (!string.IsNullOrWhiteSpace(status) && status != "all")
            query = query.Where(c => c.Status == status);
        if (!string.IsNullOrWhiteSpace(owner))
            query = query.Where(c => c.Owner == owner);

        var total = await query.CountAsync();
        var entities = await query
            .OrderByDescending(c => c.Id)
            .Skip((page - 1) * page_size)
            .Take(page_size)
            .ToListAsync();

        // 在线窗口数：实时从 Redis 窗口表读取（不在 SQL 里算）
        var list = entities.Select(c => new
        {
            c.Code, c.Days, c.MaxWindows, c.Status, c.ActivateAt, c.ExpireAt,
            c.Owner, c.Remark, c.Project, c.BatchId,
            online = _windows.CountOnline(c.Code)
        }).ToList();

        return Ok(new { code = 0, total, page, page_size, list });
    }

    // ── 禁用 / 启用 / 解绑 / 延期 ──

    [HttpPost("cards/disable")]
    public async Task<IActionResult> Disable([FromBody] CodeRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code))
            return Ok(new { code = 1, msg = "参数不完整" });
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == req.code.Trim().ToUpperInvariant());
        if (card == null)
            return Ok(new { code = 1, msg = "卡密不存在" });
        card.Status = "disabled";
        _windows.RemoveCard(card.Code);
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "禁用", card.Code);
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("cards/enable")]
    public async Task<IActionResult> Enable([FromBody] CodeRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code))
            return Ok(new { code = 1, msg = "参数不完整" });
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == req.code.Trim().ToUpperInvariant());
        if (card == null)
            return Ok(new { code = 1, msg = "卡密不存在" });
        if (card.Status == "disabled")
        {
            card.Status = "unused"; // 禁用恢复为未使用，需重新激活
            card.FailCount = 0;
            card.LockUntil = 0;
            card.BindFPs = ""; // 清空设备绑定，重新激活后绑定新设备
        }
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "启用", card.Code);
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("cards/unbind")]
    public async Task<IActionResult> Unbind([FromBody] CodeRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code))
            return Ok(new { code = 1, msg = "参数不完整" });
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == req.code.Trim().ToUpperInvariant());
        if (card == null)
            return Ok(new { code = 1, msg = "卡密不存在" });
        // 重置为未使用：清除激活状态与到期时间，客户可换设备重新激活
        card.Status = "unused";
        card.ActivateAt = 0;
        card.ExpireAt = 0;
        card.FailCount = 0;
        card.LockUntil = 0;
        card.BindFPs = ""; // 清空设备绑定，重新激活后绑定新设备
        _windows.RemoveCard(card.Code);
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "解绑", card.Code);
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("cards/renew")]
    public async Task<IActionResult> Renew([FromBody] RenewRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code) || req.days <= 0)
            return Ok(new { code = 1, msg = "参数不完整" });
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == req.code.Trim().ToUpperInvariant());
        if (card == null)
            return Ok(new { code = 1, msg = "卡密不存在" });
        // 手动延期：未过期从原到期日顺延，已过期从当前起算
        var now = Now();
        if (card.Status == "active")
            card.ExpireAt = Math.Max(now, card.ExpireAt) + (long)req.days * 86400;
        else if (card.Status == "unused")
            card.ExpireAt = now + (long)req.days * 86400; // 预激活未使用卡也记录到期
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "延期", $"{card.Code} +{req.days}天");
        return Ok(new { code = 0, msg = "ok", expire = card.ExpireAt });
    }

    [HttpPost("cards/delete")]
    public async Task<IActionResult> Delete([FromBody] CodeRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code))
            return Ok(new { code = 1, msg = "参数不完整" });
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == req.code.Trim().ToUpperInvariant());
        if (card == null)
            return Ok(new { code = 1, msg = "卡密不存在" });
        _db.Cards.Remove(card);
        _windows.RemoveCard(card.Code);
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "删除卡密", card.Code);
        return Ok(new { code = 0, msg = "ok" });
    }

    // ── 在线统计（每卡每窗口明细） ──

    [HttpGet("windows")]
    public async Task<IActionResult> Windows()
    {
        var all = _windows.GetAll();
        var cardCodes = all.Keys.ToList();
        var cards = await _db.Cards
            .Where(c => cardCodes.Contains(c.Code))
            .Select(c => new { c.Code, c.MaxWindows, c.Owner })
            .ToListAsync();
        var result = new List<object>();
        foreach (var c in cards)
        {
            var wins = all.TryGetValue(c.Code, out var w) ? w : new List<WindowEntry>();
            result.Add(new
            {
                c.Code,
                c.MaxWindows,
                c.Owner,
                online = wins.Count,
                windows = wins.Select(e => new
                {
                    e.WindowID,
                    fp = e.FP.Length > 8 ? e.FP[..8] : e.FP,
                    e.LastSeen
                })
            });
        }
        return Ok(new { code = 0, list = result });
    }

    // ── 操作日志（时间倒序，可按操作者/动作筛选） ──

    [HttpGet("logs")]
    public async Task<IActionResult> Logs(string? actor = null, string? action = null,
        int page = 1, int page_size = 50)
    {
        var query = _db.OperationLogs.AsQueryable();
        if (!string.IsNullOrWhiteSpace(actor))
            query = query.Where(l => l.Actor == actor);
        if (!string.IsNullOrWhiteSpace(action))
            query = query.Where(l => l.Action == action);
        var total = await query.CountAsync();
        var list = await query
            .OrderByDescending(l => l.Id)
            .Skip((page - 1) * page_size)
            .Take(page_size)
            .Select(l => new { l.Time, l.Actor, l.Action, l.Detail })
            .ToListAsync();
        return Ok(new { code = 0, total, page, page_size, list });
    }

    // ── 代理管理 ──

    [HttpGet("agents")]
    public async Task<IActionResult> Agents()
    {
        var agents = await _db.Agents.Where(a => a.Role == "agent").ToListAsync();
        var issued = await _db.Cards
            .Where(c => c.Owner != "admin")
            .GroupBy(c => c.Owner)
            .Select(g => new { owner = g.Key, cnt = g.Count() })
            .ToDictionaryAsync(x => x.owner, x => x.cnt);
        var list = agents.Select(a => new
        {
            a.Username,
            a.Quota,
            a.CreatedAt,
            a.Parent,
            issued_count = issued.TryGetValue(a.Username, out var n) ? n : 0
        }).ToList();
        return Ok(new { code = 0, list });
    }

    [HttpPost("agents/create")]
    public async Task<IActionResult> CreateAgent([FromBody] CreateAgentRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.username) || string.IsNullOrWhiteSpace(req.password))
            return Ok(new { code = 1, msg = "用户名或密码不能为空" });
        if (req.username.Trim().Equals("admin", StringComparison.OrdinalIgnoreCase))
            return Ok(new { code = 1, msg = "不能创建 admin 同名账号" });
        if (req.quota < 0)
            return Ok(new { code = 1, msg = "额度不能为负数" });
        if (await _db.Agents.AnyAsync(a => a.Username == req.username.Trim()))
            return Ok(new { code = 1, msg = "用户名已存在" });

        var salt = PasswordHasher.GenerateSalt();
        _db.Agents.Add(new Agent
        {
            Username = req.username.Trim(),
            Salt = salt,
            PassHash = PasswordHasher.Hash(req.password, salt),
            Role = "agent",
            Quota = req.quota,
            CreatedAt = Now(),
            Parent = Actor
        });
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "创建代理", $"用户名:{req.username} 初始额度:{req.quota}");
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("agents/recharge")]
    public async Task<IActionResult> Recharge([FromBody] RechargeRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.username) || req.quota <= 0)
            return Ok(new { code = 1, msg = "参数不完整" });
        var agent = await _db.Agents.FirstOrDefaultAsync(a => a.Username == req.username.Trim() && a.Role == "agent");
        if (agent == null)
            return Ok(new { code = 1, msg = "代理不存在" });
        agent.Quota += req.quota;
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "充额度", $"{agent.Username} +{req.quota} 当前:{agent.Quota}");
        return Ok(new { code = 0, msg = "ok", quota = agent.Quota });
    }

    // ── 项目管理（仅 admin）：项目名称 = 分类，发卡必选 ──

    public class ProjectNameRequest { public string? name { get; set; } }
    public class ProjectUpdateRequest { public int id { get; set; } public string? name { get; set; } }
    public class ProjectIdRequest { public int id { get; set; } }

    [HttpGet("projects")]
    public async Task<IActionResult> Projects()
    {
        var list = await _db.Projects.OrderBy(p => p.Id).ToListAsync();
        var names = list.Select(p => p.Name).ToList();
        var counts = await _db.Cards
            .Where(c => names.Contains(c.Project))
            .GroupBy(c => c.Project)
            .Select(g => new { name = g.Key, cnt = g.Count() })
            .ToDictionaryAsync(x => x.name, x => x.cnt);
        var result = list.Select(p => new
        {
            p.Id,
            p.Name,
            p.CreatedAt,
            card_count = counts.TryGetValue(p.Name, out var n) ? n : 0
        });
        return Ok(new { code = 0, list = result });
    }

    [HttpPost("projects/create")]
    public async Task<IActionResult> CreateProject([FromBody] ProjectNameRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.name))
            return Ok(new { code = 1, msg = "项目名称不能为空" });
        var name = req.name.Trim();
        if (name.Length > 50)
            return Ok(new { code = 1, msg = "项目名称过长（最多 50 字）" });
        if (await _db.Projects.AnyAsync(p => p.Name == name))
            return Ok(new { code = 1, msg = "项目已存在" });
        _db.Projects.Add(new Project { Name = name, CreatedAt = Now() });
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "创建项目", name);
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("projects/update")]
    public async Task<IActionResult> UpdateProject([FromBody] ProjectUpdateRequest req)
    {
        var p = await _db.Projects.FirstOrDefaultAsync(x => x.Id == req.id);
        if (p == null)
            return Ok(new { code = 1, msg = "项目不存在" });
        if (string.IsNullOrWhiteSpace(req.name))
            return Ok(new { code = 1, msg = "项目名称不能为空" });
        var name = req.name.Trim();
        if (name.Length > 50)
            return Ok(new { code = 1, msg = "项目名称过长（最多 50 字）" });
        if (await _db.Projects.AnyAsync(x => x.Name == name && x.Id != req.id))
            return Ok(new { code = 1, msg = "项目已存在" });
        var old = p.Name;
        p.Name = name;
        // 同步更新该分类下所有卡密
        var cards = _db.Cards.Where(c => c.Project == old);
        foreach (var c in cards)
            c.Project = name;
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "重命名项目", $"{old} → {name}");
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("projects/delete")]
    public async Task<IActionResult> DeleteProject([FromBody] ProjectIdRequest req)
    {
        var p = await _db.Projects.FirstOrDefaultAsync(x => x.Id == req.id);
        if (p == null)
            return Ok(new { code = 1, msg = "项目不存在" });
        var cnt = await _db.Cards.CountAsync(c => c.Project == p.Name);
        if (cnt > 0)
            return Ok(new { code = 1, msg = $"该项目下还有 {cnt} 张卡密，请先处理后再删除" });
        _db.Projects.Remove(p);
        await _db.SaveChangesAsync();
        OpLog.Write(_db, Actor, "删除项目", p.Name);
        return Ok(new { code = 0, msg = "ok" });
    }

    // ── 热更新管理（仅 admin；文件落盘 hotupdate 目录，与 HotUpdateEndpoints 同路径） ──

    public class VersionRequest { public string? version { get; set; } }

    [HttpGet("hotupdate/status")]
    public IActionResult HotUpdateStatus()
    {
        Directory.CreateDirectory(_hotUpdateRoot);
        var verFile = Path.Combine(_hotUpdateRoot, "version.txt");
        var appFile = Path.Combine(_hotUpdateRoot, "app");
        var version = System.IO.File.Exists(verFile) ? System.IO.File.ReadAllText(verFile).Trim() : "";
        var appExists = System.IO.File.Exists(appFile);
        long appSize = 0;
        string? appMtime = null;
        if (appExists)
        {
            var fi = new FileInfo(appFile);
            appSize = fi.Length;
            appMtime = fi.LastWriteTime.ToString("yyyy-MM-dd HH:mm:ss");
        }
        return Ok(new { code = 0, version, nextVersion = BumpVersion(version), appExists, appSize, appMtime });
    }

    [HttpPost("hotupdate/upload")]
    [RequestSizeLimit(200 * 1024 * 1024)]
    public async Task<IActionResult> UploadHotUpdate(IFormFile file, [FromForm] string? version = null)
    {
        if (file == null || file.Length == 0)
            return Ok(new { code = 1, msg = "请选择要上传的程序文件" });

        // 若附带版本号，先校验合法性（按用户填写值保存，不做自动递增）
        string? ver = null;
        if (!string.IsNullOrWhiteSpace(version))
        {
            ver = version.Trim();
            if (ver.Length > 32 || ver.Contains('\n') || ver.Contains('\r'))
                return Ok(new { code = 1, msg = "版本号过长或包含换行" });
        }

        Directory.CreateDirectory(_hotUpdateRoot);
        // 上传前先把当前版本备份进历史（最多保留 10 条），保证可随时回滚
        BackupCurrentToHistory();
        var appFile = Path.Combine(_hotUpdateRoot, "app");
        var tmpFile = Path.Combine(_hotUpdateRoot, "app.tmp");

        // 先写临时文件，再原子覆盖，防止上传中断产生半个 app 被设备端下载
        try
        {
            await using (var fs = new FileStream(tmpFile, FileMode.Create, FileAccess.Write))
                await file.CopyToAsync(fs);
            if (System.IO.File.Exists(appFile))
                System.IO.File.Delete(appFile);
            System.IO.File.Move(tmpFile, appFile);
        }
        catch (Exception ex)
        {
            try { if (System.IO.File.Exists(tmpFile)) System.IO.File.Delete(tmpFile); } catch { }
            return Ok(new { code = 1, msg = "保存失败: " + ex.Message });
        }

        OpLog.Write(_db, Actor, "热更上传", $"新程序 {file.Length} 字节");

        // 前端随上传提交的版本号原样写入（用户填什么就是什么）
        if (ver != null)
        {
            var verFile = Path.Combine(_hotUpdateRoot, "version.txt");
            await System.IO.File.WriteAllTextAsync(verFile, ver);
            OpLog.Write(_db, Actor, "热更版本", ver);
        }

        return Ok(new { code = 0, msg = "ok", size = file.Length, version = ver });
    }

    // 版本号递增：取末尾数字段 +1，"1.1.0"→"1.1.1"，"2.5"→"2.6"，"10"→"11"，空则默认 "1.0"
    private static string BumpVersion(string? ver)
    {
        var s = ver?.Trim() ?? "";
        if (s.Length == 0) return "1.0";
        int i = s.Length;
        while (i > 0 && char.IsDigit(s[i - 1])) i--;
        if (i == s.Length) return s + ".1"; // 末尾无数字，追加 .1
        return s[..i] + (long.Parse(s[i..]) + 1);
    }

    [HttpPost("hotupdate/version")]
    public async Task<IActionResult> SetHotUpdateVersion([FromBody] VersionRequest req)
    {
        var ver = req.version?.Trim() ?? "";
        if (ver.Length == 0)
            return Ok(new { code = 1, msg = "版本号不能为空" });
        if (ver.Length > 32)
            return Ok(new { code = 1, msg = "版本号过长（最多 32 字符）" });
        if (ver.Contains('\n') || ver.Contains('\r'))
            return Ok(new { code = 1, msg = "版本号不能包含换行" });

        Directory.CreateDirectory(_hotUpdateRoot);
        await System.IO.File.WriteAllTextAsync(Path.Combine(_hotUpdateRoot, "version.txt"), ver);
        OpLog.Write(_db, Actor, "热更版本", ver);
        return Ok(new { code = 0, msg = "ok", version = ver });
    }

    // ── 热更新历史版本（上传/回滚前自动备份，最多保留 10 条，可随时回滚） ──

    public class HotUpdateHistoryItem
    {
        public string id { get; set; } = "";      // 备份文件名（时间戳）
        public string version { get; set; } = ""; // 备份时服务器版本号
        public long size { get; set; }            // 备份文件字节数
        public string time { get; set; } = "";    // 备份时间
    }

    public class RollbackRequest { public string? id { get; set; } }

    private string HistoryDir => Path.Combine(_hotUpdateRoot, "history");
    private string HistoryJson => Path.Combine(HistoryDir, "history.json");

    private List<HotUpdateHistoryItem> ReadHistory()
    {
        try
        {
            if (!System.IO.File.Exists(HistoryJson)) return new List<HotUpdateHistoryItem>();
            var json = System.IO.File.ReadAllText(HistoryJson);
            return JsonSerializer.Deserialize<List<HotUpdateHistoryItem>>(json) ?? new List<HotUpdateHistoryItem>();
        }
        catch { return new List<HotUpdateHistoryItem>(); }
    }

    private void SaveHistory(List<HotUpdateHistoryItem> list)
    {
        Directory.CreateDirectory(HistoryDir);
        // 超上限截断：最多保留 10 条，删最旧（连同备份文件）
        while (list.Count > 10)
        {
            var old = list[^1];
            list.RemoveAt(list.Count - 1);
            try { System.IO.File.Delete(Path.Combine(HistoryDir, old.id)); } catch { }
        }
        System.IO.File.WriteAllText(HistoryJson, JsonSerializer.Serialize(list));
    }

    // 上传新版本/回滚前调用：把当前 app + version.txt 备份进历史（app 不存在则跳过）
    private void BackupCurrentToHistory()
    {
        var appFile = Path.Combine(_hotUpdateRoot, "app");
        if (!System.IO.File.Exists(appFile)) return;
        var verFile = Path.Combine(_hotUpdateRoot, "version.txt");
        var version = System.IO.File.Exists(verFile) ? System.IO.File.ReadAllText(verFile).Trim() : "";
        var id = DateTime.Now.ToString("yyyyMMddHHmmssfff");
        Directory.CreateDirectory(HistoryDir);
        System.IO.File.Copy(appFile, Path.Combine(HistoryDir, id), true);
        var list = ReadHistory();
        list.Insert(0, new HotUpdateHistoryItem
        {
            id = id,
            version = version,
            size = new FileInfo(appFile).Length,
            time = DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss")
        });
        SaveHistory(list);
    }

    [HttpGet("hotupdate/history")]
    public IActionResult HotUpdateHistory()
    {
        return Ok(new { code = 0, list = ReadHistory() });
    }

    [HttpPost("hotupdate/history/delete")]
    public IActionResult DeleteHotUpdateHistory([FromBody] RollbackRequest req)
    {
        var list = ReadHistory();
        var item = list.FirstOrDefault(x => x.id == req?.id);
        if (item == null)
            return Ok(new { code = 1, msg = "历史记录不存在或已失效" });

        list.Remove(item);
        // 删除备份文件（越界文件不存在时忽略，不影响索引清理）
        try { System.IO.File.Delete(Path.Combine(HistoryDir, item.id)); } catch { }
        SaveHistory(list);

        OpLog.Write(_db, Actor, "热更历史删除", $"删除历史版本 {(string.IsNullOrEmpty(item.version) ? "（无版本号）" : item.version)}");
        return Ok(new { code = 0, msg = "ok" });
    }

    [HttpPost("hotupdate/rollback")]
    public IActionResult RollbackHotUpdate([FromBody] RollbackRequest req)
    {
        var list = ReadHistory();
        var item = list.FirstOrDefault(x => x.id == req?.id);
        if (item == null)
            return Ok(new { code = 1, msg = "历史记录不存在或已失效" });

        var src = Path.Combine(HistoryDir, item.id);
        if (!System.IO.File.Exists(src))
        {
            list.Remove(item);
            SaveHistory(list);
            return Ok(new { code = 1, msg = "历史文件已丢失" });
        }

        // 回滚前先把当前版本备份进历史，保证可以来回回滚
        BackupCurrentToHistory();

        var appFile = Path.Combine(_hotUpdateRoot, "app");
        var tmpFile = Path.Combine(_hotUpdateRoot, "app.tmp");
        try
        {
            System.IO.File.Copy(src, tmpFile, true);
            if (System.IO.File.Exists(appFile))
                System.IO.File.Delete(appFile);
            System.IO.File.Move(tmpFile, appFile);
        }
        catch (Exception ex)
        {
            try { if (System.IO.File.Exists(tmpFile)) System.IO.File.Delete(tmpFile); } catch { }
            return Ok(new { code = 1, msg = "回滚失败: " + ex.Message });
        }

        // 版本号一并回滚
        if (!string.IsNullOrEmpty(item.version))
            System.IO.File.WriteAllText(Path.Combine(_hotUpdateRoot, "version.txt"), item.version);

        // 从历史中移除已回滚的这条（当前版本已在备份中）
        list = ReadHistory();
        list.RemoveAll(x => x.id == item.id);
        SaveHistory(list);

        OpLog.Write(_db, Actor, "热更回滚", $"回滚到版本 {(string.IsNullOrEmpty(item.version) ? "（无版本号）" : item.version)}");
        return Ok(new { code = 0, msg = "ok", version = item.version });
    }
}
