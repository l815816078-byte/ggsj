using System.Security.Cryptography;

namespace CardServer.Services;

// PBKDF2 密码哈希（Rfc2898DeriveBytes，随机盐），用于管理员/代理密码存储
public static class PasswordHasher
{
    private const int Iterations = 100_000;
    private const int SaltSize = 16;
    private const int KeySize = 32;

    // 生成随机盐（Base64）
    public static string GenerateSalt()
    {
        return Convert.ToBase64String(RandomNumberGenerator.GetBytes(SaltSize));
    }

    // 计算密码哈希（Base64）：PBKDF2-SHA256(salt, password, 100000, 32 bytes)
    public static string Hash(string password, string saltBase64)
    {
        var salt = Convert.FromBase64String(saltBase64);
        using var pbkdf2 = new Rfc2898DeriveBytes(password, salt, Iterations, HashAlgorithmName.SHA256);
        return Convert.ToBase64String(pbkdf2.GetBytes(KeySize));
    }

    // 校验密码是否正确
    public static bool Verify(string password, string saltBase64, string hashBase64)
    {
        if (string.IsNullOrEmpty(saltBase64) || string.IsNullOrEmpty(hashBase64))
            return false;
        var computed = Hash(password, saltBase64);
        // 恒定时间比较，防时序攻击
        return CryptographicOperations.FixedTimeEquals(
            Convert.FromBase64String(computed),
            Convert.FromBase64String(hashBase64));
    }
}
