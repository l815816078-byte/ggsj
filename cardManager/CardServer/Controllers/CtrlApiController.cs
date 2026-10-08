using CardServer.Data;
using CardServer.Models;
using CardServer.Services;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Controllers;

// 中控台 HTTP API（面向客户，用卡密登录）：
//   - 认证：请求头 X-Card-Token（或 ?token=），由 /api/ctrl/login 用卡密换取
//   - 数据范围：只返回「该卡密下」的设备，客户之间互不可见
//   - 权限：全套操作（停止/重启/重置/下发配置）
[ApiController]
[Route("api/ctrl")]
public class CtrlApiController : ControllerBase
{
    private readonly AppDbContext _db;
    private readonly ConnectionManager _cm;
    private readonly CtrlConfigService _cfg;
    private readonly CardSessionStore _sessions;

    public CtrlApiController(AppDbContext db, ConnectionManager cm, CtrlConfigService cfg, CardSessionStore sessions)
    {
        _db = db;
        _cm = cm;
        _cfg = cfg;
        _sessions = sessions;
    }

    private static long Now() => DateTimeOffset.UtcNow.ToUnixTimeSeconds();

    // 从请求取当前登录卡密；未登录返回 null
    private string? CurrentCard()
    {
        var token = Request.Headers["X-Card-Token"].ToString();
        if (string.IsNullOrWhiteSpace(token))
        {
            token = Request.Query["token"].ToString();
        }
        return _sessions.Validate(token);
    }

    private IActionResult Unauthorized2() => Ok(new { code = 401, msg = "登录已失效，请重新登录" });

    // ── 登录 ──

    public class LoginRequest
    {
        // 中控台登录只需卡密（设备号是手机端脚本登记设备用的，与中控登录无关）
        public string? card { get; set; }
    }

    // POST /api/ctrl/login  卡密登录，换取 token
    [HttpPost("login")]
    public async Task<IActionResult> Login([FromBody] LoginRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.card))
        {
            return Ok(new { code = 1, msg = "请输入卡密" });
        }
        var code = req.card.Trim().ToUpperInvariant();
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == code);
        if (card == null)
        {
            return Ok(new { code = 1, msg = "卡密不存在" });
        }
        if (card.Status == "disabled")
        {
            return Ok(new { code = 1, msg = "卡密已被禁用" });
        }
        if (card.Status != "active")
        {
            return Ok(new { code = 1, msg = "卡密未激活，请先在脚本中激活" });
        }
        if (card.ExpireAt <= Now())
        {
            return Ok(new { code = 1, msg = "卡密已过期" });
        }

        var token = _sessions.Create(code);
        Console.WriteLine($"[CtrlApi] 中控台登录: card={code}");
        return Ok(new
        {
            code = 0,
            msg = "ok",
            data = new
            {
                token,
                card = code,
                expire = card.ExpireAt,
                maxwin = card.MaxWindows
            }
        });
    }

    // POST /api/ctrl/logout
    [HttpPost("logout")]
    public IActionResult Logout()
    {
        _sessions.Remove(Request.Headers["X-Card-Token"].ToString());
        return Ok(new { code = 0, msg = "ok" });
    }

    // ── 设备列表（只返回本卡设备）──

    // GET /api/ctrl/devices
    [HttpGet("devices")]
    public async Task<IActionResult> Devices()
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();

        var list = await _db.CtrlDevices
            .Where(d => d.CardCode == card)
            .OrderBy(d => d.Id)
            .ToListAsync();

        // ── 查询期去重（兜底）──
        // 设备身份 = 「卡密 + 设备号」，同一卡密下设备号唯一。
        // 库里若有历史脏数据（早期索引未建成导致重复写入）也不该显示成多行：
        // 按设备号归并，保留「在线」的那条，都离线则保留 Id 最大（最新上报）的那条。
        list = DedupByDeviceNo(list);

        // 在线状态以内存连接为准
        foreach (var d in list)
        {
            if (!_cm.IsScriptOnline(d.WindowId))
            {
                d.State = "offline";
            }
        }
        return Ok(new { code = 0, data = list });
    }

    // DELETE /api/ctrl/devices?deviceNo=xxx  移除设备（真删库）
    // 只按「卡密 + 设备号」删本卡的设备；同时断开该设备的 WS 连接，
    // 否则设备重连时 UpsertDeviceAsync 会立刻把它重新建回来（删了等于没删）。
    [HttpDelete("devices")]
    public async Task<IActionResult> RemoveDevice([FromQuery] string deviceNo)
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        if (string.IsNullOrWhiteSpace(deviceNo))
        {
            return Ok(new { code = 1, msg = "参数不完整" });
        }

        var rows = await _db.CtrlDevices
            .Where(d => d.CardCode == card && d.DeviceNo == deviceNo)
            .ToListAsync();
        if (rows.Count == 0)
        {
            return Ok(new { code = 1, msg = "设备不存在" });
        }

        // 先断开该设备所有在线连接（按 WindowId 找脚本连接），再删库
        foreach (var r in rows)
        {
            _cm.KickScript(r.WindowId);
        }
        _db.CtrlDevices.RemoveRange(rows);
        await _db.SaveChangesAsync();

        // 通知本卡中控台：该设备已移除
        await _cm.BroadcastToCtrlsAsync(new
        {
            type = "deviceRemoved",
            @params = new { deviceNo }
        }, card);

        Console.WriteLine($"[CtrlApi] 移除设备: card={card} deviceNo={deviceNo} 行数={rows.Count}");
        return Ok(new { code = 0, msg = "已移除" });
    }

    // GET /api/ctrl/stats  本卡统计
    // 口径：窗口数 = 不同设备号的个数（设备身份 = 卡密 + 设备号），
    // 因此必须先按设备号去重再统计，否则历史脏数据会把窗口数算多。
    [HttpGet("stats")]
    public async Task<IActionResult> Stats()
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();

        var all = await _db.CtrlDevices.Where(d => d.CardCode == card).ToListAsync();
        var devices = DedupByDeviceNo(all);
        int online = devices.Count(d => _cm.IsScriptOnline(d.WindowId));
        // 授权窗口：本卡当前在线窗口数 / 该卡上限
        var cardRow = await _db.Cards.FirstOrDefaultAsync(c => c.Code == card);
        return Ok(new
        {
            code = 0,
            data = new
            {
                online,
                total = devices.Count,
                windows = online,
                maxwin = cardRow?.MaxWindows ?? 0,
                expire = cardRow?.ExpireAt ?? 0,
                serverTime = Now()
            }
        });
    }

    // 按设备号归并（设备身份 = 卡密 + 设备号；调用方已按卡过滤）。
    // 同一设备号保留一行：优先在线的那条，都离线则保留 Id 最大（最新上报）的那条。
    private List<CtrlDevice> DedupByDeviceNo(List<CtrlDevice> list) =>
        list.GroupBy(d => d.DeviceNo)
            .Select(g => g
                .OrderByDescending(d => _cm.IsScriptOnline(d.WindowId))
                .ThenByDescending(d => d.Id)
                .First())
            .OrderBy(d => d.Id)
            .ToList();

    // ── 配置方案（本卡）──

    // GET /api/ctrl/configs
    [HttpGet("configs")]
    public IActionResult Configs()
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        return Ok(new { code = 0, data = _cfg.ListSchemes(card) });
    }

    public class SaveConfigRequest
    {
        public string? schemeName { get; set; }
        public string? configJson { get; set; }
        public int hangPointId { get; set; }
    }

    // POST /api/ctrl/configs  新建/覆盖方案（自动归属当前卡密）
    [HttpPost("configs")]
    public IActionResult SaveConfig([FromBody] SaveConfigRequest req)
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        if (string.IsNullOrWhiteSpace(req.schemeName))
        {
            return Ok(new { code = 1, msg = "请填写方案名" });
        }
        var row = _cfg.SaveScheme(card, req.schemeName!, req.configJson ?? "{}", req.hangPointId);
        return Ok(new { code = 0, data = row });
    }

    // DELETE /api/ctrl/configs?scheme=
    [HttpDelete("configs")]
    public IActionResult DeleteConfig([FromQuery] string scheme)
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        if (string.IsNullOrWhiteSpace(scheme))
        {
            return Ok(new { code = 1, msg = "参数不完整" });
        }
        var ok = _cfg.DeleteScheme(card, scheme);
        return Ok(new { code = ok ? 0 : 1, msg = ok ? "ok" : "方案不存在" });
    }

    // ── 下发配置（只能下发到本卡设备）──

    public class PushRequest
    {
        public List<string>? windowIds { get; set; }
        public string? schemeName { get; set; }
    }

    // POST /api/ctrl/push
    [HttpPost("push")]
    public async Task<IActionResult> Push([FromBody] PushRequest req)
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        if (req.windowIds == null || req.windowIds.Count == 0)
        {
            return Ok(new { code = 1, msg = "未选择设备" });
        }
        if (string.IsNullOrWhiteSpace(req.schemeName))
        {
            return Ok(new { code = 1, msg = "未选择方案" });
        }

        var scheme = _cfg.GetScheme(card, req.schemeName!);
        if (scheme == null)
        {
            return Ok(new { code = 1, msg = "方案不存在" });
        }

        int okCount = 0, failCount = 0;
        foreach (var wid in req.windowIds)
        {
            // 必须属于本卡，防止越权下发到别人的设备
            var dev = await _db.CtrlDevices.FirstOrDefaultAsync(d => d.WindowId == wid && d.CardCode == card);
            if (dev == null || !_cm.IsScriptOnline(wid))
            {
                failCount++;
                continue;
            }

            var sent = await _cm.SendToScriptAsync(wid, new
            {
                type = "config",
                @params = new
                {
                    schemeName = scheme.SchemeName,
                    configJson = scheme.ConfigJson,
                    hangPointId = scheme.HangPointId
                }
            });
            if (sent)
            {
                okCount++;
                dev.SchemeName = scheme.SchemeName;
                await _db.SaveChangesAsync();
                // 立即广播"配置"列变化，避免中控台要等 30 秒定时刷新才看到
                await _cm.BroadcastToCtrlsAsync(new
                {
                    type = "deviceUpdate",
                    @params = new { windowId = dev.WindowId, cardCode = card, deviceNo = dev.DeviceNo, schemeName = dev.SchemeName }
                }, card);
            }
            else
            {
                failCount++;
            }
        }

        return Ok(new { code = 0, msg = $"成功 {okCount} 台，失败 {failCount} 台", okCount, failCount });
    }

    // ── 命令（只能操作本卡设备）──

    public class CommandRequest
    {
        public List<string>? windowIds { get; set; }
        public string? cmd { get; set; }
    }

    // POST /api/ctrl/command
    [HttpPost("command")]
    public async Task<IActionResult> Command([FromBody] CommandRequest req)
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        if (req.windowIds == null || req.windowIds.Count == 0)
        {
            return Ok(new { code = 1, msg = "未选择设备" });
        }
        var valid = new[] { "stop", "restart", "resetScript" };
        if (string.IsNullOrWhiteSpace(req.cmd) || !valid.Contains(req.cmd))
        {
            return Ok(new { code = 1, msg = "不支持的命令" });
        }

        int okCount = 0;
        foreach (var wid in req.windowIds)
        {
            var dev = await _db.CtrlDevices.FirstOrDefaultAsync(d => d.WindowId == wid && d.CardCode == card);
            if (dev == null || !_cm.IsScriptOnline(wid))
            {
                continue;
            }
            var sent = await _cm.SendToScriptAsync(wid, new
            {
                type = "cmd",
                @params = new { cmd = req.cmd }
            });
            if (sent) okCount++;
        }
        return Ok(new { code = 0, msg = $"已下发 {okCount} 台", okCount });
    }

    // ── 挂机点池（全局共享，所有客户可见）──

    // GET /api/ctrl/hangpoints
    [HttpGet("hangpoints")]
    public IActionResult HangPoints()
    {
        var card = CurrentCard();
        if (card == null) return Unauthorized2();
        return Ok(new { code = 0, data = _cfg.ListHangPoints() });
    }

    // ── 健康检查（无需登录）──

    [HttpGet("health")]
    public IActionResult Health()
    {
        return Ok(new { code = 0, data = new { serverTime = Now(), scripts = _cm.ScriptCount } });
    }
}
