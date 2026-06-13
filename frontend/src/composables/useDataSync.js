// src/composables/useDataSync.js
import { useDroneStore } from '@/stores/drones'
import logger from '@/utils/logger'

export function useDataSync() {
  const store = useDroneStore()
  let pollInterval = null

  const startSync = async () => {
    await refreshData()
    pollInterval = setInterval(refreshData, 5000)
  }

  // 🎯 核心：只负责把数据拉进 Store，绝不操作地图！
  const refreshData = async () => {
    try {
      await store.loadActiveDrones()
      store.cleanStaleDrones()
      await store.loadAlerts()
      
      // ✅ 注意：这里没有任何 updateDroneMarkers 的代码！
      // 地图更新由 MapArea.vue 里的 watch 自动触发。
    } catch (e) {
      logger.error('Polling refresh error:', e)
    }
  }

  const stopSync = () => {
    if (pollInterval) {
      clearInterval(pollInterval)
      pollInterval = null
    }
  }

  return { startSync, stopSync }
}