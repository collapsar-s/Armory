# Armory 重构 · 团队协作约定与技术规范

> 本文件是全体 Agent 的**唯一权威约定**。所有实现必须遵守本文，避免返工。
> 目标：在 `/Users/collapsars/Code/tools/Armory/` 重构出一个与 `recovered/` 版功能**完全一致**、但架构与工程质量**更优**的 Armory。

---

## 0. 参考物（只读，绝不修改）
- `recovered/` —— 原始恢复源码，是"一模一样"的功能基线。可读、可抄逻辑，**不要改**。
- `recovered/frontend/` —— 原版前端产物（100% 提取），UI/CSS 的最终真值来源。
- `/tmp/wailsref/Armory/` —— wails3 官方 vanilla 工程，构建脚手架/打包流程的参考。

---

## 1. 产品一句话
Armory 是 macOS 渗透工具**收纳/启动器**（Spotlight 式无边框悬浮面板）。把 `.app/.jar/.py/.sh/二进制` 收录成卡片，可搜索、可分类、可拖拽收录、可指定 Java/Python 运行时启动，支持图标提取、说明文档、备份导入导出、主题/字体/动效设置。

- 应用标识：`com.armory.launcher`，版本 `0.1.0`，名称 `Armory`（早期代号 Vulpes）。
- 数据目录：`~/.armory/`（`tools.json` + `icons/*.png`）。
- 默认热键：`Alt+Space`（⌥Space）呼出/隐藏。

---

## 2. 技术栈（与原版一致）
- 后端：Go 1.26.5 + Wails v3 `v3.0.0-beta.26`，`CGO_ENABLED=1`。
- 前端：Vue 3.5 + Vite 5（生产用 `@vitejs/plugin-vue`）。
- 打包：wails3 Taskfile（darwin build → .app → codesign adhoc → dmg）。

---

## 3. 目录结构（重构后）
```
Armory/
├── main.go                     # 应用装配（Lead 负责）
├── go.mod / go.sum
├── internal/
│   ├── model/model.go          # 数据模型（Lead 已定稿，勿改字段/JSON tag）
│   ├── store/store.go          # 持久化（后端 Agent）
│   ├── service/               # 业务（后端 Agent）
│   │   ├── api.go  registry.go  launcher.go  iconextract.go
│   │   ├── envprobe.go  readme.go  backup.go  util.go
│   └── platform/               # macOS 原生（平台 Agent）
│       ├── platform_darwin.go  platform_stub.go
│       ├── glass_darwin.m  window_anim_darwin.m  traffic_lights_darwin.m
│       └── hotkey_darwin.m  loginitem_darwin.m
├── frontend/
│   ├── index.html  package.json  vite.config.js  .npmrc
│   ├── src/
│   │   ├── main.js  App.vue
│   │   ├── components/  composables/  bindings/  styles/
│   └── dist/                   # 构建产物（go:embed）
├── build/                      # wails3 打包脚手架（已从参考工程拷入）
├── Taskfile.yml
├── Makefile
└── docs/                       # 文档
```

---

## 4. 数据模型（`internal/model/model.go`，已定稿）
**字段名与 JSON tag 必须与 recovered 完全一致**（保证读写同一份 `~/.armory/tools.json`）。

```go
type Tool struct {
	ID string `json:"id"`; Name string `json:"name"`; Path string `json:"path"`
	Args string `json:"args,omitempty"`; WorkingDir string `json:"workingDir,omitempty"`
	CategoryID string `json:"categoryId"`; IconPath string `json:"iconPath,omitempty"`
	HasIcon bool `json:"hasIcon"`; Type string `json:"type"`; EnvVersion string `json:"envVersion,omitempty"`
	Notes string `json:"notes,omitempty"`; RunCount int `json:"runCount,omitempty"`
	LastRunAt string `json:"lastRunAt,omitempty"`; CreatedAt string `json:"createdAt"`
}
type Category struct { ID string `json:"id"`; Name string `json:"name"`; Order float64 `json:"order"` }
type RuntimeVersion struct { Name string `json:"name"`; Path string `json:"path"` }
type Settings struct {
	Theme string `json:"theme,omitempty"`; ReduceMotion bool `json:"reduceMotion,omitempty"`
	AlwaysOnTop bool `json:"alwaysOnTop,omitempty"`; FontScale string `json:"fontScale,omitempty"`
	LaunchOnLogin bool `json:"launchOnLogin,omitempty"`; Hotkey string `json:"hotkey,omitempty"`
	JavaVersions []RuntimeVersion `json:"javaVersions,omitempty"`
	PythonVersions []RuntimeVersion `json:"pythonVersions,omitempty"`
}
type RegistryData struct { Tools []Tool `json:"Tools"`; Categories []Category `json:"Categories"`; Settings Settings `json:"Settings"` }
```

Type 取值：`app | jar | py | sh | bin`。

---

## 5. 绑定 API（前端 `$` 对象，共 27 方法 + 3 事件）
**方法名（前端可见名）必须逐一对应**（这是"一模一样"的硬约束）：

`listCategories` `listTools` `addByPath` `update` `remove` `addCategory` `renameCategory`
`deleteCategory` `launch` `getIcon` `getSettings` `setTheme` `setReduceMotion`
`setLaunchAtLogin` `setAlwaysOnTop` `setFontScale` `getAccentColor` `setGlassTheme`
`saveJavaVersions` `savePythonVersions` `detectJavaVersions` `detectPythonVersions`
`listRecent` `getReadme` `exportBackup` `importBackup` `hideWindow`

事件：`files-dropped`(data.files []string) / `toast`(data string) / `panel-shown`(无)。

### 5.1 前端调用约定（关键决策）
- **采用 wails3 官方 bindings**（`wails3 generate bindings` 产物，位于 `frontend/src/bindings/`，运行 `wails3 generate bindings -d frontend/src/bindings` 生成）。
- 后端方法名（Go 导出名）→ 前端调用名映射如下，**前端 composable `useApi()` 要提供与原 `wails.js` 完全相同的 27 个方法签名**，内部转调官方 bindings，保持 App/组件代码不变：
  - `ListCategories→listCategories` `ListTools→listTools` `AddToolByPath→addByPath`
  - `UpdateTool→update` `DeleteTool→remove` `AddCategory→addCategory`
  - `RenameCategory→renameCategory` `DeleteCategory→deleteCategory` `Launch→launch`
  - `GetIcon→getIcon` `GetSettings→getSettings` `SetTheme→setTheme`
  - `SetReduceMotion→setReduceMotion` `SetLaunchAtLogin→setLaunchAtLogin`
  - `SetAlwaysOnTop→setAlwaysOnTop` `SetFontScale→setFontScale`
  - `GetAccentColor→getAccentColor` `SetGlassTheme→setGlassTheme`
  - `SaveJavaVersions→saveJavaVersions` `SavePythonVersions→savePythonVersions`
  - `DetectJavaVersions→detectJavaVersions` `DetectPythonVersions→detectPythonVersions`
  - `ListRecentTools→listRecent` `GetToolReadme→getReadme` `ExportBackup→exportBackup`
  - `ImportBackup→importBackup` `HidePanel→hideWindow`
- 若官方 bindings 生成因环境不稳定失败，**回退**：保留 `window._wails.invoke(methodID,args)` 方式（methodID 见 recovered/frontend-src/src/wails.js），但要封装在一个文件里，便于将来切换。
- 返回值语义保持：`listTools/listRecent` 每项含 `hasIcon`；`getIcon` 返回 dataURL 或空串；`getReadme` 返回原始 markdown。

---

## 6. 会话产物与运行
- 前端 `vite build` 输出到 `frontend/dist`（`base: './'`），后端 `//go:embed all:frontend/dist`。
- 后端启动：`service.NewRegistry()` 打开 `~/.armory/tools.json`，`service.NewAPI(reg)` 作为 Wails Service 绑定。

---

## 7. 构建环境（重要！已实测）
本机工具链齐全，但有 **sandbox 限制**，所有构建命令必须带以下环境变量：
```bash
export PATH="$HOME/go/bin:$PATH"
export GOCACHE=/tmp/gocache-armory          # 默认 ~/Library/Caches/go-build 被沙箱拦截
export npm_config_cache=/tmp/npmcache-armory # 默认 ~/.npm 被沙箱拦截(root属主)
export CGO_ENABLED=1                          # 该环境默认 0，wails/production 必需
source ~/.nvm/nvm.sh                          # node 24 / pnpm 11 在 nvm 里
```
- 前端构建：`cd frontend && pnpm install && ./node_modules/.bin/vite build`
  （pnpm 的 install 检查会报 esbuild 构建审批，可忽略；直接用 `./node_modules/.bin/vite build` 构建即可。package.json 里把 `pnpm.onlyBuiltDependencies` 删掉，改为 `frontend/.npmrc` + `frontend/pnpm-workspace.yaml` 设 `onlyBuiltDependencies: [esbuild]`。）
- 后端构建：`go build -tags private_mac_apis -o bin/armory .`（开发）
  或 wails 生产：`wails3 build`（= `-tags production`）。
- **注意 build tag**：开发/调试可用 `-tags private_mac_apis` 或 `dev`；正式打包 `wails3 task darwin:package`（`-tags production`）。平台层代码要同时兼容 `production` 与 `private_mac_apis` 构建（用 `//go:build darwin`，不要依赖仅 dev 的 tag）。
- **`.dmg` 创建（hdiutil）在当前文件沙箱下会被拒绝**（`hdiutil: create failed`）：`.app` 能正常生成+ad-hoc 签名；`.dmg` 由 Lead 在打包阶段用 `danger-full-access` 一次性执行或说明约束。Agent 无需处理 dmg。

---

## 8. 代码风格与质量要求（"优化"的具体含义）
1. **修掉恢复版的缺陷**：
   - `ToolView` 不要再重复嵌入 `HasIcon` 造成与 `model.Tool.HasIcon` 冲突（Tool 本身就有 HasIcon 字段，直接返回 `[]model.Tool` 或独立 View 仅含必要派生）。
   - 删除无用函数：`itoa/containsName/prependPath/prependPathEnv/majorVersion/filepathAbs/homeDir/readmeCandidates里未用的`、`platform.ToggleWindow/InstallToggleHotkey` 空壳（改为真实实现）。
   - 清理未用 import（如 main.go 的 `events`）。
2. 错误处理：所有 I/O 返回有意义错误；启动/删除等不静默吞错。
3. 并发安全：Registry 用 `sync.RWMutex`，读写分离。
4. 原子写：先写 `.tmp` 再 `rename`。
5. 路径安全：`ExpandPath` 展开 `~`/env 并 `Abs`；启动前校验文件存在。
6. 注释清晰、中文（与原版一致）。
7. 每组改动要能通过 `gofmt`/`go vet`。

---

## 9. 交付验收（Lead 最终执行）
- [ ] `cd frontend && ./node_modules/.bin/vite build` 成功，产物在 `frontend/dist/`
- [ ] `go build -tags production -o bin/armory .`（CGO_ENABLED=1）成功
- [ ] `go vet ./...` 通过
- [ ] 启动 `bin/armory.app` 能出面板（冒烟：进程起、无 panic）
- [ ] `wails3 task darwin:package` 生成 `bin/armory.app` + ad-hoc 签名
- [ ] `.dmg` 打包（Lead 用 escalated 权限）或记录约束
- [ ] 27 个 API + 3 事件与 recovered 一一对应
- [ ] 读写 recovered 版的 `~/.armory/tools.json` 兼容

---

## 10. 分工与写入边界（避免冲突）
| 成员 | 写入范围（目录/文件） |
|---|---|
| **Lead** | `main.go` `go.mod` `go.sum` `internal/model/**` `Taskfile.yml` `Makefile` `build/**` `docs/**` `frontend/{index.html,package.json,vite.config.js,.npmrc,pnpm-workspace.yaml}` `frontend/dist/**` 集成修复 |
| **backend-agent** | `internal/store/**` `internal/service/**` |
| **platform-agent** | `internal/platform/**` |
| **frontend-agent** | `frontend/src/**`（除 bindings 由生成） |

写入范围互不重叠。跨范围修改先在群里/任务里说。公共契约（model、API 名单、目录）以本文件为准。
