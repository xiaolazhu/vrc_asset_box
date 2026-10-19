# VRC Asset Box 前端

技术栈：Vue 3、TypeScript、Vite、Naive UI 和 vue-i18n

## 开发与验证

在项目根目录安装前端依赖，并启动 Wails 开发模式：

```powershell
npm --prefix frontend ci
wails dev
```

仅查看前端页面时可运行 `npm --prefix frontend run dev`；浏览器预览没有 Wails 桌面桥，无法执行数据库、文件选择及其他桌面操作

```powershell
npm --prefix frontend run test -- --run
npm --prefix frontend run build
```

生产构建输出到 `frontend/dist/`，由 Go 嵌入桌面程序

## 源码结构

- `src/app/`：应用外壳、导航、偏好保存及全局状态
- `src/features/`：业务页面和功能
- `src/api/`：桌面桥封装、共享类型及任务轮询
- `src/i18n/`：语言解析、提示翻译和语言词典
- `src/shared/`：跨页面共享组件
- `src/style.css`：主题变量和全局样式
- `wailsjs/`：Wails 生成的 Go 绑定及运行库，业务代码通过 `src/api/` 调用
