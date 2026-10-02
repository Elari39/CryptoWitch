import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";

// 独立于 vite.config.ts：启用 Vue 组件测试，不加载 Wails 构建插件。
// happy-dom 提供 DOMParser/Element 等 DOM API，供消毒器测试使用。
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: "happy-dom",
    environmentOptions: {
      happyDOM: {
        settings: {
          enableJavaScriptEvaluation: false,
          navigation: { disableChildFrameNavigation: true },
          disableJavaScriptFileLoading: true,
          disableCSSFileLoading: true,
          handleDisabledFileLoadingAsSuccess: true,
        },
      },
    },
    include: ["src/**/*.test.ts"],
    // 每个测试文件独立进程，隔离模块单例状态。
    pool: "forks",
  },
});
