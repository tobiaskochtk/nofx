import { useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowRight,
  Copy,
  RefreshCw,
  Shield,
  Wallet,
  X,
} from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { toast } from 'sonner'
import { useLanguage } from '../contexts/LanguageContext'
import { api } from '../lib/api'
import type { BeginnerOnboardingResponse } from '../types'
import { setBeginnerWalletAddress, markBeginnerOnboardingCompleted } from '../lib/onboarding'

export function BeginnerOnboardingPage() {
  const { language } = useLanguage()
  const [data, setData] = useState<BeginnerOnboardingResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [refreshingBalance, setRefreshingBalance] = useState(false)
  const hasRequestedRef = useRef(false)
  const isZh = language === 'zh'
  const pickText = (values: Record<string, string>) =>
    values[language] ?? values.en

  const loadOnboarding = async (showLoading: boolean) => {
    if (showLoading) {
      setLoading(true)
    } else {
      setRefreshingBalance(true)
    }

    setError('')
    try {
      const result = await api.prepareBeginnerOnboarding()
      setData(result)
      setBeginnerWalletAddress(result.address)
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : pickText({
              zh: '新手钱包准备失败',
              en: 'Failed to prepare beginner wallet',
              de: 'Beginner-Wallet konnte nicht vorbereitet werden',
            })
      )
    } finally {
      if (showLoading) {
        setLoading(false)
      } else {
        setRefreshingBalance(false)
      }
    }
  }

  useEffect(() => {
    if (hasRequestedRef.current) {
      return
    }
    hasRequestedRef.current = true
    void loadOnboarding(true)
  }, [])

  const noticeText = useMemo(
    () =>
      pickText({
        zh: '此钱包仅用于大模型调用费用，不会自动充到交易所。私钥丢失后无法恢复，只充 Base 链 USDC。',
        en: 'This wallet only pays for model calls. It does not fund your exchange automatically. The private key cannot be recovered, and you should only deposit Base USDC.',
        de: 'Dieses Wallet bezahlt nur Modellaufrufe. Es fuellt dein Boersenkonto nicht automatisch auf. Der Private Key kann nicht wiederhergestellt werden, und du solltest nur Base-USDC einzahlen.',
      }),
    [language]
  )

  const copyText = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(
        pickText({
          zh: `${label}已复制`,
          en: `${label} copied`,
          de: `${label} kopiert`,
        })
      )
    } catch {
      toast.error(
        pickText({
          zh: '复制失败',
          en: 'Copy failed',
          de: 'Kopieren fehlgeschlagen',
        })
      )
    }
  }

  const handleContinue = () => {
    markBeginnerOnboardingCompleted()
    window.history.pushState({}, '', '/traders')
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  return (
    <div className="fixed inset-0 z-[80]">
      <div className="absolute inset-0 bg-black/58 backdrop-blur-[2px]" />
      <div className="relative flex min-h-screen items-center justify-center px-4 py-10 sm:px-6">
        <button
          type="button"
          onClick={handleContinue}
          className="absolute right-6 top-6 z-10 inline-flex h-10 w-10 items-center justify-center rounded-full border border-white/10 bg-white/5 text-zinc-400 transition hover:border-white/20 hover:bg-white/10 hover:text-white"
          aria-label={pickText({ zh: '跳过', en: 'Skip', de: 'Ueberspringen' })}
        >
          <X className="h-5 w-5" />
        </button>
        <div className="w-full max-w-[1120px]">
          <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div className="flex items-center gap-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-[22px] border border-nofx-gold/20 bg-nofx-gold/8 text-nofx-gold shadow-[0_0_30px_rgba(240,185,11,0.12)]">
                <Shield className="h-6 w-6" />
              </div>
              <div>
                <div
                  className={`font-semibold uppercase text-nofx-gold/80 ${
                    isZh ? 'text-[11px] tracking-[0.34em]' : 'text-[10px] tracking-[0.2em]'
                  }`}
                >
                  {pickText({
                    zh: '新手保护',
                    en: 'Beginner Guard',
                    de: 'Beginner Guard',
                  })}
                </div>
                <h1
                  className={`mt-2 font-bold leading-[1.04] text-white ${
                    isZh
                      ? 'text-[34px] tracking-tight sm:text-[44px] xl:text-[52px] xl:whitespace-nowrap'
                      : 'max-w-[720px] text-[27px] tracking-[-0.03em] sm:text-[35px] xl:text-[42px]'
                  }`}
                >
                  {pickText({
                    zh: '钱包已经帮你准备好了',
                    en: 'Your wallet is ready',
                    de: 'Dein Wallet ist bereit',
                  })}
                </h1>
              </div>
            </div>

            <div
              className={`pb-2 text-zinc-500 lg:text-right ${
                isZh
                  ? 'text-sm tracking-[0.18em] lg:whitespace-nowrap'
                  : 'text-[13px] tracking-[0.12em] lg:whitespace-nowrap'
              }`}
            >
              Claw402 + DeepSeek <span className="mx-2 text-zinc-700">·</span>
              {pickText({
                zh: '按次付费',
                en: 'Pay per call',
                de: 'Bezahlung pro Aufruf',
              })}
            </div>
          </div>

          <div className="overflow-hidden rounded-[32px] border border-white/10 bg-[linear-gradient(180deg,rgba(8,11,16,0.94),rgba(5,7,10,0.88))] shadow-[0_24px_120px_rgba(0,0,0,0.58)] backdrop-blur-2xl">
            {loading ? (
              <div className="flex min-h-[390px] items-center justify-center px-6 text-sm text-zinc-400">
                {pickText({
                  zh: '正在准备你的 Base 钱包...',
                  en: 'Preparing your Base wallet...',
                  de: 'Dein Base-Wallet wird vorbereitet...',
                })}
              </div>
            ) : data ? (
              <div className="grid lg:grid-cols-[0.82fr_1.18fr]">
                <section className="flex flex-col justify-center px-8 py-7 sm:px-9 lg:min-h-[430px]">
                  <div className="mx-auto w-full max-w-[248px] text-center">
                    <div className="mx-auto inline-flex rounded-[28px] border border-black/10 bg-white p-4 shadow-[0_20px_60px_rgba(255,255,255,0.08)]">
                      <QRCodeSVG value={data.address} size={164} level="M" />
                    </div>

                    <div className="mt-4 text-[15px] font-medium text-zinc-300">
                      {pickText({
                        zh: '充值地址（Base USDC）',
                        en: 'Deposit address (Base USDC)',
                        de: 'Einzahlungsadresse (Base USDC)',
                      })}
                    </div>

                    <div className="mt-4 flex items-center justify-between gap-3 rounded-[24px] border border-emerald-400/20 bg-emerald-500/7 px-5 py-3.5 shadow-[0_0_0_1px_rgba(16,185,129,0.08)]">
                      <div className="text-left">
                        <div className="flex items-baseline gap-3 font-mono font-bold tracking-tight text-emerald-300">
                          <span className="text-[22px]">{data.balance_usdc}</span>
                          <span className="text-[20px]">USDC</span>
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={() => void loadOnboarding(false)}
                        disabled={refreshingBalance}
                        className="inline-flex h-12 w-12 items-center justify-center rounded-2xl border border-emerald-300/20 bg-black/20 text-emerald-300 transition hover:bg-emerald-500/10 disabled:cursor-not-allowed disabled:opacity-60"
                        aria-label={pickText({
                          zh: '刷新余额',
                          en: 'Refresh balance',
                          de: 'Guthaben aktualisieren',
                        })}
                      >
                        <RefreshCw className={`h-4 w-4 ${refreshingBalance ? 'animate-spin' : ''}`} />
                      </button>
                    </div>

                    <div className="mt-4 text-sm text-zinc-500">
                      {pickText({
                        zh: '$5-$10 可以用很久',
                        en: '$5-$10 usually lasts a long time',
                        de: '$5-$10 reichen in der Regel lange',
                      })}
                    </div>
                  </div>
                </section>

                <section className="border-t border-white/8 px-8 py-7 lg:border-l lg:border-t-0 lg:px-9">
                  <div className="space-y-5">
                    <div>
                      <div className="mb-3 flex items-center gap-2 text-sm font-medium text-nofx-gold">
                        <Wallet className="h-4 w-4" />
                        <span>{pickText({ zh: '钱包地址', en: 'Wallet address', de: 'Wallet-Adresse' })}</span>
                      </div>
                      <div className="flex items-stretch gap-3">
                        <div className="min-w-0 flex-1 rounded-2xl border border-white/10 bg-black/30 px-5 py-3 font-mono text-[14px] text-zinc-200 shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">
                          <div className="break-all">{data.address}</div>
                        </div>
                        <button
                          type="button"
                          onClick={() =>
                            copyText(
                              data.address,
                              pickText({ zh: '地址', en: 'Address', de: 'Adresse' })
                            )
                          }
                          className="inline-flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/5 text-zinc-300 transition hover:border-white/20 hover:bg-white/10 hover:text-white"
                          aria-label={pickText({
                            zh: '复制地址',
                            en: 'Copy address',
                            de: 'Adresse kopieren',
                          })}
                        >
                          <Copy className="h-5 w-5" />
                        </button>
                      </div>
                    </div>

                    <div className="pt-1">
                      <div className="mb-3 flex items-center gap-2 text-sm font-medium text-nofx-gold">
                        <Shield className="h-4 w-4" />
                        <span>
                          {pickText({
                            zh: '私钥，请立即备份',
                            en: 'Private key, back it up now',
                            de: 'Private Key, bitte jetzt sichern',
                          })}
                        </span>
                      </div>
                      <div className="flex items-stretch gap-3">
                        <div className="min-w-0 flex-1 rounded-[24px] border border-nofx-gold/20 bg-[linear-gradient(180deg,rgba(32,25,7,0.44),rgba(14,10,3,0.28))] px-5 py-3 font-mono text-[13px] leading-6 text-amber-100 shadow-[0_0_0_1px_rgba(240,185,11,0.05)]">
                          <div className="overflow-x-auto whitespace-nowrap">{data.private_key}</div>
                        </div>
                        <div className="flex shrink-0 flex-col justify-end">
                          <button
                            type="button"
                            onClick={() =>
                              copyText(
                                data.private_key,
                                pickText({
                                  zh: '私钥',
                                  en: 'Private key',
                                  de: 'Private Key',
                                })
                              )
                            }
                            className="inline-flex h-14 w-14 items-center justify-center rounded-2xl border border-nofx-gold/20 bg-nofx-gold/10 text-nofx-gold transition hover:bg-nofx-gold/15"
                            aria-label={pickText({
                              zh: '复制私钥',
                              en: 'Copy private key',
                              de: 'Private Key kopieren',
                            })}
                          >
                            <Copy className="h-5 w-5" />
                          </button>
                        </div>
                      </div>
                    </div>

                    <div
                      className={`rounded-[24px] border border-white/15 bg-black/18 px-5 py-3.5 text-zinc-500 ${
                        isZh ? 'text-xs lg:whitespace-nowrap' : 'text-[11px] leading-6'
                      }`}
                    >
                      <span className="mr-2 text-zinc-600">•</span>
                      {noticeText}
                    </div>

                    {data.env_warning ? (
                      <div className="rounded-2xl border border-amber-500/20 bg-amber-500/10 px-4 py-3 text-sm text-amber-200">
                        {data.env_warning}
                      </div>
                    ) : null}

                    {error ? (
                      <div className="rounded-2xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                        {error}
                      </div>
                    ) : null}

                    <button
                      type="button"
                      onClick={handleContinue}
                      className={`mt-1 flex w-full items-center justify-center gap-3 rounded-[24px] bg-nofx-gold px-5 py-3.5 font-bold text-black shadow-[0_10px_40px_rgba(240,185,11,0.22)] transition hover:bg-yellow-400 ${
                        isZh ? 'text-[20px]' : 'text-[16px] sm:text-[18px]'
                      }`}
                    >
                      <span>
                        {pickText({
                          zh: '我已保存，进入下一步',
                          en: 'I saved it, continue',
                          de: 'Gespeichert, weiter',
                        })}
                      </span>
                      <ArrowRight className="h-5 w-5" />
                    </button>

                    {data.env_saved ? (
                      <div className="pt-1 text-xs text-zinc-600">
                        {pickText({
                          zh: `钱包信息已同步保存到 ${data.env_path || '.env'}`,
                          en: `Wallet details were also saved to ${data.env_path || '.env'}`,
                          de: `Wallet-Daten wurden auch in ${data.env_path || '.env'} gespeichert`,
                        })}
                      </div>
                    ) : null}
                  </div>
                </section>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}
