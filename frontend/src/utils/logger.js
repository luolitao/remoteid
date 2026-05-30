/**
 * 统一日志工具
 * 生产环境通过环境变量控制日志级别，避免泄露调试信息
 */

const LOG_LEVELS = { debug: 0, info: 1, warn: 2, error: 3, silent: 4 }

const currentLevel = (() => {
  const env = import.meta.env.VITE_LOG_LEVEL
  if (env && LOG_LEVELS[env] !== undefined) return LOG_LEVELS[env]
  return import.meta.env.PROD ? LOG_LEVELS.warn : LOG_LEVELS.debug
})()

const logger = {
  debug(...args) {
    if (currentLevel <= LOG_LEVELS.debug) console.debug('[DEBUG]', ...args)
  },
  info(...args) {
    if (currentLevel <= LOG_LEVELS.info) console.log('[INFO]', ...args)
  },
  warn(...args) {
    if (currentLevel <= LOG_LEVELS.warn) console.warn('[WARN]', ...args)
  },
  error(...args) {
    if (currentLevel <= LOG_LEVELS.error) console.error('[ERROR]', ...args)
  }
}

export default logger
