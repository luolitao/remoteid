/**
 * 通用工具函数 - 集中管理避免重复
 */

/**
 * 格式化时间戳为相对时间
 * @param {string|Date} timestamp
 * @returns {string}
 */
export const timeAgo = (timestamp) => {
  if (!timestamp) return 'Unknown'
  const now = new Date()
  const then = new Date(timestamp)
  if (isNaN(then.getTime())) return 'Invalid time'

  const diff = Math.floor((now - then) / 1000)
  if (diff < 60) return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

/**
 * 检查时间戳是否在最近 n 分钟内
 * @param {string|Date} timestamp
 * @param {number} minutes - 默认 5 分钟
 * @returns {boolean}
 */
export const isRecent = (timestamp, minutes = 5) => {
  if (!timestamp) return false
  const now = new Date()
  const then = new Date(timestamp)
  return now - then < minutes * 60000
}

/**
 * 格式化时间为本地时间字符串
 * @param {string|Date} timestamp
 * @returns {string}
 */
export const formatTime = (timestamp) => {
  if (!timestamp) return 'N/A'
  return new Date(timestamp).toLocaleTimeString()
}

/**
 * 格式化坐标为 6 位小数
 * @param {number} value
 * @returns {string}
 */
export const formatCoord = (value) => {
  if (value === null || value === undefined) return 'N/A'
  return Number(value).toFixed(6)
}

/**
 * 缩短 MAC 地址显示
 * @param {string} mac
 * @param {boolean} [compact=false] - 是否紧凑格式（去掉冒号）
 * @returns {string}
 */
export const shortenMac = (mac, compact = false) => {
  if (!mac) return 'Unknown'
  if (compact) return mac.replace(/:/g, '').toUpperCase()
  const parts = mac.split(':')
  return `${parts[0]}:${parts[1]}:${parts[2]}:...:${parts[4]}:${parts[5]}`
}

/**
 * 下载 Blob 为文件
 * @param {Blob} blob
 * @param {string} filename
 */
export const downloadBlob = (blob, filename) => {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.style.visibility = 'hidden'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
