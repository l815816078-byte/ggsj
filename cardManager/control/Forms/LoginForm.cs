using Control.Services;

namespace Control.Forms;

// 登录窗体：客户只需输入卡密（服务端地址已写死在程序中）。
public class LoginForm : Form
{
    private readonly TextBox _txtCard = new();
    private readonly Button _btnLogin = new();
    private readonly Label _lblTip = new();

    public LoginForm()
    {
        Text = "AutoGo 中控台";
        Size = new Size(440, 360);
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = Theme.BgDark;
        ForeColor = Theme.TextMain;
        Font = Theme.FontBody;
        FormBorderStyle = FormBorderStyle.FixedSingle;
        MaximizeBox = false;

        var title = new Label
        {
            Text = "AutoGo 中控台",
            Font = new Font("Microsoft YaHei UI", 18f, FontStyle.Bold),
            ForeColor = Theme.TextMain,
            AutoSize = true,
            Location = new Point(40, 34)
        };
        Controls.Add(title);

        var sub = new Label
        {
            Text = "请输入卡密登录，管理您的挂机设备",
            Font = Theme.FontBody,
            ForeColor = Theme.TextDim,
            AutoSize = true,
            Location = new Point(42, 76)
        };
        Controls.Add(sub);

        AddLabel("卡密", 40, 126);
        _txtCard.Location = new Point(120, 122);
        _txtCard.Size = new Size(270, 30);
        _txtCard.CharacterCasing = CharacterCasing.Upper;
        StyleInput(_txtCard);
        Controls.Add(_txtCard);

        _btnLogin.Text = "登 录";
        _btnLogin.Location = new Point(40, 186);
        _btnLogin.Size = new Size(350, 42);
        _btnLogin.FlatStyle = FlatStyle.Flat;
        _btnLogin.FlatAppearance.BorderSize = 0;
        _btnLogin.BackColor = Theme.Primary;
        _btnLogin.ForeColor = Color.White;
        _btnLogin.Font = new Font("Microsoft YaHei UI", 10.5f, FontStyle.Bold);
        _btnLogin.Cursor = Cursors.Hand;
        _btnLogin.Click += async (_, _) => await DoLoginAsync();
        Controls.Add(_btnLogin);

        _lblTip.Location = new Point(40, 238);
        _lblTip.Size = new Size(350, 22);
        _lblTip.ForeColor = Theme.Red;
        Controls.Add(_lblTip);

        AcceptButton = _btnLogin;
    }

    private void AddLabel(string text, int x, int y)
    {
        Controls.Add(new Label
        {
            Text = text,
            Location = new Point(x, y + 6),
            Size = new Size(75, 20),
            ForeColor = Theme.TextSub
        });
    }

    private static void StyleInput(TextBox tb)
    {
        tb.BackColor = Theme.BgRow;
        tb.ForeColor = Theme.TextMain;
        tb.BorderStyle = BorderStyle.FixedSingle;
        tb.Font = new Font("Microsoft YaHei UI", 11f);
    }

    private async Task DoLoginAsync()
    {
        var card = _txtCard.Text.Trim();
        if (string.IsNullOrEmpty(card))
        {
            _lblTip.Text = "请输入卡密";
            return;
        }

        _btnLogin.Enabled = false;
        _lblTip.ForeColor = Theme.TextSub;
        _lblTip.Text = "正在登录...";

        var api = new ApiClient();
        var (ok, msg) = await api.LoginAsync(card);

        _btnLogin.Enabled = true;
        if (!ok)
        {
            _lblTip.ForeColor = Theme.Red;
            _lblTip.Text = msg;
            return;
        }

        Hide();
        var main = new MainForm(api);
        main.FormClosed += (_, _) => Close();
        main.Show();
    }
}
