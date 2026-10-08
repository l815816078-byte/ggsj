using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using System.Text.Json.Nodes;
using CardServer.Data;
using CardServer.Models;
using CardServer.Services;
using Microsoft.EntityFrameworkCore;

// 命名空间不能叫 CardServer.WebSocket：System.Net.WebSockets.WebSocket 是常用类型，
// 同名会导致 "WebSocket 是命名空间但被当作类型使用"。故用 WsEndpoints。
namespace CardServer.WsEndpoints;

// 脚本端 WS 端点：/ws/script
// 流程：
//   1) 手机端带 ?card=xx&window=xx&device=xx 连入
//   2) 服务端校验卡密有效后接受注册，写入/更新 CtrlDevices
//   3) 手机端上报（type=report/heartbeat），服务端落库并广播给中控台
//   4) 服务端下发（下发配置 / 停止 / 重启 / 重置脚本），手机端执行后回执
public static class ScriptSocketEndpoint
{
    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull
    };

    public static void Map(WebApplication app)
    {
        app.UseWebSockets();
        app.Map("/ws/script", async context =>
        {
            if (!context.WebSockets.IsWebSocketRequest)
            {
                context.Response.StatusCode = 400;
                await context.Response.WriteAsync("websocket required");
                return;
            }

            var cm = context.RequestServices.GetRequiredService<ConnectionManager>();
            var cfgSvc = context.RequestServices.GetRequiredService<CtrlConfigService>();
            var diamonds = context.RequestServices.GetRequiredService<DiamondStats>();

            var card = context.Request.Query["card"].ToString().Trim().ToUpperInvariant();
            var windowId = context.Request.Query["window"].ToString();
            var deviceNo = context.Request.Query["device"].ToString();

            // ── 卡密校验 ──
            using (var scope = context.RequestServices.CreateScope())
            {
                var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
                var c = await db.Cards.FirstOrDefaultAsync(x => x.Code == card);
                if (c == null || c.Status != "active" || c.ExpireAt <= DateTimeOffset.UtcNow.ToUnixTimeSeconds())
                {
                    context.Response.StatusCode = 403;
                    await context.Response.WriteAsync("card invalid");
                    return;
                }
            }

            if (string.IsNullOrEmpty(windowId))
            {
                windowId = Guid.NewGuid().ToString("N");
            }

            var socket = await context.WebSockets.AcceptWebSocketAsync();
            var conn = new ConnectionManager.ScriptConn
            {
                WindowId = windowId,
                CardCode = card,
                DeviceNo = deviceNo,
                Socket = socket
            };
            cm.RegisterScript(conn);

            // 上线：确保设备行存在
            await UpsertDeviceAsync(context, windowId, card, deviceNo, online: true);

            // 通知中控台：某设备上线（只推给该卡密的中控台）
            await cm.BroadcastToCtrlsAsync(new
            {
                type = "deviceOnline",
                @params = new { windowId, cardCode = card, deviceNo }
            }, card);

            // 把该设备当前使用的方案下发给刚上线的脚本（首次同步）
            var scheme = cfgSvc.ListSchemes(card).FirstOrDefault();
            if (scheme != null)
            {
                await cm.SendToScriptAsync(windowId, new
                {
                    type = "config",
                    @params = new
                    {
                        schemeName = scheme.SchemeName,
                        configJson = scheme.ConfigJson,
                        hangPointId = scheme.HangPointId
                    }
                });
            }

            try
            {
                await ReceiveLoopAsync(context, socket, conn, cm, diamonds);
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[ScriptSocket] 连接异常 win={windowId} err={ex.Message}");
            }
            finally
            {
                cm.UnregisterScript(windowId);
                await UpsertDeviceAsync(context, windowId, card, deviceNo, online: false);
                await cm.BroadcastToCtrlsAsync(new
                {
                    type = "deviceOffline",
                    @params = new { windowId }
                }, card);
            }
        });
    }

    // 接收脚本端消息
    private static async Task ReceiveLoopAsync(
        HttpContext context, WebSocket socket, ConnectionManager.ScriptConn conn,
        ConnectionManager cm, DiamondStats diamonds)
    {
        var buffer = new byte[16 * 1024];
        var sb = new StringBuilder();

        while (socket.State == WebSocketState.Open)
        {
            sb.Clear();
            WebSocketReceiveResult result;
            do
            {
                result = await socket.ReceiveAsync(buffer, CancellationToken.None);
                if (result.MessageType == WebSocketMessageType.Close)
                {
                    await socket.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None);
                    return;
                }
                sb.Append(Encoding.UTF8.GetString(buffer, 0, result.Count));
            } while (!result.EndOfMessage);

            JsonObject? msg;
            try
            {
                msg = JsonNode.Parse(sb.ToString()) as JsonObject;
            }
            catch
            {
                continue;
            }
            if (msg == null)
            {
                continue;
            }

            var type = msg["type"]?.GetValue<string>() ?? "";
            var p = msg["params"] as JsonObject ?? new JsonObject();

            switch (type)
            {
                case "report":
                    await HandleReportAsync(context, conn, p, diamonds, cm);
                    break;

                case "heartbeat":
                    HandleHeartbeat(context, conn);
                    break;

                case "configApplied":
                    Console.WriteLine($"[ScriptSocket] 配置已生效 win={conn.WindowId} scheme={p["schemeName"]}");
                    await cm.BroadcastToCtrlsAsync(new
                    {
                        type = "configApplied",
                        @params = new { windowId = conn.WindowId, schemeName = p["schemeName"]?.GetValue<string>() }
                    }, conn.CardCode);
                    break;

                case "cmdAck":
                    Console.WriteLine($"[ScriptSocket] 命令回执 win={conn.WindowId} cmd={p["cmd"]}");
                    await cm.BroadcastToCtrlsAsync(new
                    {
                        type = "cmdAck",
                        @params = new { windowId = conn.WindowId, cmd = p["cmd"]?.GetValue<string>() }
                    }, conn.CardCode);
                    break;
            }
        }
    }

    // 处理数据上报（全量）
    private static async Task HandleReportAsync(
        HttpContext context, ConnectionManager.ScriptConn conn,
        JsonObject p, DiamondStats diamonds, ConnectionManager cm)
    {
        // 解析上报字段（识别缺失的字段可能为空/0，原样落库）
        string server = p["server"]?.GetValue<string>() ?? "";
        string role = p["role"]?.GetValue<string>() ?? "";
        string job = p["job"]?.GetValue<string>() ?? "";
        int level = GetInt(p, "level");
        int power = GetInt(p, "power");
        long gold = GetLong(p, "gold");
        long diamond = GetLong(p, "diamond");
        int bagFree = GetInt(p, "bagFree");
        string state = p["state"]?.GetValue<string>() ?? "running";

        // 设备号是身份，空设备号无法定位设备行，丢弃本次上报（避免产生 DeviceNo="" 的脏行）
        if (string.IsNullOrWhiteSpace(conn.DeviceNo))
        {
            Console.WriteLine($"[ScriptSocket] 丢弃上报：设备号为空 win={conn.WindowId} card={conn.CardCode}");
            return;
        }

        // 今日/当月钻石由服务端算
        var (today, month) = diamonds.Apply(conn.WindowId, diamond);

        using var scope = context.RequestServices.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        // 按「卡密 + 设备号」定位设备行（不是 WindowId）：
        // 同一台设备重启脚本后 WindowId 会变，但设备号不变，应更新同一行而不是新增。
        var row = await db.CtrlDevices.FirstOrDefaultAsync(d => d.CardCode == conn.CardCode && d.DeviceNo == conn.DeviceNo);
        if (row == null)
        {
            row = new CtrlDevice { CardCode = conn.CardCode, DeviceNo = conn.DeviceNo };
            db.CtrlDevices.Add(row);
        }
        // 同步最新的 WindowId（连接标识会变，用它判断在线）
        row.WindowId = conn.WindowId;
        row.CardCode = conn.CardCode;
        row.DeviceNo = conn.DeviceNo;
        row.Server = server;
        row.Role = role;
        row.Job = job;
        row.Level = level;
        row.Power = power;
        row.Gold = gold;
        row.Diamond = diamond;
        row.TodayDiamond = today;
        row.MonthDiamond = month;
        row.BagFree = bagFree;
        row.State = state;
        row.LastSeen = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        await db.SaveChangesAsync();

        // 广播给中控台（只推变化的那台设备，且只推给该卡密的中控台）
        await cm.BroadcastToCtrlsAsync(new
        {
            type = "deviceUpdate",
            @params = new
            {
                windowId = conn.WindowId,
                cardCode = conn.CardCode,
                deviceNo = conn.DeviceNo,
                server, role, job, level, power, gold, diamond,
                todayDiamond = today,
                monthDiamond = month,
                bagFree, state,
                // 带上当前生效方案名：否则中控台"配置"列要等 30 秒定时刷新才更新
                schemeName = row.SchemeName
            }
        }, conn.CardCode);
    }

    // 心跳：只刷新 LastSeen / 状态，不做全量落库
    // 定位用「卡密 + 设备号」，与上报/上下线一致（用 WindowId 会因重启换值而查不到行）
    private static void HandleHeartbeat(HttpContext context, ConnectionManager.ScriptConn conn)
    {
        using var scope = context.RequestServices.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var row = db.CtrlDevices.FirstOrDefault(d => d.CardCode == conn.CardCode && d.DeviceNo == conn.DeviceNo);
        if (row != null)
        {
            row.LastSeen = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
            db.SaveChanges();
        }
    }

    // 上线/下线：维护 CtrlDevices 的 State / LastSeen
    // 设备身份 = 「卡密 + 设备号」：同一台设备重启/重连时更新同一行，不新增。
    // WindowId 每次进程启动都会变，只用来刷新「当前连接标识」。
    private static async Task UpsertDeviceAsync(
        HttpContext context, string windowId, string card, string deviceNo, bool online)
    {
        // 设备号是身份，空设备号无法定位设备，直接拒绝登记（不写库、不广播假设备行）
        if (string.IsNullOrWhiteSpace(deviceNo))
        {
            Console.WriteLine($"[ScriptSocket] 拒绝登记：设备号为空 win={windowId} card={card}");
            return;
        }

        using var scope = context.RequestServices.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();

        var row = await db.CtrlDevices.FirstOrDefaultAsync(d => d.CardCode == card && d.DeviceNo == deviceNo);
        if (row == null)
        {
            row = new CtrlDevice { CardCode = card, DeviceNo = deviceNo };
            db.CtrlDevices.Add(row);
        }

        row.WindowId = windowId;
        row.CardCode = card;
        row.DeviceNo = deviceNo;
        row.State = online ? "running" : "offline";
        row.LastSeen = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        await db.SaveChangesAsync();
    }

    private static int GetInt(JsonObject o, string key)
    {
        var n = o[key];
        if (n == null) return 0;
        try { return n.GetValue<int>(); } catch { return 0; }
    }

    private static long GetLong(JsonObject o, string key)
    {
        var n = o[key];
        if (n == null) return 0;
        try { return n.GetValue<long>(); } catch { return 0; }
    }
}
