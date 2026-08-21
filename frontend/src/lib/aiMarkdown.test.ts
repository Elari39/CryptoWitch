import { describe, it, expect } from "vitest";
import { sanitizeHTML, renderAIMarkdown } from "./aiMarkdown";

/**
 * sanitizeHTML 是 AI 回复（不可信 LLM 输出）渲染前的最后一道 XSS 防线，
 * 本文件集中覆盖典型注入向量（对应审查报告 M1）：
 *   - 事件属性（onerror/onload 等）剥离；
 *   - 危险协议链接（javascript:/data:/vbscript:/file:）解包为纯文本；
 *   - 危险标签（script/svg/math/template/iframe/foreignObject 等）整体移除；
 *   - 实体编码绕过、嵌套注入（链接内嵌 img onerror）；
 *   - img 替换为 alt 文本；
 *   - 白名单属性（class/role/tabindex）保留、白名单协议链接保留。
 */

/** 断言输出不包含任何 on* 事件属性与危险协议。 */
function expectNoActiveContent(html: string) {
  expect(html).not.toMatch(/\son[a-z]+\s*=/i);
  expect(html).not.toMatch(/javascript\s*:/i);
  expect(html).not.toMatch(/vbscript\s*:/i);
  expect(html.toLowerCase()).not.toContain("<script");
  expect(html.toLowerCase()).not.toContain("<svg");
  expect(html.toLowerCase()).not.toContain("<iframe");
}

describe("sanitizeHTML — 事件属性剥离", () => {
  it("剥离 img 上的 onerror/onload", () => {
    const out = sanitizeHTML(
      '<p>hi</p><img src="https://evil/x.png" onerror="alert(1)" onload="alert(2)">'
    );
    expectNoActiveContent(out);
    expect(out).not.toContain("onerror");
    expect(out).not.toContain("onload");
    // img 被整体替换为占位文本（无 alt）。
    expect(out).toContain("（图片）");
  });

  it("剥离任意标签上的 onclick/onmouseover/style/data-* 属性", () => {
    const out = sanitizeHTML(
      '<div onclick="alert(1)" onmouseover="alert(2)" style="color:red" data-x="1" class="keep-me">text</div>'
    );
    expect(out).not.toContain("onclick");
    expect(out).not.toContain("onmouseover");
    expect(out).not.toContain("style=");
    expect(out).not.toContain("data-x");
    // class 属于白名单属性，保留。
    expect(out).toContain('class="keep-me"');
    expect(out).toContain("text");
  });

  it("保留 role / tabindex 白名单属性", () => {
    const out = sanitizeHTML('<span role="button" tabindex="0" aria-hidden="true">x</span>');
    expect(out).toContain('role="button"');
    expect(out).toContain('tabindex="0"');
    expect(out).not.toContain("aria-hidden");
  });
});

describe("sanitizeHTML — 危险协议链接解包", () => {
  it.each([
    '<a href="javascript:alert(1)">click</a>',
    "<a href='javascript&#58;alert(1)'>click</a>",
    '<a href="&#106;avascript:alert(1)">click</a>',
    '<a href=" JaVaScRiPt:alert(1)">click</a>',
    '<a href="vbscript:msgbox(1)">click</a>',
    '<a href="data:text/html,<script>alert(1)</script>">click</a>',
    '<a href="file:///c:/windows/win.ini">click</a>',
  ])("解包危险链接：%s", (input) => {
    const out = sanitizeHTML(input);
    expectNoActiveContent(out);
    // 链接被解包为纯文本，<a> 元素不再存在。
    expect(out).not.toMatch(/<a[\s>]/i);
    expect(out).toContain("click");
  });

  it("保留白名单协议链接（http/https/mailto/锚点/相对路径）", () => {
    expect(sanitizeHTML('<a href="https://example.com/a">a</a>')).toBe(
      '<a href="https://example.com/a">a</a>'
    );
    expect(sanitizeHTML('<a href="http://example.com/b">b</a>')).toBe(
      '<a href="http://example.com/b">b</a>'
    );
    expect(sanitizeHTML('<a href="mailto:user@example.com">c</a>')).toBe(
      '<a href="mailto:user@example.com">c</a>'
    );
    expect(sanitizeHTML('<a href="#section">d</a>')).toBe('<a href="#section">d</a>');
    expect(sanitizeHTML('<a href="./rel">e</a>')).toBe('<a href="./rel">e</a>');
    expect(sanitizeHTML('<a href="/abs">f</a>')).toBe('<a href="/abs">f</a>');
    expect(sanitizeHTML('<a href="../up">g</a>')).toBe('<a href="../up">g</a>');
  });

  it("安全链接上的 target/rel/class 被移除", () => {
    const out = sanitizeHTML(
      '<a href="https://example.com" target="_blank" rel="noopener" class="evil">x</a>'
    );
    expect(out).toBe('<a href="https://example.com">x</a>');
  });
});

describe("sanitizeHTML — 危险标签整体移除", () => {
  it.each([
    "<script>alert(1)</script>",
    '<iframe src="https://evil.example/frame"></iframe>',
    '<svg onload="alert(1)"><circle r="1"/></svg>',
    '<svg><foreignObject><div onclick="alert(1)">x</div></foreignObject></svg>',
    '<math href="javascript:alert(1)">x</math>',
    "<template><img src=x onerror=alert(1)></template>",
    '<object data="https://evil.example/o"></object>',
    '<embed src="https://evil.example/e">',
    '<form action="https://evil.example/f"><input type="text"></form>',
    '<style>body{background:url(javascript:alert(1))}</style>',
    '<video src="https://evil.example/v" onerror="alert(1)"></video>',
    '<canvas></canvas>',
    '<meta http-equiv="refresh" content="0;url=javascript:alert(1)">',
  ])("移除危险标签：%s", (input) => {
    const out = sanitizeHTML(`before${input}after`);
    expectNoActiveContent(out);
    expect(out).toContain("before");
    expect(out).toContain("after");
  });

  it("危险标签的子树被一并移除", () => {
    const out = sanitizeHTML("<div><svg><b onclick='alert(1)'>nested</b></svg></div>ok");
    expect(out).not.toContain("nested");
    expect(out).not.toContain("<svg");
    expect(out).toContain("ok");
  });
});

describe("sanitizeHTML — img 处理", () => {
  it("img 替换为 alt 文本", () => {
    const out = sanitizeHTML('<p>a <img src="https://evil/x.png" alt="截图说明"> b</p>');
    expect(out).not.toContain("<img");
    expect(out).toContain("（图片：截图说明）");
  });

  it("无 alt 的 img 替换为占位文本", () => {
    const out = sanitizeHTML('<p><img src="https://evil/x.png"></p>');
    expect(out).toContain("（图片）");
  });
});

describe("sanitizeHTML — 嵌套与编码绕过", () => {
  it("javascript: 链接内嵌 img onerror 不残留（先深度遍历再解包）", () => {
    const input = '<a href="javascript:alert(1)"><img src=x onerror="alert(2)">link</a>';
    const out = sanitizeHTML(input);
    expectNoActiveContent(out);
    expect(out).not.toContain("<img");
    expect(out).toContain("link");
  });

  it("深层嵌套的 onerror 属性被剥离", () => {
    const input = '<div><p><span><code onerror="alert(1)">x</code></span></p></div>';
    const out = sanitizeHTML(input);
    expect(out).not.toContain("onerror");
    expect(out).toContain("x");
  });

  it("实体编码的 javascript: 在 href 中仍被识别（DOMParser 先解码）", () => {
    const out = sanitizeHTML('<a href="&#x6a;avascript:alert(1)">z</a>');
    expect(out).not.toMatch(/<a[\s>]/i);
    expect(out).toContain("z");
  });
});

describe("renderAIMarkdown — 端到端消毒", () => {
  it("Markdown 源中的内联 HTML 被消毒", () => {
    const md = 'Hello <img src=x onerror="alert(1)"> **world**';
    const out = renderAIMarkdown(md);
    expectNoActiveContent(out);
    expect(out).toContain("Hello");
  });

  it("Markdown 链接中的 javascript: 协议被解包", () => {
    const md = "[click me](javascript:alert(1))";
    const out = renderAIMarkdown(md);
    expectNoActiveContent(out);
    expect(out).toContain("click me");
  });

  it("正常 Markdown 输出保留结构与代码窗", () => {
    const out = renderAIMarkdown("# Title\n\n```go\nfmt.Println(1)\n```\n\n[ok](https://example.com)");
    expect(out).toMatch(/<h1>/);
    expect(out).toContain('href="https://example.com"');
    expect(out).toContain("ai-code-window");
  });

  it("代码块中的 HTML 转义不执行", () => {
    const out = renderAIMarkdown("```\n<script>alert(1)</script>\n```");
    expectNoActiveContent(out);
    expect(out).toContain("&lt;script&gt;");
  });
});
