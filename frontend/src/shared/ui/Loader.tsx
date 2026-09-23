import React, { useEffect, useRef, useState } from 'react'

/** Загрузка держится не меньше MIN_MS, чтобы не мигать; удачная затем DONE_MS показывает галочку. */
const MIN_MS = 900
const DONE_MS = 1000

export type Phase = 'load' | 'done' | 'idle'

/**
 * Фаза индикатора поверх настоящего флага загрузки. ok - загрузилось с данными: только тогда галочка,
 * ошибка и пустой ответ сразу уходят в idle. Контент показывать на idle.
 */
export function useSettle(loading: boolean, ok: boolean): Phase {
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
      t = setTimeout(() => setPhase(p => (p === 'done' ? 'idle' : p)), DONE_MS)
    }, Math.max(0, since.current + MIN_MS - Date.now()))
    return () => clearTimeout(t)
  }, [loading, ok])
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
