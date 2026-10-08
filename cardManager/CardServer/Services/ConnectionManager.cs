using System.Collections.Concurrent;
using System.Net.WebSockets;
using System.Text;
using System.Text.Json;

namespace CardServer.Services;

// 中控连接管理器：
//   1) 持有所有「脚本端」WS 连接（/ws/script），key = windowId
//   2) 持有所有「中控台」WS 连接（/ws/ctrl），用于实时状态推送
//   3) 提供向脚本下发命令 / 向中控台广播状态的能力
//
// 说明：连接是可断的，这里只维护「当前活着的连接」；
// 设备的历史数据落在 SQLite（CtrlDevices），在线与否以连接存在为准。
public class ConnectionManager
{
    // 脚本端连接：windowId → 连接上下文
    private readonly ConcurrentDictionary<string, ScriptConn> _scripts = new();

    // 中控台连接：连接 ID → 连接（含所属卡密，用于按卡过滤推送）
    private readonly ConcurrentDictionary<string, CtrlConn> _ctrls = new();

    // 中控台连接上下文
    public sealed class CtrlConn
    {
        public string Id { get; init; } = "";
        public string CardCode { get; init; } = "";
        public WebSocket Socket { get; init; } = null!;
    }

    // 脚本连接上下文
    public sealed class ScriptConn
    {
        public string WindowId { get; init; } = "";
        public string CardCode { get; init; } = "";
        public string DeviceNo { get; init; } = "";
        public WebSocket Socket { get; init; } = null!;
        public DateTime ConnectedAt { get; init; } = DateTime.UtcNow;
    }

    // ── 脚本端 ──

    // 注册脚本连接（同一 windowId 重复连接时替换旧连接）
    public void RegisterScript(ScriptConn conn)
    {
        if (_scripts.TryGetValue(conn.WindowId, out var old) && old.Socket != conn.Socket)
        {
            _ = CloseQuietly(old.Socket);
        }
        _scripts[conn.WindowId] = conn;
        Console.WriteLine($"[ConnectionManager] 脚本上线: win={conn.WindowId} card={conn.CardCode} 当前脚本连接数={_scripts.Count}");
    }

    // 注销脚本连接
    public void UnregisterScript(string windowId)
    {
        if (_scripts.TryRemove(windowId, out _))
        {
            Console.WriteLine($"[ConnectionManager] 脚本下线: win={windowId} 当前脚本连接数={_scripts.Count}");
        }
    }

    public bool IsScriptOnline(string windowId) => _scripts.ContainsKey(windowId);

    // 踢掉指定脚本连接（设备被"移除设备"时必须断开，否则它重连会把行重建回来）
    public void KickScript(string windowId)
    {
        if (_scripts.TryRemove(windowId, out var conn))
        {
            _ = CloseQuietly(conn.Socket);
            Console.WriteLine($"[ConnectionManager] 已断开脚本: win={windowId} 当前脚本连接数={_scripts.Count}");
        }
    }

    public int ScriptCount => _scripts.Count;

    // 在线窗口 ID 集合
    public IReadOnlyCollection<string> OnlineWindowIds => _scripts.Keys.ToList();

    // 向指定脚本下发消息（命令/配置），返回是否发送成功
    public async Task<bool> SendToScriptAsync(string windowId, object payload)
    {
        if (!_scripts.TryGetValue(windowId, out var conn))
        {
            Console.WriteLine($"[ConnectionManager] 下发失败(设备离线): win={windowId}");
            return false;
        }
        try
        {
            var json = JsonSerializer.Serialize(payload, JsonOpts);
            var bytes = Encoding.UTF8.GetBytes(json);
            await conn.Socket.SendAsync(bytes, WebSocketMessageType.Text, true, CancellationToken.None);
            return true;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[ConnectionManager] 下发异常: win={windowId} err={ex.Message}");
            UnregisterScript(windowId);
            return false;
        }
    }

    // ── 中控台 ──

    public string RegisterCtrl(WebSocket socket, string cardCode)
    {
        var id = Guid.NewGuid().ToString("N");
        _ctrls[id] = new CtrlConn { Id = id, CardCode = cardCode, Socket = socket };
        Console.WriteLine($"[ConnectionManager] 中控台连接: id={id} card={cardCode} 当前中控连接数={_ctrls.Count}");
        return id;
    }

    public void UnregisterCtrl(string id)
    {
        if (_ctrls.TryRemove(id, out _))
        {
            Console.WriteLine($"[ConnectionManager] 中控台断开: id={id} 当前中控连接数={_ctrls.Count}");
        }
    }

    // 向中控台广播（设备状态变化时调用）
    // cardCode 为 null 表示广播给所有连接；否则只推给该卡密的中控台（客户只看自己的设备）
    public async Task BroadcastToCtrlsAsync(object payload, string? cardCode = null)
    {
        if (_ctrls.IsEmpty)
        {
            return;
        }
        var json = JsonSerializer.Serialize(payload, JsonOpts);
        var bytes = Encoding.UTF8.GetBytes(json);
        foreach (var kv in _ctrls.ToList())
        {
            if (cardCode != null && kv.Value.CardCode != cardCode)
            {
                continue;
            }
            try
            {
                await kv.Value.Socket.SendAsync(bytes, WebSocketMessageType.Text, true, CancellationToken.None);
            }
            catch
            {
                UnregisterCtrl(kv.Key);
            }
        }
    }

    private static async Task CloseQuietly(WebSocket socket)
    {
        try
        {
            if (socket.State == WebSocketState.Open)
            {
                await socket.CloseAsync(WebSocketCloseStatus.NormalClosure, "replaced", CancellationToken.None);
            }
        }
        catch
        {
            // 忽略关闭异常
        }
    }

    // 与 Program.cs 一致的小驼峰策略
    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull
    };
}
