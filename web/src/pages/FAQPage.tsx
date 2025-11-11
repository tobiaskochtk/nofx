import { FAQLayout } from '../components/faq/FAQLayout'
import { useLanguage } from '../contexts/LanguageContext'

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
  const { language } = useLanguage()

  return <FAQLayout language={language} />
}
