/**
 * 文本工具
 *
 * stripLeadingIcons: 去掉翻译文案或标题前面用于装饰的 Emoji/符号，
 * 以便在组件里自行放置图标时不重复显示。
 */

/**
 * 去掉开头的装饰性 Emoji/符号以及随后的分隔符（空格/冒号/点号等）。
 */
export function stripLeadingIcons(input: string | undefined | null): string {
  if (!input) return ''
  let s = String(input)

  // 1) 去除常见的 Emoji/符号块（箭头、杂项符号、几何图形、表情等）
  //    覆盖常见范围，兼容性好于使用 Unicode 属性类。
  s = s.replace(
    /^[\s\u2190-\u21FF\u2300-\u23FF\u2460-\u24FF\u25A0-\u25FF\u2600-\u27BF\u2B00-\u2BFF\u1F000-\u1FAFF]+/u,
    ''
  )

  // 2) 去掉开头可能残留的分隔符（空格、连字符、冒号、居中点等）
  s = s.replace(/^[\s\-:•·]+/, '')

  return s.trim()
}

/**
 * True when CoT has human-readable text (not only JSON/code-fence artifacts).
 */
export function hasReadableCoT(input: string | undefined | null): boolean {
  if (!input) return false

  const raw = String(input).replace(/[\u200B\u200C\u200D\uFEFF]/g, '').trim()
  if (!raw) return false

  if (isFenceArtifactText(raw)) {
    return false
  }

  const fencedMatch = raw.match(/^```json\s*([\s\S]*?)\s*```$/i)
  const candidate = (fencedMatch ? fencedMatch[1] : raw).trim()
  if (!candidate) return false

  if ((candidate.startsWith('{') || candidate.startsWith('[')) && isValidJSON(candidate)) {
    return false
  }

  return true
}

/**
 * Filters out noisy execution-log lines that are just markdown fence fragments.
 */
export function isNoiseExecutionLog(input: string | undefined | null): boolean {
  return normalizeExecutionLogLine(input) === ''
}

/**
 * Removes markdown fence artifact lines from a log item and returns readable text.
 */
export function normalizeExecutionLogLine(input: string | undefined | null): string {
  if (!input) return ''
  const raw = String(input).replace(/[\u200B\u200C\u200D\uFEFF]/g, '').trim()
  if (!raw) return ''

  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0 && !isFenceArtifactText(line))

  if (lines.length === 0) return ''
  return lines.join('\n').trim()
}

function isFenceArtifactText(input: string): boolean {
  const lower = input.toLowerCase().trim()
  if (lower === '```' || lower === '```json' || lower === '```jsonc' || lower === 'json' || lower === 'jsonc') {
    return true
  }

  const lines = input
    .split(/\r?\n/)
    .map((line) => line.trim().toLowerCase())
    .filter((line) => line.length > 0)

  if (lines.length === 0) return true
  return lines.every((line) => line.startsWith('```') || line === 'json' || line === 'jsonc')
}

function isValidJSON(value: string): boolean {
  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

export default { stripLeadingIcons, hasReadableCoT, isNoiseExecutionLog, normalizeExecutionLogLine }
