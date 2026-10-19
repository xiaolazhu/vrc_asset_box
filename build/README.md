# 构建与打包说明

## 构建资源

- `appicon.png`：应用图标源文件
- `windows/icon.ico`：Windows 可执行文件图标
- `windows/info.json`：从 `wails.json` 的 `info` 配置生成 Windows 文件版本信息
- `windows/wails.exe.manifest`：Windows 应用清单，包含 DPI 感知配置
- `windows/installer/project.nsi`、`wails_tools.nsh`：NSIS 安装包脚本；普通 EXE 构建不生成安装包
- `darwin/Info.plist`、`Info.dev.plist`：保留的 macOS 构建配置，当前未完成 macOS 平台验证

两份图标和安装脚本按当前项目配置保留。生成 NSIS 安装包还需要安装 NSIS，并在项目根目录执行 `wails build -nsis`；该安装包流程尚未验收。
