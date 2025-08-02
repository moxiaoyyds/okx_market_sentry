package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"okx-market-sentry/pkg/types"
	"strings"
	"time"
	"unicode/utf8"
)

// safePadding 安全地计算填充空格数量，避免负数
func safePadding(content string, totalWidth int) int {
	// 使用utf8.RuneCountInString计算实际显示字符数，而不是字节数
	runeCount := utf8.RuneCountInString(content)
	padding := totalWidth - runeCount - 4 // 4是边框字符数
	if padding < 0 {
		padding = 0
	}
	return padding
}

// Interface 通知接口
type Interface interface {
	SendAlert(alert *types.AlertData) error
	SendBatchAlerts(alerts []*types.AlertData) error
}

// ConsoleNotifier 控制台通知器
type ConsoleNotifier struct{}

func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{}
}

func (cn *ConsoleNotifier) SendAlert(alert *types.AlertData) error {
	// 生成漂亮的控制台输出
	cn.printAlert(alert)
	return nil
}

func (cn *ConsoleNotifier) SendBatchAlerts(alerts []*types.AlertData) error {
	if len(alerts) == 0 {
		return nil
	}

	if len(alerts) == 1 {
		return cn.SendAlert(alerts[0])
	}

	// 批量预警的控制台输出
	cn.printBatchAlerts(alerts)
	return nil
}

func (cn *ConsoleNotifier) printAlert(alert *types.AlertData) {
	// 创建一个漂亮的预警框
	border := "╔" + strings.Repeat("═", 60) + "╗"
	bottomBorder := "╚" + strings.Repeat("═", 60) + "╝"

	// 获取变化方向的箭头
	arrow := "📈"
	if alert.ChangePercent < 0 {
		arrow = "📉"
	}

	fmt.Println()
	fmt.Println(border)
	fmt.Printf("║ %s 🚨 价格预警触发！%s ║\n", arrow, strings.Repeat(" ", 34))
	fmt.Println("║" + strings.Repeat(" ", 60) + "║")
	fmt.Printf("║ 交易对: %-47s ║\n", alert.Symbol)
	fmt.Printf("║ 当前价格: $%-43.6f ║\n", alert.CurrentPrice)
	fmt.Printf("║ 5分钟前价格: $%-39.6f ║\n", alert.PastPrice)

	// 根据涨跌幅显示不同颜色的提示
	changeStr := fmt.Sprintf("%.2f%%", alert.ChangePercent)
	if alert.ChangePercent > 0 {
		fmt.Printf("║ 涨幅: +%-48s ║\n", changeStr)
	} else {
		fmt.Printf("║ 跌幅: %-49s ║\n", changeStr)
	}

	fmt.Printf("║ 预警时间: %-44s ║\n", alert.AlertTime.Format("2006-01-02 15:04:05"))
	fmt.Println("║" + strings.Repeat(" ", 60) + "║")

	// 添加提示信息
	if alert.ChangePercent > 0 {
		fmt.Printf("║ 💡 该交易对出现显著上涨，请关注市场动向！%-14s ║\n", "")
	} else {
		fmt.Printf("║ 💡 该交易对出现显著下跌，请关注风险控制！%-14s ║\n", "")
	}

	fmt.Println(bottomBorder)
	fmt.Println()
}

func (cn *ConsoleNotifier) printBatchAlerts(alerts []*types.AlertData) {
	// 创建批量预警的漂亮输出
	border := "╔" + strings.Repeat("═", 80) + "╗"
	bottomBorder := "╚" + strings.Repeat("═", 80) + "╝"

	fmt.Println()
	fmt.Println(border)

	// 标题行
	title := fmt.Sprintf("🚨 批量价格预警触发！- %d个币种", len(alerts))
	padding := safePadding(title, 80)
	fmt.Printf("║ %s%s ║\n", title, strings.Repeat(" ", padding))
	fmt.Println("║" + strings.Repeat(" ", 80) + "║")

	// 显示预警列表
	for i, alert := range alerts {
		arrow := "📈"
		if alert.ChangePercent < 0 {
			arrow = "📉"
		}

		changeStr := fmt.Sprintf("%+.2f%%", alert.ChangePercent)
		content := fmt.Sprintf("%d. %s %s: $%.6f (%s)",
			i+1, arrow, alert.Symbol, alert.CurrentPrice, changeStr)

		// 使用安全的填充计算
		padding := safePadding(content, 80)
		fmt.Printf("║ %s%s ║\n", content, strings.Repeat(" ", padding))
	}

	fmt.Println("║" + strings.Repeat(" ", 80) + "║")

	// 预警时间
	timeStr := fmt.Sprintf("预警时间: %s", alerts[0].AlertTime.Format("2006-01-02 15:04:05"))
	padding = safePadding(timeStr, 80)
	fmt.Printf("║ %s%s ║\n", timeStr, strings.Repeat(" ", padding))

	fmt.Println("║" + strings.Repeat(" ", 80) + "║")

	// 提示信息
	msg := "💡 多个交易对同时出现显著波动，请密切关注市场动向！"
	padding = safePadding(msg, 80)
	fmt.Printf("║ %s%s ║\n", msg, strings.Repeat(" ", padding))

	fmt.Println(bottomBorder)
	fmt.Println()
}

// PushPlusNotifier PushPlus通知器
type PushPlusNotifier struct {
	userToken  string
	to         string // 好友令牌，多人用逗号分隔
	enabled    bool
	httpClient *http.Client
}

type PushPlusRequest struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Template string `json:"template"`
	To       string `json:"to,omitempty"` // 好友令牌，给朋友发送通知
}

type PushPlusResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data string `json:"data"`
}

func NewPushPlusNotifier(userToken, to string) Interface {
	// 如果没有配置user token，返回控制台通知器
	if userToken == "" {
		fmt.Println("🔧 未配置PushPlus User Token，使用控制台输出模式")
		return NewConsoleNotifier()
	}

	if to != "" {
		fmt.Printf("✅ 已配置PushPlus通知服务（包含好友推送: %s）\n", to)
	} else {
		fmt.Println("✅ 已配置PushPlus通知服务")
	}

	return &PushPlusNotifier{
		userToken: userToken,
		to:        to,
		enabled:   true,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (ppn *PushPlusNotifier) SendAlert(alert *types.AlertData) error {
	if !ppn.enabled {
		// 降级为控制台输出
		console := NewConsoleNotifier()
		return console.SendAlert(alert)
	}

	// 构建PushPlus消息内容
	title := fmt.Sprintf("📈 OKX价格预警 - %s", alert.Symbol)
	content := ppn.buildHTMLContent(alert)

	// 发送PushPlus通知
	err := ppn.sendPushPlusMessage(title, content)
	if err != nil {
		fmt.Printf("❌ PushPlus发送失败: %v，降级为控制台输出\n", err)
		// 降级为控制台输出
		console := NewConsoleNotifier()
		return console.SendAlert(alert)
	}

	fmt.Printf("✅ PushPlus通知已发送: %s 变化 %+.2f%%\n", alert.Symbol, alert.ChangePercent)
	return nil
}

func (ppn *PushPlusNotifier) SendBatchAlerts(alerts []*types.AlertData) error {
	if len(alerts) == 0 {
		return nil
	}

	if len(alerts) == 1 {
		return ppn.SendAlert(alerts[0])
	}

	if !ppn.enabled {
		// 降级为控制台输出
		console := NewConsoleNotifier()
		return console.SendBatchAlerts(alerts)
	}

	// 构建批量预警消息
	title := fmt.Sprintf("📊 OKX批量价格预警 - %d个币种", len(alerts))
	content := ppn.buildBatchHTMLContent(alerts)

	// 发送PushPlus通知
	err := ppn.sendPushPlusMessage(title, content)
	if err != nil {
		fmt.Printf("❌ PushPlus批量发送失败: %v，降级为控制台输出\n", err)
		// 降级为控制台输出
		console := NewConsoleNotifier()
		return console.SendBatchAlerts(alerts)
	}

	fmt.Printf("✅ PushPlus批量通知已发送: %d个币种预警\n", len(alerts))
	return nil
}

func (ppn *PushPlusNotifier) buildHTMLContent(alert *types.AlertData) string {
	// 获取变化方向和颜色
	arrow := "📈"
	color := "#00C851" // 绿色表示上涨
	changeText := "上涨"
	if alert.ChangePercent < 0 {
		arrow = "📉"
		color = "#FF4444" // 红色表示下跌
		changeText = "下跌"
	}

	// 构建HTML格式的消息内容
	content := fmt.Sprintf(`
<div style="border: 2px solid %s; border-radius: 10px; padding: 20px; margin: 10px; background-color: #f9f9f9;">
    <h2 style="color: %s; text-align: center; margin-top: 0;">%s 价格预警触发</h2>
    
    <div style="background-color: white; padding: 15px; border-radius: 8px; margin: 10px 0;">
        <p><strong>交易对:</strong> <span style="font-size: 18px; color: #333;">%s</span></p>
        <p><strong>当前价格:</strong> <span style="font-size: 16px; color: #333;">$%.6f</span></p>
        <p><strong>5分钟前价格:</strong> <span style="font-size: 16px; color: #333;">$%.6f</span></p>
        <p><strong>价格变化:</strong> <span style="font-size: 18px; font-weight: bold; color: %s;">%+.2f%%</span></p>
        <p><strong>预警时间:</strong> <span style="color: #666;">%s</span></p>
    </div>
    
    <div style="background-color: %s; color: white; padding: 10px; border-radius: 8px; text-align: center; margin-top: 15px;">
        <strong>💡 该交易对出现显著%s，请关注市场动向！</strong>
    </div>
</div>
`,
		color, color, arrow,
		alert.Symbol,
		alert.CurrentPrice,
		alert.PastPrice,
		color, alert.ChangePercent,
		alert.AlertTime.Format("2006-01-02 15:04:05"),
		color, changeText)

	return content
}

func (ppn *PushPlusNotifier) sendPushPlusMessage(title, content string) error {
	// 构建请求数据
	reqData := PushPlusRequest{
		Token:    ppn.userToken,
		Title:    title,
		Content:  content,
		Template: "html",
		To:       ppn.to, // 添加好友令牌支持
	}

	// 序列化为JSON
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 发送HTTP请求
	resp, err := ppn.httpClient.Post(
		"http://www.pushplus.plus/send",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var pushResp PushPlusResponse
	if err := json.NewDecoder(resp.Body).Decode(&pushResp); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查返回结果
	if pushResp.Code != 200 {
		return fmt.Errorf("PushPlus API错误: %s", pushResp.Msg)
	}

	return nil
}

func (ppn *PushPlusNotifier) buildBatchHTMLContent(alerts []*types.AlertData) string {
	if len(alerts) == 0 {
		return ""
	}

	// 统计涨跌情况
	upCount := 0
	downCount := 0
	for _, alert := range alerts {
		if alert.ChangePercent > 0 {
			upCount++
		} else {
			downCount++
		}
	}

	// 构建HTML格式的批量消息内容
	content := fmt.Sprintf(`
<div style="border: 2px solid #FF6B6B; border-radius: 10px; padding: 20px; margin: 10px; background-color: #f9f9f9;">
    <h2 style="color: #FF6B6B; text-align: center; margin-top: 0;">🚨 批量价格预警触发</h2>
    
    <div style="background-color: #E3F2FD; padding: 15px; border-radius: 8px; margin: 10px 0;">
        <p style="font-size: 16px; margin: 5px 0;"><strong>预警统计:</strong></p>
        <p style="margin: 5px 0;">📈 上涨币种: <span style="color: #00C851; font-weight: bold;">%d个</span></p>
        <p style="margin: 5px 0;">📉 下跌币种: <span style="color: #FF4444; font-weight: bold;">%d个</span></p>
        <p style="margin: 5px 0;">🕐 预警时间: <span style="color: #666;">%s</span></p>
    </div>
    
    <div style="background-color: white; padding: 15px; border-radius: 8px; margin: 10px 0; max-height: 400px; overflow-y: auto;">
        <h3 style="color: #333; margin-top: 0;">详细列表:</h3>
        <table style="width: 100%%; border-collapse: collapse;">
            <tr style="background-color: #f0f0f0;">
                <th style="padding: 8px; text-align: left; border-bottom: 1px solid #ddd;">币种</th>
                <th style="padding: 8px; text-align: right; border-bottom: 1px solid #ddd;">当前价格</th>
                <th style="padding: 8px; text-align: right; border-bottom: 1px solid #ddd;">涨跌幅</th>
            </tr>`,
		upCount, downCount, alerts[0].AlertTime.Format("2006-01-02 15:04:05"))

	// 添加每个预警的详细信息
	for _, alert := range alerts {
		arrow := "📈"
		color := "#00C851"
		if alert.ChangePercent < 0 {
			arrow = "📉"
			color = "#FF4444"
		}

		content += fmt.Sprintf(`
            <tr>
                <td style="padding: 8px; border-bottom: 1px solid #eee;">%s %s</td>
                <td style="padding: 8px; text-align: right; border-bottom: 1px solid #eee;">$%.6f</td>
                <td style="padding: 8px; text-align: right; border-bottom: 1px solid #eee; color: %s; font-weight: bold;">%+.2f%%</td>
            </tr>`,
			arrow, alert.Symbol, alert.CurrentPrice, color, alert.ChangePercent)
	}

	content += `
        </table>
    </div>
    
    <div style="background-color: #FF6B6B; color: white; padding: 15px; border-radius: 8px; text-align: center; margin-top: 15px;">
        <strong>⚠️ 多个交易对同时出现显著波动，请密切关注市场动向！</strong>
    </div>
</div>`

	return content
}

// DingTalkNotifier 钉钉通知器（保留兼容性）
type DingTalkNotifier struct {
	webhookURL string
	enabled    bool
}

func NewDingTalkNotifier(webhookURL string) Interface {
	// 如果没有配置webhook URL，返回控制台通知器
	if webhookURL == "" {
		fmt.Println("🔧 未配置钉钉Webhook URL，使用控制台输出模式")
		return NewConsoleNotifier()
	}

	return &DingTalkNotifier{
		webhookURL: webhookURL,
		enabled:    true,
	}
}

func (dtn *DingTalkNotifier) SendAlert(alert *types.AlertData) error {
	if !dtn.enabled {
		// 降级为控制台输出
		console := NewConsoleNotifier()
		return console.SendAlert(alert)
	}

	// TODO: 实现真实的钉钉发送逻辑
	fmt.Printf("📤 [钉钉通知] %s 涨幅 %.2f%% (未实现钉钉发送)\n",
		alert.Symbol, alert.ChangePercent)

	return nil
}

func (dtn *DingTalkNotifier) SendBatchAlerts(alerts []*types.AlertData) error {
	if len(alerts) == 0 {
		return nil
	}

	if len(alerts) == 1 {
		return dtn.SendAlert(alerts[0])
	}

	if !dtn.enabled {
		// 降级为控制台输出
		console := NewConsoleNotifier()
		return console.SendBatchAlerts(alerts)
	}

	// TODO: 实现真实的钉钉批量发送逻辑
	fmt.Printf("📤 [钉钉批量通知] %d个币种预警 (未实现钉钉发送)\n", len(alerts))
	return nil
}
