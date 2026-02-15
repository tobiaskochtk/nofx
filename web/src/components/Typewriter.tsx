import { useEffect, useMemo, useRef, useState } from 'react'

interface TypewriterProps {
  lines: string[]
  typingSpeed?: number // ミリ秒/文字 | milliseconds per character
  lineDelay?: number // 各行終了時の追加待機時間 | extra wait at end of each line
  className?: string
  style?: React.CSSProperties
}

export default function Typewriter({
  lines,
  typingSpeed = 50,
  lineDelay = 600,
  className,
  style,
}: TypewriterProps) {
  const [typedLines, setTypedLines] = useState<string[]>([''])
  const [showCursor, setShowCursor] = useState(true)
  const lineIndexRef = useRef(0)
  const charIndexRef = useRef(0)
  const timerRef = useRef<number | null>(null)
  const blinkRef = useRef<number | null>(null)
  const sanitizedLines = useMemo(
    () => lines.map((l) => String(l ?? '')),
    [lines]
  )

  useEffect(() => {
    // ステートをリセット | Reset state
    lineIndexRef.current = 0
    charIndexRef.current = 0
    setTypedLines([''])

    function typeNext() {
      const currentLine = sanitizedLines[lineIndexRef.current] ?? ''
      if (charIndexRef.current < currentLine.length) {
        const ch = currentLine.charAt(charIndexRef.current)
        setTypedLines((prev) => {
          const next = [...prev]
          const lastIndex = next.length - 1
          next[lastIndex] = (next[lastIndex] ?? '') + ch
          return next
        })
        charIndexRef.current += 1
        timerRef.current = window.setTimeout(typeNext, typingSpeed)
      } else {
        // 行終了 | Line end
        if (lineIndexRef.current < sanitizedLines.length - 1) {
          lineIndexRef.current += 1
          charIndexRef.current = 0
          setTypedLines((prev) => [...prev, ''])
          timerRef.current = window.setTimeout(typeNext, lineDelay)
        } else {
          // 最後の行の入力完了 | Last line input complete
          timerRef.current = null
        }
      }
    }

    // 1フレーム遅延してタイピング開始、ステートがリセット済みであることを確認 | Delay one frame to start typing, ensure state is reset
    timerRef.current = window.setTimeout(typeNext, 0)

    // カーソル点滅 | Cursor blink
    blinkRef.current = window.setInterval(() => {
      setShowCursor((v) => !v)
    }, 500)

    return () => {
      if (timerRef.current) window.clearTimeout(timerRef.current)
      if (blinkRef.current) window.clearInterval(blinkRef.current)
    }
  }, [sanitizedLines, typingSpeed, lineDelay])

  const displayText = useMemo(
    () => typedLines.join('\n').replace(/undefined/g, ''),
    [typedLines]
  )

  return (
    <pre className={className} style={{ whiteSpace: 'pre-wrap', ...style }}>
      {displayText}
      <span style={{ opacity: showCursor ? 1 : 0 }}> ▍</span>
    </pre>
  )
}
