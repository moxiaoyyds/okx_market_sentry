# OKX Market Sentry

一个智能的加密货币价格监控机器人，实时监控 OKX 交易所 USDT 本位现货交易对价格，检测短时间内的剧烈波动并发送多平台预警通知。

## ✨ 功能特性

- 🔍 **全面监控**: 监控 OKX 交易所所有 USDT 本位现货交易对 (200+ 交易对)
- 📊 **智能分析**: 使用可配置监控周期检测价格异常波动 (默认5分钟)
- 🚨 **多平台通知**: 支持钉钉、PushPlus 微信推送和控制台输出
- 📈 **批量预警**: 多个币种同时异动时合并为单个通知，避免消息轰炸
- 💾 **双重存储**: 内存主存储 + Redis 异步备份，确保数据安全
- 🌐 **网络优化**: 支持 HTTP 代理，适应各种网络环境
- 🐳 **容器化**: 完整的 Docker 部署方案
- ⚙️ **高度可配置**: 监控周期、预警阈值、通知方式均可自定义

## 🚀 快速开始

### 环境要求

- Go 1.21+
- Redis (可选，推荐用于生产环境)
- Docker & Docker Compose (可选)

### 方式一：本地开发

1. **克隆项目并安装依赖**
```bash
git clone <repository-url>
cd okx-market-sentry
go mod download
```

2. **配置应用**
```bash
# 复制配置文件模板
cp configs/config.yaml configs/config.local.yaml

# 编辑配置文件，填入你的通知配置
nano configs/config.local.yaml
```

3. **运行项目**
```bash
# 直接运行
go run cmd/main.go

# 或使用 Makefile
make run
```

### 方式二：Docker 部署

1. **配置应用**
```bash
# 复制配置文件模板
cp configs/config.yaml configs/config.yaml

# 编辑配置文件，填入你的通知配置
nano configs/config.yaml
```

2. **启动服务**
```bash
# 启动 OKX Market Sentry + Redis
docker-compose up -d

# 查看运行状态
docker-compose ps
```

3. **查看日志**
```bash
# 查看实时日志
docker-compose logs -f okx-sentry

# 查看特定服务日志
docker-compose logs -f redis
```

## ⚙️ 配置详解

### 基础配置 (config.yaml)

```yaml
log_level: info

redis:
  url: localhost:6379        # Redis 连接地址
  password:                  # Redis 密码 (可选)
  db: 0                      # Redis 数据库编号

dingtalk:
  webhook_url:               # 钉钉机器人 Webhook URL
  secret:                    # 钉钉机器人加签密钥 (SEC开头)

pushplus:
  user_token:                # PushPlus 用户令牌
  to:                        # 好友令牌 (可选，多人用逗号分隔)

alert:
  threshold: 3.0             # 预警阈值百分比
  monitor_period: 5m         # 监控周期 (1m, 3m, 5m, 10m, 1h 等)

fetch:
  interval: 1m               # 数据获取间隔

network:
  proxy:                     # HTTP代理地址 (如: http://127.0.0.1:7890)
  timeout: 30s               # 网络请求超时时间
```

### 通知配置优先级

系统按以下优先级选择通知方式：
1. **钉钉通知** (最高优先级) - 适用于团队协作
2. **PushPlus 微信推送** - 适用于个人使用
3. **控制台输出** (默认) - 适用于开发调试

### 监控周期配置

支持灵活的时间格式：
- `1m` = 1分钟 (快速响应)
- `3m` = 3分钟 (平衡模式)
- `5m` = 5分钟 (默认推荐)
- `15m` = 15分钟 (减少噪音)
- `1h` = 1小时 (长期趋势)

## 🔔 通知服务配置

### 钉钉机器人

1. **创建钉钉群和机器人**
   - 在钉钉群中添加"自定义机器人"
   - 安全设置选择"加签"方式
   - 复制 Webhook URL 和 Secret

2. **配置钉钉参数**
```yaml
dingtalk:
  webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"
  secret: "SEC***"  # 以 SEC 开头的加签密钥
```

### PushPlus 微信推送

1. **获取 PushPlus 令牌**
   - 访问 [PushPlus官网](http://www.pushplus.plus)
   - 微信扫码登录获取用户令牌

2. **配置推送参数**
```yaml
pushplus:
  user_token: "your_token_here"
  to: "friend_token1,friend_token2"  # 好友令牌 (可选)
```

## 📊 运行状态示例

### 控制台输出
```
✅ 初始化goex v2 OKX客户端
✅ 已配置钉钉通知服务（含加签验证）
✅ Redis连接成功
🚀 数据获取器启动，开始获取OKX V5真实市场数据...
📊 使用goex v2从 500+ 个交易对中筛选出 200+ 个USDT交易对

--- 价格分析任务 [17:21:30] ---
📊 存储状态: 内存中200个交易对, Redis中200个key
开始分析 200 个交易对的价格变化...
✅ 分析完成，触发 3 个预警
--- 分析任务完成 ---
```

### 批量预警通知
当多个币种同时异动时，系统会自动合并为单个通知：

```
📊 OKX批量价格预警 - 3个币种

预警统计:
📈 上涨币种: 2个
📉 下跌币种: 1个  
🕐 预警时间: 2025-01-23 17:21:35

详细列表:
- 📈 BTC-USDT: $95432.10 (+3.35%)
- 📈 ETH-USDT: $2841.50 (+4.12%)  
- 📉 SOL-USDT: $198.20 (-3.67%)

⚠️ 多个交易对同时出现显著波动，请密切关注市场动向！
```

## 🏗️ 项目架构

```
okx-market-sentry/
├── cmd/main.go              # 应用程序入口点
├── internal/                # 私有应用代码
│   ├── analyzer/           # 分析引擎模块 - 价格变化分析和预警触发
│   ├── fetcher/            # 数据获取模块 - OKX API集成
│   ├── notifier/           # 通知服务模块 - 多平台消息推送
│   ├── scheduler/          # 调度器模块 - 任务协调和执行
│   └── storage/            # 存储管理模块 - 内存+Redis双重存储
├── pkg/                    # 公共库代码
│   ├── config/             # 配置管理 - 多层级配置文件系统
│   ├── logger/             # 日志服务 - 结构化日志输出
│   └── types/              # 数据类型定义 - 核心数据结构
├── configs/                # 配置文件目录
│   ├── config.yaml         # 默认配置模板
│   └── config.local.yaml   # 本地开发配置 (不会提交到git)
├── docs/                   # 文档目录
├── docker-compose.yml      # Docker编排文件
├── Dockerfile              # Docker构建文件
└── Makefile               # 构建脚本
```

## 🛠️ 开发指南

### 常用命令

```bash
# 构建项目
make build
go build -o bin/okx-sentry cmd/main.go

# 运行项目
make run
go run cmd/main.go

# 运行测试
make test
go test ./...

# 代码检查
make lint
golangci-lint run

# 依赖管理
make deps
go mod tidy

# Docker 操作
make docker-build    # 构建镜像
make docker-run      # 启动服务
make docker-stop     # 停止服务
make logs           # 查看日志
```

### 开发环境设置

1. **配置本地开发环境**
```bash
# 复制配置模板
cp configs/config.local.yaml.example configs/config.local.yaml

# 使用更低的预警阈值便于测试
sed -i 's/threshold: 3.0/threshold: 0.1/' configs/config.local.yaml

# 配置代理 (如需要)
sed -i 's/proxy:/proxy: "http://127.0.0.1:7890"/' configs/config.local.yaml
```

2. **启动Redis (可选)**
```bash
# 使用Docker启动Redis
docker run -d --name redis -p 6379:6379 redis:7-alpine

# 或使用已有Redis实例
redis-server
```

### 故障排除

#### 网络连接问题
```bash
# 检查OKX API连通性
curl -x http://127.0.0.1:7890 https://www.okx.com/api/v5/market/tickers?instType=SPOT

# 验证DNS解析
nslookup www.okx.com
```

#### Redis连接问题
```bash
# 检查Redis服务状态
redis-cli ping

# 查看Redis数据
redis-cli
> KEYS okx:price:*
> ZRANGE okx:price:BTC-USDT 0 -1 WITHSCORES
```

#### 通知服务测试
```bash
# 测试钉钉机器人 (需要真实的URL和Secret)
curl -X POST "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"msgtype":"text","text":{"content":"测试消息"}}'

# 测试PushPlus服务
curl -X POST "http://www.pushplus.plus/send" \
  -H "Content-Type: application/json" \
  -d '{"token":"YOUR_TOKEN","title":"测试","content":"测试消息"}'
```

## 🔐 安全注意事项

- **配置文件**: `config.local.yaml` 包含敏感信息，已在 `.gitignore` 中排除
- **敏感信息**: 本地开发使用配置文件，生产环境可选择环境变量或配置文件
- **网络代理**: 如使用代理，确保代理服务器的安全性
- **API限制**: 遵守OKX API使用规范，避免频繁请求

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 📞 技术支持

如遇问题，请通过以下方式寻求帮助：
- 提交 [GitHub Issue](https://github.com/your-repo/okx-market-sentry/issues)
- 查看项目文档和代码注释
- 参考 `CLAUDE.md` 中的开发指导