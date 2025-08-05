# OKX Market Sentry - 量化交易系统

一个功能强大的加密货币量化交易系统，集成价格监控预警与唐奇安通道交易策略。支持 OKX 交易所实时数据监控、技术指标分析、自动交易信号检测，并提供多平台通知服务。

## ✨ 功能特性

### 📊 价格监控系统 (Legacy)
- 🔍 **全面监控**: 监控 OKX 交易所所有 USDT 本位现货交易对 (200+ 交易对)
- 📊 **智能分析**: 使用可配置监控周期检测价格异常波动 (默认5分钟)
- 🚨 **多平台通知**: 支持钉钉、PushPlus 微信推送和控制台输出
- 📈 **批量预警**: 多个币种同时异动时合并为单个通知，避免消息轰炸

### 📈 唐奇安通道量化策略 (New)
- 🎯 **专业策略**: 基于唐奇安通道 + ATR + 成交量的量化交易策略
- 📡 **实时数据**: WebSocket 实时接收 OKX 市场数据，5分钟K线分析
- 🧮 **技术指标**: 集成唐奇安通道、ATR线性回归分析、成交量突破检测
- 🎚️ **信号检测**: 多重条件验证，包括盘整检测、趋势确认、突破验证
- 📊 **性能监控**: 实时统计信号质量、数据库性能、WebSocket连接状态
- 🗄️ **数据持久化**: MySQL存储K线数据和交易信号，支持历史回测

### 🛠️ 系统特性
- 💾 **双重存储**: 内存主存储 + Redis 异步备份，确保数据安全
- 🌐 **网络优化**: 支持 HTTP 代理，适应各种网络环境
- 🐳 **容器化**: 完整的 Docker 部署方案，包含 MySQL + Redis + 管理工具
- ⚙️ **高度可配置**: 策略参数、监控周期、通知方式均可自定义
- 🔧 **开发友好**: 模块化架构，完整的开发工具链和调试配置

## 🚀 快速开始

### 环境要求

- Go 1.23+
- MySQL 8.0+ (唐奇安通道策略必需)
- Redis (推荐用于生产环境)
- Docker & Docker Compose (推荐)

### 方式一：本地开发

1. **克隆项目并安装依赖**
```bash
git clone <repository-url>
cd okx-market-sentry
go mod tidy
```

2. **配置应用**
```bash
# 复制并编辑本地配置文件
cp configs/config.local.yaml.example configs/config.local.yaml
vim configs/config.local.yaml

# 配置数据库连接 (唐奇安策略需要)
# 配置通知服务 (钉钉/PushPlus)
# 配置策略参数
```

3. **运行项目**
```bash
# 直接运行
go run cmd/main.go

# 或使用 Makefile
make run
```

### 方式二：Docker 部署 (推荐)

1. **配置应用**
```bash
# 复制并编辑本地配置文件
cp configs/config.local.yaml.example configs/config.local.yaml
vim configs/config.local.yaml
```

2. **启动完整服务栈**
```bash
# 启动核心服务: OKX Sentry + MySQL + Redis
docker compose up -d

# 查看运行状态
docker compose ps
```

3. **查看日志和管理**
```bash
# 查看实时日志
docker compose logs -f okx-sentry

# 查看服务健康状态
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"

# 访问数据库 (命令行)
docker compose exec mysql mysql -u root -pokx123456 -D okx_strategy
docker compose exec redis redis-cli
```

## ⚙️ 配置详解

### 基础配置 (config.yaml)

```yaml
# 日志配置
log:
  level: info                    # 日志级别 (debug, info, warn, error)
  file_path: log                # 日志文件存放目录
  max_size: 200                 # 日志文件大小限制 (MB)
  max_age: 30                   # 日志文件保留天数
  max_backups: 7                # 日志文件备份数量
  compress: false               # 是否压缩日志文件

# Redis配置
redis:
  url: localhost:6379        # Redis 连接地址
  password:                  # Redis 密码 (可选)
  db: 0                      # Redis 数据库编号

# 通知服务配置
dingtalk:
  webhook_url:               # 钉钉机器人 Webhook URL
  secret:                    # 钉钉机器人加签密钥 (SEC开头)

pushplus:
  user_token:                # PushPlus 用户令牌
  to:                        # 好友令牌 (可选，多人用逗号分隔)

# 价格监控配置 (Legacy)
alert:
  threshold: 3.0             # 预警阈值百分比
  monitor_period: 5m         # 监控周期 (1m, 3m, 5m, 10m, 1h 等)

fetch:
  interval: 1m               # 数据获取间隔

# 唐奇安通道策略配置
strategy:
  donchian:
    enabled: true                      # 是否启用唐奇安通道策略
    symbols: ["BTC-USDT", "ETH-USDT", "SOL-USDT"]  # 监控的交易对列表
    interval: "5m"                     # K线周期 (1m, 3m, 5m, 15m, 30m, 1H, 2H, 4H)
    donchian_length: 15                # 唐奇安通道长度 (开发环境建议15)
    donchian_offset: 1                 # 唐奇安通道偏移
    atr_length: 7                      # ATR指标长度 (开发环境建议7)
    consolidation_bars: 15             # 盘整检测K线数 (开发环境建议15)
    volume_multiplier: 1.5             # 成交量倍数 (开发环境建议1.5)
    min_signal_strength: 0.3           # 最小信号强度 (开发环境建议0.3)

# 数据库配置
database:
  mysql:
    host: localhost                    # MySQL服务器地址
    port: 3306                         # MySQL端口
    username: root                     # 数据库用户名
    password: okx123456                # 数据库密码
    database: okx_strategy             # 数据库名称
    max_idle_conns: 10                 # 最大空闲连接数
    max_open_conns: 100                # 最大打开连接数

# 网络配置
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
📊 OKX批量价格预警 - 4个币种

预警统计:
📈 上涨币种: 2个
📉 下跌币种: 2个  
🕐 预警时间: 2025-01-23 17:21:35

详细列表:

📈 上涨币种 (按涨幅排序):
- 📈 [ETH-USDT](https://www.bybits.io/trade/usdt/ETHUSDT): $2841.50 (+4.12%)
- 📈 [BTC-USDT](https://www.bybits.io/trade/usdt/BTCUSDT): $95432.10 (+3.35%)

📉 下跌币种 (按跌幅排序):
- 📉 [SOL-USDT](https://www.bybits.io/trade/usdt/SOLUSDT): $198.20 (-3.67%)
- 📉 [ADA-USDT](https://www.bybits.io/trade/usdt/ADAUSDT): $0.8241 (-3.24%)

⚠️ 多个交易对同时出现显著波动，请密切关注市场动向！
```

## 🏗️ 项目架构

```
okx-market-sentry/
├── cmd/                     # 应用程序入口
│   ├── main.go             # 主入口点 - 配置加载和应用启动
│   └── app.go              # 应用管理 - 系统启动和生命周期管理
├── internal/                # 私有应用代码
│   ├── analyzer/           # 价格分析引擎 (Legacy系统)
│   ├── fetcher/            # OKX API数据获取 (Legacy系统)
│   ├── notifier/           # 多平台通知服务
│   ├── scheduler/          # 任务调度器 (Legacy系统)
│   ├── storage/            # 存储管理 (Legacy系统)
│   └── strategy/           # 量化交易策略模块 (New)
│       ├── websocket/      # WebSocket实时数据获取
│       ├── indicators/     # 技术指标计算 (唐奇安通道、ATR)
│       ├── signals/        # 交易信号检测
│       ├── database/       # MySQL数据持久化
│       ├── engine/         # 策略引擎核心
│       └── monitor/        # 性能监控
├── pkg/                    # 公共库代码
│   ├── config/             # 配置管理 - 多层级配置系统
│   ├── logger/             # 日志服务 - zap结构化日志
│   └── types/              # 数据类型定义
│       ├── config.go       # 配置结构体
│       ├── market.go       # 市场数据类型
│       ├── strategy.go     # 策略配置类型
│       ├── indicators.go   # 技术指标类型
│       └── types.go        # 核心类型定义
├── configs/                # 配置文件目录
│   ├── config.yaml         # 默认配置模板
│   ├── config.local.yaml   # 本地开发配置 (gitignore)
│   └── config.local.yaml.example  # 本地配置示例
├── docs/                   # 文档目录
│   ├── donchian_channel.md # 唐奇安通道策略文档
│   ├── dingtalk.md         # 钉钉机器人配置文档
│   └── pushplus.md         # PushPlus配置文档
├── log/                    # 日志文件目录
├── init.sql                # MySQL初始化脚本
├── docker-compose.yml      # Docker编排 (MySQL+Redis+管理工具)
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
make docker-run      # 启动核心服务
make docker-stop     # 停止服务
make logs           # 查看日志

# 服务健康检查
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"
```

### 开发环境设置

1. **配置本地开发环境**
```bash
# 复制配置模板
cp configs/config.local.yaml.example configs/config.local.yaml

# 启动数据库服务 (使用Docker)
docker compose up -d mysql redis

# 验证服务状态
docker compose ps
```

2. **开发环境快速测试配置**
```bash
# 编辑配置文件，使用敏感参数便于触发
vim configs/config.local.yaml

# 关键参数调整:
# consolidation_bars: 15      # 降低盘整要求
# volume_multiplier: 1.5       # 降低成交量要求  
# min_signal_strength: 0.3     # 降低信号强度门槛
# interval: "5m"               # 使用5分钟K线
```

3. **数据库初始化验证**
```bash
# 检查数据库表结构
mysql -h localhost -u root -pokx123456 -D okx_strategy -e "SHOW TABLES;"

# 查看初始化状态
mysql -h localhost -u root -pokx123456 -D okx_strategy -e "DESCRIBE klines;"
```

### 故障排除

#### 网络连接问题
```bash
# 检查OKX API连通性
curl -x http://127.0.0.1:7890 https://www.okx.com/api/v5/market/tickers?instType=SPOT

# 测试WebSocket连接 (唐奇安策略)
wscat -c wss://ws.okx.com:8443/ws/v5/public

# 验证DNS解析
nslookup www.okx.com
```

#### 数据库连接问题
```bash
# 检查MySQL连接
mysql -h localhost -u root -pokx123456 -D okx_strategy -e "SHOW TABLES;"

# 查看K线数据
mysql -h localhost -u root -pokx123456 -D okx_strategy -e "SELECT COUNT(*) FROM klines;"

# 查看交易信号
mysql -h localhost -u root -pokx123456 -D okx_strategy -e "SELECT * FROM trading_signals ORDER BY created_at DESC LIMIT 10;"
```

#### Redis连接问题
```bash
# 检查Redis服务状态
redis-cli ping

# 查看Legacy系统数据
redis-cli
> KEYS okx:price:*
> ZRANGE okx:price:BTC-USDT 0 -1 WITHSCORES
```

#### Docker环境故障排除
```bash
# 检查容器健康状态
docker compose ps

# 查看容器详细状态
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"

# 重建镜像 (代码更新后)
docker compose down && docker compose build --no-cache && docker compose up -d

# 查看MySQL慢查询日志
docker compose exec mysql tail -f /var/log/mysql/slow.log
```

#### 通知服务测试
```bash
# 测试钉钉机器人
curl -X POST "https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"msgtype":"text","text":{"content":"测试消息"}}'

# 测试PushPlus服务
curl -X POST "http://www.pushplus.plus/send" \
  -H "Content-Type: application/json" \
  -d '{"token":"YOUR_TOKEN","title":"测试","content":"测试消息"}'
```

## 📊 系统监控

### 性能指标
```bash
# 查看系统运行状态 (实时日志)
tail -f log/$(date +%Y-%m-%d).log | grep -E "(INFO|WARN|ERROR)"

# 监控数据库性能
docker compose exec mysql mysqladmin -u root -pokx123456 status

# 监控Redis内存使用
docker compose exec redis redis-cli info memory

# 查看容器资源使用
docker stats okx-sentry okx-mysql okx-redis
```

### 开发环境测试配置
```yaml
# configs/config.local.yaml - 开发环境快速触发配置
strategy:
  donchian:
    enabled: true
    symbols: ["BTC-USDT", "ETH-USDT", "SOL-USDT", "DOGE-USDT", "SHIB-USDT"]
    interval: "5m"                     # 5分钟K线
    donchian_length: 15                # 缩短通道长度
    atr_length: 7                      # 缩短ATR周期
    consolidation_bars: 15             # 降低盘整要求
    volume_multiplier: 1.5             # 降低成交量要求
    min_signal_strength: 0.3           # 降低信号强度门槛
```

## 🔐 安全注意事项

- **配置文件**: `config.local.yaml` 包含敏感信息，已在 `.gitignore` 中排除
- **数据库安全**: 生产环境修改默认密码，启用SSL连接
- **网络代理**: 如使用代理，确保代理服务器的安全性
- **API限制**: 遵守OKX API使用规范，避免频繁请求导致限流
- **端口管理**: 生产环境关闭不必要的端口暴露 (如8080、8001)
- **日志安全**: 避免在日志中记录敏感配置信息

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 🎯 策略说明

### 唐奇安通道策略原理
唐奇安通道策略基于以下技术指标的组合判断:

1. **唐奇安通道**: 计算N周期内的最高价和最低价，形成价格通道
2. **ATR趋势分析**: 使用线性回归分析ATR的趋势方向，确认市场波动性
3. **盘整检测**: 检测价格在一定周期内的盘整状态
4. **成交量确认**: 突破时必须伴随成交量放大
5. **信号强度评估**: 综合多个因子计算信号可信度

### 入场条件 (多头)
- 市场盘整至少N根K线 (consolidation_bars)
- ATR呈下降趋势 (线性回归斜率为负或处于25%分位数以下)
- 多头K线突破唐奇安通道上轨
- 成交量大于前一根K线的指定倍数 (volume_multiplier)
- 综合信号强度超过最小阈值 (min_signal_strength)

## 📞 技术支持

如遇问题，请通过以下方式寻求帮助：
- 提交 [GitHub Issue](https://github.com/your-repo/okx-market-sentry/issues)
- 查看项目文档和代码注释
- 参考 `CLAUDE.md` 中的开发指导
- 查看 `docs/donchian_channel.md` 中的策略详细说明