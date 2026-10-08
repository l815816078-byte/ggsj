using System.Collections.Concurrent;
using CardServer.Data;
using CardServer.Models;
using CardServer.Services;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Controllers;

// 设备端授权接口（无认证）：activate（含续费）/verify/heartbeat
// 响应契约: {"code":0,"msg":"ok","card":"FY-...","expire":...,"maxwin":N,"sig":"base64"}
// sig = RSA-SHA256("code=0|card=FY-...|expire=...|maxwin=N")
[ApiController]
[Route("api/card")]
public class CardApiController : ControllerBase
{
    private readonly AppDbContext _db;
    private readonly RsaSigner _signer;
    private readonly WindowTable _windows;

    // 防爆破内存表：cardCode -> (失败次数, 锁定截止)。含不存在的卡号，重启后自动清零
    private readonly ConcurrentDictionary<string, (int count, long lockUntil)> _fail = new();

    private const int MaxFailCount = 5;
    private const long LockDurationSec = 10 * 60;

    public CardApiController(AppDbContext db, RsaSigner signer, WindowTable windows)
    {
        _db = db;
        _signer = signer;
        _windows = windows;
    }

    public class ActivateRequest
    {
        public string? code { get; set; }
        public string? fp { get; set; }
        public string? window_id { get; set; }
    }

    public class HeartbeatRequest
    {
        public string? code { get; set; }
        public string? window_id { get; set; }
    }

    public class CardResp
    {
        public int code { get; set; }
        public string msg { get; set; } = "";
        public string card { get; set; } = "";
        public long expire { get; set; }
        public int maxwin { get; set; }
        public string sig { get; set; } = "";
    }

    private static long Now() => DateTimeOffset.UtcNow.ToUnixTimeSeconds();

    private static CardResp Error(string msg) => new() { code = 1, msg = msg };

    private CardResp Success(Card card)
    {
        var plain = $"code=0|card={card.Code}|expire={card.ExpireAt}|maxwin={card.MaxWindows}";
        return new CardResp
        {
            code = 0,
            msg = "ok",
            card = card.Code,
            expire = card.ExpireAt,
            maxwin = card.MaxWindows,
            sig = _signer.Sign(plain)
        };
    }

    // ── 防爆破 ──

    private string? LockedError(string code)
    {
        if (_fail.TryGetValue(code, out var f) && f.lockUntil > Now())
            return $"尝试次数过多，已锁定 {LockDurationSec / 60} 分钟";
        return null;
    }

    private void RecordFail(string code)
    {
        var now = Now();
        _fail.AddOrUpdate(code,
            _ => (1, 0L),
            (_, old) => old.lockUntil > now
                ? old
                : (old.count + 1, old.count + 1 >= MaxFailCount ? now + LockDurationSec : 0));
    }

    private void ClearFail(string code) => _fail.TryRemove(code, out _);

    // ── 接口 ──

    // 激活（已激活卡再次激活 = 续费，ExpireAt 顺延，不换设备不丢窗口）
    [HttpPost("activate")]
    public async Task<IActionResult> Activate([FromBody] ActivateRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code) || string.IsNullOrWhiteSpace(req.fp) || string.IsNullOrWhiteSpace(req.window_id))
            return Ok(Error("参数不完整"));

        var code = req.code.Trim().ToUpperInvariant();
        if (LockedError(code) is { } locked)
            return Ok(Error(locked));

        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == code);
        if (card == null)
        {
            RecordFail(code);
            return Ok(Error("卡密不存在"));
        }
        if (card.Status == "disabled")
        {
            RecordFail(code);
            return Ok(Error("卡密已被禁用"));
        }

        var now = Now();
        if (card.Status == "active")
        {
            // 续费模式：未过期从原到期日顺延，已过期从当前时间起算
            card.ExpireAt = Math.Max(now, card.ExpireAt) + (long)card.Days * 86400;
        }
        else
        {
            // 首次激活（unused 或已解绑重置）
            card.Status = "active";
            card.ActivateAt = now;
            card.ExpireAt = now + (long)card.Days * 86400;
        }

        // 窗口注册（在线数上限控制）
        if (!_windows.TryRegister(code, req.window_id!, req.fp!, card.MaxWindows))
        {
            RecordFail(code);
            return Ok(Error("窗口数量已满"));
        }

        await _db.SaveChangesAsync();
        ClearFail(code);
        return Ok(Success(card));
    }

    // 启动校验（已激活卡在启动时调用，成功即占一个在线窗口）
    [HttpPost("verify")]
    public async Task<IActionResult> Verify([FromBody] ActivateRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code) || string.IsNullOrWhiteSpace(req.fp) || string.IsNullOrWhiteSpace(req.window_id))
            return Ok(Error("参数不完整"));

        var code = req.code.Trim().ToUpperInvariant();
        if (LockedError(code) is { } locked)
            return Ok(Error(locked));

        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == code);
        if (card == null)
        {
            RecordFail(code);
            return Ok(Error("卡密不存在"));
        }
        if (card.Status == "disabled")
        {
            RecordFail(code);
            return Ok(Error("卡密已被禁用"));
        }
        if (card.Status != "active")
        {
            RecordFail(code);
            return Ok(Error("卡密未激活"));
        }
        if (card.ExpireAt <= Now())
            return Ok(Error("卡密已过期"));

        if (!_windows.TryRegister(code, req.window_id!, req.fp!, card.MaxWindows))
        {
            RecordFail(code);
            return Ok(Error("窗口数量已满"));
        }

        ClearFail(code);
        return Ok(Success(card));
    }

    // 心跳：每 10 分钟一次，刷新窗口在线时间
    [HttpPost("heartbeat")]
    public async Task<IActionResult> Heartbeat([FromBody] HeartbeatRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.code) || string.IsNullOrWhiteSpace(req.window_id))
            return Ok(Error("参数不完整"));

        var code = req.code.Trim().ToUpperInvariant();
        var card = await _db.Cards.FirstOrDefaultAsync(c => c.Code == code);
        if (card == null)
            return Ok(Error("卡密不存在"));
        if (card.Status == "disabled")
            return Ok(Error("卡密已被禁用"));
        if (card.Status != "active")
            return Ok(Error("卡密未激活"));

        if (!_windows.Touch(code, req.window_id!))
        {
            // 窗口已被清理（30 分钟无心跳），尝试重新注册
            if (!_windows.TryRegister(code, req.window_id!, "unknown", card.MaxWindows))
                return Ok(Error("窗口数量已满"));
        }
        return Ok(new CardResp { code = 0, msg = "ok" });
    }
}
