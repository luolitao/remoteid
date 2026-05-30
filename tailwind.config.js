/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // tar1090 风格配色
        'bg1':     '#F8F8F8',
        'bg2':     '#CCCCCC',
        'accent':  '#00596b',
        'accent-hover': '#003f4b',
        'txt1':    '#003f4b',
        'txt2':    '#050505',
        'txt3':    '#101010',
        // 数据源行颜色 (tar1090 table row)
        'row-adsb':  '#d8f4ff',
        'row-mlat':  '#FDF7DD',
        'row-other': '#d8d8ff',
        'row-tisb':  '#ffd8e6',
        'row-sel':   '#88DDFF',
        // 语义色
        'drone-red':    '#e53e3e',
        'drone-blue':   '#3182ce',
        'drone-green':  '#38a169',
        'drone-yellow': '#dd6b20',
      },
      fontFamily: {
        'sans': ['"Helvetica Neue"', 'Helvetica', 'Verdana', 'sans-serif'],
        'mono': ['"Courier New"', 'monospace'],
      },
      fontSize: {
        'tar-sm':  '10px',
        'tar-base':'13px',
        'tar-lg':  '17px',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      },
    },
  },
  plugins: [],
}
