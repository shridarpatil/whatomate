import { ref, watch, onMounted } from 'vue'

export type ColorMode = 'light' | 'dark' | 'system'

const colorMode = ref<ColorMode>('dark')
const isDark = ref(true)

export function resolveScheme(mode: ColorMode, systemDark: boolean): 'light' | 'dark' {
  if (mode === 'system') return systemDark ? 'dark' : 'light'
  return mode
}

function getSystemTheme(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function updateTheme() {
  const scheme = resolveScheme(colorMode.value, getSystemTheme())
  isDark.value = scheme === 'dark'
  document.documentElement.style.colorScheme = scheme

  // Dark-first: we use .light class for light mode, .dark class for dark mode
  if (isDark.value) {
    document.documentElement.classList.remove('light')
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
    document.documentElement.classList.add('light')
  }
}

export function useColorMode() {
  onMounted(() => {
    // Load saved preference
    const saved = localStorage.getItem('color-mode') as ColorMode | null
    if (saved) {
      colorMode.value = saved
    }
    updateTheme()

    // Listen for system theme changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (colorMode.value === 'system') {
        updateTheme()
      }
    })
  })

  watch(colorMode, (newMode) => {
    localStorage.setItem('color-mode', newMode)
    updateTheme()
  })

  function setColorMode(mode: ColorMode) {
    colorMode.value = mode
  }

  return {
    colorMode,
    isDark,
    setColorMode
  }
}
