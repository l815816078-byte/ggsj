namespace CardServer.Models;

// 挂机点实体（SQLite 表 HangPoints）
// 中控台"优先挂机点"下拉的选项来源
public class HangPoint
{
    public int Id { get; set; }

    // 挂机点名称（下拉显示文本）
    public string Name { get; set; } = "";

    // 所在地图名
    public string MapName { get; set; } = "";

    // 最低等级要求
    public int MinLevel { get; set; }

    // 最低战力要求
    public int MinPower { get; set; }

    // 换点次数
    public int SwitchTimes { get; set; }

    // 是否启用（停用的不出现在下拉里）
    public bool Enabled { get; set; } = true;
}
