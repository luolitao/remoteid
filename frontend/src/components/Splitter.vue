<template>
  <div id="splitter" @mousedown="startResize" title="Drag to resize"></div>
</template>

<script setup>
// 接收父组件传来的当前侧边栏宽度
const props = defineProps({
  width: { type: Number, default: 350 }
})

// 定义 update:width 事件，用于 v-model 双向绑定
const emit = defineEmits(['update:width'])

let isResizing = false

const startResize = (e) => {
  isResizing = true
  // 🎯 核心修复：拖拽时禁用文本选中，并固定鼠标指针
  // 如果不加这两行，鼠标稍微移动一点就会选中文字，导致浏览器中断拖拽逻辑
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'ew-resize'
  
  // 🎯 核心修复：事件必须绑定到 document，确保鼠标移出分割条区域后依然能响应
  document.addEventListener('mousemove', doResize)
  document.addEventListener('mouseup', stopResize)
  e.preventDefault() // 阻止默认拖拽行为
}

const doResize = (e) => {
  if (!isResizing) return
  
  // 计算新宽度：屏幕总宽 - 鼠标X坐标 - Splitter自身宽度(6px)
  // 限制宽度在 250px 到 600px 之间
  const newWidth = Math.max(250, Math.min(600, window.innerWidth - e.clientX - 6))
  
  emit('update:width', newWidth) // 通知父组件更新
}

const stopResize = () => {
  isResizing = false
  // 恢复文本选中和鼠标指针
  document.body.style.userSelect = ''
  document.body.style.cursor = ''
  
  document.removeEventListener('mousemove', doResize)
  document.removeEventListener('mouseup', stopResize)
}
</script>

<style scoped>
#splitter {
  flex-shrink: 0;
  width: 6px; /* 稍微加宽一点，更容易抓取 */
  background: var(--BGCOLOR2, #ccc);
  cursor: ew-resize;
  position: relative;
  z-index: 1000; /* 关键：确保层级高于地图，防止被遮挡 */
  transition: background 0.2s;
}

#splitter:hover {
  background: var(--ACCENT, #3182ce); /* 悬停高亮 */
}
</style>