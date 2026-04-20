import { useState, useEffect } from 'react'
import { toast } from 'sonner'
import { User, Cpu, Building2, MessageCircle, Eye, EyeOff, ChevronRight, Plus, Pencil } from 'lucide-react'
import { useAuth } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import {
  getPostAuthPath,
  getUserMode,
  setUserMode,
  type UserMode,
} from '../lib/onboarding'
import { LANGUAGE_OPTIONS } from '../i18n/locale'
import { ExchangeConfigModal } from '../components/trader/ExchangeConfigModal'
import { TelegramConfigModal } from '../components/trader/TelegramConfigModal'
import { ModelConfigModal } from '../components/trader/ModelConfigModal'
import type { Exchange, AIModel } from '../types'
import type { Language } from '../i18n/translations'

type Tab = 'account' | 'models' | 'exchanges' | 'telegram'

export function SettingsPage() {
  const { user } = useAuth()
  const { language, setLanguage } = useLanguage()
  const pickText = (values: Record<Language, string>) => values[language]
  const [activeTab, setActiveTab] = useState<Tab>('account')
  const [userMode, setUserModeState] = useState<UserMode>(() => getUserMode() ?? 'advanced')

  // Account state
  const [newPassword, setNewPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [changingPassword, setChangingPassword] = useState(false)

  // AI Models state
  const [configuredModels, setConfiguredModels] = useState<AIModel[]>([])
  const [supportedModels, setSupportedModels] = useState<AIModel[]>([])
  const [showModelModal, setShowModelModal] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)

  // Exchanges state
  const [exchanges, setExchanges] = useState<Exchange[]>([])
  const [showExchangeModal, setShowExchangeModal] = useState(false)
  const [editingExchange, setEditingExchange] = useState<string | null>(null)

  // Telegram state
  const [showTelegramModal, setShowTelegramModal] = useState(false)

  // Fetch data when tabs are visited
  useEffect(() => {
    if (activeTab === 'models') {
      Promise.all([api.getModelConfigs(), api.getSupportedModels()])
        .then(([configs, supported]) => {
          setConfiguredModels(configs)
          setSupportedModels(supported)
        })
        .catch(() =>
          toast.error(
            pickText({
              zh: '加载 AI 模型失败',
              en: 'Failed to load AI models',
              de: 'AI-Modelle konnten nicht geladen werden',
              id: 'Failed to load AI models',
            })
          )
        )
    }
    if (activeTab === 'exchanges') {
      api.getExchangeConfigs()
        .then(setExchanges)
        .catch(() =>
          toast.error(
            pickText({
              zh: '加载交易所失败',
              en: 'Failed to load exchanges',
              de: 'Boersen konnten nicht geladen werden',
              id: 'Failed to load exchanges',
            })
          )
        )
    }
  }, [activeTab])

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword.length < 8) {
      toast.error(
        pickText({
          zh: '密码至少需要 8 个字符',
          en: 'Password must be at least 8 characters',
          de: 'Das Passwort muss mindestens 8 Zeichen lang sein',
          id: 'Password must be at least 8 characters',
        })
      )
      return
    }
    setChangingPassword(true)
    try {
      const res = await fetch('/api/user/password', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('token') || ''}`,
        },
        body: JSON.stringify({ new_password: newPassword }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(
          data.error ||
            pickText({
              zh: '密码更新失败',
              en: 'Failed to update password',
              de: 'Passwort konnte nicht aktualisiert werden',
              id: 'Failed to update password',
            })
        )
      }
      toast.success(
        pickText({
          zh: '密码更新成功',
          en: 'Password updated successfully',
          de: 'Passwort erfolgreich aktualisiert',
          id: 'Password updated successfully',
        })
      )
      setNewPassword('')
    } catch (err) {
      toast.error(
        err instanceof Error
          ? err.message
          : pickText({
              zh: '密码更新失败',
              en: 'Failed to update password',
              de: 'Passwort konnte nicht aktualisiert werden',
              id: 'Failed to update password',
            })
      )
    } finally {
      setChangingPassword(false)
    }
  }

  const handleSwitchMode = (nextMode: UserMode) => {
    if (nextMode === userMode) {
      return
    }

    setUserMode(nextMode)
    setUserModeState(nextMode)
    toast.success(
      nextMode === 'beginner'
        ? pickText({
            zh: '已切换到新手模式',
            en: 'Switched to beginner mode',
            de: 'Zum Beginner-Modus gewechselt',
            id: 'Switched to beginner mode',
          })
        : pickText({
            zh: '已切换到老手模式',
            en: 'Switched to advanced mode',
            de: 'Zum Advanced-Modus gewechselt',
            id: 'Switched to advanced mode',
          })
    )

    const nextPath = getPostAuthPath(nextMode)
    window.history.pushState({}, '', nextPath)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  const handleSaveModel = async (
    modelId: string,
    apiKey: string,
    customApiUrl?: string,
    customModelName?: string
  ) => {
    try {
      const existingModel = configuredModels.find((m) => m.id === modelId)
      const modelTemplate = supportedModels.find((m) => m.id === modelId)
      const modelToUpdate = existingModel || modelTemplate
      if (!modelToUpdate) {
        toast.error(
          pickText({
            zh: '未找到模型',
            en: 'Model not found',
            de: 'Modell nicht gefunden',
            id: 'Model not found',
          })
        )
        return
      }

      let updatedModels: AIModel[]
      if (existingModel) {
        updatedModels = configuredModels.map((m) =>
          m.id === modelId
            ? { ...m, apiKey, customApiUrl: customApiUrl || '', customModelName: customModelName || '', enabled: true }
            : m
        )
      } else {
        updatedModels = [...configuredModels, {
          ...modelToUpdate,
          apiKey,
          customApiUrl: customApiUrl || '',
          customModelName: customModelName || '',
          enabled: true,
        }]
      }

      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [m.provider, {
            enabled: m.enabled,
            api_key: m.apiKey || '',
            custom_api_url: m.customApiUrl || '',
            custom_model_name: m.customModelName || '',
          }])
        ),
      }
      await api.updateModelConfigs(request)
      toast.success(
        pickText({
          zh: '模型配置已保存',
          en: 'Model config saved',
          de: 'Modellkonfiguration gespeichert',
          id: 'Model config saved',
        })
      )
      const refreshed = await api.getModelConfigs()
      setConfiguredModels(refreshed)
      setShowModelModal(false)
      setEditingModel(null)
    } catch {
      toast.error(
        pickText({
          zh: '模型配置保存失败',
          en: 'Failed to save model config',
          de: 'Modellkonfiguration konnte nicht gespeichert werden',
          id: 'Failed to save model config',
        })
      )
    }
  }

  const handleDeleteModel = async (modelId: string) => {
    try {
      const updatedModels = configuredModels.map((m) =>
        m.id === modelId ? { ...m, apiKey: '', customApiUrl: '', customModelName: '', enabled: false } : m
      )
      const request = {
        models: Object.fromEntries(
          updatedModels.map((m) => [m.provider, {
            enabled: m.enabled,
            api_key: m.apiKey || '',
            custom_api_url: m.customApiUrl || '',
            custom_model_name: m.customModelName || '',
          }])
        ),
      }
      await api.updateModelConfigs(request)
      const refreshed = await api.getModelConfigs()
      setConfiguredModels(refreshed)
      setShowModelModal(false)
      setEditingModel(null)
      toast.success(
        pickText({
          zh: '模型配置已移除',
          en: 'Model config removed',
          de: 'Modellkonfiguration entfernt',
          id: 'Model config removed',
        })
      )
    } catch {
      toast.error(
        pickText({
          zh: '模型配置移除失败',
          en: 'Failed to remove model config',
          de: 'Modellkonfiguration konnte nicht entfernt werden',
          id: 'Failed to remove model config',
        })
      )
    }
  }

  const handleSaveExchange = async (
    exchangeId: string | null,
    exchangeType: string,
    accountName: string,
    apiKey: string,
    secretKey?: string,
    passphrase?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    lighterWalletAddr?: string,
    lighterPrivateKey?: string,
    lighterApiKeyPrivateKey?: string,
    lighterApiKeyIndex?: number
  ) => {
    try {
      if (exchangeId) {
        const request = {
          exchanges: {
            [exchangeId]: {
              enabled: true,
              api_key: apiKey || '',
              secret_key: secretKey || '',
              passphrase: passphrase || '',
              testnet: testnet || false,
              hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
              aster_user: asterUser || '',
              aster_signer: asterSigner || '',
              aster_private_key: asterPrivateKey || '',
              lighter_wallet_addr: lighterWalletAddr || '',
              lighter_private_key: lighterPrivateKey || '',
              lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
              lighter_api_key_index: lighterApiKeyIndex || 0,
            },
          },
        }
        await api.updateExchangeConfigsEncrypted(request)
        toast.success(
          pickText({
            zh: '交易所配置已更新',
            en: 'Exchange config updated',
            de: 'Boersenkonfiguration aktualisiert',
            id: 'Exchange config updated',
          })
        )
      } else {
        const createRequest = {
          exchange_type: exchangeType,
          account_name: accountName,
          enabled: true,
          api_key: apiKey || '',
          secret_key: secretKey || '',
          passphrase: passphrase || '',
          testnet: testnet || false,
          hyperliquid_wallet_addr: hyperliquidWalletAddr || '',
          aster_user: asterUser || '',
          aster_signer: asterSigner || '',
          aster_private_key: asterPrivateKey || '',
          lighter_wallet_addr: lighterWalletAddr || '',
          lighter_private_key: lighterPrivateKey || '',
          lighter_api_key_private_key: lighterApiKeyPrivateKey || '',
          lighter_api_key_index: lighterApiKeyIndex || 0,
        }
        await api.createExchangeEncrypted(createRequest)
        toast.success(
          pickText({
            zh: '交易所账户已创建',
            en: 'Exchange account created',
            de: 'Boersenkonto erstellt',
            id: 'Exchange account created',
          })
        )
      }
      const refreshed = await api.getExchangeConfigs()
      setExchanges(refreshed)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error(
        pickText({
          zh: '交易所配置保存失败',
          en: 'Failed to save exchange config',
          de: 'Boersenkonfiguration konnte nicht gespeichert werden',
          id: 'Failed to save exchange config',
        })
      )
    }
  }

  const handleDeleteExchange = async (exchangeId: string) => {
    try {
      await api.deleteExchange(exchangeId)
      toast.success(
        pickText({
          zh: '交易所账户已删除',
          en: 'Exchange account deleted',
          de: 'Boersenkonto geloescht',
          id: 'Exchange account deleted',
        })
      )
      const refreshed = await api.getExchangeConfigs()
      setExchanges(refreshed)
      setShowExchangeModal(false)
      setEditingExchange(null)
    } catch {
      toast.error(
        pickText({
          zh: '交易所账户删除失败',
          en: 'Failed to delete exchange account',
          de: 'Boersenkonto konnte nicht geloescht werden',
          id: 'Failed to delete exchange account',
        })
      )
    }
  }

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    {
      key: 'account',
      label: pickText({
        zh: '账户',
        en: 'Account',
        de: 'Konto',
        id: 'Account',
      }),
      icon: <User size={16} />,
    },
    {
      key: 'models',
      label: pickText({
        zh: 'AI 模型',
        en: 'AI Models',
        de: 'AI-Modelle',
        id: 'AI Models',
      }),
      icon: <Cpu size={16} />,
    },
    {
      key: 'exchanges',
      label: pickText({
        zh: '交易所',
        en: 'Exchanges',
        de: 'Boersen',
        id: 'Exchanges',
      }),
      icon: <Building2 size={16} />,
    },
    { key: 'telegram', label: 'Telegram', icon: <MessageCircle size={16} /> },
  ]

  return (
    <div className="min-h-screen pt-20 pb-12 px-4" style={{ background: '#0B0E11' }}>
      <div className="max-w-2xl mx-auto">
        <h1 className="text-xl font-bold text-white mb-6">
          {pickText({
            zh: '设置',
            en: 'Settings',
            de: 'Einstellungen',
            id: 'Settings',
          })}
        </h1>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-zinc-900/60 border border-zinc-800 rounded-xl p-1">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-all
                ${activeTab === tab.key
                  ? 'bg-nofx-gold text-black'
                  : 'text-zinc-400 hover:text-white'
                }`}
            >
              {tab.icon}
              <span className="hidden sm:inline">{tab.label}</span>
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div className="bg-zinc-900/60 backdrop-blur-xl border border-zinc-800/80 rounded-2xl p-6">

          {/* Account Tab */}
          {activeTab === 'account' && (
            <div className="space-y-6">
              <div>
                <p className="text-xs text-zinc-500 mb-1">
                  {pickText({
                    zh: '邮箱',
                    en: 'Email',
                    de: 'E-Mail',
                    id: 'Email',
                  })}
                </p>
                <p className="text-sm text-white font-medium">{user?.email}</p>
              </div>

              <div className="border-t border-zinc-800 pt-6">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="text-sm font-semibold text-white">
                      {pickText({
                        zh: '界面语言',
                        en: 'Interface Language',
                        de: 'Oberflaechensprache',
                        id: 'Bahasa Antarmuka',
                      })}
                    </h3>
                    <p className="mt-1 text-xs text-zinc-500">
                      {pickText({
                        zh: '这里只影响你看到的界面文本；内部配置和 AI 提示仍保持英文。',
                        en: 'This only changes visible UI text. Internal config and AI prompts stay in English.',
                        de: 'Das aendert nur sichtbare UI-Texte. Interne Konfiguration und AI-Prompts bleiben Englisch.',
                        id: 'Ini hanya mengubah teks UI yang terlihat. Konfigurasi internal dan prompt AI tetap berbahasa Inggris.',
                      })}
                    </p>
                  </div>
                  <span className="rounded-full border border-nofx-gold/20 bg-nofx-gold/10 px-3 py-1 text-xs font-semibold text-nofx-gold">
                    {pickText({
                      zh: '当前语言',
                      en: 'Current Language',
                      de: 'Aktuelle Sprache',
                      id: 'Bahasa Saat Ini',
                    })}
                  </span>
                </div>

                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  {LANGUAGE_OPTIONS.map((option) => (
                    <button
                      key={option.code}
                      type="button"
                      onClick={() => setLanguage(option.code)}
                      className={`rounded-2xl border px-4 py-4 text-left transition-all ${
                        language === option.code
                          ? 'border-nofx-gold bg-nofx-gold/10'
                          : 'border-zinc-800 bg-zinc-950/70 hover:border-zinc-700'
                      }`}
                    >
                      <div className="flex items-center justify-between gap-3">
                        <div className="flex items-center gap-3">
                          <span className="text-lg">{option.flag}</span>
                          <div>
                            <div className="text-sm font-semibold text-white">
                              {option.label}
                            </div>
                            <div className="mt-1 text-xs text-zinc-500">
                              {option.shortLabel}
                            </div>
                          </div>
                        </div>
                        {language === option.code ? (
                          <span className="rounded-full border border-nofx-gold/20 bg-nofx-gold/10 px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-nofx-gold">
                            {pickText({
                              zh: '已启用',
                              en: 'Active',
                              de: 'Aktiv',
                              id: 'Aktif',
                            })}
                          </span>
                        ) : null}
                      </div>
                    </button>
                  ))}
                </div>
              </div>

              <div className="border-t border-zinc-800 pt-6">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h3 className="text-sm font-semibold text-white">
                      {pickText({
                        zh: '使用模式',
                        en: 'Usage Mode',
                        de: 'Nutzungsmodus',
                        id: 'Usage Mode',
                      })}
                    </h3>
                    <p className="mt-1 text-xs text-zinc-500">
                      {pickText({
                        zh: '新手模式会显示钱包引导和 4 步卡片；老手模式保持原来的专业界面。',
                        en: 'Beginner mode shows wallet onboarding and quickstart cards. Advanced mode keeps the original pro workflow.',
                        de: 'Im Beginner-Modus siehst du Wallet-Onboarding und Schnellstartkarten. Advanced behaelt den bisherigen Profi-Workflow.',
                        id: 'Beginner mode shows wallet onboarding and quickstart cards. Advanced mode keeps the original pro workflow.',
                      })}
                    </p>
                  </div>
                  <span className="rounded-full border border-nofx-gold/20 bg-nofx-gold/10 px-3 py-1 text-xs font-semibold text-nofx-gold">
                    {userMode === 'beginner'
                      ? pickText({
                          zh: '当前：新手模式',
                          en: 'Current: Beginner',
                          de: 'Aktuell: Beginner',
                          id: 'Current: Beginner',
                        })
                      : pickText({
                          zh: '当前：老手模式',
                          en: 'Current: Advanced',
                          de: 'Aktuell: Advanced',
                          id: 'Current: Advanced',
                        })}
                  </span>
                </div>

                <div className="mt-4 grid gap-3 sm:grid-cols-2">
                  <button
                    type="button"
                    onClick={() => handleSwitchMode('beginner')}
                    className={`rounded-2xl border px-4 py-4 text-left transition-all ${
                      userMode === 'beginner'
                        ? 'border-nofx-gold bg-nofx-gold/10'
                        : 'border-zinc-800 bg-zinc-950/70 hover:border-zinc-700'
                    }`}
                  >
                    <div className="text-sm font-semibold text-white">
                      {pickText({
                        zh: '新手模式',
                        en: 'Beginner Mode',
                        de: 'Beginner-Modus',
                        id: 'Beginner Mode',
                      })}
                    </div>
                    <div className="mt-1 text-xs text-zinc-500">
                      {pickText({
                        zh: '更简单，优先显示钱包、充值和快速上手引导。',
                        en: 'Simpler flow with wallet, funding, and quickstart guidance first.',
                        de: 'Einfacherer Ablauf mit Fokus auf Wallet, Funding und Schnellstart-Hinweise.',
                        id: 'Simpler flow with wallet, funding, and quickstart guidance first.',
                      })}
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={() => handleSwitchMode('advanced')}
                    className={`rounded-2xl border px-4 py-4 text-left transition-all ${
                      userMode === 'advanced'
                        ? 'border-nofx-gold bg-nofx-gold/10'
                        : 'border-zinc-800 bg-zinc-950/70 hover:border-zinc-700'
                    }`}
                  >
                    <div className="text-sm font-semibold text-white">
                      {pickText({
                        zh: '老手模式',
                        en: 'Advanced Mode',
                        de: 'Advanced-Modus',
                        id: 'Advanced Mode',
                      })}
                    </div>
                    <div className="mt-1 text-xs text-zinc-500">
                      {pickText({
                        zh: '保持原来的配置与交易流程，不展示新手引导。',
                        en: 'Keeps the original configuration and trading workflow without beginner hints.',
                        de: 'Behaelt den bisherigen Konfigurations- und Trading-Workflow ohne Beginner-Hinweise.',
                        id: 'Keeps the original configuration and trading workflow without beginner hints.',
                      })}
                    </div>
                  </button>
                </div>
              </div>

              <div className="border-t border-zinc-800 pt-6">
                <h3 className="text-sm font-semibold text-white mb-4">
                  {pickText({
                    zh: '密码修改',
                    en: 'Change Password',
                    de: 'Passwort aendern',
                    id: 'Change Password',
                  })}
                </h3>
                <form onSubmit={handleChangePassword} className="space-y-4">
                  <div>
                    <label className="block text-xs font-medium text-zinc-400 mb-2">
                      {pickText({
                        zh: '新密码',
                        en: 'New Password',
                        de: 'Neues Passwort',
                        id: 'New Password',
                      })}
                    </label>
                    <div className="relative">
                      <input
                        type={showPassword ? 'text' : 'password'}
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        className="w-full bg-zinc-950/80 border border-zinc-700/80 rounded-xl px-4 py-3 pr-11 text-sm text-white placeholder-zinc-600 focus:outline-none focus:border-nofx-gold/60 focus:ring-1 focus:ring-nofx-gold/30 transition-all"
                        placeholder={pickText({
                          zh: '至少 8 个字符',
                          en: 'At least 8 characters',
                          de: 'Mindestens 8 Zeichen',
                          id: 'At least 8 characters',
                        })}
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 transition-colors"
                      >
                        {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                      </button>
                    </div>
                  </div>
                  <button
                    type="submit"
                    disabled={changingPassword || newPassword.length < 8}
                    className="w-full bg-nofx-gold hover:bg-yellow-400 active:scale-[0.98] text-black font-semibold py-3 rounded-xl text-sm transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {changingPassword
                      ? pickText({
                          zh: '更新中...',
                          en: 'Updating...',
                          de: 'Aktualisiert...',
                          id: 'Updating...',
                        })
                      : pickText({
                          zh: '更新密码',
                          en: 'Update Password',
                          de: 'Passwort aktualisieren',
                          id: 'Update Password',
                        })}
                  </button>
                </form>
              </div>
            </div>
          )}

          {/* AI Models Tab */}
          {activeTab === 'models' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-zinc-400">
                  {pickText({
                    zh: `已配置 ${configuredModels.length} 个模型`,
                    en: `${configuredModels.length} model${configuredModels.length !== 1 ? 's' : ''} configured`,
                    de: `${configuredModels.length} Modell${configuredModels.length !== 1 ? 'e' : ''} konfiguriert`,
                    id: `${configuredModels.length} model${configuredModels.length !== 1 ? 's' : ''} configured`,
                  })}
                </p>
                <button
                  onClick={() => { setEditingModel(null); setShowModelModal(true) }}
                  className="flex items-center gap-1.5 text-xs font-medium bg-nofx-gold/10 hover:bg-nofx-gold/20 text-nofx-gold px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus size={14} />
                  {pickText({
                    zh: '添加模型',
                    en: 'Add Model',
                    de: 'Modell hinzufuegen',
                    id: 'Add Model',
                  })}
                </button>
              </div>

              {configuredModels.length === 0 ? (
                <div className="text-center py-8 text-zinc-600 text-sm">
                  {pickText({
                    zh: '还没有配置 AI 模型',
                    en: 'No AI models configured yet',
                    de: 'Noch keine AI-Modelle konfiguriert',
                    id: 'No AI models configured yet',
                  })}
                </div>
              ) : (
                <div className="space-y-2">
                  {configuredModels.map((model) => (
                    <button
                      key={model.id}
                      onClick={() => { setEditingModel(model.id); setShowModelModal(true) }}
                      className="w-full flex items-center justify-between px-4 py-3 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700/50 transition-colors group"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-zinc-700 flex items-center justify-center">
                          <Cpu size={14} className="text-zinc-300" />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium text-white">{model.name}</p>
                          <p className="text-xs text-zinc-500">{model.provider}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-xs px-2 py-0.5 rounded-full ${model.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-700 text-zinc-500'}`}>
                          {model.enabled
                            ? pickText({
                                zh: '启用',
                                en: 'Active',
                                de: 'Aktiv',
                                id: 'Active',
                              })
                            : pickText({
                                zh: '停用',
                                en: 'Inactive',
                                de: 'Inaktiv',
                                id: 'Inactive',
                              })}
                        </span>
                        <Pencil size={14} className="text-zinc-600 group-hover:text-zinc-400 transition-colors" />
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Exchanges Tab */}
          {activeTab === 'exchanges' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-zinc-400">
                  {pickText({
                    zh: `已连接 ${exchanges.length} 个账户`,
                    en: `${exchanges.length} account${exchanges.length !== 1 ? 's' : ''} connected`,
                    de: `${exchanges.length} Konto${exchanges.length !== 1 ? 'en' : ''} verbunden`,
                    id: `${exchanges.length} account${exchanges.length !== 1 ? 's' : ''} connected`,
                  })}
                </p>
                <button
                  onClick={() => { setEditingExchange(null); setShowExchangeModal(true) }}
                  className="flex items-center gap-1.5 text-xs font-medium bg-nofx-gold/10 hover:bg-nofx-gold/20 text-nofx-gold px-3 py-1.5 rounded-lg transition-colors"
                >
                  <Plus size={14} />
                  {pickText({
                    zh: '添加交易所',
                    en: 'Add Exchange',
                    de: 'Boerse hinzufuegen',
                    id: 'Add Exchange',
                  })}
                </button>
              </div>

              {exchanges.length === 0 ? (
                <div className="text-center py-8 text-zinc-600 text-sm">
                  {pickText({
                    zh: '还没有连接交易所账户',
                    en: 'No exchange accounts connected yet',
                    de: 'Noch keine Boersenkonten verbunden',
                    id: 'No exchange accounts connected yet',
                  })}
                </div>
              ) : (
                <div className="space-y-2">
                  {exchanges.map((exchange) => (
                    <button
                      key={exchange.id}
                      onClick={() => { setEditingExchange(exchange.id); setShowExchangeModal(true) }}
                      className="w-full flex items-center justify-between px-4 py-3 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700/50 transition-colors group"
                    >
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 rounded-lg bg-zinc-700 flex items-center justify-center">
                          <Building2 size={14} className="text-zinc-300" />
                        </div>
                        <div className="text-left">
                          <p className="text-sm font-medium text-white">{exchange.account_name || exchange.name}</p>
                          <p className="text-xs text-zinc-500 capitalize">{exchange.exchange_type || exchange.type}</p>
                        </div>
                      </div>
                      <ChevronRight size={14} className="text-zinc-600 group-hover:text-zinc-400 transition-colors" />
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Telegram Tab */}
          {activeTab === 'telegram' && (
            <div className="space-y-4">
              <p className="text-sm text-zinc-400">
                {pickText({
                  zh: '连接 Telegram Bot，以接收交易通知并与交易员交互。',
                  en: 'Connect a Telegram bot to receive trading notifications and interact with your traders.',
                  de: 'Verbinde einen Telegram-Bot, um Trading-Benachrichtigungen zu erhalten und mit deinen Tradern zu interagieren.',
                  id: 'Connect a Telegram bot to receive trading notifications and interact with your traders.',
                })}
              </p>
              <button
                onClick={() => setShowTelegramModal(true)}
                className="w-full flex items-center justify-between px-4 py-3 rounded-xl bg-zinc-800/50 hover:bg-zinc-800 border border-zinc-700/50 transition-colors group"
              >
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-[#0088cc]/20 flex items-center justify-center">
                    <MessageCircle size={14} className="text-[#0088cc]" />
                  </div>
                  <span className="text-sm font-medium text-white">
                    {pickText({
                      zh: '配置 Telegram Bot',
                      en: 'Configure Telegram Bot',
                      de: 'Telegram-Bot konfigurieren',
                      id: 'Configure Telegram Bot',
                    })}
                  </span>
                </div>
                <ChevronRight size={14} className="text-zinc-600 group-hover:text-zinc-400 transition-colors" />
              </button>
            </div>
          )}
        </div>
      </div>

      {/* AI Model Modal */}
      {showModelModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <ModelConfigModal
            allModels={supportedModels}
            configuredModels={configuredModels}
            editingModelId={editingModel}
            onSave={handleSaveModel}
            onDelete={handleDeleteModel}
            onClose={() => { setShowModelModal(false); setEditingModel(null) }}
            language={language}
          />
        </div>
      )}

      {/* Exchange Modal */}
      {showExchangeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <ExchangeConfigModal
            allExchanges={exchanges}
            editingExchangeId={editingExchange}
            onSave={handleSaveExchange}
            onDelete={handleDeleteExchange}
            onClose={() => { setShowExchangeModal(false); setEditingExchange(null) }}
            language={language}
          />
        </div>
      )}

      {/* Telegram Modal */}
      {showTelegramModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm px-4">
          <TelegramConfigModal
            onClose={() => setShowTelegramModal(false)}
            language={language}
          />
        </div>
      )}
    </div>
  )
}
