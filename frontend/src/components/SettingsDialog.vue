<script setup>
// 设置对话框（分区顺序与运行版一致）：
//   通用（开机自启 / 窗口置顶） → 外观（主题 / 界面字体 / 减弱动态效果）
//   → Java 环境（+提示） → Python 环境（+提示）
//   → 快捷键（⌥Space，固定） → 收录（拖拽收录 / 真实图标，绿色圆点）
//   → 数据（导出备份 / 导入还原两步确认） → 页脚。
import { computed } from 'vue'

const props = defineProps({
  settings: { type: Object, required: true }, // theme, fontScale, reduceMotion, alwaysOnTop, launchOnLogin
  hotkey: { type: String, default: 'Alt+Space' },
  javaVersions: { type: Array, required: true },
  pythonVersions: { type: Array, required: true },
  savingEnv: { type: String, default: '' }, // '' | 'java' | 'py'
  confirmImport: { type: Boolean, default: false },
})
const emit = defineEmits([
  'close',
  'set-theme',
  'set-font-scale',
  'set-reduce-motion',
  'set-always-on-top',
  'set-launch-on-login',
  'scan-java',
  'scan-python',
  'add-env',
  'del-env',
  'save-env',
  'export-backup',
  'import-backup',
])

// 子组件内直接编辑版本行对象（版本数组由父级持有，此处原地编辑 v-model 生效）。
function addEnv(list) {
  emit('add-env', list)
}
function delEnv(list, i) {
  emit('del-env', list, i)
}

// 热键固定为 ⌥Space（与原版一致，不可自定义），chips 仅作展示。
const hotkeyChips = computed(() => ['⌥', 'Space'])
</script>

<template>
  <div class="settings-mask" @click="emit('close')"></div>
  <div class="settings-dialog" role="dialog" aria-label="设置">
    <div class="settings-head">
      <div class="drawer-title">设置</div>
      <button class="iconbtn" data-tip="关闭" title="关闭" @click="emit('close')">×</button>
    </div>
    <div class="settings-body">
      <div class="group-title">通用</div>
      <div class="group">
        <div class="row">
          <div class="rinfo"><div class="rlabel">开机自启</div><div class="rsub">登录 macOS 时自动启动（可在系统设置 → 登录项中管理）</div></div>
          <div class="seg">
            <button :class="{ on: settings.launchOnLogin }" @click="emit('set-launch-on-login', true)">开</button>
            <button :class="{ on: !settings.launchOnLogin }" @click="emit('set-launch-on-login', false)">关</button>
          </div>
        </div>
        <div class="row">
          <div class="rinfo"><div class="rlabel">窗口置顶</div><div class="rsub">面板始终浮于其他窗口之上（Spotlight 式），关闭后与其他窗口正常争抢层级</div></div>
          <div class="seg">
            <button :class="{ on: settings.alwaysOnTop }" @click="emit('set-always-on-top', true)">开</button>
            <button :class="{ on: !settings.alwaysOnTop }" @click="emit('set-always-on-top', false)">关</button>
          </div>
        </div>
      </div>

      <div class="group-title">外观</div>
      <div class="group">
        <div class="row">
          <div class="rinfo"><div class="rlabel">主题</div><div class="rsub">跟随系统明暗，或手动指定</div></div>
          <div class="seg">
            <button :class="{ on: settings.theme === 'auto' }" @click="emit('set-theme', 'auto')">自动</button>
            <button :class="{ on: settings.theme === 'light' }" @click="emit('set-theme', 'light')">浅色</button>
            <button :class="{ on: settings.theme === 'dark' }" @click="emit('set-theme', 'dark')">深色</button>
          </div>
        </div>
        <div class="row">
          <div class="rinfo"><div class="rlabel">界面字体</div><div class="rsub">全局缩放，立即生效</div></div>
          <div class="seg">
            <button :class="{ on: settings.fontScale === 'standard' }" @click="emit('set-font-scale', 'standard')">标准</button>
            <button :class="{ on: settings.fontScale === 'large' }" @click="emit('set-font-scale', 'large')">大</button>
            <button :class="{ on: settings.fontScale === 'xlarge' }" @click="emit('set-font-scale', 'xlarge')">特大</button>
          </div>
        </div>
        <div class="row">
          <div class="rinfo"><div class="rlabel">减弱动态效果</div><div class="rsub">动画降级为快速淡入淡出</div></div>
          <div class="seg">
            <button :class="{ on: settings.reduceMotion }" @click="emit('set-reduce-motion', true)">开</button>
            <button :class="{ on: !settings.reduceMotion }" @click="emit('set-reduce-motion', false)">关</button>
          </div>
        </div>
      </div>

      <div class="group-title">Java 环境</div>
      <div class="group">
        <div class="env-grid">
          <div v-for="(v, i) in javaVersions" :key="'j' + i" class="env-row">
            <input v-model="v.name" class="env-name" placeholder="版本名" spellcheck="false" />
            <input v-model="v.path" placeholder="java 可执行文件路径" spellcheck="false" autocomplete="off" />
            <button class="env-del" title="删除此版本" @click="delEnv(javaVersions, i)">
              <svg viewBox="0 0 24 24"><path d="M9 3v1H4v2h16V4h-5V3H9zm-3.5 5 .8 12.1c.1 1 .9 1.9 2 1.9h7.4c1.1 0 1.9-.9 2-1.9L18.5 8h-13zM10 11h1.5v7H10v-7zm2.5 0H14v7h-1.5v-7z" /></svg>
            </button>
          </div>
          <button v-if="!javaVersions.length" class="env-empty" @click="addEnv(javaVersions)">尚未配置，点击添加 Java 版本</button>
        </div>
        <div class="env-foot">
          <span class="env-foot-left">
            <button class="env-add" @click="addEnv(javaVersions)">＋ 添加版本</button>
            <button class="env-add" @click="emit('scan-java')">⟳ 扫描本机</button>
          </span>
          <button class="btn btn-primary env-save" :disabled="savingEnv === 'java'" @click="emit('save-env', 'java')">{{ savingEnv === 'java' ? '保存中…' : '保存' }}</button>
        </div>
      </div>
      <div class="group-hint">配置后，JAR 工具启动时按「编辑 → Java 版本」使用对应解释器；未指定则用系统默认 java。</div>

      <div class="group-title">Python 环境</div>
      <div class="group">
        <div class="env-grid">
          <div v-for="(v, i) in pythonVersions" :key="'p' + i" class="env-row">
            <input v-model="v.name" class="env-name" placeholder="版本名" spellcheck="false" />
            <input v-model="v.path" placeholder="python 可执行文件路径" spellcheck="false" autocomplete="off" />
            <button class="env-del" title="删除此版本" @click="delEnv(pythonVersions, i)">
              <svg viewBox="0 0 24 24"><path d="M9 3v1H4v2h16V4h-5V3H9zm-3.5 5 .8 12.1c.1 1 .9 1.9 2 1.9h7.4c1.1 0 1.9-.9 2-1.9L18.5 8h-13zM10 11h1.5v7H10v-7zm2.5 0H14v7h-1.5v-7z" /></svg>
            </button>
          </div>
          <button v-if="!pythonVersions.length" class="env-empty" @click="addEnv(pythonVersions)">尚未配置，点击添加 Python 版本</button>
        </div>
        <div class="env-foot">
          <span class="env-foot-left">
            <button class="env-add" @click="addEnv(pythonVersions)">＋ 添加版本</button>
            <button class="env-add" @click="emit('scan-python')">⟳ 扫描本机</button>
          </span>
          <button class="btn btn-primary env-save" :disabled="savingEnv === 'py'" @click="emit('save-env', 'py')">{{ savingEnv === 'py' ? '保存中…' : '保存' }}</button>
        </div>
      </div>
      <div class="group-hint">Python 工具可在「编辑 → Python 版本」中选择这里配置的版本；未选择则用系统默认 python3。</div>

      <div class="group-title">快捷键</div>
      <div class="group">
        <div class="row">
          <div class="rinfo"><div class="rlabel">呼出 / 隐藏面板</div><div class="rsub">全局热键，任何界面按下即生效</div></div>
          <div class="kbd-chips">
            <span v-for="(k, i) in hotkeyChips" :key="i">{{ k }}</span>
          </div>
        </div>
      </div>

      <div class="group-title">收录</div>
      <div class="group">
        <div class="row">
          <div class="rinfo"><div class="rlabel">拖拽收录</div><div class="rsub">把 .app / .jar / .py / .sh 拖进面板</div></div>
          <span class="state-dot on"></span>
        </div>
        <div class="row">
          <div class="rinfo"><div class="rlabel">真实图标</div><div class="rsub">自动提取 .app 原版图标并缓存</div></div>
          <span class="state-dot on"></span>
        </div>
      </div>

      <div class="group-title">数据</div>
      <div class="group">
        <div class="row">
          <div class="rinfo"><div class="rlabel">导出备份</div><div class="rsub">工具、分类、设置与图标打包为 zip</div></div>
          <button class="btn btn-ghost env-save" @click="emit('export-backup')">导出</button>
        </div>
        <div class="row">
          <div class="rinfo"><div class="rlabel">导入还原</div><div class="rsub">从备份包恢复，会覆盖当前全部数据</div></div>
          <button class="btn btn-danger env-save" @click="emit('import-backup')">{{ confirmImport ? '确认还原？' : '导入' }}</button>
        </div>
      </div>
      <div class="settings-foot">Armory v0.1.0 · 个人专用 · 数据存储于 ~/.armory</div>
    </div>
  </div>
</template>

<style scoped>
/* 环境列表底栏与 .env-grid 的左右内边距(12px)对齐，避免按钮贴到卡片边缘；
   底部留 10px 使内容与圆角卡片下沿有呼吸，呈现与原版一致的观感。 */
.env-foot {
  padding: 0 12px 10px;
}
</style>
