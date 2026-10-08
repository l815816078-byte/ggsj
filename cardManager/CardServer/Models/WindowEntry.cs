namespace CardServer.Models;

// 在线窗口条目（存于 Redis Hash，field = WindowID，value = JSON）
public class WindowEntry
{
    // 窗口 ID（设备每次启动随机生成）
    public string WindowID { get; set; } = "";

    // 设备指纹（完整存储，展示时前端截取前几位）
    public string FP { get; set; } = "";

    // 最后心跳时间（Unix 秒）
    public long LastSeen { get; set; }
}
