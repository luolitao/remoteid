/**
 * Leaflet 图标配置 — tar1090 风格按数据源/状态着色
 */
import * as L from 'leaflet'
import iconRetinaUrl from 'leaflet/dist/images/marker-icon-2x.png'
import iconUrl from 'leaflet/dist/images/marker-icon.png'
import shadowUrl from 'leaflet/dist/images/marker-shadow.png'

let _initialized = false

export const setupLeafletIcons = () => {
  if (_initialized) return
  delete L.Icon.Default.prototype._getIconUrl
  L.Icon.Default.mergeOptions({ iconRetinaUrl, iconUrl, shadowUrl })
  _initialized = true
}

/**
 * 创建无人机标记图标 — tar1090 按数据源着色风格
 * 绿色 = 合规/确认
 * 蓝色 = ASTM 标准
 * 红色 = 不合规/警告
 * 黄色 = 未知/待确认
 */
export const createDroneIcon = (color = 'green', size = 12) => {
  const palette = {
    green: { bg: '#19d27a', border: '#0f9d58' },
    blue: { bg: '#56a8ff', border: '#3182ce' },
    red: { bg: '#ff4d6d', border: '#cc0000' },
    yellow: { bg: '#fbcc4a', border: '#d4a017' },
    orange: { bg: '#ff9633', border: '#cc6600' },
    gray: { bg: '#999', border: '#666' },
  }
  const c = palette[color] || palette.green

  return L.divIcon({
    html: `<div style="
      width: ${size}px; height: ${size}px;
      background: ${c.bg};
      border: 2px solid ${c.border};
      border-radius: 50%;
      box-shadow: 0 0 4px rgba(0,0,0,.3);
    "></div>`,
    className: '',
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2],
  })
}

/**
 * 创建飞机方向标记（三角形，带方向指示）
 */
export const createDirectionIcon = (heading = 0, color = 'blue', size = 14) => {
  const palette = {
    green: '#19d27a',
    blue: '#56a8ff',
    red: '#ff4d6d',
    yellow: '#fbcc4a',
  }
  const fill = palette[color] || palette.blue

  return L.divIcon({
    html: `<div style="
      width: 0; height: 0;
      border-left: ${size / 2}px solid transparent;
      border-right: ${size / 2}px solid transparent;
      border-bottom: ${size}px solid ${fill};
      transform: rotate(${heading}deg);
      filter: drop-shadow(0 1px 2px rgba(0,0,0,.3));
    "></div>`,
    className: '',
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2],
  })
}
