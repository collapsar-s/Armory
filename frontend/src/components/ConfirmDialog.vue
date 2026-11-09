<script setup>
// 通用确认弹窗（轻量、居中、玻璃材质）——右键删除分类等破坏性操作的确认步骤。
// 与 settings-dialog 同一视觉体系（surface-1 + blur + r-card），但小而聚焦：
// 图标 + 标题 + 说明（支持 HTML 转义后的富文本行）+ 取消/确认 双按钮。
// 键盘：Enter = 确认，Esc = 取消；3.5s 无操作自动取消（不驻留打扰）。
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'

const props = defineProps({
  title: { type: String, default: '确认操作' },
  // { name, path, consequence } —— path/consequence 可为空
  target: { type: Object, default: () => ({ name: '', path: '', consequence: '' }) },
  confirmLabel: { type: String, default: '删除' },
  cancelLabel: { type: String, default: '取消' },
  danger: { type: Boolean, default: true }, // 红色警示样式
})
const emit = defineEmits(['confirm', 'cancel'])

const busy = ref(false) // 防双击：确认后置忙
const canConfirm = computed(() => !busy.value && !!props.target?.name)

function close(kind) {
  if (kind === 'confirm') {
    if (!canConfirm.value) return
    busy.value = true
    emit('confirm')
  } else {
    emit('cancel')
  }
}

function onKey(e) {
  if (e.key === 'Enter') {
    e.preventDefault()
    close('confirm')
  } else if (e.key === 'Escape') {
    e.preventDefault()
    e.stopPropagation()
    close('cancel')
  }
}
onMounted(() => window.addEventListener('keydown', onKey, true))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey, true))
</script>

<template>
  <div class="settings-mask" @click="close('cancel')"></div>
  <div class="confirm-dialog" role="alertdialog" :aria-label="title">
    <div class="confirm-icon" :class="{ danger }">!</div>
    <div class="confirm-title">{{ title }}</div>
    <div class="confirm-msg">确定删除 <b class="confirm-name">{{ target.name }}</b>？</div>
    <div v-if="target.path" class="confirm-path">{{ target.path }}</div>
    <div v-if="target.consequence" class="confirm-sub">{{ target.consequence }}</div>
    <div class="confirm-actions">
      <button class="btn btn-ghost" @click="close('cancel')">{{ cancelLabel }}</button>
      <button class="btn btn-danger-solid" :disabled="busy" @click="close('confirm')">
        {{ confirmLabel }}
      </button>
    </div>
  </div>
</template>
