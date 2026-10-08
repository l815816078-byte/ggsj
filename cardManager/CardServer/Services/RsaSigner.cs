using System.Security.Cryptography;

namespace CardServer.Services;

// RSA 私钥签名服务：加载 private.pem，RSA-SHA256 Pkcs1 签名授权响应
// 跨语言互操作关键：公钥必须导出 SubjectPublicKeyInfo 格式（BEGIN PUBLIC KEY），
// Go 端 x509.ParsePKIXPublicKey 才能解析；禁止 RSAParameters/XML 格式
public class RsaSigner
{
    private readonly RSA _rsa;

    // privateKeyPemPath: 私钥 PEM 文件路径（appsettings RsaPrivateKeyPath）
    public RsaSigner(string privateKeyPemPath)
    {
        if (!string.IsNullOrEmpty(privateKeyPemPath) && File.Exists(privateKeyPemPath))
        {
            _rsa = RSA.Create();
            _rsa.ImportFromPem(File.ReadAllText(privateKeyPemPath));
            Console.WriteLine($"[CardServer] 已加载 RSA 私钥: {privateKeyPemPath}");
        }
        else
        {
            // 开发模式：无私钥文件则生成临时密钥对（重启后签名失效，仅用于本地联调）
            _rsa = RSA.Create(2048);
            Console.WriteLine("[CardServer] 警告: 未找到私钥文件，已生成临时密钥对。");
            Console.WriteLine("[CardServer] 生产环境必须放置 private.pem（见 docs/卡密授权系统.md 第9节）");
            Console.WriteLine("[CardServer] 临时公钥(BEGIN PUBLIC KEY):");
            Console.WriteLine(ExportPublicKeyPem());
        }
    }

    // 签名: sig = base64(RSA-SHA256(plain))，等价 Go rsa.SignPKCS1v15
    public string Sign(string plain)
    {
        var bytes = System.Text.Encoding.UTF8.GetBytes(plain);
        var sig = _rsa.SignData(bytes, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
        return Convert.ToBase64String(sig);
    }

    // 导出公钥（SubjectPublicKeyInfo 格式 PEM），供 Go 端 x509.ParsePKIXPublicKey 验签
    public string ExportPublicKeyPem()
    {
        return _rsa.ExportSubjectPublicKeyInfoPem();
    }
}
