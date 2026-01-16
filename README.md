# 🎓 Mosoteach Pro - 自动化助手

[![Go 版本](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![许可证](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![平台](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey)](https://github.com/aobo-y/Mosoteach/releases)

一个为云班课 (Mosoteach) 设计的高性能、AI 驱动的自动化工具，拥有专业仪表盘界面，提供直观的控制和实时监控。

![Dashboard 预览](./.github/img/t1.png)

## ✨ 核心特性

- 🤖 **自动答题**: 自动登录并使用 AI 完成测验。
- 🖥️ **专业仪表盘 UI**: 采用现代化的双栏布局（侧边栏 + 主内容区），操作高效。
- 📜 **独立日志终端**: 配备一个全屏、并支持主题切换的终端页面，用于实时监控。
- 🎯 **多题库选择**: 支持一次选择并运行多个指定的题库。
- 🧠 **AI 集成**: 支持所有兼容 OpenAI API 格式的服务（如 DeepSeek, Gemini, Claude, Ollama 等）。
- 🎨 **主题切换**: 流畅、带动画的亮色/暗色主题（"Fuwari Pro" 风格）。
- 💻 **跨平台**: 可编译为单个可执行文件，在 Windows, Linux 和 macOS 上原生运行。

## 🚀 快速上手

### 1. 下载

从 [Releases](https://github.com/aobo-y/Mosoteach/releases) 页面下载您操作系统对应的最新版本。

### 2. 运行

```bash
# Windows 用户
./mosoteach.exe

# Linux / macOS 用户
chmod +x mosoteach
./mosoteach
```

Web 操作界面将在 `http://localhost:11451` 启动。

### 3. 配置

1.  **账号**: 访问 Web 界面，进入 **系统设置** 页面，填写您的云班课手机号和密码。
2.  **AI 模型**: 进入 **模型管理** 页面，添加并启用至少一个 AI 模型，并填入您的 API Key。

### 4. 使用

1.  **登录**: 在 **自动答题** 页面，点击 **🔑 登录刷新** 按钮以获取最新的会话 Cookie。
2.  **获取题库**: 点击 **刷新** 按钮加载可用的题库列表。
3.  **选择与运行**: 选择一个或多个想作答的题库（留空则默认全部作答），然后点击 **▶ 开始答题**。

## 🔧 从源码编译

### 环境要求

- **Go**: 1.22 或更高版本。
- **Chrome/Chromium**: 已在您的系统中安装。

### 编译步骤

```bash
# 1. 克隆仓库
git clone https://github.com/aobo-y/Mosoteach.git
cd Mosoteach

# 2. 下载依赖
go mod tidy

# 3. 编译
go build -o mosoteach.exe ./cmd/main.go
```

## ⚙️ 配置详情

应用的所有设置都保存在 `user_data.json` 文件中，大部分选项都可以通过 Web 界面进行配置。

### AI 模型支持

任何兼容 OpenAI `v1/chat/completions` API 格式的服务都受支持，例如：

| 服务商            | Base URL 示例                    |
| ----------------- | -------------------------------- |
| **DeepSeek**      | `https://api.deepseek.com`       |
| **Google Gemini** | (需要使用兼容 OpenAI 格式的代理) |
| **OpenAI**        | `https://api.openai.com/v1`      |
| **月之暗面 Kimi** | `https://api.moonshot.cn/v1`     |
| **Ollama (本地)** | `http://localhost:11434/v1`      |

## 🛠️ 技术栈

| 模块             | 技术                                  |
| ---------------- | ------------------------------------- |
| **后端**         | Go (net/http, SSE)                    |
| **浏览器自动化** | `chromedp` (Chrome DevTools Protocol) |
| **前端**         | Vue 3 (通过 CDN)                      |
| **样式**         | "Fuwari Pro" 风格，使用 CSS 变量      |
| **HTML 解析**    | `goquery`                             |

## 📄 许可证

本项目基于 [MIT License](LICENSE) 开源。

---

> **免责声明**: 本工具仅供学习和研究使用，请遵守相关平台的使用条款。
