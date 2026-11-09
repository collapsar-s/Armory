<script setup>
// 分类对话框（Maye 式布局版）：
//   - 从侧栏「＋」进入（无预设）：「所属层级」下拉 = 「主分组（侧栏顶层）」+ 每个主分组一项
//     「X 内的分组」，父级由下拉选项直接携带。
//   - 从分节头/主分组右键「新建分组」进入（有预设）：显示禁用的「类型」输入框。
//   - 名称 label：新建=分类名 / 编辑=重命名；placeholder 按有无父级区分。
//   - 编辑态删除为两步确认（确认删除？，3 秒窗口）。
//   - 说明：布局改为「侧栏只放主分组、子分组页内分节」后，新建统一为「分组」（dir）；
//     旧数据的「标题」（title）仍正常显示为 ◇ 分节，但不再提供新建入口。
import { ref, computed, onBeforeUnmount } from 'vue'

const props = defineProps({
  mode: { type: String, default: 'catadd' }, // catadd | catedit
  category: { type: Object, default: null }, // 编辑模式: { id, name, parentId, kind }
  modules: { type: Array, default: () => [] }, // 一级目录 [{id,name}]（供层级下拉）
  presetLevel: { type: String, default: '' }, // 右键预设：'' | 'title' | 'sub'
  presetParent: { type: String, default: '' }, // 预设/预选父级 id
  presetArmed: { type: Boolean, default: false }, // 编辑态删除预武装（右键「删除」进入）
  catPath: { type: Function, required: true }, // 分类完整路径函数（"父 / 子"）
})
const emit = defineEmits(['close', 'submit', 'delete'])

const catMode = computed(() => props.mode)
const preset = computed(() => catMode.value === 'catadd' && !!props.presetLevel)

const parent = ref(props.presetParent || '')
const nameInput = ref(catMode.value === 'catedit' && props.category ? props.category.name : '')

const dlgTitle = computed(() => {
  if (catMode.value === 'catedit') return '分类管理'
  return parent.value || preset.value ? '新建分组' : '新建主分组'
})
const nameLabel = computed(() => (catMode.value === 'catedit' ? '重命名' : '分类名'))
const placeholder = computed(() => {
  if (catMode.value !== 'catadd') return ''
  return parent.value ? '例如：Spring 系' : '例如：内网渗透'
})
const presetTypeText = computed(() => '分组 · 位于 ' + props.catPath(parent.value))
// 提示跟随所选层级变化：主分组讲侧栏与独立页面；分组讲页内分节。
const levelHint = computed(() => {
  if (catMode.value !== 'catadd') return ''
  if (parent.value) {
    const m = props.modules.find((x) => x.id === parent.value)
    return `分组显示在「${m?.name ?? ''}」页面内，作为分节收纳工具（可折叠、可拖入工具）；分组下不再嵌套，侧栏只显示主分组。`
  }
  return '主分组显示在侧栏，是独立的工具页面；可在其页面内再建分组作为分节。'
})

// ---- 两步删除 ----
// presetArmed=true（右键「删除」进入）时按钮初始即为「确认删除？」状态，
// 3 秒窗口内再点执行——与原版 z(e, !0) 语义一致。
const deleteArmed = ref(catMode.value === 'catedit' && props.presetArmed)
let armTimer = null
onBeforeUnmount(() => clearTimeout(armTimer))
function onDelete() {
  if (catMode.value !== 'catedit' || !props.category?.id) return
  if (!deleteArmed.value) {
    deleteArmed.value = true
    armTimer = setTimeout(() => (deleteArmed.value = false), 3000)
    return
  }
  deleteArmed.value = false
  clearTimeout(armTimer)
  emit('delete', props.category.id)
}

function submit() {
  if (catMode.value === 'catadd') {
    const level = preset.value ? props.presetLevel : parent.value ? 'sub' : 'dir'
    emit('submit', {
      mode: 'catadd',
      level,
      name: nameInput.value,
      parentId: level === 'dir' ? '' : parent.value,
    })
  } else {
    emit('submit', {
      mode: 'catedit',
      id: props.category?.id,
      name: nameInput.value,
      originalName: props.category?.name ?? '',
    })
  }
}
</script>

<template>
  <div class="settings-mask" @click="emit('close')"></div>
  <div class="settings-dialog cat-dialog" role="dialog" aria-label="分类">
    <div class="settings-head">
      <div class="dlg-title">
        <span class="dlg-ico">{{ catMode === 'catadd' ? '＋' : '✎' }}</span>
        {{ dlgTitle }}
      </div>
      <button class="iconbtn" data-tip="关闭" title="关闭" @click="emit('close')">×</button>
    </div>
    <div class="settings-body cat-body">
      <!-- 无预设：层级下拉（主分组 / X 内的分组） -->
      <div v-if="catMode === 'catadd' && !preset" class="form-row">
        <label>所属层级</label>
        <select v-model="parent">
          <option value="">主分组（侧栏顶层）</option>
          <option v-for="m in modules" :key="m.id" :value="m.id">「{{ m.name }}」内的分组</option>
        </select>
      </div>
      <!-- 有预设（右键进入）：只读类型展示 -->
      <div v-else-if="catMode === 'catadd' && preset" class="form-row">
        <label>类型</label>
        <input :value="presetTypeText" disabled />
      </div>
      <div v-if="levelHint" class="form-hint" style="padding: 2px 2px 2px 84px">
        {{ levelHint }}
      </div>
      <div class="form-row">
        <label>{{ nameLabel }}</label>
        <input v-model="nameInput" :placeholder="placeholder" @keydown.enter="submit" />
      </div>
      <div v-if="catMode === 'catedit'" class="form-hint" style="padding: 8px 2px 2px 84px">
        修改后保存立即生效；删除后，其中的工具会自动归入上级分类或其他模块
      </div>
    </div>
    <div class="dlg-actions">
      <button v-if="catMode === 'catedit'" class="btn btn-danger" :class="{ arm: deleteArmed }" @click="onDelete">
        {{ deleteArmed ? '确认删除？' : '删除' }}
      </button>
      <button class="btn btn-ghost" @click="emit('close')">取消</button>
      <button class="btn btn-primary" @click="submit">{{ catMode === 'catadd' ? '创建' : '保存' }}</button>
    </div>
  </div>
</template>
