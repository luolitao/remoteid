/**
 * Leaflet icon path fix + custom drone marker icons
 */
import * as L from 'leaflet'
import iconRetinaUrl from 'leaflet/dist/images/marker-icon-2x.png'
import iconUrl from 'leaflet/dist/images/marker-icon.png'
import shadowUrl from 'leaflet/dist/images/marker-shadow.png'

let _initialized = false

/**
 * Fix Leaflet default icon paths (runs once).
 */
export const setupLeafletIcons = () => {
  if (_initialized) return
  delete L.Icon.Default.prototype._getIconUrl
  L.Icon.Default.mergeOptions({
    iconRetinaUrl,
    iconUrl,
    shadowUrl
  })
  _initialized = true
}

/**
 * Color palette matching the dark theme semantic colors.
 */
const DRONE_COLORS = {
  red:    { bg: '#ff4d6d', glow: '#ff4d6d', border: '#cc1a3d' },
  green:  { bg: '#19d27a', glow: '#19d27a', border: '#0e9d5a' },
  blue:   { bg: '#56a8ff', glow: '#56a8ff', border: '#2e8ae6' },
  orange: { bg: '#ff9633', glow: '#ff9633', border: '#e67a1a' },
  yellow: { bg: '#fbcc4a', glow: '#fbcc4a', border: '#e0b02a' },
}

/**
 * Create a custom DivIcon for drone markers.
 * @param {'red'|'green'|'blue'|'orange'|'yellow'} color
 * @param {number} [size=12] - icon pixel size
 * @param {boolean} [pulse=false] - add glow pulse animation
 * @returns {L.DivIcon}
 */
export const createDroneIcon = (color = 'red', size = 12, pulse = false) => {
  const c = DRONE_COLORS[color] || DRONE_COLORS.red
  const glow = pulse
    ? `box-shadow:0 0 ${size}px ${c.glow};animation:dronePulse 1.6s ease-in-out infinite;`
    : `box-shadow:0 0 ${Math.max(size/2, 4)}px ${c.glow};`

  const style = `
    width:${size}px;height:${size}px;
    border-radius:50%;
    background:${c.bg};
    border:2px solid #fff;
    ${glow}
  `

  return L.divIcon({
    html: `<div style="${style}"></div>`,
    className: '',
    iconSize: [size, size],
    iconAnchor: [size / 2, size / 2],
  })
}

// Inject keyframe animation once
if (typeof document !== 'undefined') {
  const styleId = 'drone-icon-keyframes'
  if (!document.getElementById(styleId)) {
    const style = document.createElement('style')
    style.id = styleId
    style.textContent = `
      @keyframes dronePulse {
        0%, 100% { opacity: 1; transform: scale(1); }
        50%      { opacity: .6; transform: scale(1.15); }
      }
    `
    document.head.appendChild(style)
  }
}
