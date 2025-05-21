# # 🎬 M3U8-Web 下载器

一个高效、用户友好的 M3U8 视频流下载工具，支持多线程下载、自定义文件名和 MP4 转换。

![M3U8-Web 下载器](https://img.shields.io/badge/M3U8--Web-下载器-brightgreen)
![Go Version](https://img.shields.io/badge/Go-1.24+-blue)
![Vue Version](https://img.shields.io/badge/Vue-3.4+-green)

## 📝 项目简介

M3U8-Web 是一个基于 Web 界面的视频流下载工具，专为下载 M3U8 格式的流媒体内容而设计。它具有高度的可定制性和用户友好的界面，使得下载流媒体内容变得简单高效。

适用于Nas,需要远程操作的下载任务。

### ✨ 主要特性

- **🚀 多线程下载**：支持自定义线程数量，加速下载过程
- **📊 实时进度显示**：直观展示下载进度和速度
- **🎥 MP4 转换**：自动将下载的 TS 文件转换为 MP4 格式
- **📋 任务管理**：便捷的任务列表管理，包括历史记录
- **✏️ 自定义文件名**：支持为下载文件设置自定义名称
- **🎨 美观的 Web 界面**：基于 Vue 3 和 Ant Design Vue 构建的现代界面
- **🔄 并发任务控制**：支持设置最大同时下载任务数

## 📸 截图展示

### 新建下载任务

![截图](https://md-server.oss-cn-guangzhou.aliyuncs.com/images/1747722378057.png)

### 手机端适配

<img src="https://md-server.oss-cn-guangzhou.aliyuncs.com/images/7bc2d7fad00c55653e8b1b7cb14bc79b.jpg" alt="截图" width="30%" />

## 🔧 技术栈

### 💻 后端

- **🔹 语言**：Go 1.24+
- **🔹 Web 框架**：Gin
- **🔹 视频处理**：FFmpeg (通过 ffmpeg-go)

### 🖥️ 前端

- **🔸 框架**：Vue 3
- **🔸 UI 库**：Ant Design Vue 4
- **🔸 构建工具**：Vite
- **🔸 HTTP 客户端**：Axios

## 🚀 快速开始

### 📥 安装与运行 （推荐使用Docker部署）

克隆仓库

```sh
git clone https://github.com/yourusername/m3u8-web.git
cd m3u8-web
```

**🔨 手动构建**

```sh
# 构建前端
cd web
npm install
npm run build

# 构建并运行后端
cd ..
go build -o m3u8-web
./m3u8-web
```

3. 打开浏览器访问 `http://localhost:9100`

### 🐳 Docker 使用

```sh
# 使用 Docker 运行
docker pull songguangzhi/m3u8-web-download
docker run -p 9100:9100 -v /your/download/path:/downloads songguangzhi/m3u8-web-download
```

## 📖 使用说明

1. 在 Web 界面输入 M3U8 URL
2. 设置输出目录、线程数等参数
3. 点击下载按钮开始任务
4. 在任务列表中管理下载进度和状态

## 📄 许可证

[MIT License](./LICENSE)

## 🤝 贡献

欢迎提交 Issues 和 Pull Requests 来帮助改进项目！

## ⚠️ 免责声明

本工具仅用于学习和研究用途，请勿用于下载非授权内容。用户需自行承担使用过程中的法律责任。
