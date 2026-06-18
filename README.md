# CryptoWitch

> 基于 Wails v3 + Go + Vue 3 的本地桌面文档保险箱：构建期加密、运行期按密码 + 设备授权解锁。

CryptoWitch 在构建阶段把 Markdown / PDF 文档加密并嵌入应用，运行时只有 **密码正确 + 本机设备在白名单内** 才会解锁目录并按需加载正文。

它适合分发离线学习资料、内部文档、课程笔记或需要基础访问控制的只读资料包——重点不是把文件放到服务器上，而是把内容、查看器和基础防护一起打包进一个 Windows exe。

## 目录

- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [工作流概览](#工作流概览)
- [环境要求](#环境要求)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [内容打包](#内容打包)
- [开发调试](#开发调试)
- [构建发布](#构建发布)
- [项目结构](#项目结构)
- [验证命令](#验证命令)
- [安全说明](#安全说明)
- [常见问题](#常见问题)

## 功能特性

- 构建期加密 Markdown / PDF，运行时按密码解锁。
- 使用 Argon2id 派生密钥，AES-256-GCM 加密 vault 数据。
- 每篇文档独立加密，避免解锁时一次性把全部正文加载进内存。
- **设备白名单**：基于网卡 MAC 限制可查看文档的设备；`allowedMACs` 设为 `*` 或留空时退化为仅凭密码访问。
- PDF 使用 1 MiB 明文块分块加密，前端按块加载后再交给内置 PDF 查看器打开。
- Markdown 支持 GFM、代码高亮和 KaTeX 数学公式渲染，渲染结果在解锁会话内缓存。
- 前端提供目录树、文档搜索、文档类型标识和大小提示。
- Wails 窗口启用 `ContentProtectionEnabled`，并禁用右键、复制、选中、拖拽和常见快捷键。
- 提供一键 Windows 构建脚本，也保留 Wails 原生命令。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端与桌面壳 | Go、Wails v3 |
| 前端 | Vue 3、TypeScript、Vite |
| Markdown | goldmark、goldmark-highlighting |
| 数学公式 | KaTeX |
| 加密 | Argon2id、AES-256-GCM |
| 构建辅助 | Taskfile、PowerShell、pnpm |

## 工作流概览

CryptoWitch 的内容处理分为构建期和运行期两个阶段：

```text
构建期                           运行期
──────                           ──────
content/plain/*.md|*.pdf          用户输入密码
        │                              │
        ▼                              ▼
access.yaml (password+MAC)       校验本机 MAC ∈ 白名单？
        │                              │ 否 → “本机未授权”
        ▼                              │ 是
cmd/packdocs 加密 ───────────►    校验密码 → 解锁目录
        │                              │
        ▼                              ▼
generated.go (嵌入 exe)          点击文档 → 按需解密单篇正文
```

要点：

1. 构建期读取 `content/plain/**/*.md` 和 `content/plain/**/*.pdf`。
2. 从 `access.yaml` 读取构建期密码与 MAC 白名单，`cmd/packdocs` 用密码派生密钥并加密文档。
3. 加密结果与 MAC 白名单一起写入 `internal/vault/generated.go`，编译进最终应用。
4. 运行时先校验本机 MAC，再按密码解锁文档目录。
5. 用户点击文档后，后端按需解密单篇 Markdown 或 PDF 分块。

## 环境要求

- Windows 10/11
- Go 1.25 或与 `go.mod` 兼容的 Go 版本
- Node.js + pnpm
- WebView2 Runtime

如果使用 `.\build-exe.cmd` 或 `.\scripts\build-exe.ps1`，脚本会检查 `go` 和所选包管理器是否可用。默认包管理器是 `pnpm`。

## 快速开始

> 以下两种方式任选其一。首次构建需先按 [配置说明](#配置说明) 准备 `access.yaml`。

### PowerShell

```powershell
# 1. 准备访问配置（从示例复制后编辑 password 与 allowedMACs）
Copy-Item access.example.yaml access.yaml

# 2. 生成加密 vault（写入 internal/vault/generated.go）
go run ./cmd/packdocs

# 3. 构建前端
Push-Location frontend
pnpm install
pnpm run type-check
pnpm run build
Pop-Location

# 4. 构建 Windows exe
.\build-exe.cmd
```

### Bash

```bash
# 1. 准备访问配置
cp access.example.yaml access.yaml

# 2. 生成加密 vault
go run ./cmd/packdocs

# 3. 构建前端
cd frontend
pnpm install
pnpm run type-check
pnpm run build
cd ..

# 4. 构建 Windows exe
go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96 task windows:build ARCH=amd64
```

构建完成后，Windows 产物默认位于 `bin/CryptoWitch.exe`。

## 配置说明

构建期密码与 MAC 白名单都集中在 `access.yaml`（本地维护，已加入 `.gitignore`）。可从提交到仓库的 `access.example.yaml` 复制得到：

```yaml
# 构建期 vault 加密密码；运行时用户需输入相同密码才能解锁。
password: "change-this-password"
# 允许查看文档的网卡 MAC 白名单，命中任一即放行。
# 设为 "*"（或留空整段）时跳过 MAC 校验，仅凭密码即可查看。
allowedMACs:
  - "AA:BB:CC:DD:EE:FF"
  # - "*"
```

字段说明：

| 字段 | 说明 |
| --- | --- |
| `password` | 构建期派生密钥的密码。运行时输入的密码必须与之相同，否则解锁失败。该密码不进入二进制。 |
| `allowedMACs` | 网卡 MAC 白名单。命中任一即放行；含 `*` 或留空时跳过 MAC 校验。 |

获取本机 MAC：

```powershell
Get-NetAdapter | Select-Object Name, MacAddress
```

> 注意：`access.yaml` 含明文密码，仅本地维护，不要提交到仓库、脚本或命令历史。`config.yaml` 仍为可提交的构建期 KDF 配置，与访问密钥分离。

## 内容打包

把要加密的文档放到 `content/plain` 下：

```text
content/
  plain/
    NLP/
      复习上.md
      复习下.md
      资料.pdf
```

支持的文件类型：`.md`、`.pdf`。只要存在至少一个受支持文档即可生成 vault。

每次新增、删除或修改 `content/plain` 下的明文文档后，都需要重新生成 vault：

```powershell
go run ./cmd/packdocs
```

生成结果会更新 `internal/vault/generated.go`（构建产物，不要手工编辑）。若同时改动了 `access.yaml` 的密码或 MAC 白名单，也需重新执行该命令。

## 开发调试

安装前端依赖：

```powershell
Push-Location frontend
pnpm install
Pop-Location
```

生成加密 vault：

```powershell
go run ./cmd/packdocs
```

运行 Wails 开发模式：

```powershell
go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96 dev -config ./build/config.yml -port 9245
```

也可通过 Taskfile 入口运行：

```powershell
go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96 task dev
```

前端单独验证：

```powershell
Push-Location frontend
pnpm run type-check
pnpm run build
Pop-Location
```

## 构建发布

推荐在 Windows 上使用项目提供的一键脚本：

```powershell
.\build-exe.cmd
```

或直接使用 PowerShell 脚本：

```powershell
.\scripts\build-exe.ps1 -Arch amd64 -PackageManager pnpm
```

脚本依次执行：

1. 检查 `go` 和包管理器。
2. 若不存在 `access.yaml`，报错提示从 `access.example.yaml` 复制创建并填入密码与 MAC 白名单。
3. 执行 `go run ./cmd/packdocs` 生成加密 vault。
4. 执行 `go test ./...`。
5. 执行 Wails Windows 构建。
6. 检查 `bin/CryptoWitch.exe` 是否生成。

保留的 Wails 原生命令：

```powershell
go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.96 task windows:build ARCH=amd64 PACKAGE_MANAGER=pnpm
```

如需跳过文档重新打包或测试，可使用脚本已有参数：`-SkipPackDocs`、`-SkipTests`、`-Dev`。

## 项目结构

```text
.
├─ cmd/packdocs/                 # 构建期文档扫描与加密入口
├─ content/plain/                # 明文源文档目录，不应提交真实私密资料
├─ frontend/                     # Vue 3 + TypeScript 前端
│  ├─ src/components/vault/      # 解锁、目录和文档查看组件
│  ├─ src/composables/           # 前端状态与交互防护逻辑
│  └─ bindings/                  # Wails 生成的 TypeScript bindings
├─ internal/vault/               # vault 类型、加密、解密、目录树和 Markdown 渲染
├─ scripts/build-exe.ps1         # Windows exe 构建脚本
├─ build-exe.cmd                 # 双击友好的 Windows 构建入口
├─ access.example.yaml           # 构建期密码与 MAC 白名单示例配置（提交）
├─ access.yaml                   # 实际构建期密码与 MAC 白名单（gitignore，本地维护）
├─ config.yaml                   # 构建期 vault KDF 配置（可提交）
├─ Taskfile.yml                  # Wails / Taskfile 任务入口
└─ main.go                       # Wails 应用入口
```

## 验证命令

后端测试：

```powershell
go test ./...
```

Go 静态检查 / 编译检查：

```powershell
go vet ./...
go build ./...
```

前端类型检查与生产构建：

```powershell
Push-Location frontend
pnpm run type-check
pnpm run build
Pop-Location
```

完整 Windows 构建：

```powershell
.\scripts\build-exe.ps1 -Arch amd64 -PackageManager pnpm
```

## 安全说明

CryptoWitch 提供的是本地资料包的防护增强，**不是 DRM，也不是不可破解的内容保护系统**。请特别注意：

- `ContentProtectionEnabled` 依赖操作系统和 WebView2 能力，不能保证阻止所有截图或录屏。
- 禁用复制、右键、选中和快捷键只能降低误操作或普通复制成本，不能防止调试工具、管理员权限工具、驱动级工具或逆向分析。
- **MAC 白名单同样是访问增强而非 DRM**：网卡 MAC 可被伪造，不能防止逆向分析。
- 构建期密码不要写入仓库、脚本、README、命令历史或 CI 明文日志。`access.yaml` 含明文构建密码，仅本地维护，已加入 `.gitignore`。
- `content/plain` 目录用于构建期明文源文档，正式发布前应确认没有把真实私密资料提交到版本库。
- 加密后的 `generated.go` 会被编译进 exe，分发前应确认使用的是正确文档和正确密码重新生成的 vault。

## 常见问题

### 修改文档后为什么 exe 内容没有变化？

更新 `content/plain` 后必须重新执行 `go run ./cmd/packdocs`，随后再重新构建前端和 Windows exe。

### 运行时密码应该填什么？

运行时输入的密码应与构建期 `access.yaml` 中的 `password` 一致。应用不会从仓库读取明文密码，密码不进入二进制。

### 运行时提示“本机未授权，无法查看文档”怎么办？

说明 `access.yaml` 中 `allowedMACs` 配置了具体 MAC，且本机网卡都不在白名单内。获取本机 MAC（`Get-NetAdapter | Select Name, MacAddress`）加入 `allowedMACs`，或将 `allowedMACs` 设为 `["*"]` 跳过 MAC 校验，再重新执行 `go run ./cmd/packdocs`。

### 可以只打包 Markdown，不放 PDF 吗？

可以。`cmd/packdocs` 会扫描 `content/plain` 下所有 `.md` 和 `.pdf` 文件，只要存在至少一个受支持文档即可生成 vault。

### PDF 为什么要分块？

PDF 可能比普通 Markdown 大很多。分块加密和按块加载可以避免一次性解密并传输整份 PDF，降低峰值内存压力。

### README 里为什么没有写真实密码？

真实密码属于敏感信息，不应进入仓库。示例里的 `change-this-password` 只是占位符，实际构建时请换成自己的密码，并避免写入命令历史。

### `pnpm run build` 已经生成了 `frontend/dist`，为什么还要 Wails 构建？

`pnpm run build` 只生成前端静态资源。Wails 构建会把前端资源、Go 后端服务和加密 vault 一起编译进最终桌面应用。
