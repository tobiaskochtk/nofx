import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import type { Language } from '../i18n/translations'
import {
  isSupportedLanguage,
  toDocumentLanguage,
} from '../i18n/locale'

interface LanguageContextType {
  language: Language
  setLanguage: (lang: Language) => void
}

const LanguageContext = createContext<LanguageContextType | undefined>(
  undefined
)

export function LanguageProvider({ children }: { children: ReactNode }) {
  // Initialize language from localStorage or default to English
  const [language, setLanguage] = useState<Language>(() => {
    const saved = localStorage.getItem('language')
    return isSupportedLanguage(saved) ? saved : 'en'
  })

  // Save language to localStorage whenever it changes
  const handleSetLanguage = (lang: Language) => {
    setLanguage(lang)
    localStorage.setItem('language', lang)
  }

  useEffect(() => {
    document.documentElement.lang = toDocumentLanguage(language)
  }, [language])

  return (
    <LanguageContext.Provider
      value={{ language, setLanguage: handleSetLanguage }}
    >
      {children}
    </LanguageContext.Provider>
  )
}

export function useLanguage() {
  const context = useContext(LanguageContext)
  if (!context) {
    throw new Error('useLanguage must be used within LanguageProvider')
  }
  return context
}
