# CryptoWitch

CryptoWitch 是一个 Wails v3 + Vue3 + TypeScript 的 Windows 桌面应用示例，用于把构建期 Markdown 和 PDF 文档加密后嵌入单个 exe。运行时只有输入正确密码后，才会解密文档目录；正文内容会在点击文档时按需解密和渲染。

## 主要能力

- Markdown / PDF 文档构建期加密，运行时目录和单篇正文按需解锁。
- 使用 Argon2id 派生密钥，AES-256-GCM 加密 vault。
- 每篇文档独立加密，避免解锁时一次性把大 Markdown/PDF 全部加载进内存。
- v3 vault 会把 PDF 按 1 MiB 明文块独立加密，运行时按块解密和传输，前端加载完成后再交给内置 PDF 查看器打开。
- Markdown 渲染结果会在解锁期间缓存，重复打开同一文档不再重复高亮和转换。
- Wails 窗口启用 `ContentProtectionEnabled`，尽量阻止系统截图和录屏捕获。
- 前端提供目录搜索、文档类型和大小提示，并禁用选中、复制、右键、拖拽和常见快捷键。

## 构建流程

1. 设置构建期密码环境变量 `CRYPTOWITCH_VAULT_PASSWORD`。
2. 把 Markdown 或 PDF 放入 `content/plain/**/*.md`、`content/plain/**/*.pdf`。
3. 生成 v3 密文 vault。PDF 会按 1 MiB 分块独立加密；每次更新 `content/plain` 后都需要重新执行：

```bash
export CRYPTOWITCH_VAULT_PASSWORD="123456"
go run ./cmd/packdocs
```

4. 安装前端依赖并构建：

```bash
cd frontend
pnpm install
pnpm run type-check
pnpm run build
```

5. 构建 Windows exe：

```bash
go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96 task windows:build ARCH=amd64
```

产物默认位于 `bin/CryptoWitch.exe`。

## 重要限制

截图/录屏保护依赖操作系统和 WebView2 能力，不是 DRM。它不能阻止外部拍照、管理员权限工具、驱动级工具或逆向分析。正式发布前不要保留示例文档，也不要把真实密码写入仓库、脚本或命令历史。
