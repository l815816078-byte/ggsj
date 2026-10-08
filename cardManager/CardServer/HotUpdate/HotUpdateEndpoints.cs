namespace CardServer.HotUpdate;

// 热更端点：1:1 复刻原 Go updater 消费的协议
//   GET /version.txt          → 纯文本版本号（如 "2.5"）
//   GET /size                 → app 文件字节数（404 时客户端退回整包下载）
//   GET /app?start=&end=      → 1MB 分片（越界截断）；无参数时返回整包
//   GET /app                  → 整包
public static class HotUpdateEndpoints
{
    public static void Map(IEndpointRouteBuilder app, string hotUpdateDir)
    {
        var root = Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, hotUpdateDir));

        app.MapGet("/version.txt", () =>
        {
            var file = Path.Combine(root, "version.txt");
            return FileExists(file) ? Results.Text(File.ReadAllText(file)) : Results.NotFound();
        });

        app.MapGet("/size", () =>
        {
            var file = Path.Combine(root, "app");
            return FileExists(file) ? Results.Text(FileInfoLength(file).ToString()) : Results.NotFound();
        });

        app.MapGet("/app", (HttpContext ctx, int? start, int? end) =>
        {
            var file = Path.Combine(root, "app");
            if (!FileExists(file))
                return Results.NotFound();

            var data = File.ReadAllBytes(file);
            // 无分片参数：整包
            if (start == null && end == null)
                return Results.Bytes(data, "application/octet-stream");

            // 分片：越界截断，防御异常参数
            var s = Math.Max(0, start ?? 0);
            var e = Math.Min(data.Length, end ?? data.Length);
            if (e <= s)
                return Results.Bytes(Array.Empty<byte>(), "application/octet-stream");
            return Results.Bytes(data[s..e], "application/octet-stream");
        });

        // 启动时检查热更目录
        if (!Directory.Exists(root))
            Directory.CreateDirectory(root);
        Console.WriteLine($"[CardServer] 热更目录: {root}");
    }

    private static bool FileExists(string path)
    {
        try { return File.Exists(path); }
        catch { return false; }
    }

    private static long FileInfoLength(string path)
    {
        try { return new FileInfo(path).Length; }
        catch { return 0; }
    }
}
