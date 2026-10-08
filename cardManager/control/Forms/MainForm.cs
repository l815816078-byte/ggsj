using System.Text.Json;
using Control.Services;

namespace Control.Forms;

// 主窗体：顶部标签栏（带选中高亮）+ 工具栏 + 设备表格 + 右上统计卡片 + 底部状态栏
public class MainForm : Form
{
    private readonly ApiClient _api;
    private readonly CtrlSocket _socket = new();

    // 顶部
    private readonly Label _lblTitle = new();
    private readonly Label _lblConn = new();
    private readonly List<Button> _navButtons = new();
    private readonly Panel _contentHost = new();

    // 监控页
    private readonly DataGridView _grid = new();

    // 工具栏统计
    private readonly Label _lblOnline = new();
    private readonly Label _lblTotal = new();
    private readonly Label _lblWindows = new();

    // 状态栏
    private readonly Label _lblStatus = new();

    // 配置方案下拉
    private readonly ComboBox _cmbScheme = new();

    private readonly List<DeviceRow> _rows = new();
    private readonly object _rowsLock = new();

    // 标签定义
    private static readonly string[] NavItems = { "监控", "配置", "产出明细", "更新日志", "使用说明" };
    private int _currentTab = 0;

    public MainForm(ApiClient api)
    {
        _api = api;

        Text = "AutoGo 中控台";
        Size = new Size(1440, 880);
        MinimumSize = new Size(1180, 720);
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = Theme.BgDark;
        ForeColor = Theme.TextMain;
        Font = Theme.FontBody;
        DoubleBuffered = true;

        BuildLayout();
        BindSocket();

        Load += async (_, _) =>
        {
            await RefreshDevicesAsync();
            await LoadSchemesAsync();
            StartTimer();
            _socket.Start(_api.BaseUrl, _api.Token);
        };
        FormClosed += (_, _) => _socket.Stop();
    }

    // ────────────────────────── 布局 ──────────────────────────

    private void BuildLayout()
    {
        // 顶部栏：标题 + 导航 + 连通状态
        var topBar = BuildTopBar();

        // 内容区容器（切换标签时替换内容）
        _contentHost.Dock = DockStyle.Fill;
        _contentHost.BackColor = Theme.BgDark;
        _contentHost.Padding = new Padding(16, 12, 16, 0);

        // 底部状态栏
        var statusBar = new Panel { Dock = DockStyle.Bottom, Height = 28, BackColor = Theme.BgPanel };
        _lblStatus.Dock = DockStyle.Fill;
        _lblStatus.Text = "  就绪";
        _lblStatus.ForeColor = Theme.TextDim;
        _lblStatus.TextAlign = ContentAlignment.MiddleLeft;
        statusBar.Controls.Add(_lblStatus);

        Controls.Add(_contentHost);
        Controls.Add(topBar);
        Controls.Add(statusBar);

        SwitchTab(0);
    }

    private Panel BuildTopBar()
    {
        var bar = new Panel { Dock = DockStyle.Top, Height = 64, BackColor = Theme.BgPanel };

        // 左侧：品牌
        _lblTitle.Text = "AutoGo 中控台";
        _lblTitle.Font = new Font("Microsoft YaHei UI", 13f, FontStyle.Bold);
        _lblTitle.ForeColor = Theme.TextMain;
        _lblTitle.AutoSize = true;
        _lblTitle.Location = new Point(20, 18);
        bar.Controls.Add(_lblTitle);

        // 导航按钮（带选中高亮）
        int x = 200;
        for (int i = 0; i < NavItems.Length; i++)
        {
            int idx = i;
            var btn = new Button
            {
                Text = NavItems[i],
                Location = new Point(x, 16),
                Size = new Size(92, 32),
                FlatStyle = FlatStyle.Flat,
                BackColor = Theme.BgPanel,
                ForeColor = Theme.TextSub,
                Font = Theme.FontBody,
                Cursor = Cursors.Hand,
                TabStop = false
            };
            btn.FlatAppearance.BorderSize = 0;
            btn.FlatAppearance.MouseOverBackColor = Theme.BgRow;
            btn.Click += (_, _) => SwitchTab(idx);
            bar.Controls.Add(btn);
            _navButtons.Add(btn);
            x += 96;
        }

        // 右侧：连通状态
        _lblConn.Text = "● 通信检查中";
        _lblConn.ForeColor = Theme.Yellow;
        _lblConn.AutoSize = true;
        _lblConn.Location = new Point(bar.Width - 160, 24);
        _lblConn.Anchor = AnchorStyles.Top | AnchorStyles.Right;
        bar.Controls.Add(_lblConn);

        return bar;
    }

    private void SwitchTab(int idx)
    {
        if (idx < 0 || idx >= NavItems.Length)
        {
            return;
        }
        _currentTab = idx;

        // 导航选中态
        for (int i = 0; i < _navButtons.Count; i++)
        {
            var b = _navButtons[i];
            bool active = i == idx;
            b.ForeColor = active ? Color.White : Theme.TextSub;
            b.BackColor = active ? Theme.Primary : Theme.BgPanel;
            b.Font = active ? Theme.FontSub : Theme.FontBody;
        }

        // 切换内容
        // 先把 _grid 从旧父级摘掉，否则重新 Add 到新容器时会因"已有父级"而不显示
        if (_grid.Parent != null)
        {
            _grid.Parent.Controls.Remove(_grid);
        }
        _contentHost.Controls.Clear();
        System.Windows.Forms.Control content = idx switch
        {
            0 => BuildMonitorPage(),
            1 => BuildConfigPage(),
            2 => BuildPlaceholderPage("产出明细", "产出统计与明细图表建设中"),
            3 => BuildPlaceholderPage("更新日志", "热更发布记录建设中"),
            _ => BuildPlaceholderPage("使用说明", "使用说明建设中")
        };
        content.Dock = DockStyle.Fill;
        _contentHost.Controls.Add(content);
    }

    // ────────────────────────── 监控页 ──────────────────────────

    private System.Windows.Forms.Control BuildMonitorPage()
    {
        var page = new Panel { Dock = DockStyle.Fill, BackColor = Theme.BgDark };

        // ── 工具栏卡片（一行，含统计）──
        var toolbar = BuildToolbar();

        // ── 表格卡片 ──
        var gridCard = BuildGridCard();

        // 先加 gridCard（Fill），再加 toolbar（Top）——Dock 顺序：后加的先占边
        page.Controls.Add(gridCard);
        page.Controls.Add(toolbar);
        return page;
    }

    private Panel BuildToolbar()
    {
        var card = new Panel { Dock = DockStyle.Top, Height = 108, BackColor = Theme.BgPanel };
        card.Paint += (_, e) => DrawBorder(e.Graphics, card.ClientRectangle, Theme.BgRow);

        // 第一行：操作按钮
        int x = 16;
        x = AddToolButton(card, "全选", x, 20, () => SetAllChecked(true));
        x = AddToolButton(card, "反选", x, 20, InvertChecked);
        x = AddToolButton(card, "勾选相同配置", x, 20, SelectSameScheme, 108);
        x = AddToolButton(card, "移除设备", x, 20, async () => await RemoveSelectedAsync());

        // 分隔线
        var sep1 = new Panel { Location = new Point(x + 6, 24), Size = new Size(1, 24), BackColor = Theme.BgRow };
        card.Controls.Add(sep1);
        x += 20;

        x = AddToolButton(card, "停止", x, 20, async () => await CommandAsync("stop"), 70, Theme.Red);
        x = AddToolButton(card, "重启", x, 20, async () => await CommandAsync("restart"), 70, Theme.Yellow);
        x = AddToolButton(card, "重置脚本", x, 20, async () => await CommandAsync("resetScript"), 88);

        // 第二行：方案下拉 + 下发 + 刷新
        int y2 = 62;
        var lblScheme = new Label
        {
            Text = "配置方案",
            Location = new Point(16, y2 + 5),
            Size = new Size(64, 20),
            ForeColor = Theme.TextSub
        };
        card.Controls.Add(lblScheme);

        _cmbScheme.Location = new Point(84, y2);
        _cmbScheme.Size = new Size(180, 30);
        _cmbScheme.DropDownStyle = ComboBoxStyle.DropDownList;
        _cmbScheme.FlatStyle = FlatStyle.Flat;
        _cmbScheme.BackColor = Theme.BgRow;
        _cmbScheme.ForeColor = Theme.TextMain;
        card.Controls.Add(_cmbScheme);

        AddToolButton(card, "下发配置", 276, y2, async () => await PushConfigAsync(), 88, accent: Theme.Primary, filled: true);
        AddToolButton(card, "刷新", 372, y2, async () => await RefreshDevicesAsync(), 70);

        // 右侧统计（三个小卡片）
        var stats = new Panel { Location = new Point(card.Width - 380, 16), Size = new Size(364, 78), Anchor = AnchorStyles.Top | AnchorStyles.Right, BackColor = Theme.BgPanel };
        AddStatCard(stats, _lblOnline, "当前在线", 0, Theme.Green);
        AddStatCard(stats, _lblTotal, "累计设备", 122, Theme.Primary);
        AddStatCard(stats, _lblWindows, "授权窗口", 244, Theme.Yellow);
        card.Controls.Add(stats);

        return card;
    }

    private System.Windows.Forms.Control BuildGridCard()
    {
        var card = new Panel { Dock = DockStyle.Fill, BackColor = Theme.BgPanel, Padding = new Padding(1) };

        // 表格初始化只做一次（避免每次切页重复加列导致列头错位）
        if (_grid.Columns.Count == 0)
        {
            InitGrid();
        }

        card.Controls.Add(_grid);
        return card;
    }

    // 初始化表格（列 + 样式），只调用一次
    private void InitGrid()
    {
        _grid.Dock = DockStyle.Fill;
        _grid.BackgroundColor = Theme.BgPanel;
        _grid.BorderStyle = BorderStyle.None;
        _grid.AllowUserToAddRows = false;
        _grid.AllowUserToDeleteRows = false;
        _grid.RowHeadersVisible = false;
        _grid.SelectionMode = DataGridViewSelectionMode.FullRowSelect;
        _grid.MultiSelect = true;
        // 用像素宽度而非 Fill：Fill 模式下用户拖动列宽会被自动分配覆盖，
        // 这里改成 None + 显式 Width，用户才能自由拖动调整每列宽度。
        _grid.AutoSizeColumnsMode = DataGridViewAutoSizeColumnsMode.None;
        // 列头可拖动、可调整列宽
        _grid.AllowUserToResizeColumns = true;
        _grid.ColumnHeadersHeightSizeMode = DataGridViewColumnHeadersHeightSizeMode.DisableResizing;
        _grid.EnableHeadersVisualStyles = false;
        _grid.GridColor = Theme.BgRow;
        _grid.CellBorderStyle = DataGridViewCellBorderStyle.SingleHorizontal;
        _grid.ColumnHeadersBorderStyle = DataGridViewHeaderBorderStyle.None;
        _grid.ScrollBars = ScrollBars.Vertical;
        _grid.AllowUserToResizeRows = false;
        // 允许单击进入编辑（复选框列需要它才能点选），并把复选框点击交给 CellClick 处理，
        // 避免 EditProgrammatically 导致"单击没反应"。
        _grid.EditMode = DataGridViewEditMode.EditOnEnter;
        _grid.CausesValidation = false;
        // 兜底：除勾选列外，一律不允许进入编辑态。
        // 光靠列的 ReadOnly 在 EditOnEnter 下不够稳，这里直接拦掉，杜绝"值被改掉"。
        _grid.CellBeginEdit += (_, e) =>
        {
            if (e.RowIndex >= 0 && _grid.Columns[e.ColumnIndex].Name != "check")
            {
                e.Cancel = true;
            }
        };
        // 单元格转换/解析异常不弹默认错误框（避免 FormatException 打断界面）
        _grid.DataError += (_, e) => { e.ThrowException = false; };
        // 单击勾选列 = 切换勾选（不再依赖编辑态）
        _grid.CellClick += (_, e) =>
        {
            if (e.RowIndex >= 0 && e.ColumnIndex >= 0 && _grid.Columns[e.ColumnIndex].Name == "check")
            {
                var cell = _grid.Rows[e.RowIndex].Cells["check"];
                bool cur = cell.Value is bool b && b;
                cell.Value = !cur;
                // 只读单元格不会自动重绘，手动刷新该格
                _grid.InvalidateCell(cell);
                _grid.Refresh();
            }
        };

        _grid.ColumnHeadersDefaultCellStyle.BackColor = Theme.BgDark;
        // 列头：黑色 + 小字号（比正文 9f 小一档）
        _grid.ColumnHeadersDefaultCellStyle.ForeColor = Color.Black;
        _grid.ColumnHeadersDefaultCellStyle.Font = new Font("Microsoft YaHei UI", 8.5f, FontStyle.Regular);
        _grid.ColumnHeadersDefaultCellStyle.SelectionBackColor = Theme.BgDark;
        _grid.ColumnHeadersDefaultCellStyle.SelectionForeColor = Color.Black;
        _grid.ColumnHeadersHeightSizeMode = DataGridViewColumnHeadersHeightSizeMode.DisableResizing;
        _grid.ColumnHeadersHeight = 44;
        _grid.ColumnHeadersDefaultCellStyle.WrapMode = DataGridViewTriState.False;
        _grid.ColumnHeadersDefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleCenter;

        _grid.DefaultCellStyle.BackColor = Theme.BgPanel;
        _grid.DefaultCellStyle.ForeColor = Theme.TextMain;
        _grid.DefaultCellStyle.SelectionBackColor = Theme.PrimaryDark;
        _grid.DefaultCellStyle.SelectionForeColor = Color.White;
        _grid.DefaultCellStyle.Padding = new Padding(8, 0, 8, 0);
        _grid.RowTemplate.Height = 34;
        _grid.AlternatingRowsDefaultCellStyle.BackColor = Theme.RowAlt;

        // ── 列定义 ──
        // 注意：复选框列必须用 DataGridViewCheckBoxColumn，
        // 用 DataGridViewColumn + CheckBoxCell 会在编辑态抛
        // "为 DataGridViewCheckBoxCell 提供的值的类型错误"。
        // 其余列用 DataGridViewTextBoxColumn。
        _grid.Columns.Add(new DataGridViewCheckBoxColumn
        {
            Name = "check",
            HeaderText = "",
            Width = 44,
            MinimumWidth = 30,
            SortMode = DataGridViewColumnSortMode.NotSortable,
            Resizable = DataGridViewTriState.True,
            // 只读：勾选完全交给 CellClick 处理，
            // 避免编辑态与点击处理互相覆盖导致"点了没反应"
            ReadOnly = true,
            FalseValue = false,
            TrueValue = true
        });

        // 列初始宽度（px），用户可拖动调整
        var textCols = new (string name, string header, int width)[]
        {
            ("idx", "序号", 56),
            ("deviceNo", "设备编号", 130),
            ("server", "区服", 80),
            ("role", "角色", 110),
            ("job", "职业", 80),
            ("level", "等级", 64),
            ("power", "战力", 90),
            ("gold", "金币", 90),
            ("diamond", "钻石", 80),
            ("today", "今日钻石", 100),
            ("month", "当月钻石", 100),
            ("bagFree", "背包剩余", 90),
            ("scheme", "配置", 100),
            ("state", "状态", 80)
        };
        foreach (var (name, header, width) in textCols)
        {
            _grid.Columns.Add(new DataGridViewTextBoxColumn
            {
                Name = name,
                HeaderText = header,
                Width = width,
                MinimumWidth = 40,
                SortMode = DataGridViewColumnSortMode.NotSortable,
                Resizable = DataGridViewTriState.True,
                // 数据列一律只读：表格 EditMode=EditOnEnter，不加这句双击/输入就能改值
                ReadOnly = true
            });
        }

        // 对齐方式
        _grid.Columns["check"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleCenter;
        _grid.Columns["idx"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleCenter;
        _grid.Columns["level"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleCenter;
        _grid.Columns["power"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleRight;
        _grid.Columns["gold"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleRight;
        _grid.Columns["diamond"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleRight;
        _grid.Columns["today"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleRight;
        _grid.Columns["month"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleRight;
        _grid.Columns["bagFree"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleCenter;
        _grid.Columns["state"]!.DefaultCellStyle.Alignment = DataGridViewContentAlignment.MiddleCenter;
    }

    private System.Windows.Forms.Control BuildConfigPage()
    {
        // 配置页：直接用 ConfigForm 的表单内容（以内嵌 Panel 形式）
        var host = new Panel { Dock = DockStyle.Fill, BackColor = Theme.BgPanel };
        var form = new ConfigForm(_api) { TopLevel = false, FormBorderStyle = FormBorderStyle.None, Dock = DockStyle.Fill };
        // 方案保存后刷新主界面"配置方案"下拉，否则新方案要重启中控台才出现
        form.SchemesChanged += async () => await LoadSchemesAsync();
        host.Controls.Add(form);
        form.Show();
        return host;
    }

    private System.Windows.Forms.Control BuildPlaceholderPage(string title, string desc)
    {
        var page = new Panel { Dock = DockStyle.Fill, BackColor = Theme.BgPanel };

        var lblTitle = new Label
        {
            Text = title,
            Font = Theme.FontHeading,
            ForeColor = Theme.TextMain,
            AutoSize = true,
            Location = new Point(28, 28)
        };
        var lblDesc = new Label
        {
            Text = desc,
            Font = Theme.FontBody,
            ForeColor = Theme.TextDim,
            AutoSize = true,
            Location = new Point(28, 64)
        };
        page.Controls.Add(lblTitle);
        page.Controls.Add(lblDesc);
        return page;
    }

    // ────────────────────────── 控件工厂 ──────────────────────────

    private int AddToolButton(Panel parent, string text, int x, int y, Action onClick,
        int width = 78, Color? accent = null, bool filled = false)
    {
        var btn = new Button
        {
            Text = text,
            Location = new Point(x, y),
            Size = new Size(width, 32),
            FlatStyle = FlatStyle.Flat,
            BackColor = filled ? Theme.Primary : Theme.BgRow,
            ForeColor = accent ?? (filled ? Color.White : Theme.TextMain),
            Font = Theme.FontBody,
            Cursor = Cursors.Hand,
            TabStop = false
        };
        btn.FlatAppearance.BorderSize = 0;
        btn.FlatAppearance.MouseOverBackColor = filled ? Theme.PrimaryHover : Theme.ButtonHover;
        btn.Click += (_, _) => onClick();
        parent.Controls.Add(btn);
        return x + width + 8;
    }

    private static void AddStatCard(Panel parent, Label valueLbl, string title, int x, Color accent)
    {
        var card = new Panel
        {
            Location = new Point(x, 0),
            Size = new Size(114, 78),
            BackColor = Theme.BgDark
        };
        card.Paint += (_, e) => DrawAccent(e.Graphics, card.ClientRectangle, accent);

        var lblTitle = new Label
        {
            Text = title,
            Location = new Point(14, 12),
            Size = new Size(90, 18),
            ForeColor = Theme.TextDim,
            Font = new Font("Microsoft YaHei UI", 8.5f)
        };
        valueLbl.Text = "0";
        valueLbl.Location = new Point(14, 32);
        valueLbl.Size = new Size(90, 30);
        valueLbl.ForeColor = accent;
        valueLbl.Font = new Font("Microsoft YaHei UI", 16f, FontStyle.Bold);

        card.Controls.Add(lblTitle);
        card.Controls.Add(valueLbl);
        parent.Controls.Add(card);
    }

    // 卡片描边
    private static void DrawBorder(Graphics g, Rectangle r, Color color)
    {
        using var pen = new Pen(color);
        g.DrawRectangle(pen, 0, 0, r.Width - 1, r.Height - 1);
    }

    // 左侧强调竖条
    private static void DrawAccent(Graphics g, Rectangle r, Color color)
    {
        using var brush = new SolidBrush(color);
        g.FillRectangle(brush, 0, 14, 3, r.Height - 28);
    }

    // ────────────────────────── 实时推送 ──────────────────────────

    private void BindSocket()
    {
        _socket.OnConnectionChanged += connected =>
        {
            if (IsDisposed) return;
            BeginInvoke(() =>
            {
                _lblConn.Text = connected ? "● 通信正常" : "● 通信断开";
                _lblConn.ForeColor = connected ? Theme.Green : Theme.Red;
            });
        };

        _socket.OnMessage += (type, data) =>
        {
            if (IsDisposed) return;
            BeginInvoke(() => HandleSocketMessage(type, data));
        };
    }

    private void HandleSocketMessage(string type, JsonElement data)
    {
        switch (type)
        {
            case "snapshot":
                if (data.TryGetProperty("devices", out var arr))
                {
                    lock (_rowsLock)
                    {
                        _rows.Clear();
                        foreach (var d in arr.EnumerateArray())
                        {
                            _rows.Add(ParseDevice(d));
                        }
                    }
                    RenderRows();
                }
                break;

            case "deviceUpdate":
                lock (_rowsLock)
                {
                    // 按「设备号」找行（设备身份）；重启换 WindowId 也能对上
                    var devNo = Str(data, "deviceNo");
                    var idx = !string.IsNullOrEmpty(devNo)
                        ? _rows.FindIndex(r => r.DeviceNo == devNo)
                        : _rows.FindIndex(r => r.WindowId == Str(data, "windowId"));

                    if (idx >= 0)
                    {
                        // 局部更新：服务端可能只推变化的部分字段（如只推 schemeName），
                        // 若用 ParseDevice 整体替换会把没推的字段清成空。
                        ApplyPartial(_rows[idx], data);
                    }
                    else
                    {
                        _rows.Add(ParseDevice(data));
                    }
                }
                RenderRows();
                break;

            case "deviceRemoved":
                // 某设备被移除（可能是别的操作触发），重新拉列表
                _ = RefreshDevicesAsync();
                break;

            case "cmdAck":
                // 设备端命令回执：让用户看到"确实执行了"
                SetStatus($"设备已执行: {Str(data, "cmd")}", Theme.Green);
                break;

            case "deviceOnline":
            case "deviceOffline":
                _ = RefreshDevicesAsync();
                break;
        }
    }

    private static DeviceRow ParseDevice(JsonElement d)
    {
        string S(string k) => d.TryGetProperty(k, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() ?? "" : "";
        int I(string k) => d.TryGetProperty(k, out var v) && v.ValueKind == JsonValueKind.Number ? v.GetInt32() : 0;
        long L(string k) => d.TryGetProperty(k, out var v) && v.ValueKind == JsonValueKind.Number ? v.GetInt64() : 0;

        return new DeviceRow
        {
            WindowId = S("windowId"),
            DeviceNo = S("deviceNo"),
            Server = S("server"),
            Role = S("role"),
            Job = S("job"),
            Level = I("level"),
            Power = I("power"),
            Gold = L("gold"),
            Diamond = L("diamond"),
            TodayDiamond = L("todayDiamond"),
            MonthDiamond = L("monthDiamond"),
            BagFree = I("bagFree"),
            Scheme = S("schemeName"),
            State = S("state")
        };
    }

    // 取 JSON 字符串字段（缺失返回空串）
    private static string Str(JsonElement d, string key) =>
        d.ValueKind == JsonValueKind.Object && d.TryGetProperty(key, out var v) && v.ValueKind == JsonValueKind.String
            ? v.GetString() ?? ""
            : "";

    // 局部更新：只覆盖推送里出现的字段（服务端可能只推变化的部分，不能整体替换）
    private static void ApplyPartial(DeviceRow row, JsonElement d)
    {
        if (d.ValueKind != JsonValueKind.Object) return;

        bool Has(string k) => d.TryGetProperty(k, out _);
        string S(string k) => Str(d, k);
        int I(string k) => d.TryGetProperty(k, out var v) && v.ValueKind == JsonValueKind.Number ? v.GetInt32() : 0;
        long L(string k) => d.TryGetProperty(k, out var v) && v.ValueKind == JsonValueKind.Number ? v.GetInt64() : 0;

        if (Has("windowId")) row.WindowId = S("windowId");
        if (Has("deviceNo")) row.DeviceNo = S("deviceNo");
        if (Has("server")) row.Server = S("server");
        if (Has("role")) row.Role = S("role");
        if (Has("job")) row.Job = S("job");
        if (Has("level")) row.Level = I("level");
        if (Has("power")) row.Power = I("power");
        if (Has("gold")) row.Gold = L("gold");
        if (Has("diamond")) row.Diamond = L("diamond");
        if (Has("todayDiamond")) row.TodayDiamond = L("todayDiamond");
        if (Has("monthDiamond")) row.MonthDiamond = L("monthDiamond");
        if (Has("bagFree")) row.BagFree = I("bagFree");
        if (Has("schemeName")) row.Scheme = S("schemeName");
        if (Has("state")) row.State = S("state");
    }

    // ────────────────────────── 数据刷新 ──────────────────────────

    private async Task RefreshDevicesAsync()
    {
        var data = await _api.GetDevicesAsync();
        if (data == null)
        {
            SetStatus("拉取设备列表失败", Theme.Red);
            return;
        }

        lock (_rowsLock)
        {
            _rows.Clear();
            foreach (var d in data.Value.EnumerateArray())
            {
                _rows.Add(ParseDevice(d));
            }
        }
        RenderRows();
        await RefreshStatsAsync();
    }

    private async Task RefreshStatsAsync()
    {
        var stats = await _api.GetStatsAsync();
        if (stats == null) return;
        int Get(string k) => stats.Value.TryGetProperty(k, out var v) && v.ValueKind == JsonValueKind.Number ? v.GetInt32() : 0;
        _lblOnline.Text = Get("online").ToString();
        _lblTotal.Text = Get("total").ToString();
        // 授权窗口显示 "在线/上限"（maxwin 是该卡的窗口数上限）
        int maxwin = Get("maxwin");
        _lblWindows.Text = maxwin > 0 ? $"{Get("online")}/{maxwin}" : Get("windows").ToString();
    }

    // 加载本卡的配置方案到下拉
    private async Task LoadSchemesAsync()
    {
        var data = await _api.GetConfigsAsync();
        _cmbScheme.Items.Clear();
        if (data == null) return;
        foreach (var item in data.Value.EnumerateArray())
        {
            var name = item.TryGetProperty("schemeName", out var n) ? n.GetString() : null;
            if (!string.IsNullOrEmpty(name))
            {
                _cmbScheme.Items.Add(name);
            }
        }
        if (_cmbScheme.Items.Count > 0)
        {
            _cmbScheme.SelectedIndex = 0;
        }
    }

    private void RenderRows()
    {
        // 勾选状态以「设备号」为键（设备身份 = 卡密 + 设备号；重启换 WindowId 勾选也不丢）
        var checkedIds = new HashSet<string>();
        foreach (DataGridViewRow r in _grid.Rows)
        {
            if (r.Cells["check"].Value is true && r.Tag is string key)
            {
                checkedIds.Add(key);
            }
        }

        _grid.Rows.Clear();
        lock (_rowsLock)
        {
            int i = 1;
            foreach (var d in _rows)
            {
                int idx = _grid.Rows.Add();
                var row = _grid.Rows[idx];
                row.Tag = d.DeviceNo;
                // 单元格值统一用 string：混用 int/null 会触发
                // "单元格的 Formatted 值的类型错误"（FormatException）
                row.Cells["check"].Value = checkedIds.Contains(d.DeviceNo);
                row.Cells["idx"].Value = i++.ToString();
                row.Cells["deviceNo"].Value = d.DeviceNo;
                row.Cells["server"].Value = d.Server;
                row.Cells["role"].Value = d.Role;
                row.Cells["job"].Value = d.Job;
                row.Cells["level"].Value = d.Level > 0 ? d.Level.ToString() : "";
                row.Cells["power"].Value = d.Power > 0 ? d.Power.ToString() : "";
                row.Cells["gold"].Value = d.Gold > 0 ? d.Gold.ToString() : "";
                row.Cells["diamond"].Value = d.Diamond > 0 ? d.Diamond.ToString() : "";
                row.Cells["today"].Value = d.TodayDiamond.ToString();
                row.Cells["month"].Value = d.MonthDiamond.ToString();
                row.Cells["bagFree"].Value = d.BagFree > 0 ? d.BagFree.ToString() : "";
                row.Cells["scheme"].Value = d.Scheme;
                row.Cells["state"].Value = Theme.StateText(d.State);

                if (d.State == "offline")
                {
                    row.DefaultCellStyle.ForeColor = Theme.Gray;
                }
                row.Cells["state"].Style.ForeColor = Theme.StateColor(d.State);
                row.Cells["state"].Style.Font = Theme.FontSub;
            }
        }
        SetStatus($"共 {_grid.Rows.Count} 台设备", Theme.TextDim);
    }

    private void SetStatus(string text, Color color)
    {
        _lblStatus.Text = "  " + text;
        _lblStatus.ForeColor = color;
    }

    // ────────────────────────── 工具栏动作 ──────────────────────────

    private void SetAllChecked(bool v)
    {
        foreach (DataGridViewRow r in _grid.Rows)
        {
            r.Cells["check"].Value = v;
        }
    }

    private void InvertChecked()
    {
        foreach (DataGridViewRow r in _grid.Rows)
        {
            r.Cells["check"].Value = !(r.Cells["check"].Value is true);
        }
    }

    private void SelectSameScheme()
    {
        var first = _grid.Rows.Cast<DataGridViewRow>().FirstOrDefault(r => r.Cells["check"].Value is true);
        if (first == null)
        {
            MessageBox.Show("请先勾选一台设备", "提示");
            return;
        }
        var scheme = first.Cells["scheme"].Value?.ToString() ?? "";
        foreach (DataGridViewRow r in _grid.Rows)
        {
            r.Cells["check"].Value = (r.Cells["scheme"].Value?.ToString() ?? "") == scheme;
        }
    }

    // 移除设备：真删库（调服务端 DELETE /api/ctrl/devices，按设备号删）
    private async Task RemoveSelectedAsync()
    {
        var deviceNos = SelectedDeviceNos();
        if (deviceNos.Count == 0)
        {
            MessageBox.Show("请先勾选要移除的设备", "提示");
            return;
        }
        if (MessageBox.Show($"确定移除选中的 {deviceNos.Count} 台设备？\n（设备端会断开连接，数据一并删除）",
                "确认移除", MessageBoxButtons.OKCancel, MessageBoxIcon.Warning) != DialogResult.OK)
        {
            return;
        }

        int okCount = 0;
        string lastMsg = "";
        foreach (var no in deviceNos)
        {
            var (ok, msg) = await _api.RemoveDeviceAsync(no);
            if (ok) okCount++;
            else lastMsg = msg;
        }

        await RefreshDevicesAsync();
        if (okCount == deviceNos.Count)
            SetStatus($"已移除 {okCount} 台设备", Theme.Green);
        else
            SetStatus($"移除 {okCount}/{deviceNos.Count} 台，失败原因: {lastMsg}", Theme.Red);
    }

    // 勾选行的设备号（= 行 Tag；设备身份，删除/下发都用它定位）
    private List<string> SelectedDeviceNos()
    {
        var list = new List<string>();
        foreach (DataGridViewRow r in _grid.Rows)
        {
            if (r.Cells["check"].Value is true && r.Tag is string no)
            {
                list.Add(no);
            }
        }
        return list;
    }

    // 勾选行对应的 WindowId（下发给服务端找连接用，服务端连接表按 WindowId 存）
    private List<string> SelectedWindowIds()
    {
        var nos = SelectedDeviceNos();
        var list = new List<string>();
        lock (_rowsLock)
        {
            foreach (var no in nos)
            {
                var d = _rows.FirstOrDefault(x => x.DeviceNo == no);
                if (d != null && !string.IsNullOrEmpty(d.WindowId))
                {
                    list.Add(d.WindowId);
                }
            }
        }
        return list;
    }

    private async Task CommandAsync(string cmd)
    {
        var ids = SelectedWindowIds();
        if (ids.Count == 0)
        {
            MessageBox.Show("请先勾选设备", "提示");
            return;
        }
        var (ok, msg) = await _api.SendCommandAsync(ids, cmd);
        SetStatus(ok ? $"命令已下发: {cmd} ({msg})" : $"命令失败: {msg}", ok ? Theme.Green : Theme.Red);
    }

    private async Task PushConfigAsync()
    {
        var ids = SelectedWindowIds();
        if (ids.Count == 0)
        {
            MessageBox.Show("请先勾选设备", "提示");
            return;
        }
        var scheme = _cmbScheme.Text.Trim();
        if (string.IsNullOrEmpty(scheme))
        {
            MessageBox.Show("请先选择配置方案", "提示");
            return;
        }
        var (ok, msg) = await _api.PushConfigAsync(ids, scheme);
        SetStatus(ok ? $"配置已下发: {msg}" : $"下发失败: {msg}", ok ? Theme.Green : Theme.Red);
    }

    // ────────────────────────── 定时刷新 ──────────────────────────

    private System.Windows.Forms.Timer? _timer;

    private void StartTimer()
    {
        _timer = new System.Windows.Forms.Timer { Interval = 30000 };
        _timer.Tick += async (_, _) => await RefreshDevicesAsync();
        _timer.Start();
    }
}

// 设备行数据
public class DeviceRow
{
    public string WindowId { get; set; } = "";
    public string DeviceNo { get; set; } = "";
    public string Server { get; set; } = "";
    public string Role { get; set; } = "";
    public string Job { get; set; } = "";
    public int Level { get; set; }
    public int Power { get; set; }
    public long Gold { get; set; }
    public long Diamond { get; set; }
    public long TodayDiamond { get; set; }
    public long MonthDiamond { get; set; }
    public int BagFree { get; set; }
    public string Scheme { get; set; } = "";
    public string State { get; set; } = "offline";
}
