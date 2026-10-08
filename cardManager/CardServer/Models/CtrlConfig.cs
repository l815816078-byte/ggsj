namespace CardServer.Models;

// 中控配置实体（SQLite 表 CtrlConfigs）
// 一份配置 = 某张卡下的某个「方案」（如 方案A/方案B），存在服务端，下发时逐项合并到手机端
public class CtrlConfig
{
    public int Id { get; set; }

    // 绑定卡密（配置按卡密归属）
    public string CardCode { get; set; } = "";

    // 方案名（如 方案A/方案B），同卡下唯一
    public string SchemeName { get; set; } = "";

    // 配置内容 JSON（只存中控台可见字段，未下发的字段保留手机内置值）
    public string ConfigJson { get; set; } = "{}";

    // 关联挂机点 ID（0 表示未指定）
    public int HangPointId { get; set; }

    public long UpdatedAt { get; set; }
}
