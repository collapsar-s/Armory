<script setup>
// 编辑工具对话框：名称/路径/参数/分类/env 版本/说明文档 + 两步删除确认。
// 删除与原版一致为两步：首次点击布防（3 秒窗口）变为「确认删除？」，再次点击才执行。
import { reactive, ref, computed, watch, onBeforeUnmount } from 'vue'
import { typeEmojis } from '../composables/useApi'
import { buildCatOptions } from '../composables/useCatOptions'

const props = defineProps({
  tool: { type: Object, required: true }, // { id, type, envVersion, name, path, args, jvmArgs, tags, categoryId }
  categories: { type: Array, required: true },
  javaVersions: { type: Array, required: true },
  pythonVersions: { type: Array, required: true },
  allTags: { type: Array, default: () => [] }, // 全库已有标签（输入补全）
  icon: { type: String, default: '' }, // 已拼好前缀的 data URL（可空）
  readme: { type: String, default: '' },
})
const emit = defineEmits(['close', 'save', 'delete', 'toast'])

const form = reactive({ name: '', path: '', args: '', jvmArgs: '', categoryId: '', tags: [], term: false, workingDir: '' })
const tagInput = ref('')
// 分类下拉与添加弹窗同源：树序 + 子级全路径（否则子分类看起来像与主分组同级）。
const catOptions = computed(() => buildCatOptions(props.categories))
function addTag() {
  const tag = tagInput.value.trim()
  if (tag && !form.tags.includes(tag)) form.tags.push(tag)
  tagInput.value = ''
}
const envVersion = ref('')
const editType = ref('')
const deleteArmed = ref(false)
let armTimer = null

// 运行统计（只读展示）：从未运行过则不显示。
const runStat = computed(() => {
  const t = props.tool
  if (!t?.runCount) return ''
  const last = t.lastRunAt ? ` · 最近 ${new Date(t.lastRunAt).toLocaleString('zh-CN', { hour12: false })}` : ''
  return `已运行 ${t.runCount} 次${last}`
})

watch(
  () => props.tool,
  (t) => {
    if (!t) return
    Object.assign(form, { name: t.name, path: t.path, args: t.args, jvmArgs: t.jvmArgs, categoryId: t.categoryId, tags: [...(t.tags || [])], term: !!t.term, workingDir: t.workingDir || '' })
    envVersion.value = t.envVersion || ''
    editType.value = t.type
    disarm()
  },
  { immediate: true }
)

function disarm() {
  deleteArmed.value = false
  clearTimeout(armTimer)
}
onBeforeUnmount(disarm)
function onDelete() {
  // 与原版一致：首次点击布防，3 秒内再点执行。
  if (!deleteArmed.value) {
    deleteArmed.value = true
    armTimer = setTimeout(() => (deleteArmed.value = false), 3000)
    return
  }
  disarm()
  emit('delete', props.tool.id)
}

function save() {
  // 防呆：JVM 参数是拼在 -jar 之前的选项，写 -jar 会导致启动命令出现两个 -jar（进程瞬间退出）。
  if (editType.value === 'jar' && /(^|\s)-jar(\s|$)/.test(form.jvmArgs.trim())) {
    emit('toast', 'JVM 参数里不要写 -jar：程序 jar 由路径字段自动拼接，请把 -jar 之后的内容移到「启动参数」')
    return
  }
  emit('save', {
    id: props.tool.id,
    name: form.name,
    path: form.path,
    args: form.args,
    jvmArgs: form.jvmArgs,
    tags: [...form.tags],
    term: form.term,
    categoryId: form.categoryId,
    envVersion: envVersion.value,
    workingDir: form.workingDir,
  })
}
</script>

<template>
  <div class="settings-mask" @click="emit('close')"></div>
  <div class="settings-dialog edit-dialog" role="dialog" aria-label="编辑工具">
    <div class="settings-head">
      <div class="dlg-title">
        <span class="edit-ico">
          <img v-if="icon" :src="icon" :alt="form.name" />
          <template v-else>{{ typeEmojis[editType] }}</template>
        </span> 编辑工具
      </div>
      <button class="iconbtn" data-tip="关闭" title="关闭" @click="emit('close')">×</button>
    </div>
    <div class="settings-body edit-body">
      <div class="form-row"><label>工具名称</label><input v-model="form.name" @keydown.enter="save" /></div>
      <div class="form-row"><label>路径</label><input v-model="form.path" @keydown.enter="save" /></div>
      <div v-if="editType === 'jar'" class="form-row">
        <label>Java 版本</label>
        <select v-model="envVersion">
          <option value="">自动（系统默认）</option>
          <option v-for="v in javaVersions" :key="v.name" :value="v.name">{{ v.name }}</option>
        </select>
      </div>
      <div v-if="editType === 'jar'" class="form-row">
        <label>JVM 参数</label>
        <input v-model="form.jvmArgs" placeholder="选填，如 -noverify -javaagent:agent.jar（拼在 -jar 之前）" @keydown.enter="save" />
      </div>
      <div class="form-row">
        <label>启动参数</label>
        <input v-model="form.args" :placeholder="editType === 'jar' ? '选填，传给程序本身（在 -jar 之后）' : '选填，如 -sV -p 1-65535'" @keydown.enter="save" />
      </div>
      <div v-if="['jar', 'bin', 'sh', 'py'].includes(editType)" class="form-row">
        <label>工作目录</label>
        <input v-model="form.workingDir" placeholder="选填，留空默认工具所在目录" @keydown.enter="save" />
      </div>
      <div v-if="['bin', 'sh', 'py'].includes(editType)" class="form-row">
        <label>终端运行</label>
        <div class="switch-row">
          <label class="switch">
            <input v-model="form.term" type="checkbox" />
            <span class="track"><span class="thumb"></span></span>
          </label>
          <div class="min-w-0">
            <div class="switch-label">在系统终端新窗口中运行</div>
            <div class="switch-sub">CLI 工具勾选后可见输出与交互（自动 cd 到工具目录）</div>
          </div>
        </div>
      </div>
      <div class="form-row">
        <label>标签</label>
        <div class="tags-edit">
          <span v-for="(tag, i) in form.tags" :key="tag" class="tag-chip">
            {{ tag }}
            <button class="tag-x" title="移除" @click.stop="form.tags.splice(i, 1)">×</button>
          </span>
          <input
            v-model="tagInput"
            list="armory-all-tags"
            placeholder="回车添加标签"
            @keydown.enter.prevent="addTag"
          />
          <datalist id="armory-all-tags">
            <option v-for="t in allTags" :key="t" :value="t" />
          </datalist>
        </div>
      </div>
      <div v-if="editType === 'py'" class="form-row">
        <label>Python 版本</label>
        <select v-model="envVersion">
          <option value="">自动（系统默认）</option>
          <option v-for="v in pythonVersions" :key="v.name" :value="v.name">{{ v.name }}</option>
        </select>
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
      <div v-if="runStat" class="form-hint" style="padding: 8px 2px 2px 84px">{{ runStat }}</div>
      <div v-if="readme" class="readme-box">
        <div class="readme-title">说明文档</div>
        <div class="readme-body" v-html="readme"></div>
      </div>
    </div>
    <div class="dlg-actions">
      <button class="btn btn-danger" :class="{ arm: deleteArmed }" @click="onDelete">{{ deleteArmed ? '确认删除？' : '删除' }}</button>
      <button class="btn btn-ghost" @click="emit('close')">取消</button>
      <button class="btn btn-primary" @click="save">保存</button>
    </div>
  </div>
</template>
