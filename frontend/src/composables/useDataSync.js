import { useDroneStore } from '@/stores/drones'
import logger from '@/utils/logger'

export function useDataSync() {
  const store = useDroneStore()
  let pollInterval = null

  const startSync = async () => {
    await refreshData()
    pollInterval = setInterval(refreshData, 5000)
  }

  const refreshData = async () => {
    try {
      await store.loadActiveDrones()
      store.cleanStaleDrones()
      await store.loadAlerts()
      // 🎯 核心：数据进入 Store 后，MapArea 和 Sidebar 的 watch/computed 会自动响应
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
