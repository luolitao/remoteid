/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // 深色主题层次
        'void':      '#08090c',
        'surface-1': '#0e1117',
        'surface-2': '#131820',
        'surface-3': '#181f29',
        'hairline':  '#1a2027',
        'hairline-strong': '#232b35',
        // 前景色
        'fg-hi':   '#e8eef7',
        'fg-mid':  '#b3becf',
        'fg-mute': '#687586',
        'fg-dim':  '#424c5a',
        // 语义色
        'accent':     '#56a8ff',
        'confirmed':  '#19d27a',
        'phantom':    '#ff9633',
        'deception':  '#ff4d6d',
        'suspect':    '#fbcc4a',
        // 保留原有颜色
        'drone-red': '#e53e3e',
        'drone-blue': '#3182ce',
        'drone-green': '#38a169',
        'drone-yellow': '#dd6b20'
      },
      fontFamily: {
        'sans': ['-apple-system', 'system-ui', 'Inter', 'Segoe UI', 'sans-serif'],
        'mono': ['ui-monospace', 'JetBrains Mono', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'pop': 'pop 280ms cubic-bezier(.2,.7,.2,1)',
        'flash': 'flash 380ms ease-out',
        'float-up': 'floatUp 120ms ease',
      },
      keyframes: {
        pop: {
          from: { opacity: '0', transform: 'translateY(-4px)' },
          to:   { opacity: '1', transform: 'translateY(0)' },
        },
        flash: {
          '0%':   { backgroundColor: 'var(--tw-bg-surface-3)' },
          '100%': { backgroundColor: 'var(--tw-bg-surface-2)' },
        },
        floatUp: {
          from: { transform: 'translateY(0)' },
          to:   { transform: 'translateY(-1px)' },
        },
      },
    },
  },
  plugins: [],
}
