import { defineConfig } from "vitest/config";

// 独立于 vite.config.ts：仅用于单元测试，不加载 Vue/Wails 插件。
// happy-dom 提供 DOMParser/Element 等 DOM API，供消毒器测试使用。
export default defineConfig({
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.ts"],
    // forks 池每个测试文件独立进程，规避 happy-dom 主进程 teardown 时
    // AsyncTaskManager 的非致命报错噪音。
    pool: "forks",
  },
});
