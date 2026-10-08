using Control.Services;

namespace Control.Forms;

// 统计弹窗（先留空位）：后续可放产出汇总、设备分布等图表。
public class StatsForm : Form
{
    public StatsForm()
    {
        Text = "统计";
        Size = new Size(520, 380);
        StartPosition = FormStartPosition.CenterParent;
        BackColor = Theme.BgPanel;
        ForeColor = Theme.TextMain;
        Font = Theme.FontBody;

        var lbl = new Label
        {
            Text = "统计功能建设中\n\n（产出明细、设备分布等图表将在此展示）",
            Dock = DockStyle.Fill,
            TextAlign = ContentAlignment.MiddleCenter,
            ForeColor = Theme.TextDim
        };
        Controls.Add(lbl);
    }
}
