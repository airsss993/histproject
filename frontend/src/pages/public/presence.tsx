import React, { useEffect, useRef, useState } from 'react'

/** Сколько слой живёт после закрытия: не меньше анимаций ухода в public.css (pm-out, лист). */
export const LEAVE_MS = 180

/** true ещё LEAVE_MS после того, как open стал false. Считается в рендере, чтобы слой не пропал ни на кадр. */
export function useLeaving(open: boolean) {
  const [prev, setPrev] = useState(open)
  const [leaving, setLeaving] = useState(false)
  if (open !== prev) {
    setPrev(open)
    setLeaving(!open)
  }
  useEffect(() => {
    if (!leaving) return
    const t = setTimeout(() => setLeaving(false), LEAVE_MS)
    return () => clearTimeout(t)
  }, [leaving])
  return leaving
}

type El = React.ReactElement<{ className?: string }>

/**
 * Показывает последний элемент с классом out, пока играет анимация ухода.
 * Элемент не пересоздаётся: состояние внутри него живёт до конца анимации.
 */
export function Presence({ children }: { children: El | false | null | undefined }) {
  const leaving = useLeaving(!!children)
  const last = useRef<El | undefined>(undefined)
  if (children) {
    last.current = children
    return children
  }
  if (!leaving || !last.current) return null
  const el = last.current
  return React.cloneElement(el, { className: `${el.props.className ?? ''} out` })
}
