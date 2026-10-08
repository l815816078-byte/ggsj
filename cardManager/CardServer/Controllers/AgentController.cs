using System.Security.Claims;
using CardServer.Data;
using CardServer.Models;
using CardServer.Services;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Controllers;

// 代理 API（需 agent 角色）：余额/额度内发卡/名下卡列表（事务扣 Quota）
[ApiController]
[Route("api/agent")]
[Authorize(Roles = "agent")]
public class AgentController : ControllerBase
{
    private readonly AppDbContext _db;

    public AgentController(AppDbContext db)
    {
        _db = db;
    }

    private string Actor => User.FindFirstValue(ClaimTypes.Name) ?? "";

    public class GenerateRequest
    {
        public int count { get; set; }
        public int days { get; set; }
        public int max_windows { get; set; }
        public string? remark { get; set; }
    }

    // 当前代理信息：用户名/剩余额度/已发卡数
    [HttpGet("me")]
    public async Task<IActionResult> Me()
    {
        var agent = await _db.Agents.FirstOrDefaultAsync(a => a.Username == Actor);
        if (agent == null)
            return NotFound(new { code = 1, msg = "账号不存在" });
        var issued = await _db.Cards.CountAsync(c => c.Owner == Actor);
        return Ok(new
        {
            code = 0,
            username = agent.Username,
            quota = agent.Quota,
            issued_count = issued,
            created_at = agent.CreatedAt
        });
    }

    // 额度内发卡：事务保证并发下不超卖
    [HttpPost("generate")]
    public async Task<IActionResult> Generate([FromBody] GenerateRequest req)
    {
        if (req.count <= 0 || req.count > 5000)
            return Ok(new { code = 1, msg = "数量需在 1-5000 之间" });
        if (req.days <= 0 || req.days > 3650)
            return Ok(new { code = 1, msg = "天数需在 1-3650 之间" });
        if (req.max_windows is < 1 or > 64)
            return Ok(new { code = 1, msg = "窗口数需在 1-64 之间" });

        await using var tx = await _db.Database.BeginTransactionAsync();
        var agent = await _db.Agents.FirstOrDefaultAsync(a => a.Username == Actor);
        if (agent == null)
            return Ok(new { code = 1, msg = "账号不存在" });
        if (agent.Quota < req.count)
            return Ok(new { code = 1, msg = "余额不足" });

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
                BatchId = batchId
            });
        }
        agent.Quota -= req.count;
        await _db.SaveChangesAsync();
        await tx.CommitAsync();

        OpLog.Write(_db, Actor, "发卡", $"数量:{req.count} 天数:{req.days} 窗口:{req.max_windows} 批次:{batchId}");
        return Ok(new { code = 0, msg = "ok", batch_id = batchId, count = codes.Count, cards = codes, quota_left = agent.Quota });
    }

    // 名下卡列表（只看 Owner 为自己的卡）
    [HttpGet("cards")]
    public async Task<IActionResult> Cards(
        string? search = null, string? status = null,
        int page = 1, int page_size = 20)
    {
        var query = _db.Cards.Where(c => c.Owner == Actor);
        if (!string.IsNullOrWhiteSpace(search))
        {
            var s = search.Trim().ToUpperInvariant();
            query = query.Where(c => c.Code.Contains(s));
        }
        if (!string.IsNullOrWhiteSpace(status) && status != "all")
            query = query.Where(c => c.Status == status);

        var total = await query.CountAsync();
        var list = await query
            .OrderByDescending(c => c.Id)
            .Skip((page - 1) * page_size)
            .Take(page_size)
            .Select(c => new
            {
                c.Code, c.Days, c.MaxWindows, c.Status, c.ActivateAt, c.ExpireAt, c.Remark, c.BatchId
            })
            .ToListAsync();
        return Ok(new { code = 0, total, page, page_size, list });
    }
}
