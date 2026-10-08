using System.Text.Json.Serialization;
using CardServer.Data;
using CardServer.HotUpdate;
using CardServer.Services;
using Microsoft.AspNetCore.Authentication.Cookies;
using Microsoft.AspNetCore.Http.Features;
using Microsoft.EntityFrameworkCore;
using StackExchange.Redis;

var builder = WebApplication.CreateBuilder(args);

// ── 配置 ──
var cardCfg = builder.Configuration.GetSection("CardServer");

// SQLite：单文件数据库，相对路径基于程序目录解析
var dbPath = cardCfg["DbPath"] ?? "cards.db";
if (!Path.IsPathRooted(dbPath))
    dbPath = Path.Combine(AppContext.BaseDirectory, dbPath);

builder.Services.AddDbContext<AppDbContext>(opt =>
    opt.UseSqlite($"Data Source={dbPath}"));

// ── 服务注册 ──
var rsaKeyPath = cardCfg["RsaPrivateKeyPath"] ?? "private.pem";
if (!Path.IsPathRooted(rsaKeyPath))
    rsaKeyPath = Path.Combine(AppContext.BaseDirectory, rsaKeyPath);

// Redis：在线窗口存储（注册/心跳/统计全部走 Redis，替代原 SQLite WindowOnlines 表）
var redisConn = builder.Configuration["Redis:Connection"] ?? "127.0.0.1:6379";
var redisMux = ConnectionMultiplexer.Connect($"{redisConn},abortConnect=false,connectTimeout=3000");
try
{
    redisMux.GetDatabase().Ping();
    Console.WriteLine($"[CardServer] Redis 连接成功: {redisConn}");
}
catch (Exception ex)
{
    Console.WriteLine($"[CardServer] Redis 连接失败（在线统计不可用，请检查 Redis）: {ex.Message}");
}
builder.Services.AddSingleton(redisMux);

builder.Services.AddSingleton(new RsaSigner(rsaKeyPath));
// 窗口无心跳超时（秒）：默认 900（15 分钟），可用 appsettings.json 的 CardServer:WindowTimeoutSec 覆盖
var winTimeout = cardCfg.GetValue<long>("WindowTimeoutSec", 15 * 60);
builder.Services.AddSingleton(new WindowTable(redisMux, winTimeout));

// ── 中控服务 ──
// ConnectionManager：持有脚本端/中控台 WS 连接（单例，全进程共享）
builder.Services.AddSingleton<ConnectionManager>();
// CtrlConfigService：配置方案存储 + 逐项合并 + 挂机点池
builder.Services.AddSingleton<CtrlConfigService>();
// DiamondStats：今日/当月钻石基准
builder.Services.AddSingleton<DiamondStats>();
// CardSessionStore：客户中控台用卡密登录后的会话（token）
builder.Services.AddSingleton<CardSessionStore>();
// 后台任务：每分钟检查一次是否跨天/跨月，需重置钻石基准
builder.Services.AddHostedService<DiamondResetWorker>();

// ── Cookie 认证 ──
builder.Services.AddAuthentication(CookieAuthenticationDefaults.AuthenticationScheme)
    .AddCookie(opt =>
    {
        opt.LoginPath = "/api/auth/me";      // 未登录访问受保护接口 → 401 引导前端跳登录
        opt.Cookie.Name = "cardserver.auth";
        opt.ExpireTimeSpan = TimeSpan.FromDays(7);
        opt.SlidingExpiration = true;
    });
builder.Services.AddAuthorization();

// 放宽 multipart 上传限制（Go 编译产物可能超过默认 30MB，热更上传接口最大 200MB）
builder.Services.Configure<FormOptions>(o => o.MultipartBodyLengthLimit = 200 * 1024 * 1024);

builder.Services.AddControllers()
    .AddJsonOptions(opt =>
    {
        // 小驼峰 JSON，匹配 Go/前端契约
        opt.JsonSerializerOptions.PropertyNamingPolicy = System.Text.Json.JsonNamingPolicy.CamelCase;
        opt.JsonSerializerOptions.DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull;
    });

var app = builder.Build();

// ── 数据库初始化 ──
DbInit.Initialize(app.Services, app.Configuration);

// ── 中间件 ──
app.UseAuthentication();
app.UseAuthorization();

// ── 路由 ──
app.MapControllers();

// ── 中控 WS 端点（脚本端上报 / 中控台推送） ──
CardServer.WsEndpoints.ScriptSocketEndpoint.Map(app);
CardServer.WsEndpoints.CtrlSocketEndpoint.Map(app);

// ── 热更端点（/version.txt /size /app） ──
HotUpdateEndpoints.Map(app, cardCfg["HotUpdateDir"] ?? "hotupdate");

// ── 根路径 → 登录页 ──
app.MapGet("/", () => Results.Redirect("/login.html"));

// ── 静态文件（wwwroot：登录/管理员/代理三页） ──
app.UseDefaultFiles();
app.UseStaticFiles();

// ── 启动日志 ──
Console.WriteLine($"[CardServer] 数据库: {dbPath}");
Console.WriteLine($"[CardServer] 管理页面: /login.html (管理员) /admin.html /agent.html");
Console.WriteLine($"[CardServer] 卡密系统已启动: {builder.Configuration["Urls"]}");

app.Run();
