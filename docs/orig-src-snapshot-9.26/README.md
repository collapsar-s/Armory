# Armory 启动器

> **⚠️ 快照存档（2026-11-06 落盘）**：本目录是 9-26 时点的**原版源码快照**，从会话恢复产物
> `/tmp/recover/files/armory/` 复制固化（/tmp 重启即失效）。用途：
> 1. `internal/platform/` 6 个 .go（cgo 内嵌 C）是**原版平台层的权威底本**，第六轮还原据此移植；
> 2. `main.go` 是原版装配结构参考。
> 快照窗口配置（760×520、圆角 24、动画 0.22/0.16）是 9-26 旧值，9-27 安装版实测为
> 880×560、0.18/0.15 + 2% 缩放——校准以 `docs/RUNNING-APP-FEATURES.md` 为准。
> `internal/service/api.go` 快照缺 SetAlwaysOnTop/AddSubCategory/SetTopHook/Detect* 等方法
> （重放丢失所致），**不是完整原版**，勿据其"回退"服务层。

---

个人专用的 macOS 26 工具启动器。上游文档见工作区根目录：《Armory启动器_工具任务书_V1.0.md》《Armory启动器_开发计划_V1.0.md》。

## 当前状态

- **M0（玻璃骨架）代码完成**：Wails v3.0.0-beta.26 + 原生 Liquid Glass + ⌥Space 全局热键 + 失焦隐藏。
- 运行验证通过（macOS 26.7 / arm64 / `-tags private_mac_apis`，热键注册无错误，窗口正常加载）。
- 视觉验收（原生玻璃质感、⌥Space 实按）待用户确认。

## 常用命令（项目根目录执行）

```bash
# 开发模式（前端热重载，已内置 private_mac_apis 标签）
wails3 dev

# 编译（Taskfile 已固定 CGO_ENABLED=1 与 min target 26.0）
wails3 task build

# 打包 .app
wails3 task package
```

手动编译时必须显式 `CGO_ENABLED=1`：

```bash
CGO_ENABLED=1 go build -tags "private_mac_apis dev" -o bin/armory .
```

> ⚠️ **本机注意**：`~/.zshrc` 全局设置了 `export CGO_ENABLED=0`（安全开发默认），
> 且该 OS 环境变量会**压制** Taskfile 内声明的 `CGO_ENABLED: 1` 和 `go env -w`。
> 因此本项目构建必须任选其一：
> 1. 终端构建时显式前缀 `CGO_ENABLED=1 wails3 task build`；
> 2. 在 IDE 的 go 构建配置中添加环境变量 `CGO_ENABLED=1`；
> 3. 修改 ~/.zshrc 全局默认（需用户确认）。
>
> 症状（忘了开 CGO 时）：报大量 `undefined: macosApp / newPlatformApp ...`——
> Wails 的 macOS 界面层是 Objective-C（依赖 CGO），CGO 关闭即整层被排除。

## 架构铁律（开发计划 §9）

1. Wails/平台私有 API 只进 `internal/platform`；
2. `internal/service` 禁止 import Wails；
3. 动画/主题 token 以 `ui-prototype/index.html` 为唯一视觉基准；
4. tools.json 字段变更保持向后兼容（新字段 omitempty）。

## 目录

```
main.go                  入口装配
internal/platform/       window.go 玻璃窗口 | hotkey.go Carbon ⌥Space（cgo）
internal/service/        registry.go 工具库读写（M1 接入）
internal/model/          数据结构
frontend/                Vue3 + TS + Vite（src/App.vue 当前为 M0 占位）
build/darwin/            Info.plist（LSMinimumSystemVersion=26.0）与打包资源
```
