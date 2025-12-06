.PHONY: all build build-dev build-frontend clean help

# 默认目标
all: build

# 构建前端
build-frontend:
	@echo "==> 构建前端..."
	cd web && npm install && npm run build:fast
	@echo "==> 复制前端文件到 internal/api/static/..."
	rm -rf internal/api/static
	mkdir -p internal/api/static
	cp -r data/static/* internal/api/static/

# 构建单体应用（包含嵌入的前端）
build: build-frontend
	@echo "==> 构建单体应用..."
	go build -tags embed -o sss ./cmd/server
	@echo "==> 构建完成: ./sss"
	@echo "==> 提示: 运行 ./sss 启动服务，无需额外的静态文件目录"

# 开发模式构建（不嵌入前端，从 ./data/static 读取）
build-dev:
	@echo "==> 构建开发版本..."
	go build -o sss ./cmd/server
	@echo "==> 构建完成: ./sss"
	@echo "==> 提示: 需要将前端文件放到 ./data/static 目录"

# 运行开发服务器
dev:
	@echo "==> 启动后端开发服务器..."
	go run ./cmd/server &
	@echo "==> 启动前端开发服务器..."
	cd web && npm run dev

# 清理构建文件
clean:
	rm -f sss
	rm -rf internal/api/static

# 帮助信息
help:
	@echo "SSS (Simple S3 Server) 构建命令"
	@echo ""
	@echo "命令:"
	@echo "  make build          构建单体应用（前端嵌入二进制）"
	@echo "  make build-dev      构建开发版本（前端从文件系统读取）"
	@echo "  make build-frontend 仅构建前端"
	@echo "  make clean          清理构建文件"
	@echo "  make help           显示帮助信息"
	@echo ""
	@echo "单体应用使用方法:"
	@echo "  1. make build"
	@echo "  2. ./sss"
	@echo "  3. 访问 http://localhost:8080"
