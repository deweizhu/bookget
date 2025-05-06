# 编译配置
GO ?= go
GOFLAGS ?= -trimpath
LDFLAGS ?= -s -w
TARGET ?= bookget
DIST_DIR ?= dist

# 构建目标
.PHONY: build
build:
	@echo "Building $(TARGET) for $(GOOS)-$(GOARCH)"
	@mkdir -p $(DIST_DIR)/$(GOOS)-$(GOARCH)
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/$(GOOS)-$(GOARCH)/$(TARGET)$(SUFFIX) ./cmd/

# 跨平台构建（调用示例）
.PHONY: release
release: linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

linux-amd64:
	@$(MAKE) build GOOS=linux GOARCH=amd64 SUFFIX=-linux

linux-arm64:
	@$(MAKE) build GOOS=linux GOARCH=arm64 SUFFIX=-linux-arm64

darwin-amd64:
	@$(MAKE) build GOOS=darwin GOARCH=amd64 SUFFIX=-macos

darwin-arm64:
	@$(MAKE) build GOOS=darwin GOARCH=arm64 SUFFIX=-macos-arm64

windows-amd64:
	@$(MAKE) build GOOS=windows GOARCH=amd64 SUFFIX=-windows.exe

# 清理
.PHONY: clean
clean:
	@rm -rf $(DIST_DIR)