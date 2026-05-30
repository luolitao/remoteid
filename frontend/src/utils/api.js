/**
 * API 配置与基础请求 - 集中管理避免硬编码
 */
import axios from 'axios'
import logger from '@/utils/logger'

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api'

// 创建带默认配置的 axios 实例
const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' }
})

/**
 * 获取活跃无人机列表
 * @returns {Promise<Array>}
 */
export const fetchActiveDrones = async () => {
  const response = await apiClient.get('/drones/')
  return response.data.drones || response.data || []
}

/**
 * 获取无人机详情
 * @param {string} mac
 * @returns {Promise<Object>}
 */
export const fetchDroneDetail = async (mac) => {
  const response = await apiClient.get(`/drones/${mac}/`)
  return response.data
}

/**
 * 获取无人机轨迹数据
 * @param {string} mac
 * @returns {Promise<Array>}
 */
export const fetchDroneTrajectory = async (mac) => {
  const response = await apiClient.get(`/drones/${mac}/trajectory`)
  const data = response.data
  return data?.trajectory?.points || data?.points || []
}

/**
 * 获取无人机导出数据
 * @param {string} mac
 * @returns {Promise<Object>}
 */
export const fetchDroneExport = async (mac) => {
  const response = await apiClient.get(`/drones/${mac}/export`)
  return response.data
}

/**
 * 获取警报列表
 * @param {number} [limit=10]
 * @returns {Promise<Array>}
 */
export const fetchAlerts = async (limit = 10) => {
  const response = await apiClient.get(`/alerts/?limit=${limit}`)
  // 后端可能返回 { alerts: [...] } 或直接返回数组
  return response.data.alerts || response.data || []
}

/**
 * 连接 WebSocket
 * @param {Object} callbacks - { onDroneUpdate, onClose, onError }
 * @returns {WebSocket}
 */
export const connectWebSocket = (callbacks = {}) => {
  const apiUrl = import.meta.env.VITE_API_URL
  let wsUrl
  if (apiUrl) {
    wsUrl = apiUrl
      .replace(/^http/, 'ws')
      .replace(/\/api\/?$/, '')
  } else {
    // 开发模式下使用当前页面的 host（通过 Vite proxy）
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    wsUrl = `${protocol}//${window.location.host}`
  }

  const ws = new WebSocket(`${wsUrl}/ws`)

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data.type === 'drone_update' && callbacks.onDroneUpdate) {
        callbacks.onDroneUpdate(data.mac, data.data)
      }
    } catch (e) {
      logger.error('WebSocket message parse error:', e)
    }
  }

  ws.onclose = () => {
    logger.info('WebSocket disconnected, reconnecting in 3s...')
    setTimeout(() => connectWebSocket(callbacks), 3000)
  }

  ws.onerror = (error) => {
    logger.error('WebSocket error:', error)
    callbacks.onError?.(error)
  }

  return ws
}

/**
 * 将无人机数据转换为 CSV
 * @param {Array} dronesData - [{drone, positions}]
 * @returns {string}
 */
export const dronesToCSV = (dronesData) => {
  const headers = 'MAC,UAS_ID,UA_Type,Latitude,Longitude,Altitude,Speed,Heading,Timestamp\n'
  const rows = dronesData.flatMap(({ drone, positions }) =>
    (positions || []).map(pos =>
      `"${drone.mac}","${drone.uas_id}","${drone.ua_type}",${pos.latitude},${pos.longitude},${pos.altitude},${pos.speed},${pos.heading},"${pos.timestamp}"`
    )
  )
  return headers + rows.join('\n')
}

export { API_BASE_URL, apiClient }
