namespace CardServer.Models;

// 在线窗口持久化表：服务器重启后从数据库恢复在线状态，
// 避免"服务器重启 → 内存表清空 → 一窗卡被第二台设备趁重启抢占"。
// 复合主键 (CardCode, WindowID)，一卡一窗口一行。
public class WindowOnline
{
    public string CardCode { get; set; } = "";
    public string WindowID { get; set; } = "";
    public string FP { get; set; } = "";
    public long LastSeen { get; set; }
}
