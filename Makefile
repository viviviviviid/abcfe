# Makefile for ABCFe Node

# 바이너리 이름 설정
BINARY_NAME=abcfed
DASHBOARD_NAME=abcfe-dashboard
VERSION=1.0.0
BUILD_TIME=$(shell date +%FT%T%z)
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# 기본 타겟
.PHONY: all
all: build build-dashboard

# 빌드
.PHONY: build
build:
	@echo "Building ${BINARY_NAME}..."
	go build ${LDFLAGS} -o ./${BINARY_NAME} cmd/node/main.go
	@echo "Build complete: ./${BINARY_NAME}"

# 대시보드 빌드
.PHONY: build-dashboard
build-dashboard:
	@echo "Building ${DASHBOARD_NAME}..."
	go build ${LDFLAGS} -o ./${DASHBOARD_NAME} cmd/dashboard/main.go
	@echo "Build complete: ./${DASHBOARD_NAME}"

# 개발용 빌드 (디버그 정보 포함)
.PHONY: build-dev
build-dev:
	@echo "Building ${BINARY_NAME} (development)..."
	go build -race -gcflags="all=-N -l" -o bin/${BINARY_NAME}-dev cmd/node/main.go
	@echo "Build complete: bin/${BINARY_NAME}-dev"

# 릴리즈 빌드 (최적화)
.PHONY: build-release
build-release:
	@echo "Building ${BINARY_NAME} (release)..."
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o bin/${BINARY_NAME}-linux cmd/node/main.go
	CGO_ENABLED=0 GOOS=darwin go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o bin/${BINARY_NAME}-darwin cmd/node/main.go
	CGO_ENABLED=0 GOOS=windows go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o bin/${BINARY_NAME}-windows.exe cmd/node/main.go
	@echo "Release builds complete"

# 클린
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f ${BINARY_NAME} ${DASHBOARD_NAME}
	@echo "Clean complete"

# 테스트
.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

# 테스트 커버리지
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# 실행
.PHONY: run
run:
	@echo "Running ${BINARY_NAME}..."
	./bin/${BINARY_NAME}

# 설치
.PHONY: install
install:
	@echo "Installing ${BINARY_NAME}..."
	go install cmd/node/main.go

# 도움말
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build the node application"
	@echo "  build-dashboard- Build the dashboard TUI"
	@echo "  build-dev      - Build with debug information"
	@echo "  build-release  - Build for multiple platforms"
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage"
	@echo "  run            - Run the application"
	@echo "  install        - Install the application"
	@echo "  help           - Show this help" 