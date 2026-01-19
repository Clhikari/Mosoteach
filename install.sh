#!/bin/bash
set -e

# Mosoteach 一键安装脚本
# 用法: curl -sSL https://raw.githubusercontent.com/Clhikari/Mosoteach/main/install.sh | bash

REPO="Clhikari/Mosoteach"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="mosoteach"

echo "==================================="
echo "  Mosoteach 安装脚本"
echo "==================================="

# 检测操作系统
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    linux)
        OS="linux"
        ;;
    darwin)
        OS="darwin"
        ;;
    *)
        echo "错误: 不支持的操作系统 $OS"
        exit 1
        ;;
esac

# 检测架构
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "错误: 不支持的架构 $ARCH"
        exit 1
        ;;
esac

echo "系统: $OS"
echo "架构: $ARCH"

# 获取最新版本
echo "正在获取最新版本..."
LATEST_VERSION=$(curl -sI "https://github.com/$REPO/releases/latest" | grep -i "location:" | sed 's/.*tag\///' | tr -d '\r\n')

if [ -z "$LATEST_VERSION" ]; then
    echo "警告: 无法获取版本号，使用 main 分支"
    DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/mosoteach_${OS}_${ARCH}"
else
    echo "最新版本: $LATEST_VERSION"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/${LATEST_VERSION}/mosoteach_${OS}_${ARCH}"
fi

# 下载
TEMP_FILE=$(mktemp)
echo "正在下载: $DOWNLOAD_URL"
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_FILE"; then
    echo "错误: 下载失败"
    rm -f "$TEMP_FILE"
    exit 1
fi

# 安装
echo "正在安装到 $INSTALL_DIR/$BINARY_NAME ..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
else
    echo "需要 sudo 权限..."
    sudo mv "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
    sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
fi

# 验证
if command -v "$BINARY_NAME" &> /dev/null; then
    echo ""
    echo "==================================="
    echo "  安装成功!"
    echo "==================================="
    echo ""
    echo "运行方式:"
    echo "  $BINARY_NAME"
    echo ""
    echo "然后访问 http://localhost:11451"
else
    echo ""
    echo "安装完成，但 $BINARY_NAME 不在 PATH 中"
    echo "请手动运行: $INSTALL_DIR/$BINARY_NAME"
fi
