using System.Security.Cryptography;

namespace CardServer.Services;

// 卡密生成器：FY-XXXX-XXXX-XXXX
// 字符集去掉易混淆字符 0/O/1/I，避免客户输错
public static class KeyGen
{
    private const string Chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";

    private static readonly Random _rng = RandomNumberGenerator.GetInt32(0, 1) == 0
        ? new Random(RandomNumberGenerator.GetInt32(int.MaxValue))
        : new Random(RandomNumberGenerator.GetInt32(int.MaxValue));

    // 生成单个卡号，格式 FY-XXXX-XXXX-XXXX（17 字符）
    public static string Generate()
    {
        Span<char> buf = stackalloc char[17];
        buf[0] = 'F';
        buf[1] = 'Y';
        buf[2] = '-';
        for (int i = 3; i < 17; i++)
        {
            if (i == 7 || i == 12)
            {
                buf[i] = '-';
                continue;
            }
            buf[i] = Chars[_rng.Next(Chars.Length)];
        }
        return new string(buf);
    }

    // 批量生成不重复卡号
    // exists: 判断卡号是否已存在于数据库；count: 需要生成的数量
    public static List<string> GenerateUnique(Func<string, bool> exists, int count)
    {
        var list = new List<string>(count);
        var seen = new HashSet<string>(StringComparer.Ordinal);
        // 字符集 31 个字符，撞库概率极低；最多重试 1000 次兜底
        int guard = 0;
        while (list.Count < count && guard++ < 1000)
        {
            var code = Generate();
            if (seen.Add(code) && !exists(code))
                list.Add(code);
        }
        return list;
    }
}
