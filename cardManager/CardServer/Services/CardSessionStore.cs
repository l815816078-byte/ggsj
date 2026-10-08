using System.Collections.Concurrent;
using System.Security.Cryptography;

namespace CardServer.Services;

// 卡密会话：客户在中控台用卡密登录后，服务端发一个 token，
// 后续 /api/ctrl/* 请求带这个 token，服务端据此确定「是哪张卡密」，从而只返回该卡的设备。
//
// 为什么不用 Cookie/OAuth：中控台是分发给客户的独立 exe，
// 客户只需要填卡密，不需要注册账号，token 机制最轻。
public class CardSessionStore
{
    private sealed class Session
    {
        public string CardCode { get; init; } = "";
        public long ExpireAt { get; init; }
    }

    // token → 会话
    private readonly ConcurrentDictionary<string, Session> _sessions = new();

    // 会话有效期：12 小时（客户不用频繁登录）
    private const long TtlSeconds = 12 * 3600;

    // 创建会话，返回 token
    public string Create(string cardCode)
    {
        var token = Convert.ToHexString(RandomNumberGenerator.GetBytes(24));
        _sessions[token] = new Session
        {
            CardCode = cardCode,
            ExpireAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds() + TtlSeconds
        };
        return token;
    }

    // 校验 token，返回卡密；无效/过期返回 null
    public string? Validate(string? token)
    {
        if (string.IsNullOrWhiteSpace(token))
        {
            return null;
        }
        if (!_sessions.TryGetValue(token, out var s))
        {
            return null;
        }
        if (s.ExpireAt <= DateTimeOffset.UtcNow.ToUnixTimeSeconds())
        {
            _sessions.TryRemove(token, out _);
            return null;
        }
        return s.CardCode;
    }

    // 主动注销
    public void Remove(string? token)
    {
        if (!string.IsNullOrWhiteSpace(token))
        {
            _sessions.TryRemove(token!, out _);
        }
    }

    // 清理过期会话（由后台任务调用）
    public void CleanExpired()
    {
        var now = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        foreach (var kv in _sessions)
        {
            if (kv.Value.ExpireAt <= now)
            {
                _sessions.TryRemove(kv.Key, out _);
            }
        }
    }
}
