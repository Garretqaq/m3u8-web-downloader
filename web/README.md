# M3U8下载器 Web界面

基于Vue 3和Ant Design Vue的M3U8下载器前端界面。

## 技术栈

- Vue 3 - 渐进式JavaScript框架
- Vite - 现代前端构建工具
- Ant Design Vue 4 - 基于Vue的UI组件库
- Pinia - Vue的状态管理库
- Vue Router - Vue的官方路由
- Axios - 基于Promise的HTTP客户端

## 功能特性

- 创建和管理M3U8下载任务
- 实时监控下载进度
- 支持多线程下载
- 记住上次下载配置
- 响应式布局

## 项目结构

```
src/
├── main.js           # 应用程序入口
├── App.vue           # 主布局组件
├── router/           # 路由配置
├── stores/           # Pinia状态管理
└── views/            # 页面视图组件
    ├── DownloadManager.vue  # 下载管理页面
    └── Settings.vue         # 设置页面
```

## 开发环境设置

### 安装依赖

```bash
cd m3u8-go/web
npm install
```

### 启动开发服务器

```bash
npm run dev
```

### 构建生产版本

```bash
npm run build
```

## 后端API

前端需要连接到Go后端服务器，默认代理配置指向`http://localhost:8080`。

### API端点

- `GET /api/tasks` - 获取所有下载任务
- `POST /api/download` - 创建新的下载任务
- `DELETE /api/tasks/:id` - 删除指定任务

## 许可证

MIT 