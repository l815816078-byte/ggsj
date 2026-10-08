namespace CardServer.Models;

// 账号实体（SQLite 表 Agents）：管理员 + 代理统一存储，用 Role 区分
public class Agent
{
    public int Id { get; set; }

    // 用户名（唯一索引）
    public string Username { get; set; } = "";

    // PBKDF2 密码哈希（Base64）
    public string PassHash { get; set; } = "";

    // 随机盐（Base64）
    public string Salt { get; set; } = "";

    // 角色: admin=管理员 agent=代理
    public string Role { get; set; } = "agent";

    // 剩余可发卡数；admin 为 -1 表示不限
    public int Quota { get; set; }

    // 创建时间（Unix 秒）
    public long CreatedAt { get; set; }

    // 上级用户名（预留二级分销，普通代理恒为 "admin"）
    public string Parent { get; set; } = "admin";
}
