<template>
  <div 
    id="splitter" 
    @mousedown="startResize"
    title="Drag to resize"
  ></div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  width: { type: Number, default: 350 }
})
const emit = defineEmits(['update:width'])

let isResizing = false

const startResize = (e) => {
  isResizing = true
  document.body.style.cursor = 'ew-resize'
  document.body.style.userSelect = 'none' // 防止拖拽时选中文本
  
  document.addEventListener('mousemove', doResize)
  document.addEventListener('mouseup', stopResize)
  e.preventDefault()
}

const doResize = (e) => {
  if (!isResizing) return
  // 限制侧边栏宽度在 250px 到 600px 之间
  // 注意：因为侧边栏在右侧，所以宽度 = 窗口总宽 - 鼠标X坐标
  const newWidth = Math.max(250, Math.min(600, window.innerWidth - e.clientX))
  emit('update:width', newWidth)
}

const stopResize = () => {
  isResizing = false
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
  document.removeEventListener('mousemove', doResize)
  document.removeEventListener('mouseup', stopResize)
}
</script>

<style scoped>
#splitter {
  flex-shrink: 0;
  width: 4px;
  background: var(--BGCOLOR2, #ccc);
  cursor: ew-resize;
  z-index: 50;
  transition: background 0.2s;
}
#splitter:hover {
  background: var(--ACCENT, #3182ce);
}
</style>