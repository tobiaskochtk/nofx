import HeaderBar from '../components/landing/HeaderBar'
import { FAQLayout } from '../components/faq/FAQLayout'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { useSystemConfig } from '../hooks/useSystemConfig'
import { t } from '../i18n/translations'

/**
 * FAQページ | FAQ Page
 *
 * このページはコンポーネントの集合であり、以下を担当します: | This page is a collection of components, responsible for:
 * - HeaderBar と FAQLayout の組み立て | Assembling HeaderBar and FAQLayout
 * - グローバルステート（言語、ユーザー、システム設定）の提供 | Providing global state (language, user, system config)
 * - ページレベルのナビゲーション処理 | Handling page-level navigation
 *
 * すべてのFAQ関連のロジックはサブコンポーネントにあります: | All FAQ related logic is in subcomponents:
 * - FAQLayout: 全体レイアウトと検索ロジック | Overall layout and search logic
 * - FAQSearchBar: 検索ボックス | Search box
 * - FAQSidebar: 左側の目次 | Left sidebar table of contents
 * - FAQContent: 右側のコンテンツエリア | Right content area
 *
 * FAQデータ設定は data/faqData.ts にあります | FAQ data configuration is in data/faqData.ts
 */
export function FAQPage() {
  const { language, setLanguage } = useLanguage()
  const { user, logout } = useAuth()
  useSystemConfig() // Load system config but don't use it

  return (
    <div
      className="min-h-screen"
      style={{ background: '#000000', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage="faq"
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page) => {
          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            window.location.href = '/competition'
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            window.location.href = '/traders'
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            window.location.href = '/dashboard'
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            window.location.href = '/faq'
          }
        }}
      />

      <FAQLayout language={language} />

      {/* Footer */}
      <footer
        className="mt-16"
        style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
      >
        <div
          className="max-w-7xl mx-auto px-6 py-6 text-center text-sm"
          style={{ color: '#5E6673' }}
        >
          <p>{t('footerTitle', language)}</p>
          <p className="mt-1">{t('footerWarning', language)}</p>
        </div>
      </footer>
    </div>
  )
}
