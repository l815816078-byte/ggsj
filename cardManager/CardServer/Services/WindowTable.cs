using System.Text.Json;
using CardServer.Models;
using StackExchange.Redis;

namespace CardServer.Services;

// 在线窗口表（Redis 版）：每张卡一个 Hash，Key = fy:win:{cardCode}，
// field = 窗口 ID，value = JSON {"fp":"...","lastSeen":Unix秒}。
//
// 在线判定：读取时惰性清理超过 timeout 无心跳的窗口（HDEL）；
// Key 级 TTL 兜底（timeout + 心跳间隔余量），防止僵尸 Key。
// 服务器重启后在线数据天然保留在 Redis，无需再回放 SQLite。
public class WindowTable
{
    private const string KeyPrefix = "fy:win:";
    private const long HeartbeatIntervalSec = 600; // 设备心跳间隔 10 分钟，TTL 留此余量

    private readonly IDatabase _db;
    private readonly IServer? _server;
    private readonly long _timeoutSec;
    private readonly long _keyTtlSec;

    // 注册 Lua（原子）：清理过期窗口 → 判定名额/同设备回收 → HSET → EXPIRE
    private static readonly string _registerScript = """
        local key = KEYS[1]
        local windowId = ARGV[1]
        local fp = ARGV[2]
        local maxWindows = tonumber(ARGV[3])
        local now = tonumber(ARGV[4])
        local timeout = tonumber(ARGV[5])
        local ttl = tonumber(ARGV[6])

        local fields = redis.call('HGETALL', key)
        local count = 0
        local victim = nil
        for i = 1, #fields, 2 do
            local wid = fields[i]
            local val = cjson.decode(fields[i + 1])
            if now - tonumber(val.lastSeen) > timeout then
                redis.call('HDEL', key, wid)
            else
                count = count + 1
                if victim == nil and fp ~= '' and val.fp == fp and wid ~= windowId then
                    victim = wid
                end
            end
        end

        if count >= maxWindows then
            if victim == nil then
                return 0
            end
            redis.call('HDEL', key, victim)
        end

        redis.call('HSET', key, windowId, cjson.encode({ fp = fp, lastSeen = now }))
        redis.call('EXPIRE', key, ttl)
        return 1
        """;

    // 心跳 Lua（原子）：窗口存在则刷新 lastSeen 并续 TTL，否则返回 0
    private static readonly string _touchScript = """
        local key = KEYS[1]
        local windowId = ARGV[1]
        local now = tonumber(ARGV[2])
        local ttl = tonumber(ARGV[3])

        local val = redis.call('HGET', key, windowId)
        if val == false then
            return 0
        end

        local obj = cjson.decode(val)
        obj.lastSeen = now
        redis.call('HSET', key, windowId, cjson.encode(obj))
        redis.call('EXPIRE', key, ttl)
        return 1
        """;

    // 删除单窗口（原子）：HDEL 后 Hash 空则删 Key
    private static readonly string _removeScript = """
        local key = KEYS[1]
        local windowId = ARGV[1]

        redis.call('HDEL', key, windowId)
        if redis.call('HLEN', key) == 0 then
            redis.call('DEL', key)
        end
        return 1
        """;

    public WindowTable(IConnectionMultiplexer mux, long windowTimeoutSec = 15 * 60)
    {
        _db = mux.GetDatabase();
        var endpoints = mux.GetEndPoints();
        _server = endpoints.Length > 0 ? mux.GetServer(endpoints[0]) : null;
        _timeoutSec = windowTimeoutSec;
        _keyTtlSec = windowTimeoutSec + HeartbeatIntervalSec;
    }

    private static string Key(string cardCode) => KeyPrefix + cardCode;
    private static long Now() => DateTimeOffset.UtcNow.ToUnixTimeSeconds();

    // 指纹截取前 8 位用于日志（完整 64 位 hex 太长）
    private static string ShortFp(string fp) =>
        string.IsNullOrEmpty(fp) ? "?" : (fp.Length > 8 ? fp[..8] : fp);

    // Hash 值 JSON 结构：小写字段与 Lua cjson 保持一致
    private sealed class WinVal
    {
        public string fp { get; set; } = "";
        public long lastSeen { get; set; }
    }

    // 尝试注册窗口：在线数 < maxWindows 时成功；
    // 已满时若存在同设备（同 fp）的旧窗口，则回收该窗口腾出名额
    public bool TryRegister(string cardCode, string windowId, string fp, int maxWindows)
    {
        try
        {
            var res = _db.ScriptEvaluate(_registerScript, new RedisKey[] { Key(cardCode) },
                new RedisValue[] { windowId, fp ?? "", maxWindows, Now(), _timeoutSec, _keyTtlSec });
            if ((long)res == 1)
            {
                Console.WriteLine($"[WindowTable] 注册成功: card={cardCode} win={windowId} fp={ShortFp(fp)}");
                return true;
            }
            Console.WriteLine($"[WindowTable] 注册失败(窗口已满): card={cardCode} win={windowId} max={maxWindows}");
            return false;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[WindowTable] 注册异常(Redis 不可用?): {ex.Message}");
            return false;
        }
    }

    // 心跳续命：窗口存在则刷新最后心跳时间；不存在返回 false（调用方触发重新注册）
    public bool Touch(string cardCode, string windowId)
    {
        try
        {
            var res = _db.ScriptEvaluate(_touchScript, new RedisKey[] { Key(cardCode) },
                new RedisValue[] { windowId, Now(), _keyTtlSec });
            if ((long)res == 1)
            {
                Console.WriteLine($"[WindowTable] 心跳 Touch 成功: card={cardCode} win={windowId} 在线时间已刷新");
                return true;
            }
            Console.WriteLine($"[WindowTable] 心跳 Touch 未命中: card={cardCode} win={windowId} 不在线 → 将触发重新注册");
            return false;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[WindowTable] 心跳 Touch 异常(Redis 不可用?): {ex.Message}");
            return false;
        }
    }

    // 移除单个窗口（设备下线/管理员解绑时调用）
    public void Remove(string cardCode, string windowId)
    {
        try
        {
            _db.ScriptEvaluate(_removeScript, new RedisKey[] { Key(cardCode) },
                new RedisValue[] { windowId });
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[WindowTable] 移除窗口异常: {ex.Message}");
        }
    }

    // 移除整卡全部窗口（删除卡密时调用）
    public void RemoveCard(string cardCode)
    {
        try
        {
            _db.KeyDelete(Key(cardCode));
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[WindowTable] 移除整卡异常: {ex.Message}");
        }
    }

    // 某卡当前在线窗口数
    public int CountOnline(string cardCode) => GetWindows(cardCode).Count;

    // 某卡的在线窗口明细（惰性清理过期窗口，按最近心跳倒序）
    public List<WindowEntry> GetWindows(string cardCode)
    {
        try
        {
            var key = Key(cardCode);
            var hash = _db.HashGetAll(key);
            var now = Now();
            var list = new List<WindowEntry>();
            var stale = new List<RedisValue>();
            foreach (var h in hash)
            {
                WinVal? v = null;
                try { v = JsonSerializer.Deserialize<WinVal>(h.Value.ToString()); }
                catch { /* 坏数据按过期处理 */ }
                if (v == null || now - v.lastSeen > _timeoutSec)
                {
                    stale.Add(h.Name);
                    continue;
                }
                list.Add(new WindowEntry { WindowID = h.Name!, FP = v.fp, LastSeen = v.lastSeen });
            }
            if (stale.Count > 0)
            {
                _db.HashDelete(key, stale.ToArray());
                if (_db.HashLength(key) == 0)
                    _db.KeyDelete(key);
            }
            return list.OrderByDescending(w => w.LastSeen).ToList();
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[WindowTable] 查询窗口异常: {ex.Message}");
            return new List<WindowEntry>();
        }
    }

    // 全部在线窗口（管理端统计页数据源）
    public Dictionary<string, List<WindowEntry>> GetAll()
    {
        var result = new Dictionary<string, List<WindowEntry>>();
        if (_server == null)
            return result;
        try
        {
            foreach (var key in _server.Keys(pattern: KeyPrefix + "*"))
            {
                var cardCode = key.ToString()[KeyPrefix.Length..];
                var list = GetWindows(cardCode);
                if (list.Count > 0)
                    result[cardCode] = list;
            }
        }
        catch (Exception ex)
        {
            Console.WriteLine($"[WindowTable] 全表统计异常: {ex.Message}");
        }
        return result;
    }

    // 全局在线窗口总数
    public int TotalOnline()
    {
        int total = 0;
        foreach (var list in GetAll().Values)
            total += list.Count;
        return total;
    }
}
