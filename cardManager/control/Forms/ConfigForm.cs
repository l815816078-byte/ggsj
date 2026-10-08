using System.Text.Json;
using System.Text.Json.Nodes;
using Control.Services;

namespace Control.Forms;

// 配置页：图形化表单，字段与 assets/脚本配置.json 的中控可见项对应。
// 保存为「方案」，存在服务端并绑定卡密；下发时逐项合并到手机端。
public class ConfigForm : Form
{
    private readonly ApiClient _api;

    // 方案保存/删除后触发，通知主窗体刷新"配置方案"下拉
    public event Action? SchemesChanged;

    private readonly TextBox _txtScheme = new();

    // 基础配置
    private readonly NumericUpDown _numEnhanceLimit = new();
    private readonly CheckBox _chkNormal = new();
    private readonly CheckBox _chkHigh = new();
    private readonly CheckBox _chkRare = new();
    private readonly NumericUpDown _numBagExtend = new();
    private readonly ComboBox _cmbHangPoint = new();

    // 自动上架 / 物品模式
    private readonly CheckBox _chkAutoShelf = new();
    private readonly NumericUpDown _numEquipLevel = new();

    private readonly Label _lblTip = new();

    public ConfigForm(ApiClient api)
    {
        _api = api;
        Text = "配置方案";
        Size = new Size(760, 720);
        StartPosition = FormStartPosition.CenterParent;
        BackColor = Theme.BgPanel;
        ForeColor = Theme.TextMain;
        Font = Theme.FontBody;

        BuildLayout();
        Load += async (_, _) => await LoadHangPointsAsync();
    }

    private void BuildLayout()
    {
        int y = 16;

        // 卡密由登录态决定，这里只显示当前账号 + 方案名
        var lblAccount = new Label
        {
            Text = $"当前卡密：{_api.Card}",
            Location = new Point(20, y + 5),
            Size = new Size(300, 20),
            ForeColor = Theme.TextSub
        };
        Controls.Add(lblAccount);

        AddLabel("方案名", 370, y);
        _txtScheme.Location = new Point(440, y);
        _txtScheme.Size = new Size(200, 28);
        _txtScheme.Text = "方案A";
        StyleInput(_txtScheme);
        Controls.Add(_txtScheme);
        y += 48;

        // ── 基础配置 ──
        y = AddGroup("基础配置", y);

        AddLabel("装备强化数量上限", 20, y);
        _numEnhanceLimit.Location = new Point(200, y);
        _numEnhanceLimit.Size = new Size(100, 28);
        _numEnhanceLimit.Maximum = 9999;
        _numEnhanceLimit.Value = 10;
        StyleNumeric(_numEnhanceLimit);
        Controls.Add(_numEnhanceLimit);
        y += 40;

        AddLabel("自动分解配置", 20, y);
        _chkNormal.Text = "一般";
        _chkNormal.Location = new Point(200, y);
        _chkNormal.AutoSize = true;
        _chkNormal.ForeColor = Theme.TextMain;
        _chkNormal.Checked = true;
        Controls.Add(_chkNormal);

        _chkHigh.Text = "高级";
        _chkHigh.Location = new Point(270, y);
        _chkHigh.AutoSize = true;
        _chkHigh.ForeColor = Theme.TextMain;
        _chkHigh.Checked = true;
        Controls.Add(_chkHigh);

        _chkRare.Text = "稀有";
        _chkRare.Location = new Point(340, y);
        _chkRare.AutoSize = true;
        _chkRare.ForeColor = Theme.TextMain;
        Controls.Add(_chkRare);
        y += 40;

        AddLabel("扩展背包配置", 20, y);
        _numBagExtend.Location = new Point(200, y);
        _numBagExtend.Size = new Size(100, 28);
        _numBagExtend.Maximum = 9999;
        _numBagExtend.Value = 100;
        StyleNumeric(_numBagExtend);
        Controls.Add(_numBagExtend);
        y += 40;

        AddLabel("优先挂机点", 20, y);
        _cmbHangPoint.Location = new Point(200, y);
        _cmbHangPoint.Size = new Size(240, 28);
        _cmbHangPoint.DropDownStyle = ComboBoxStyle.DropDownList;
        _cmbHangPoint.BackColor = Theme.BgRow;
        _cmbHangPoint.ForeColor = Theme.TextMain;
        Controls.Add(_cmbHangPoint);
        y += 48;

        // ── 拍卖提战 ──
        y = AddGroup("拍卖提战", y);

        _chkAutoShelf.Text = "自动上架";
        _chkAutoShelf.Location = new Point(20, y);
        _chkAutoShelf.AutoSize = true;
        _chkAutoShelf.ForeColor = Theme.TextMain;
        Controls.Add(_chkAutoShelf);
        y += 36;

        AddLabel("物品装备强化等级", 20, y);
        _numEquipLevel.Location = new Point(200, y);
        _numEquipLevel.Size = new Size(100, 28);
        _numEquipLevel.Maximum = 999;
        _numEquipLevel.Value = 6;
        StyleNumeric(_numEquipLevel);
        Controls.Add(_numEquipLevel);
        y += 48;

        // ── 按钮 ──
        var btnSave = new Button
        {
            Text = "保存方案",
            Location = new Point(20, y),
            Size = new Size(110, 38),
            FlatStyle = FlatStyle.Flat,
            BackColor = Theme.Primary,
            ForeColor = Color.White,
            Cursor = Cursors.Hand
        };
        btnSave.FlatAppearance.BorderSize = 0;
        btnSave.Click += async (_, _) => await SaveAsync();
        Controls.Add(btnSave);

        var btnLoad = new Button
        {
            Text = "读取方案",
            Location = new Point(140, y),
            Size = new Size(110, 38),
            FlatStyle = FlatStyle.Flat,
            BackColor = Theme.BgRow,
            ForeColor = Theme.TextMain,
            Cursor = Cursors.Hand
        };
        btnLoad.FlatAppearance.BorderSize = 0;
        btnLoad.Click += async (_, _) => await LoadSchemeAsync();
        Controls.Add(btnLoad);

        _lblTip.Location = new Point(20, y + 48);
        _lblTip.Size = new Size(700, 24);
        _lblTip.ForeColor = Theme.TextSub;
        Controls.Add(_lblTip);
    }

    private int AddGroup(string title, int y)
    {
        var lbl = new Label
        {
            Text = title,
            Location = new Point(20, y),
            Size = new Size(700, 24),
            ForeColor = Theme.Primary,
            Font = Theme.FontSub
        };
        Controls.Add(lbl);
        var line = new Panel
        {
            Location = new Point(20, y + 26),
            Size = new Size(700, 1),
            BackColor = Theme.BgRow
        };
        Controls.Add(line);
        return y + 40;
    }

    private void AddLabel(string text, int x, int y)
    {
        Controls.Add(new Label
        {
            Text = text,
            Location = new Point(x, y + 5),
            Size = new Size(175, 20),
            ForeColor = Theme.TextSub
        });
    }

    private static void StyleInput(TextBox tb)
    {
        tb.BackColor = Theme.BgRow;
        tb.ForeColor = Theme.TextMain;
        tb.BorderStyle = BorderStyle.FixedSingle;
    }

    private static void StyleNumeric(NumericUpDown n)
    {
        n.BackColor = Theme.BgRow;
        n.ForeColor = Theme.TextMain;
        n.BorderStyle = BorderStyle.FixedSingle;
    }

    private async Task LoadHangPointsAsync()
    {
        var data = await _api.GetHangPointsAsync();
        _cmbHangPoint.Items.Clear();
        if (data == null) return;
        foreach (var h in data.Value.EnumerateArray())
        {
            var name = h.TryGetProperty("name", out var n) ? n.GetString() : null;
            if (!string.IsNullOrEmpty(name))
            {
                _cmbHangPoint.Items.Add(name);
            }
        }
        if (_cmbHangPoint.Items.Count > 0)
        {
            _cmbHangPoint.SelectedIndex = 0;
        }
    }

    // 收集表单 → 配置 JSON（只含中控可见字段）
    private string BuildConfigJson()
    {
        var levels = new JsonArray();
        if (_chkNormal.Checked) levels.Add("一般");
        if (_chkHigh.Checked) levels.Add("高级");
        if (_chkRare.Checked) levels.Add("稀有");

        var obj = new JsonObject
        {
            ["装备强化数量上限"] = new JsonObject { ["值"] = (int)_numEnhanceLimit.Value },
            ["自动分解配置"] = new JsonObject { ["值"] = levels },
            ["扩展背包配置"] = new JsonObject { ["值"] = (int)_numBagExtend.Value },
            ["优先挂机点"] = new JsonObject { ["值"] = _cmbHangPoint.Text },
            ["拍卖提战配置"] = new JsonObject
            {
                ["自动上架"] = _chkAutoShelf.Checked,
                ["物品装备强化等级"] = (int)_numEquipLevel.Value
            }
        };
        return obj.ToJsonString(new JsonSerializerOptions
        {
            Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
            WriteIndented = true
        });
    }

    private async Task SaveAsync()
    {
        var scheme = _txtScheme.Text.Trim();
        if (string.IsNullOrEmpty(scheme))
        {
            _lblTip.ForeColor = Theme.Red;
            _lblTip.Text = "请填写方案名";
            return;
        }
        var (ok, msg) = await _api.SaveConfigAsync(scheme, BuildConfigJson(), 0);
        _lblTip.ForeColor = ok ? Theme.Green : Theme.Red;
        _lblTip.Text = ok ? "方案已保存" : "保存失败: " + msg;
        if (ok)
        {
            // 通知主窗体刷新下拉，否则新方案要重启中控台才出现
            SchemesChanged?.Invoke();
        }
    }

    private async Task LoadSchemeAsync()
    {
        var scheme = _txtScheme.Text.Trim();
        if (string.IsNullOrEmpty(scheme))
        {
            _lblTip.ForeColor = Theme.Red;
            _lblTip.Text = "请填写方案名";
            return;
        }
        var data = await _api.GetConfigsAsync();
        if (data == null)
        {
            _lblTip.ForeColor = Theme.Red;
            _lblTip.Text = "读取失败";
            return;
        }
        foreach (var item in data.Value.EnumerateArray())
        {
            var name = item.TryGetProperty("schemeName", out var n) ? n.GetString() : null;
            if (name != scheme) continue;

            var json = item.TryGetProperty("configJson", out var j) ? j.GetString() : null;
            ApplyJson(json);
            _lblTip.ForeColor = Theme.Green;
            _lblTip.Text = "方案已载入";
            return;
        }
        _lblTip.ForeColor = Theme.Red;
        _lblTip.Text = "未找到该方案";
    }

    // 把配置 JSON 回填到表单
    private void ApplyJson(string? json)
    {
        if (string.IsNullOrWhiteSpace(json)) return;
        try
        {
            var obj = JsonNode.Parse(json) as JsonObject;
            if (obj == null) return;

            if (obj["装备强化数量上限"] is JsonObject a && a["值"] != null)
                _numEnhanceLimit.Value = Clamp(a["值"]!.GetValue<int>(), _numEnhanceLimit);

            if (obj["扩展背包配置"] is JsonObject b && b["值"] != null)
                _numBagExtend.Value = Clamp(b["值"]!.GetValue<int>(), _numBagExtend);

            if (obj["优先挂机点"] is JsonObject c && c["值"] != null)
                _cmbHangPoint.Text = c["值"]!.ToString();

            if (obj["自动分解配置"] is JsonObject d && d["值"] is JsonArray arr)
            {
                var set = arr.Select(x => x?.ToString() ?? "").ToHashSet();
                _chkNormal.Checked = set.Contains("一般");
                _chkHigh.Checked = set.Contains("高级");
                _chkRare.Checked = set.Contains("稀有");
            }

            if (obj["拍卖提战配置"] is JsonObject e)
            {
                if (e["自动上架"] != null) _chkAutoShelf.Checked = e["自动上架"]!.GetValue<bool>();
                if (e["物品装备强化等级"] != null)
                    _numEquipLevel.Value = Clamp(e["物品装备强化等级"]!.GetValue<int>(), _numEquipLevel);
            }
        }
        catch
        {
            // 忽略坏配置
        }
    }

    private static decimal Clamp(int v, NumericUpDown n)
    {
        if (v < n.Minimum) return n.Minimum;
        if (v > n.Maximum) return n.Maximum;
        return v;
    }
}
