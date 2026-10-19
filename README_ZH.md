# VRC Asset Box

[English](README.md) | 简体中文

VRC Asset Box 是一款主要面向 Windows 平台的桌面应用，用于整理 VRChat 改模过程中积累的素材。你可以为素材记录名称、分类、可选的 BOOTH ID、封面图、适配素体、自由标签、备注和关联文件夹。

## 预览

<table>
  <tr>
    <td>
      <img src="docs/images/home_cards.jpg" width="100%" alt="">
      <p align="center">首页卡片布局</p>
    </td>
    <td>
      <img src="docs/images/home_table.jpg" width="100%" alt="">
      <p align="center">首页表格布局</p>
    </td>
  </tr>
  <tr>
    <td>
      <img src="docs/images/asset_details.jpg" width="100%" alt="">
      <p align="center">素材详情</p>
    </td>
    <td>
      <img src="docs/images/settings.jpg" width="100%" alt="">
      <p align="center">设置页</p>
    </td>
  </tr>
</table>

## 支持的语言

可在设置中自由切换以下语言，默认使用“跟随系统”：

- 英语
- 简体中文
- 繁体中文
- 日语
- 韩语

## 下载

前往 [GitHub Releases](https://github.com/xiaolazhu/vrc_asset_box/releases) 页面下载

运行应用需要 WebView2 运行时

## 从源码构建

### 环境要求

- Go 1.25+
- Node.js 22.12+
- npm
- Wails CLI v2

安装 Wails CLI 并检查构建环境：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

以开发模式启动桌面应用：

```powershell
wails dev
```

构建 Windows 可执行文件：

```powershell
wails build -clean
```

或附加 UPX 压缩

```powershell
wails build -clean -upx
```

## 参与贡献

可以通过 [GitHub Issues](https://github.com/xiaolazhu/vrc_asset_box/issues) 提交问题或功能建议。欢迎提交 Pull Request。

## 许可证

本项目使用 [MIT License](LICENSE) 发布。
