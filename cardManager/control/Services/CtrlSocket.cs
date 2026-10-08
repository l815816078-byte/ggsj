using System.Net.WebSockets;
using System.Text;
using System.Text.Json;

namespace Control.Services;

// 中控台 WS 客户端：连接服务端 /ws/ctrl，接收设备状态实时推送。
// 推送类型：snapshot（全量）/ deviceUpdate（单台更新）/ deviceOnline / deviceOffline / configApplied / cmdAck
public class CtrlSocket : IDisposable
{
    private ClientWebSocket? _ws;
    private CancellationTokenSource? _cts;
    private string _baseUrl = "";

    // 收到推送时触发（在后台线程，UI 侧需 Invoke 回主线程）
    public event Action<string, JsonElement>? OnMessage;

    // 连接状态变化
    public event Action<bool>? OnConnectionChanged;

    public bool Connected => _ws?.State == WebSocketState.Open;

    // token：卡密登录后拿到的会话凭证，服务端据此只推送本卡设备
    private string _token = "";

    public void Start(string baseUrl, string token)
    {
        _baseUrl = baseUrl.TrimEnd('/');
        _token = token;
        Stop();
        _cts = new CancellationTokenSource();
        _ = Task.Run(() => LoopAsync(_cts.Token));
    }

    public void Stop()
    {
        try { _cts?.Cancel(); } catch { }
        try { _ws?.Abort(); } catch { }
        _ws?.Dispose();
        _ws = null;
        _cts = null;
    }

    private async Task LoopAsync(CancellationToken ct)
    {
        var backoff = TimeSpan.FromSeconds(2);
        while (!ct.IsCancellationRequested)
        {
            try
            {
                _ws = new ClientWebSocket();
                var wsUrl = _baseUrl.Replace("https://", "wss://").Replace("http://", "ws://") + "/ws/ctrl?token=" + Uri.EscapeDataString(_token);
                await _ws.ConnectAsync(new Uri(wsUrl), ct);
                OnConnectionChanged?.Invoke(true);

                var buffer = new byte[64 * 1024];
                var sb = new StringBuilder();
                while (_ws.State == WebSocketState.Open && !ct.IsCancellationRequested)
                {
                    sb.Clear();
                    WebSocketReceiveResult result;
                    do
                    {
                        result = await _ws.ReceiveAsync(buffer, ct);
                        if (result.MessageType == WebSocketMessageType.Close)
                        {
                            await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "", ct);
                            break;
                        }
                        sb.Append(Encoding.UTF8.GetString(buffer, 0, result.Count));
                    } while (!result.EndOfMessage);

                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        break;
                    }

                    Dispatch(sb.ToString());
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch
            {
                // 忽略，走重连
            }
            finally
            {
                OnConnectionChanged?.Invoke(false);
            }

            if (ct.IsCancellationRequested)
            {
                break;
            }

            try
            {
                await Task.Delay(backoff, ct);
            }
            catch (OperationCanceledException)
            {
                break;
            }
        }
    }

    private void Dispatch(string json)
    {
        try
        {
            using var doc = JsonDocument.Parse(json);
            var root = doc.RootElement;
            var type = root.TryGetProperty("type", out var t) ? t.GetString() ?? "" : "";
            var paramsEl = root.TryGetProperty("params", out var p) ? p.Clone() : default;
            OnMessage?.Invoke(type, paramsEl);
        }
        catch
        {
            // 忽略坏消息
        }
    }

    public void Dispose()
    {
        Stop();
    }
}
