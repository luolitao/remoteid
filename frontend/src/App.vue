<template>
  <div id="layout_container">
    <!-- 左侧：地图区域 -->
    <MapArea v-show="mapOpen" ref="mapAreaRef" v-model:selected-drone="selectedDrone" />

    <!-- 中间：拖拽分隔条 -->
    <Splitter v-show="sidebarOpen && mapOpen" v-model:width="sidebarWidth" />

    <!-- 右侧：侧边栏 -->
    <Sidebar
      v-show="sidebarOpen"
      v-model:selected-drone="selectedDrone"
      :initial-width="sidebarWidth"
      @show-trajectories="mapAreaRef?.showAllTrajectories()"
      @clear-trajectories="mapAreaRef?.clearTrajectories()"
    />

    <!-- ========== 隐藏状态下的恢复按钮 ========== -->
    <!-- 地图隐藏时：显示恢复地图按钮 -->
    <button
      v-if="!mapOpen"
      class="sidebarButton"
      style="position: absolute; top: 12px; left: 12px; z-index: 10000"
      title="Show map"
      @click="mapOpen = true"
    >
      🗺 Show Map
    </button>

    <!-- 侧边栏隐藏时：显示恢复侧边栏按钮 -->
    <button
      v-if="!sidebarOpen"
      class="sidebarButton"
      style="position: absolute; top: 12px; right: 12px; z-index: 10000"
      title="Show sidebar"
      @click="sidebarOpen = true"
    >
      ☰ Show Sidebar
    </button>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import MapArea from '@/components/MapArea.vue'
import Sidebar from '@/components/Sidebar.vue'
import Splitter from '@/components/Splitter.vue'
import { useDataSync } from '@/composables/useDataSync' // 见下方补充

// ---- 顶层 UI 状态 ----
const mapOpen = ref(true)
const sidebarOpen = ref(true)
const sidebarWidth = ref(350)

// ---- 跨组件共享状态 ----
// 当侧边栏或地图选中无人机时，双向绑定同步状态
const selectedDrone = ref(null)
const mapAreaRef = ref(null)
// ---- 侧边栏拖拽（增强版：防止文本选中中断） ----
let isResizing = false

const startResize = (e) => {
  isResizing = true
  // 🎯 核心修复：拖拽时禁用文本选中，并改变鼠标指针
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'ew-resize'

  document.addEventListener('mousemove', doResize)
  document.addEventListener('mouseup', stopResize)
  e.preventDefault() // 阻止默认行为
}

const doResize = (e) => {
  if (!isResizing) return
  // 计算右侧边栏宽度，并限制在 250px 到 600px 之间
  const w = Math.max(250, Math.min(600, window.innerWidth - e.clientX))
  sidebarWidth.value = w
}

const stopResize = () => {
  isResizing = false
  // 🎯 核心修复：恢复文本选中和默认鼠标指针
  document.body.style.userSelect = ''
  document.body.style.cursor = ''

  document.removeEventListener('mousemove', doResize)
  document.removeEventListener('mouseup', stopResize)
}
// ---- 初始化全局数据同步 (WebSocket + 轮询) ----
// 将原有的 refreshData 和 initWebSocket 逻辑抽离到此 composable 中
const { startSync, stopSync } = useDataSync()

onMounted(() => {
  startSync()
})

onUnmounted(() => {
  stopSync()
})
</script>

<style>
/* 仅保留最顶层的布局 CSS，其余已移至各组件内部 */
#layout_container {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background: var(--BGCOLOR1, #f5f5f5);
  position: relative;
}

.restore-btn {
  position: absolute;
  top: 12px;
  z-index: 9999;
  padding: 6px 12px;
  border-radius: 4px;
  background: var(--ACCENT, #3182ce);
  color: #fff;
  border: none;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  cursor: pointer;
  font-size: 13px;
  font-weight: bold;
  transition: opacity 0.2s;
}
.restore-btn:hover {
  opacity: 0.9;
}
/* =========================================================================
🆕 补充：通用侧边栏/地图操作按钮样式 (修复恢复按钮不可见问题)
========================================================================= */
.sidebarButton {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 6px 12px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid var(--BGCOLOR2, #ccc);
  cursor: pointer;
  color: var(--TXTCOLOR2, #666);
  font-weight: bold;
  font-size: 12px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
  transition: all 0.2s ease;
  /* 确保按钮层级最高，不被地图或侧边栏遮挡 */
  position: relative;
  z-index: 10000;
}

.sidebarButton:hover {
  background: var(--ACCENT, #3182ce);
  color: #fff;
  border-color: var(--ACCENT, #3182ce);
  transform: translateY(-1px);
}

.sidebarButton:active {
  transform: translateY(0);
}
</style>
