# AGENTS.md

## 项目简介

- Windows 桌面端 VRChat 素材管理工具
- 技术栈：Go 1.25、Wails 2、Vue 3、TypeScript、Naive UI、SQLite

## 项目结构

- `main.go`、`app*.go`：Wails 应用入口和桌面 API
- `internal/`：后端业务功能模块
- `frontend/src/app/`：应用外壳和导航
- `frontend/src/features/`：业务页面和功能
- `frontend/src/api/`：桌面 API 封装与前端数据类型
- `frontend/src/i18n/`：多语言资源
- `frontend/src/shared/`：共享组件与工具
- `frontend/wailsjs/`：Wails 生成文件

## 工作约定

- 开始前先阅读相关代码、测试和文档，并检查 `git status`、暂存区与未暂存差异
- 保留用户已有的修改，不覆盖或回退无关变更
- 每次修改文件前，先说明修改范围并等待用户确认。用户已明确确认当前方案时，不重复询问
- 未经用户要求，不提交、不推送、不合并
- 修改范围应紧贴用户需求；不要顺手重构无关模块
- 代码保持清晰易读，函数和逻辑之间适当分段，避免过度紧凑
- 说明性代码注释以中文为主，只在需要解释原因或复杂逻辑时添加
- 不直接修改 `frontend/wailsjs/` 中的生成文件；需要更新时使用项目生成流程

## 架构约定

- 前端通过 `frontend/src/api/client.ts` 调用 Wails 桌面 API，不得直接访问 SQLite 或绕过桌面 API
- 前后端共享数据契约时，同步检查 `frontend/src/api/types.ts`、后端 DTO 和相关 API
- 后端业务逻辑放在合适的 `internal/` 模块中，`app*.go` 负责桌面 API 边界和模块协调
- 长时间操作使用任务 API，并正确处理取消、进度、错误和资源释放
- 错误使用项目已有的结构化错误格式，不向用户暴露不必要的底层细节

## 数据与文件安全

- 数据库、封面和备份位于 `%APPDATA%\\VRCAssetBox\\Library`
- WebView2 运行数据位于 `%APPDATA%\\VRCAssetBox`
- 数据库结构变更通过 `internal/database/migrations/` 管理
- 关联文件夹只保存路径。不得移动或删除关联文件夹中的实际素材
- 备份不包含关联文件夹中的实际素材文件
- 操作用户数据库或备份前，先确认目标和影响范围；不得擅自删除或重写用户数据

## 前端与多语言

- 英文是界面文案的源文本
- 新增或修改界面文案时，同步维护简体中文、繁体中文、日文和韩文
- 沿用现有 Vue、Naive UI、CSS 和测试模式，不额外引入依赖，除非需求确实需要
- 保持响应式布局和现有视觉规范；交互状态应覆盖加载、空结果、错误和禁用状态

## 测试与验证

根据改动范围运行相关检查：

- Go 测试：`go test ./...`
- Go 静态检查：`go vet ./...`
- 前端测试：`npm --prefix frontend run test -- --run`
- 前端构建：`npm --prefix frontend run build`
- Windows 打包：`wails build -clean`

- 先运行最相关的定向测试，再按影响范围补充完整检查
- 测试失败时调查原因，不得为了让测试通过而删除有效断言或扩大无关修改
- 完成前检查 `git diff`、`git diff --cached` 和 `git diff --check`
- 只报告实际运行且通过的验证；未运行的检查要明确说明

## 完成说明

完成后简要说明：

- 修改了什么，以及对应文件
- 运行了哪些测试或构建，结果如何
- 尚未验证的部分或已知限制
- 不要声称数据库、打包或界面行为已验证，除非确实执行了对应检查
