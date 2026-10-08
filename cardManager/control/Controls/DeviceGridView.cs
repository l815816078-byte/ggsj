using Control.Services;

namespace Control.Controls;

// 设备表格控件：VirtualMode 承载大量行，避免大表时 UI 卡顿。
// 方案要求「表格用虚拟模式(VM)承载大量行，避免大量行时 UI 卡顿」。
//
// 使用方式：外部维护数据源 List<T>，设置 RowCount 并在 CellValueNeeded 中填值。
public class DeviceGridView : DataGridView
{
    // 数据源（外部赋值，本控件只负责渲染）
    private List<object[]> _source = new();

    public DeviceGridView()
    {
        VirtualMode = true;
        AllowUserToAddRows = false;
        AllowUserToDeleteRows = false;
        RowHeadersVisible = false;
        SelectionMode = DataGridViewSelectionMode.FullRowSelect;
        MultiSelect = true;
        AutoSizeColumnsMode = DataGridViewAutoSizeColumnsMode.Fill;
        EnableHeadersVisualStyles = false;
        BorderStyle = BorderStyle.None;
        BackgroundColor = Theme.BgDark;

        ColumnHeadersDefaultCellStyle.BackColor = Theme.BgPanel;
        ColumnHeadersDefaultCellStyle.ForeColor = Theme.TextSub;
        ColumnHeadersDefaultCellStyle.Font = Theme.FontSub;
        ColumnHeadersHeight = 40;

        DefaultCellStyle.BackColor = Theme.BgDark;
        DefaultCellStyle.ForeColor = Theme.TextMain;
        DefaultCellStyle.SelectionBackColor = Theme.PrimaryDark;
        DefaultCellStyle.SelectionForeColor = Color.White;
        RowTemplate.Height = 32;

        CellValueNeeded += OnCellValueNeeded;
    }

    // 设置数据源并刷新
    public void SetSource(List<object[]> rows)
    {
        _source = rows ?? new List<object[]>();
        RowCount = _source.Count;
        Invalidate();
    }

    private void OnCellValueNeeded(object? sender, DataGridViewCellValueEventArgs e)
    {
        if (e.RowIndex < 0 || e.RowIndex >= _source.Count)
        {
            return;
        }
        var row = _source[e.RowIndex];
        if (e.ColumnIndex < 0 || e.ColumnIndex >= row.Length)
        {
            return;
        }
        e.Value = row[e.ColumnIndex];
    }
}
