import { createI18n } from 'vue-i18n'
import en from './locales/en.json'

export type MessageSchema = typeof en

// English is bundled (it is also the fallback). Every other locale is split
// into its own chunk and fetched only when a user actually selects it.
const localeLoaders = import.meta.glob(['./locales/*.json', '!./locales/en.json'], {
  import: 'default',
}) as Record<string, () => Promise<MessageSchema>>

const codeFromPath = (path: string) => path.replace('./locales/', '').replace('.json', '')
const availableCodes = ['en', ...Object.keys(localeLoaders).map(codeFromPath)]

const localeNames: Record<string, { name: string; nativeName: string }> = {
  en: { name: 'English', nativeName: 'English' },
  es: { name: 'Spanish', nativeName: 'Español' },
  fr: { name: 'French', nativeName: 'Français' },
  de: { name: 'German', nativeName: 'Deutsch' },
  hi: { name: 'Hindi', nativeName: 'हिंदी' },
  pt: { name: 'Portuguese', nativeName: 'Português' },
  zh: { name: 'Chinese', nativeName: '中文' },
  ja: { name: 'Japanese', nativeName: '日本語' },
  ko: { name: 'Korean', nativeName: '한국어' },
  ar: { name: 'Arabic', nativeName: 'العربية' },
  ru: { name: 'Russian', nativeName: 'Русский' },
  it: { name: 'Italian', nativeName: 'Italiano' },
  nl: { name: 'Dutch', nativeName: 'Nederlands' },
  tr: { name: 'Turkish', nativeName: 'Türkçe' },
  vi: { name: 'Vietnamese', nativeName: 'Tiếng Việt' },
  th: { name: 'Thai', nativeName: 'ไทย' },
  id: { name: 'Indonesian', nativeName: 'Bahasa Indonesia' },
  ms: { name: 'Malay', nativeName: 'Bahasa Melayu' },
  pl: { name: 'Polish', nativeName: 'Polski' },
  uk: { name: 'Ukrainian', nativeName: 'Українська' },
  ta: { name: 'Tamil', nativeName: 'தமிழ்' },

}

// Auto-generate SUPPORTED_LOCALES from available files
export const SUPPORTED_LOCALES = availableCodes.map(code => {
  const names = localeNames[code] || { name: code, nativeName: code }
  return { code, ...names }
})

export type SupportedLocale = string

const loadedLocales = new Set<string>(['en'])

// Get saved locale or detect from browser
function getDefaultLocale(): string {
  const saved = localStorage.getItem('locale')
  if (saved && availableCodes.includes(saved)) {
    return saved
  }
  const browserLang = navigator.language.split('-')[0]
  if (availableCodes.includes(browserLang)) {
    return browserLang
  }
  return 'en'
}

export const i18n = createI18n({
  legacy: false, // Use Composition API
  locale: 'en',
  fallbackLocale: 'en',
  messages: { en } as Record<string, MessageSchema>,
})

// Fetch a locale's messages once and register them with vue-i18n.
async function ensureLocaleLoaded(locale: string): Promise<boolean> {
  if (loadedLocales.has(locale)) return true
  const loader = localeLoaders[`./locales/${locale}.json`]
  if (!loader) return false
  i18n.global.setLocaleMessage(locale, await loader())
  loadedLocales.add(locale)
  return true
}

function applyLocale(locale: string) {
  i18n.global.locale.value = locale
  document.documentElement.setAttribute('lang', locale)
}

// Called once before the app mounts so the first render is in the user's language.
export async function initLocale(): Promise<void> {
  const locale = getDefaultLocale()
  if (locale !== 'en' && !(await ensureLocaleLoaded(locale).catch(() => false))) {
    applyLocale('en')
    return
  }
  applyLocale(locale)
}

// Helper to change locale (loads the locale's messages on first use)
export async function setLocale(locale: string): Promise<void> {
  if (!(await ensureLocaleLoaded(locale))) {
    console.warn(`Locale '${locale}' not available`)
    return
  }
  applyLocale(locale)
  localStorage.setItem('locale', locale)
}

// Get current locale
export function getLocale(): string {
  return i18n.global.locale.value
}
