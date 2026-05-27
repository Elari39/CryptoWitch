# CryptoWitch

CryptoWitch 是一个 Wails v3 + Vue3 + TypeScript 的 Windows 桌面应用示例，用于把构建期 Markdown 文档加密后嵌入单个 exe。运行时只有输入正确密码后，才会解密目录和文档内容。

## 主要能力

- Markdown 文档构建期加密，运行时只在内存中解锁。
- 使用 Argon2id 派生密钥，AES-256-GCM 加密 vault。
- Wails 窗口启用 `ContentProtectionEnabled`，尽量阻止系统截图和录屏捕获。
- 前端禁用选中、复制、右键、拖拽和常见快捷键。

## 构建流程

1. 修改 `config.yaml` 中的 `vault.password`。
2. 把 Markdown 放入 `content/plain/**/*.md`。
3. 生成密文 vault：

```bash
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

截图/录屏保护依赖操作系统和 WebView2 能力，不是 DRM。它不能阻止外部拍照、管理员权限工具、驱动级工具或逆向分析。正式发布前不要保留示例密码和示例文档。
