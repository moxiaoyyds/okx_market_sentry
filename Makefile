# 构建项目
build:
	go build -o bin/okx-sentry cmd/main.go

# 运行项目
run:
	go run cmd/main.go

# 测试
test:
	go test -v ./...

# 代码检查
lint:
	golangci-lint run

# 依赖管理
deps:
	go mod tidy
	go mod download

# 清理
clean:
	rm -rf bin/
	rm -rf logs/

# Docker 构建
docker-build:
	docker build -t okx-market-sentry .

# Docker 运行
docker-run:
	docker-compose up -d

# Docker 停止
docker-stop:
	docker-compose down

# 查看日志
logs:
	docker-compose logs -f okx-sentry

.PHONY: build run test lint deps clean docker-build docker-run docker-stop logs