using System.Drawing;

namespace Control.Services;

// 浅色简洁风格统一配色（白底 + 浅灰卡片 + 蓝色主色）
public static class Theme
{
    // 主色
    public static readonly Color Primary = ColorTranslator.FromHtml("#2D7FF9");
    public static readonly Color PrimaryHover = ColorTranslator.FromHtml("#3A8DFF");
    public static readonly Color PrimaryDark = ColorTranslator.FromHtml("#1E6FE0");

    // 背景层次
    public static readonly Color BgDark = ColorTranslator.FromHtml("#F5F7FA");   // 窗体底（浅灰）
    public static readonly Color BgPanel = ColorTranslator.FromHtml("#FFFFFF");  // 面板/卡片（纯白）
    public static readonly Color BgRow = ColorTranslator.FromHtml("#EEF1F5");    // 行/控件底

    // 描边
    public static readonly Color Border = ColorTranslator.FromHtml("#DFE3E8");   // 卡片描边
    public static readonly Color Divider = ColorTranslator.FromHtml("#E5E9EF"); // 分隔线/表格线

    // 文字
    public static readonly Color TextMain = ColorTranslator.FromHtml("#1F2933");
    public static readonly Color TextSub = ColorTranslator.FromHtml("#52606D");
    public static readonly Color TextDim = ColorTranslator.FromHtml("#9AA5B1");

    // 状态色
    public static readonly Color Green = ColorTranslator.FromHtml("#4CAF50");   // 运行中
    public static readonly Color Yellow = ColorTranslator.FromHtml("#FFC107");  // 暂停
    public static readonly Color Red = ColorTranslator.FromHtml("#F44336");     // 停止
    public static readonly Color Gray = ColorTranslator.FromHtml("#8A93A6");    // 离线

    // 表格隔行
    public static readonly Color RowAlt = ColorTranslator.FromHtml("#FAFBFC");
    public static readonly Color ButtonHover = ColorTranslator.FromHtml("#E4E9F0");

    // 状态文本 → 颜色
    public static Color StateColor(string state) => state switch
    {
        "running" => Green,
        "paused" => ColorTranslator.FromHtml("#E6A100"),   // 浅底上黄色偏深才看得清
        "stopped" => Red,
        _ => Gray
    };

    public static string StateText(string state) => state switch
    {
        "running" => "运行中",
        "paused" => "暂停",
        "stopped" => "停止",
        _ => "离线"
    };

    // 字体体系
    public static readonly Font FontHeading = new("Microsoft YaHei UI", 11f, FontStyle.Bold);
    public static readonly Font FontSub = new("Microsoft YaHei UI", 9.5f, FontStyle.Bold);
    public static readonly Font FontBody = new("Microsoft YaHei UI", 9f, FontStyle.Regular);
}
