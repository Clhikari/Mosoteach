# Mosoteach Pro - 自动化助手

[![Go 版本](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![许可证](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)
[![平台](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey)](https://github.com/Clhikari/Mosoteach/releases)

一个为云班课 (Mosoteach) 设计的高性能、AI 驱动的自动化工具，拥有专业仪表盘界面，提供直观的控制和实时监控。

![Dashboard 预览](./.github/img/t1.png)

## 核心特性

- **自动答题**: 自动登录并使用 AI 完成测验
- **专业仪表盘 UI**: 现代化的双栏布局，操作高效
- **实时日志**: 独立终端页面，支持主题切换
- **多题库选择**: 支持一次选择并运行多个题库
- **延迟提交**: 支持设置答完后等待时间，适用于有最低时长要求的考试
- **访问控制**: 支持设置 Web 访问密码，可安全部署到服务器
- **AI 集成**: 支持所有 OpenAI 兼容 API（DeepSeek, Gemini, Claude, Ollama 等）
- **跨平台**: 单文件可执行，支持 Windows、Linux、macOS

## 快速上手

### 1. 下载

**Linux / macOS 一键安装：**

# 下载安装脚本
curl -sSL https://raw.githubusercontent.com/Clhikari/Mosoteach/main/install.sh -o install.sh

# (可选但推荐) 审查脚本内容以确保安全
# cat install.sh

# 执行脚本
bash install.sh

**手动下载：**

从 [Releases](https://github.com/Clhikari/Mosoteach/releases) 页面下载对应版本：

| 平台 | 文件 |
|------|------|
| Windows 64位 | `mosoteach_windows_amd64.exe` |
| Linux 64位 | `mosoteach_linux_amd64` |
| macOS Intel | `mosoteach_darwin_amd64` |
| macOS Apple Silicon | `mosoteach_darwin_arm64` |

### 2. 运行

```bash
# Windows
./mosoteach_windows_amd64.exe

# Linux
chmod +x mosoteach_linux_amd64
./mosoteach_linux_amd64

# macOS Intel
chmod +x mosoteach_darwin_amd64
./mosoteach_darwin_amd64

# macOS Apple Silicon
chmod +x mosoteach_darwin_arm64
./mosoteach_darwin_arm64
```

Web 界面地址：`http://localhost:11451`

### 3. 配置

1. **账号**: 系统设置 → 填写云班课手机号和密码
2. **AI 模型**: 模型管理 → 添加并启用 AI 模型，填入 API Key
3. **访问密码**（可选）: 系统设置 → 访问控制 → 设置 Web 访问密码

### 4. 使用

1. 自动答题页面，点击 **登录刷新** 获取会话
2. 点击 **刷新** 加载题库列表
3. 选择题库（可多选，留空则全部作答），点击 **开始答题**

## 高级功能

### 延迟提交

在 **系统设置 → 答题设置** 中设置提交延迟（秒）。答完题后会倒计时等待，适用于有最低作答时长要求的考试。

### 服务器部署

部署到服务器后，建议设置访问密码：

**系统设置 → 访问控制 → 设置密码**

设置后访问页面会显示登录界面，输入正确密码后方可使用。密码以 SHA256 哈希存储在配置文件中。

## 从源码编译

### 环境要求

- **Go**: 1.22+
- **Chrome/Chromium**: 系统已安装

### 编译

```bash
git clone https://github.com/Clhikari/Mosoteach.git
cd Mosoteach
go mod tidy
go build -o mosoteach ./cmd/main.go
```

### 交叉编译

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o mosoteach_linux_amd64 ./cmd/main.go

# macOS
GOOS=darwin GOARCH=arm64 go build -o mosoteach_darwin_arm64 ./cmd/main.go
```

## 配置文件

配置保存在 `user_data.json`，支持的选项：

| 字段 | 说明 |
|------|------|
| `user_data.user_name` | 云班课手机号 |
| `user_data.password` | 云班课密码 |
| `models` | AI 模型配置列表 |
| `submit_delay` | 提交延迟（秒） |
| `web_password` | Web 访问密码（SHA256 哈希） |
| `debug` | 调试模式 |

### AI 模型支持

兼容 OpenAI `v1/chat/completions` API 格式：

| 服务商 | Base URL |
|--------|----------|
| DeepSeek | `https://api.deepseek.com` |
| OpenAI | `https://api.openai.com/v1` |
| Moonshot | `https://api.moonshot.cn/v1` |
| 通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| Ollama | `http://localhost:11434/v1` |

## 技术栈

| 模块 | 技术 |
|------|------|
| 后端 | Go (net/http, SSE) |
| 浏览器自动化 | chromedp (CDP) |
| 前端 | Vue 3 |
| HTML 解析 | goquery |

## 许可证

[GPL-3.0 License](LICENSE)

---

> **免责声明**: 本工具仅供学习和研究使用，请遵守相关平台的使用条款。
