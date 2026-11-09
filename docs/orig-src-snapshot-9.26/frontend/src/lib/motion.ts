/**
 * 动画常量系统（M2-1，任务书 §2.5 参数表的唯一落地处）。
 * 铁律：改动画手感只改本文件；所有组件不得自写曲线时长。
 */

/** 弹性出场 ≈ spring(stiffness 380, damping 26)，约 320ms */
export const SPRING = "cubic-bezier(0.34, 1.4, 0.64, 1)";
/** 柔和弹性（波纹、进度类） */
export const SPRING_SOFT = "cubic-bezier(0.3, 1, 0.5, 1)";
/** 收起（ease-in，"来浮去隐"） */
export const EASE_IN = "cubic-bezier(0.5, 0, 0.75, 0)";

/** 各场景时长（ms），键名对应任务书 §2.5 表行 */
export const D = {
  windowShow: 320,
  windowHide: 150,
  hover: 120,
  press: 80,
  launch: 240,
  view: 240,
  stagger: 40, // 命令面板列表错峰
  pop: 280,
  popClose: 180,
  pill: 240,
  drawer: 260,
  toast: 240,
  ripple: 360,
} as const;

const REDUCE_CLASS = "reduce-motion";

let userReduced = false;

/** 用户设置（设置抽屉）变更时调用 */
export function setUserReducedMotion(v: boolean) {
  userReduced = v;
  sync();
}

function systemReduced() {
  return matchMedia("(prefers-reduced-motion: reduce)").matches;
}

function sync() {
  document.documentElement.classList.toggle(REDUCE_CLASS, userReduced || systemReduced());
}

/** 应用启动时调用；监听系统"减弱动态效果"变化 */
export function initMotion() {
  matchMedia("(prefers-reduced-motion: reduce)").addEventListener("change", sync);
  sync();
}
