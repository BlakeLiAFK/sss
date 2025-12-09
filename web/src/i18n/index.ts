import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'

// 获取浏览器语言或本地存储的语言设置
function getDefaultLocale(): string {
  // 优先使用本地存储的语言设置
  const savedLocale = localStorage.getItem('locale')
  if (savedLocale && ['zh-CN', 'en-US'].includes(savedLocale)) {
    return savedLocale
  }

  // 其次根据浏览器语言自动选择
  const browserLang = navigator.language
  if (browserLang.startsWith('zh')) {
    return 'zh-CN'
  }
  return 'en-US'
}

// 创建 i18n 实例
const i18n = createI18n({
  legacy: false, // 使用 Composition API 模式
  locale: getDefaultLocale(),
  fallbackLocale: 'en-US',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS
  }
})

// 切换语言
export function setLocale(locale: string) {
  if (['zh-CN', 'en-US'].includes(locale)) {
    i18n.global.locale.value = locale as 'zh-CN' | 'en-US'
    localStorage.setItem('locale', locale)
    // 更新 HTML lang 属性
    document.documentElement.lang = locale === 'zh-CN' ? 'zh' : 'en'
  }
}

// 获取当前语言
export function getLocale(): string {
  return i18n.global.locale.value
}

// 切换语言（在中英文之间切换）
export function toggleLocale() {
  const current = getLocale()
  setLocale(current === 'zh-CN' ? 'en-US' : 'zh-CN')
}

export default i18n
