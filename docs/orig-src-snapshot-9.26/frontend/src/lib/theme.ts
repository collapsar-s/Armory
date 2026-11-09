/**
 * 主题系统（M2-7，任务书 §2.4 色板的唯一落地处）。
 * 色值以 CSS 变量形式定义在 App.vue 的 :root（token 区），
 * 本文件负责模式切换、系统强调色注入、原生玻璃层深浅同步。
 */
import { api } from "./api";

export type ThemeMode = "auto" | "light" | "dark";
export type FontScale = "standard" | "large" | "xlarge";

/** 字体缩放档位 → zoom 值（默认 large = 比初版大一号） */
const FONT_ZOOM: Record<FontScale, number> = { standard: 1.0, large: 1.1, xlarge: 1.2 };

/** 应用字体缩放（WKWebView 支持 CSS zoom，整站等比缩放、布局自适应） */
export function applyFontScale(scale: string | undefined) {
  const key: FontScale = (scale === "standard" || scale === "xlarge") ? scale : "large";
  document.documentElement.style.setProperty("zoom", String(FONT_ZOOM[key]));
}

let currentMode: ThemeMode = "auto";
let media: MediaQueryList | undefined;

/** 当前生效外观（auto = 系统外观） */
function effectiveDark(): boolean {
  if (currentMode === "dark") return true;
  if (currentMode === "light") return false;
  media ??= window.matchMedia("(prefers-color-scheme: dark)");
  return media.matches;
}

/** 把生效外观应用到 DOM + 通知 Go 侧同步原生玻璃 tint */
function applyEffective() {
  const dark = effectiveDark();
  document.documentElement.dataset.theme = dark ? "dark" : "light";
  api.setGlassTheme(dark);
}

/** 切换主题模式（auto 跟随系统明暗，含系统外观变化时的实时联动） */
export function applyTheme(mode: ThemeMode) {
  currentMode = mode;
  if (mode === "auto") {
    media ??= window.matchMedia("(prefers-color-scheme: dark)");
    media.onchange = () => {
      if (currentMode === "auto") applyEffective();
    };
  } else {
    if (media) media.onchange = null;
  }
  applyEffective();
}

/** 应用启动时调用：恢复持久化设置 + 注入系统强调色 */
export async function initTheme(): Promise<ThemeMode> {
  // 系统强调色（任务书 §2.4：跟随系统 accent，后端解析失败已回退 #0A84FF）
  const accent = await api.getAccentColor();
  document.documentElement.style.setProperty("--accent", accent);

  const s = await api.getSettings();
  const mode = (s.theme as ThemeMode) || "auto";
  applyTheme(mode);
  applyFontScale((s as any).fontScale); // 字体缩放随启动恢复（默认 large）
  return mode;
}
