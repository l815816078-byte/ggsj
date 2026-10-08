namespace CardServer.Models;

// 项目（分类）实体（SQLite 表 Projects）：发卡时必须归属一个项目
public class Project
{
    public int Id { get; set; }

    // 项目名称（唯一索引，等于分类）
    public string Name { get; set; } = "";

    // 创建时间（Unix 秒）
    public long CreatedAt { get; set; }
}
