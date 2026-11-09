<script setup>
// 添加工具对话框（与运行版一致）：
//   名称 / 路径 / 启动参数 / 所属分类（子级显示"父 / 子"，标题加 ◇ 前缀）
//   + JVM 参数（jar 时出现，拼在 -jar 之前）
//   + 运行版本区（jar/py 时出现）：「是否采用全局设置」开关 + 全局版本下拉。
// 提交前校验：路径必填；启用全局设置时必须已配置版本且已选择。
import { reactive, ref, computed } from 'vue'
import { detectType } from '../composables/useToolType'
import { buildCatOptions } from '../composables/useCatOptions'

const props = defineProps({
  categories: { type: Array, required: true },
  presetCategory: { type: String, default: '' }, // 打开时预选的分类（当前停在分组页时传入）
  javaVersions: { type: Array, required: true },
  pythonVersions: { type: Array, required: true },
})
const emit = defineEmits(['close', 'submit', 'toast'])

const form = reactive({ name: '', path: '', args: '', jvmArgs: '', categoryId: props.presetCategory || '' })
const useGlobal = ref(false)
const globalVer = ref('')
const addType = computed(() => detectType(form.path))
const isRunnable = computed(() => addType.value === 'jar' || addType.value === 'py')

// 分类下拉：树序 + 子级全路径「父 / 子」+ 标题 ◇ 前缀（共享 buildCatOptions）。
const catOptions = computed(() => buildCatOptions(props.categories))

const versions = computed(() => (addType.value === 'jar' ? props.javaVersions : props.pythonVersions))
const envHint = computed(() =>
  useGlobal.value
    ? `使用「设置 → ${addType.value === 'jar' ? 'Java 环境' : 'Python 环境'}」中配置的版本`
    : '使用系统默认解释器'
)

function submit() {
  if (!form.path.trim()) {
    emit('toast', '路径不能为空')
    return
  }
  // 防呆：JVM 参数是拼在 -jar 之前的选项，写 -jar 会启动命令出现两个 -jar（进程瞬间退出）。
  if (/(^|\s)-jar(\s|$)/.test(form.jvmArgs.trim())) {
    emit('toast', 'JVM 参数里不要写 -jar：程序 jar 由路径字段自动拼接，请把 -jar 之后的内容移到「启动参数」')
    return
  }
  if (useGlobal.value) {
    if (!versions.value.length) {
      emit('toast', '请先在「设置 → 运行环境」中配置版本')
      return
    }
    if (!globalVer.value) {
      emit('toast', '请选择一个全局版本')
      return
    }
  }
  emit('submit', { ...form, envVersion: useGlobal.value ? globalVer.value : '' })
}
</script>

<template>
  <div class="settings-mask" @click="emit('close')"></div>
  <div class="settings-dialog" role="dialog" aria-label="添加工具">
    <div class="settings-head">
      <div class="dlg-title"><span class="dlg-ico">＋</span>添加工具</div>
      <button class="iconbtn" data-tip="关闭" title="关闭" @click="emit('close')">×</button>
    </div>
    <div class="settings-body">
      <div class="form-row"><label>工具名称</label><input v-model="form.name" placeholder="选填，留空自动识别" @keydown.enter="submit" /></div>
      <div class="form-row"><label>路径</label><input v-model="form.path" placeholder="必填，如 /Applications/X.app 或 ~/Tools/x.jar" @keydown.enter="submit" /></div>
      <div v-if="addType === 'jar'" class="form-row">
        <label>JVM 参数</label><input v-model="form.jvmArgs" placeholder="选填，如 -noverify -javaagent:agent.jar（拼在 -jar 之前）" @keydown.enter="submit" />
      </div>
      <div class="form-row">
        <label>启动参数</label><input v-model="form.args" :placeholder="addType === 'jar' ? '选填，传给程序本身（在 -jar 之后）' : '选填，如 -sV -p 1-65535'" @keydown.enter="submit" />
      </div>
      <div class="form-row">
        <label>所属分类</label>
        <select v-model="form.categoryId">
          <optgroup v-for="g in catOptions" :key="g.id" :label="g.name">
            <option :value="g.id">{{ g.name }}</option>
            <option v-for="ch in g.children" :key="ch.id" :value="ch.id">{{ ch.name }}</option>
          </optgroup>
        </select>
      </div>
      <div v-if="isRunnable" class="form-row">
        <label>运行版本</label>
        <div class="switch-row">
          <label class="switch">
            <input v-model="useGlobal" type="checkbox" />
            <span class="track"><span class="thumb"></span></span>
          </label>
          <div class="min-w-0">
            <div class="switch-label">是否采用全局设置</div>
            <div class="switch-sub">{{ envHint }}</div>
          </div>
        </div>
      </div>
      <div v-if="useGlobal && isRunnable" class="form-row">
        <label>全局版本</label>
        <select v-model="globalVer">
          <option v-if="addType === 'jar' && !javaVersions.length" disabled value="">尚未在设置中配置 Java 版本</option>
          <option v-else-if="addType === 'py' && !pythonVersions.length" disabled value="">尚未在设置中配置 Python 版本</option>
          <option v-for="v in versions" :key="v.name" :value="v.name">{{ v.name }}</option>
        </select>
      </div>
      <div class="form-hint">也支持把文件直接拖进面板收录；路径填 http(s):// 可收录网页工具（默认浏览器打开）</div>
    </div>
    <div class="dlg-actions">
      <button class="btn btn-ghost" @click="emit('close')">取消</button>
      <button class="btn btn-primary" @click="submit">添加工具</button>
    </div>
  </div>
</template>
