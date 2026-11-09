<script setup>
// 通用单行输入对话框：标签重命名等轻量输入场景（复用分类对话框的尺寸与样式）。
// 打开时全选默认文本，便于直接改写；回车 = 确定。
import { ref, watch, onMounted, nextTick } from 'vue'

const props = defineProps({
  title: { type: String, default: '' },
  label: { type: String, default: '' },
  value: { type: String, default: '' },
})
const emit = defineEmits(['close', 'confirm'])

const input = ref('')
watch(() => props.value, (v) => (input.value = v ?? ''), { immediate: true })
onMounted(() => nextTick(() => document.getElementById('prompt-input')?.select()))

function submit() {
  const v = input.value.trim()
  if (!v) return
  emit('confirm', v)
}
</script>

<template>
  <div class="settings-mask" @click="emit('close')"></div>
  <div class="settings-dialog cat-dialog" role="dialog" :aria-label="title">
    <div class="settings-head">
      <div class="dlg-title"><span class="dlg-ico">✎</span>{{ title }}</div>
      <button class="iconbtn" data-tip="关闭" title="关闭" @click="emit('close')">×</button>
    </div>
    <div class="settings-body cat-body">
      <div class="form-row">
        <label>{{ label }}</label>
        <input id="prompt-input" v-model="input" @keydown.enter="submit" />
      </div>
    </div>
    <div class="dlg-actions">
      <button class="btn btn-ghost" @click="emit('close')">取消</button>
      <button class="btn btn-primary" @click="submit">确定</button>
    </div>
  </div>
</template>
