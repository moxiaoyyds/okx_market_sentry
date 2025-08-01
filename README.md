# OKX Market Sentry

加密货币价格监控机器人，实时监控 OKX 交易所 USDT 本位现货交易对价格，检测短时剧烈波动并发送钉钉预警。

## 功能特性

- 🔍 **实时监控**: 监控 OKX 交易所所有 USDT 本位现货交易对
- 📈 **智能分析**: 检测 5 分钟内涨幅超过 3% 的交易对
- 🚨 **即时预警**: 通过钉钉机器人发送格式化预警消息
- 💾 **数据存储**: 支持 Redis 持久化和内存存储
- 🐳 **容器化**: 完整的 Docker 部署方案

## 快速开始

### 环境要求

- Go 1.21+
- Redis (可选)
- Docker & Docker Compose (可选)

### 本地开发

1. 克隆项目并安装依赖
```bash
git clone <repository-url>
cd okx-market-sentry
go mod download
```

2. 配置钉钉机器人
```bash
# 复制配置文件
cp configs/config.yaml.example configs/config.yaml

# 编辑配置文件，填入钉钉 Webhook URL
nano configs/config.yaml
```

3. 运行项目
```bash
# 直接运行
go run cmd/main.go

# 或使用 Makefile
make run
```

### Docker 部署

1. 设置环境变量
```bash
export DINGTALK_WEBHOOK_URL="https://oapi.dingtalk.com/robot/send?access_token=YOUR_TOKEN"
```

2. 启动服务
```bash
docker-compose up -d
```

3. 查看日志
```bash
docker-compose logs -f okx-sentry
```

## 配置说明

```yaml
log_level: "info"           # 日志级别

redis:
  url: "localhost:6379"     # Redis 连接地址
  password: ""              # Redis 密码
  db: 0                     # Redis 数据库

dingtalk:
  webhook_url: ""           # 钉钉机器人 Webhook URL

alert:
  threshold: 3.0            # 预警阈值百分比

fetch:
  interval: "1m"            # 数据获取间隔
```

## 项目结构

```
okx-market-sentry/
├── cmd/                    # 应用入口
├── internal/              # 私有业务逻辑
│   ├── scheduler/         # 调度器
│   ├── fetcher/          # 数据获取
│   ├── storage/          # 状态管理
│   ├── analyzer/         # 分析引擎
│   └── notifier/         # 通知服务
├── pkg/                  # 公共库
├── configs/              # 配置文件
├── deployments/          # 部署相关
└── scripts/              # 脚本文件
```

## 开发指南

### 常用命令

```bash
# 构建
make build

# 测试
make test

# 代码检查
make lint

# 依赖管理
make deps

# Docker 操作
make docker-build
make docker-run
make docker-stop
```

### 健康检查

应用启动后，可通过以下端点检查服务状态：
```
GET http://localhost:8080/health
```

## 许可证

MIT License