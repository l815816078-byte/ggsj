using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using CardServer.Data;
using CardServer.Services;
using Microsoft.EntityFrameworkCore;

// 命名空间不能叫 CardServer.WebSocket（与 System.Net.WebSockets.WebSocket 同名冲突），故用 WsEndpoints
namespace CardServer.WsEndpoints;

// 中控台 WS 端点：/ws/ctrl?token=xxx
// 客户用卡密登录拿到 token 后连入，服务端只推送「该卡密下」的设备变化。
public static class CtrlSocketEndpoint
{
    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull
    };

    public static void Map(WebApplication app)
    {
        app.Map("/ws/ctrl", async context =>
        {
            if (!context.WebSockets.IsWebSocketRequest)
            {
                context.Response.StatusCode = 400;
                await context.Response.WriteAsync("websocket required");
                return;
            }

            var cm = context.RequestServices.GetRequiredService<ConnectionManager>();
            var sessions = context.RequestServices.GetRequiredService<CardSessionStore>();

            // 卡密鉴权：token 必须有效
            var token = context.Request.Query["token"].ToString();
            var card = sessions.Validate(token);
            if (card == null)
            {
                context.Response.StatusCode = 403;
                await context.Response.WriteAsync("unauthorized");
                return;
            }

            var socket = await context.WebSockets.AcceptWebSocketAsync();
            var id = cm.RegisterCtrl(socket, card);

            // 连接建立后先推一次「本卡设备」的全量快照
            await SendSnapshotAsync(context, socket, card);

            try
            {
                var buffer = new byte[8 * 1024];
                while (socket.State == WebSocketState.Open)
                {
                    var result = await socket.ReceiveAsync(buffer, CancellationToken.None);
                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        await socket.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None);
                        break;
                    }
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[CtrlSocket] 连接异常 id={id} card={card} err={ex.Message}");
            }
            finally
            {
                cm.UnregisterCtrl(id);
            }
        });
    }

    // 推送本卡设备快照
    private static async Task SendSnapshotAsync(HttpContext context, WebSocket socket, string card)
    {
        using var scope = context.RequestServices.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var list = await db.CtrlDevices.Where(d => d.CardCode == card).OrderBy(d => d.Id).ToListAsync();

        var payload = JsonSerializer.Serialize(new
        {
            type = "snapshot",
            @params = new
            {
                devices = list.Select(d => new
                {
                    d.WindowId, d.CardCode, d.DeviceNo, d.Server, d.Role, d.Job,
                    d.Level, d.Power, d.Gold, d.Diamond, d.TodayDiamond, d.MonthDiamond,
                    d.BagFree, d.SchemeName, d.State, d.LastSeen
                })
            }
        }, JsonOpts);

        var bytes = Encoding.UTF8.GetBytes(payload);
        await socket.SendAsync(bytes, WebSocketMessageType.Text, true, CancellationToken.None);
    }
}
