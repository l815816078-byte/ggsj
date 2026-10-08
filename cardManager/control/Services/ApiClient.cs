using System.Net.Http;
using System.Net.Http.Json;
using System.Text.Json;

namespace Control.Services;

// 中控台 HTTP 客户端：客户用卡密登录，拿到 token 后随请求头 X-Card-Token 携带。
// 数据范围由服务端按 token 归属的卡密过滤（客户只看自己的设备）。
public class ApiClient
{
    // 服务端地址写死在程序里，客户不需要填
    // 正式发布改这里即可（也可改成读同目录 appsettings.ini）
    public const string DefaultServer = "http://111.231.167.150:8080";

    private readonly HttpClient _http;
    public string BaseUrl { get; } = DefaultServer;

    // 登录后的卡密 token
    public string Token { get; private set; } = "";
    // 登录的卡密（用于配置页归属）
    public string Card { get; private set; } = "";

    private static readonly JsonSerializerOptions JsonOpts = new()
    {
        PropertyNameCaseInsensitive = true
    };

    public ApiClient()
    {
        _http = new HttpClient { Timeout = TimeSpan.FromSeconds(15) };
        _http.DefaultRequestHeaders.Add("User-Agent", "AutoGoCtrl/1.0");
    }

    // ── 登录（卡密）──

    public async Task<(bool ok, string msg)> LoginAsync(string card)
    {
        try
        {
            var resp = await _http.PostAsJsonAsync($"{BaseUrl}/api/ctrl/login", new { card });
            var text = await resp.Content.ReadAsStringAsync();

            // 服务端未部署新接口时会返回 404 页面/HTML，这里给出可读提示
            if (string.IsNullOrWhiteSpace(text) || !text.TrimStart().StartsWith("{"))
            {
                return (false, $"服务端响应异常(HTTP {(int)resp.StatusCode})，请确认服务端已更新并重启");
            }

            using var doc = JsonDocument.Parse(text);
            var code = doc.RootElement.TryGetProperty("code", out var c) ? c.GetInt32() : 1;
            if (code != 0)
            {
                var m = doc.RootElement.TryGetProperty("msg", out var mm) ? mm.GetString() ?? "登录失败" : "登录失败";
                return (false, m);
            }
            if (doc.RootElement.TryGetProperty("data", out var data))
            {
                Token = data.TryGetProperty("token", out var t) ? t.GetString() ?? "" : "";
                Card = data.TryGetProperty("card", out var cc) ? cc.GetString() ?? "" : card;
            }
            return (true, "登录成功");
        }
        catch (HttpRequestException)
        {
            return (false, $"无法连接服务端({BaseUrl})，请检查网络或服务端是否启动");
        }
        catch (TaskCanceledException)
        {
            return (false, "连接服务端超时，请检查网络");
        }
        catch (Exception ex)
        {
            return (false, ex.Message);
        }
    }

    public void Logout()
    {
        Token = "";
        Card = "";
    }

    // ── 设备 ──

    public async Task<JsonElement?> GetDevicesAsync() => await GetDataAsync("/api/ctrl/devices");

    public async Task<JsonElement?> GetStatsAsync() => await GetDataAsync("/api/ctrl/stats");

    // 移除设备（真删库 + 断开该设备连接）
    public async Task<(bool ok, string msg)> RemoveDeviceAsync(string deviceNo)
    {
        try
        {
            using var req = BuildRequest(HttpMethod.Delete,
                $"/api/ctrl/devices?deviceNo={Uri.EscapeDataString(deviceNo)}");
            var resp = await _http.SendAsync(req);
            return ParseResult(await resp.Content.ReadAsStringAsync());
        }
        catch (Exception ex)
        {
            return (false, ex.Message);
        }
    }

    // ── 配置方案 ──

    public async Task<JsonElement?> GetConfigsAsync() => await GetDataAsync("/api/ctrl/configs");

    public async Task<(bool ok, string msg)> SaveConfigAsync(string schemeName, string configJson, int hangPointId)
    {
        return await PostAsync("/api/ctrl/configs", new { schemeName, configJson, hangPointId });
    }

    public async Task<(bool ok, string msg)> DeleteConfigAsync(string scheme)
    {
        try
        {
            using var req = BuildRequest(HttpMethod.Delete, $"/api/ctrl/configs?scheme={Uri.EscapeDataString(scheme)}");
            var resp = await _http.SendAsync(req);
            return ParseResult(await resp.Content.ReadAsStringAsync());
        }
        catch (Exception ex)
        {
            return (false, ex.Message);
        }
    }

    // ── 下发 / 命令 ──

    public async Task<(bool ok, string msg)> PushConfigAsync(List<string> windowIds, string schemeName)
    {
        return await PostAsync("/api/ctrl/push", new { windowIds, schemeName });
    }

    public async Task<(bool ok, string msg)> SendCommandAsync(List<string> windowIds, string cmd)
    {
        return await PostAsync("/api/ctrl/command", new { windowIds, cmd });
    }

    // ── 挂机点 ──

    public async Task<JsonElement?> GetHangPointsAsync() => await GetDataAsync("/api/ctrl/hangpoints");

    // ── 连通性 ──

    public async Task<bool> CheckHealthAsync()
    {
        try
        {
            var resp = await _http.GetAsync($"{BaseUrl}/api/ctrl/health");
            return resp.IsSuccessStatusCode;
        }
        catch
        {
            return false;
        }
    }

    // ── 内部 ──

    private HttpRequestMessage BuildRequest(HttpMethod method, string path)
    {
        var req = new HttpRequestMessage(method, $"{BaseUrl}{path}");
        if (!string.IsNullOrEmpty(Token))
        {
            req.Headers.Add("X-Card-Token", Token);
        }
        return req;
    }

    private async Task<JsonElement?> GetDataAsync(string path)
    {
        try
        {
            using var req = BuildRequest(HttpMethod.Get, path);
            var resp = await _http.SendAsync(req);
            if (!resp.IsSuccessStatusCode)
            {
                return null;
            }
            var text = await resp.Content.ReadAsStringAsync();
            using var doc = JsonDocument.Parse(text);
            if (doc.RootElement.TryGetProperty("data", out var data))
            {
                return data.Clone();
            }
            return null;
        }
        catch
        {
            return null;
        }
    }

    private async Task<(bool ok, string msg)> PostAsync(string path, object body)
    {
        try
        {
            using var req = BuildRequest(HttpMethod.Post, path);
            req.Content = JsonContent.Create(body);
            var resp = await _http.SendAsync(req);
            return ParseResult(await resp.Content.ReadAsStringAsync());
        }
        catch (Exception ex)
        {
            return (false, ex.Message);
        }
    }

    private static (bool ok, string msg) ParseResult(string text)
    {
        if (string.IsNullOrWhiteSpace(text) || !text.TrimStart().StartsWith("{"))
        {
            return (false, "服务端响应异常，请确认服务端已更新并重启");
        }
        try
        {
            using var doc = JsonDocument.Parse(text);
            var code = doc.RootElement.TryGetProperty("code", out var c) ? c.GetInt32() : 1;
            var msg = doc.RootElement.TryGetProperty("msg", out var m) ? m.GetString() ?? "" : "";
            return (code == 0, msg);
        }
        catch
        {
            return (false, "响应解析失败");
        }
    }
}
