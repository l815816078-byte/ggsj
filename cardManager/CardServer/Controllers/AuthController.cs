using System.Security.Claims;
using CardServer.Data;
using CardServer.Services;
using Microsoft.AspNetCore.Authentication;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;

namespace CardServer.Controllers;

// 登录认证：同一入口，登录后按角色跳转 admin/agent 视图
[ApiController]
[Route("api/auth")]
public class AuthController : ControllerBase
{
    private readonly AppDbContext _db;

    public AuthController(AppDbContext db)
    {
        _db = db;
    }

    public class LoginRequest
    {
        public string? username { get; set; }
        public string? password { get; set; }
    }

    public class LoginResp
    {
        public int code { get; set; }
        public string msg { get; set; } = "";
        public string username { get; set; } = "";
        public string role { get; set; } = "";
    }

    // 登录：校验用户名/密码（PBKDF2），签发 Cookie
    [HttpPost("login")]
    public async Task<IActionResult> Login([FromBody] LoginRequest req)
    {
        if (string.IsNullOrWhiteSpace(req.username) || string.IsNullOrWhiteSpace(req.password))
            return Ok(new LoginResp { code = 1, msg = "用户名或密码不能为空" });

        var user = await _db.Agents.FirstOrDefaultAsync(a => a.Username == req.username.Trim());
        if (user == null || !PasswordHasher.Verify(req.password, user.Salt, user.PassHash))
        {
            // 防爆破：延迟固定时间，避免用户名枚举与暴力破解
            await Task.Delay(500);
            return Ok(new LoginResp { code = 1, msg = "用户名或密码错误" });
        }

        var claims = new List<Claim>
        {
            new(ClaimTypes.Name, user.Username),
            new(ClaimTypes.Role, user.Role)
        };
        var identity = new ClaimsIdentity(claims, CookieAuthenticationDefaults.AuthenticationScheme);
        await HttpContext.SignInAsync(
            CookieAuthenticationDefaults.AuthenticationScheme,
            new ClaimsPrincipal(identity),
            new AuthenticationProperties { IsPersistent = true });

        OpLog.Write(_db, user.Username, "登录", $"角色: {user.Role}");
        return Ok(new LoginResp { code = 0, msg = "ok", username = user.Username, role = user.Role });
    }

    // 登出
    [HttpPost("logout")]
    public async Task<IActionResult> Logout()
    {
        var name = User.Identity?.Name ?? "";
        await HttpContext.SignOutAsync(CookieAuthenticationDefaults.AuthenticationScheme);
        if (!string.IsNullOrEmpty(name))
            OpLog.Write(_db, name, "登出", "");
        return Ok(new { code = 0, msg = "ok" });
    }

    // 当前登录用户（前端启动时探测角色跳转；未登录 401）
    [HttpGet("me")]
    public IActionResult Me()
    {
        if (User.Identity?.IsAuthenticated != true)
            return Unauthorized(new { code = 1, msg = "未登录" });
        return Ok(new
        {
            code = 0,
            username = User.Identity.Name,
            role = User.FindFirstValue(ClaimTypes.Role) ?? ""
        });
    }
}
