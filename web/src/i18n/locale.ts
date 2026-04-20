import type { Language } from './translations'

export interface LanguageOption {
  code: Language
  label: string
  shortLabel: string
  flag: string
}

export const LANGUAGE_OPTIONS: LanguageOption[] = [
  { code: 'zh', label: '中文', shortLabel: 'CN', flag: '🇨🇳' },
  { code: 'en', label: 'English', shortLabel: 'EN', flag: '🇺🇸' },
  { code: 'de', label: 'Deutsch', shortLabel: 'DE', flag: '🇩🇪' },
  { code: 'id', label: 'Bahasa', shortLabel: 'ID', flag: '🇮🇩' },
]

export function isSupportedLanguage(
  value: string | null | undefined
): value is Language {
  return LANGUAGE_OPTIONS.some((option) => option.code === value)
}

export function getLanguageOption(language: Language): LanguageOption {
  return (
    LANGUAGE_OPTIONS.find((option) => option.code === language) ??
    LANGUAGE_OPTIONS[1]
  )
}

export function toDocumentLanguage(language: Language): string {
  switch (language) {
    case 'zh':
      return 'zh-CN'
    case 'de':
      return 'de'
    case 'id':
      return 'id'
    default:
      return 'en'
  }
}

export function toDateTimeLocale(language: Language): string {
  switch (language) {
    case 'zh':
      return 'zh-CN'
    case 'de':
      return 'de-DE'
    case 'id':
      return 'id-ID'
    default:
      return 'en-US'
  }
}

export function toTradingViewLocale(language: Language): string {
  switch (language) {
    case 'zh':
      return 'zh_CN'
    default:
      return 'en'
  }
}
