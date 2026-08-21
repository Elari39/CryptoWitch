package main

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"cryptowitch/internal/vault"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	vaultService := vault.NewService(vault.EmbeddedVault)

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "CryptoWitch",
		Description: "Encrypted Markdown document viewer",
		Services: []application.Service{
			application.NewService(vaultService),
		},
		Assets: application.AssetOptions{
			Handler: cspAssetHandler(assets, vaultService),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:                      "CryptoWitch",
		Width:                      1180,
		Height:                     760,
		MinWidth:                   920,
		MinHeight:                  620,
		EnableFileDrop:             false,
		DevToolsEnabled:            false,
		DefaultContextMenuDisabled: true,
		ContentProtectionEnabled:   true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		Windows: application.WindowsWindow{
			Theme:                   application.Dark,
			GeneralAutofillEnabled:  false,
			PasswordAutosaveEnabled: false,
		},
		BackgroundColour: application.NewRGB(14, 18, 24),
		URL:              "/",
	})

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}

// cspPolicy 生成 Content-Security-Policy 内容。
//   - script-src 仅 'self'：页面内注入的内联脚本一律无法执行（Wails 运行时经 WebView2
//     原生注入，不受页面 CSP 约束，无需 'unsafe-inline'；KaTeX 只依赖 style-src）；
//   - style-src 保留 'unsafe-inline'：兼容 KaTeX / goldmark 高亮的内联样式；
//   - connect-src 'self' 封死页面内 XSS 外联（Wails IPC 为同源 fetch，不受影响）；
//   - frame-src blob: 放行 PDF 分块查看器 iframe；
//   - img-src 默认仅同源/data/blob，allowRemoteImages 开启时追加 http/https。
func cspPolicy(allowRemoteImages bool) string {
	imgSrc := "img-src 'self' data: blob:"
	if allowRemoteImages {
		imgSrc = "img-src 'self' data: blob: https: http:"
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		imgSrc,
		"font-src 'self' data:",
		"frame-src blob:",
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
	}, "; ")
}

// cspAssetHandler 包装 Wails 静态资源服务：仅对 text/html 响应在 </head> 前注入
// Content-Security-Policy meta，作为 XSS 的防御纵深。其余资源（JS/CSS/字体等）
// 不缓冲、直接透传，避免大体积静态资产的内存翻倍。
// dev 模式页面由 Vite dev server 提供，不经过此 handler，因此 CSP 仅在生产构建生效。
func cspAssetHandler(assets embed.FS, vaultService *vault.Service) http.Handler {
	base := application.AssetFileServerFS(assets)
	policy, err := vaultService.GetSecurityPolicy()
	if err != nil {
		log.Printf("warn: get security policy: %v", err)
	}
	meta := []byte(fmt.Sprintf(
		`<meta http-equiv="Content-Security-Policy" content="%s">`,
		cspPolicy(policy.AllowRemoteImages),
	))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &htmlOnlyResponseWriter{upstream: w, header: make(http.Header)}
		base.ServeHTTP(rec, r)
		rec.finish(meta)
	})
}

// htmlOnlyResponseWriter 在收到响应头时按 Content-Type 分流：
//   - text/html：缓冲完整响应体，供上层注入 CSP 后一次性写回；
//   - 其余类型：header 与响应体直接透传 upstream，不占用额外内存。
type htmlOnlyResponseWriter struct {
	upstream http.ResponseWriter
	header   http.Header
	status   int
	decided  bool
	html     bool
	started  bool
	body     bytes.Buffer
}

func (b *htmlOnlyResponseWriter) Header() http.Header { return b.header }

func (b *htmlOnlyResponseWriter) WriteHeader(status int) {
	if b.status != 0 {
		return
	}
	b.status = status
	b.decide()
}

// decide 依据当前响应头判定是否为 HTML（WriteHeader 后、首次 Write 前调用）。
func (b *htmlOnlyResponseWriter) decide() {
	if b.decided {
		return
	}
	b.decided = true
	b.html = strings.Contains(b.header.Get("Content-Type"), "text/html")
}

// flushDirectHeader 把缓存的响应头写到底层 writer（直通模式首次写出前调用）。
func (b *htmlOnlyResponseWriter) flushDirectHeader() {
	if b.started {
		return
	}
	b.started = true
	header := b.upstream.Header()
	for key, values := range b.header {
		header[key] = values
	}
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	b.upstream.WriteHeader(status)
}

func (b *htmlOnlyResponseWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	b.decide()
	if !b.html {
		b.flushDirectHeader()
		return b.upstream.Write(p)
	}
	return b.body.Write(p)
}

// finish 在 base handler 返回后收尾：HTML 响应注入 CSP 后写回；
// 直通响应兜底输出响应头（如空响应体场景）。
func (b *htmlOnlyResponseWriter) finish(meta []byte) {
	if !b.html {
		b.flushDirectHeader()
		return
	}
	body := b.body.Bytes()
	status := b.status
	if status == 0 {
		status = http.StatusOK
	}
	// 仅对完整（非 206 分段）的成功 HTML 响应注入；注入后 Content-Length 失效，需移除。
	if status == http.StatusOK {
		if injected := injectCSPMeta(body, meta); injected != nil {
			body = injected
			b.header.Del("Content-Length")
		}
	}
	header := b.upstream.Header()
	for key, values := range b.header {
		header[key] = values
	}
	b.upstream.WriteHeader(status)
	_, _ = b.upstream.Write(body)
}

// injectCSPMeta 在 </head> 前插入 CSP meta；找不到 </head> 时返回 nil（不注入）。
func injectCSPMeta(html []byte, meta []byte) []byte {
	index := bytes.LastIndex(bytes.ToLower(html), []byte("</head>"))
	if index < 0 {
		return nil
	}
	headEnd := index + len("</head>")
	injected := make([]byte, 0, len(html)+len(meta)+1)
	injected = append(injected, html[:index]...)
	injected = append(injected, meta...)
	injected = append(injected, html[index:headEnd]...)
	injected = append(injected, html[headEnd:]...)
	return injected
}

var _ io.Writer = (*htmlOnlyResponseWriter)(nil)
