import { ref, onMounted, onUnmounted } from 'vue'
import { useDroneStore } from '@/stores/drones'

export function useRemoteIdWS() {
  const droneStore = useDroneStore()
  const ws = ref(null)
  const isConnected = ref(false)
  const wsUrl = import.meta.env.VITE_WS_URL || 'ws://localhost:8080/ws'

  const connect = () => {
    ws.value = new WebSocket(wsUrl)

    ws.value.onopen = () => {
      console.log('[WS] 连接成功')
      isConnected.value = true
    }

    ws.value.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data)
        if (message.type === 'drone_update') {
          droneStore.updateDrone(message)
        }
      } catch (error) {
        console.error('[WS] 消息解析失败:', error)
      }
    }

    ws.value.onclose = () => {
      console.warn('[WS] 连接关闭，3秒后重连...')
      isConnected.value = false
      setTimeout(connect, 3000)
    }

    ws.value.onerror = (err) => {
      console.error('[WS] 连接错误:', err)
      ws.value.close()
    }
  }

  const disconnect = () => {
    if (ws.value) {
      ws.value.close()
    }
  }

  onMounted(() => connect())
  onUnmounted(() => disconnect())

  return { isConnected }
}
