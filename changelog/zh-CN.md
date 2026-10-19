## [0.71.2] - 2026-10-19

### Fixed

- 原生窗口在 Windows 和 macOS 上可以正常打开了，此前它会为绘制代码和差异视图去查找
  fontconfig 而立即关闭。
- 原生窗口终端中的 emoji 在 Windows 上以彩色绘制，此前只显示为轮廓。
- 向 macOS 安装提供的更新命令现在会在升级前先 tap `lichdotdev/tap`。从旧的 `omartelo/tap` 安装的版本会失败并提示 "Cask 'lichdotdev/tap/lich' is unavailable"，因为 Homebrew 不会自动跟随迁移的 tap；要从较旧的 lich 升级，请先执行一次 `brew tap lichdotdev/tap`。
- 在原生窗口中，“展开长粘贴”现在会展开 Claude Code 和 Kiro CLI 的粘贴内容：剪贴板、一批评审评论、拉取请求或 issue 打开会话时使用的提示词，以及拖放或附加的文件路径。
