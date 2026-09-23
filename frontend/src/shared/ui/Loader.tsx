import React, { useEffect, useRef, useState } from 'react'

/** Загрузка держится не меньше MIN_MS, чтобы не мигать; удачная затем DONE_MS показывает галочку. */
const MIN_MS = 900
const DONE_MS = 1000

export type Phase = 'load' | 'done' | 'idle'

/**
 * Фаза индикатора поверх настоящего флага загрузки. ok - загрузилось с данными: только тогда галочка,
 * ошибка и пустой ответ сразу уходят в idle. Контент показывать на idle.
 */
export function useSettle(loading: boolean, ok: boolean, min = MIN_MS, hold = DONE_MS): Phase {
  const [phase, setPhase] = useState<Phase>(loading ? 'load' : 'idle')
  const since = useRef(0)
  useEffect(() => {
    if (loading) {
      since.current = Date.now()
      setPhase('load')
      return
    }
    let t = setTimeout(() => {
      setPhase(p => (p === 'load' ? (ok ? 'done' : 'idle') : p))
      t = setTimeout(() => setPhase(p => (p === 'done' ? 'idle' : p)), hold)
    }, Math.max(0, since.current + min - Date.now()))
    return () => clearTimeout(t)
  }, [loading, ok, min, hold])
  return phase
}

/** Кольцо: пока грузится - бегущая дуга, на done замыкается и рисует галочку. */
export const LoadMark: React.FC<{ done?: boolean; size?: number }> = ({ done, size = 40 }) => (
  <svg className={`ui-load${done ? ' done' : ''}`} width={size} height={size} viewBox="0 0 40 40" aria-hidden="true">
    <circle className="ui-load-track" cx="20" cy="20" r="17" />
    <circle className="ui-load-arc" cx="20" cy="20" r="17" pathLength="100" />
    <path className="ui-load-check" d="M13 20.5l5 5 9-10" pathLength="100" />
  </svg>
)

/** Строка загрузки: кольцо и подпись, на done подпись меняется и строка гаснет к концу DONE_MS. */
export const Loading: React.FC<{ phase: Phase; label: string; done: string; className?: string }> = ({ phase, label, done, className }) => (
  <div className={`ui-loading${phase === 'done' ? ' done' : ''}${className ? ` ${className}` : ''}`} role="status">
    <LoadMark done={phase === 'done'} size={28} />
    <span key={phase}>{phase === 'done' ? done : label}</span>
  </div>
)

const LINES = [
  'Разворачиваем карту России',
  'Собираем события с IX по XXI век',
  'Сверяем даты по летописям',
  'Отмечаем места сражений',
  'Поднимаем архивы экспедиций',
  'Вспоминаем имена героев',
]
const LINE_MS = 1400
/** Заставка держится хотя бы две строки. */
export const SPLASH_MS = 2 * LINE_MS
/** Итог заставки: галочка, затем приближение логотипа (.ui-splash.done в ui.css). */
export const SPLASH_DONE_MS = 1500

/**
 * Отсчёт с первого показа заставки: заглушка на время загрузки кода и заставка самой страницы -
 * разные элементы, и без общего отсчёта логотип и строки начинались бы заново.
 */
let bootAt = 0

/** Заставка на весь экран: логотип и что сейчас делается; на done - галочка и итог, потом логотип наезжает и гаснет. */
export const Splash: React.FC<{ phase: Phase; done?: string; onCancel?: () => void }> = ({ phase, done, onCancel }) => {
  const [since] = useState(() => {
    if (!bootAt) bootAt = Date.now()
    return Date.now() - bootAt
  })
  const [line, setLine] = useState(() => Math.floor(since / LINE_MS) % LINES.length)
  useEffect(() => {
    if (phase !== 'load') return
    const t = setInterval(() => setLine(i => (i + 1) % LINES.length), LINE_MS)
    return () => clearInterval(t)
  }, [phase])
  const ok = phase === 'done'
  return (
    <div className={`pm-root ui-splash${ok ? ' done' : ''}`} role="status" style={{ '--since': `-${since}ms` } as React.CSSProperties}>
      <div className="ui-splash-brand">
        <img className="ui-splash-logo" src="/logo.svg" alt="" />
        <div className="ui-splash-name">Россия - страна Героев</div>
      </div>
      <div className="ui-splash-line">
        <LoadMark done={ok} size={22} />
        <span key={ok ? 'done' : line}>{ok ? done : `${LINES[line]}...`}</span>
      </div>
      {onCancel && <button type="button" className="pm-btn-text ui-splash-cancel" onClick={onCancel}>Отмена</button>}
    </div>
  )
}
