import React, { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { setStar, type StarState } from './api.ts'

const STAR = 'M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z'

// Звезда с числом. Своего состояния не держит: ответ сервера уходит в onChange, владелец данных
// обновляет событие или работу, и кнопка перерисовывается из них. Гостя ведёт на вход и обратно.
export const StarButton: React.FC<{
  id: number; stars: number; starred: boolean; onChange: (state: StarState) => void; className?: string
}> = ({ id, stars, starred, onChange, className = 'pm-btn-ghost pm-pill' }) => {
  const navigate = useNavigate()
  const location = useLocation()
  const [busy, setBusy] = useState(false)
  const toggle = async (ev: React.MouseEvent) => {
    ev.preventDefault()
    ev.stopPropagation()
    setBusy(true)
    const res = await setStar(id, !starred)
    setBusy(false)
    if (res === 'guest') navigate('/login', { state: { back: location.pathname + location.search } })
    else if (res) onChange(res)
  }
  return (
    <button type="button" className={`${className} pm-star${starred ? ' on' : ''}`} onClick={toggle} disabled={busy}
      aria-pressed={starred} aria-label={starred ? `Убрать звезду, звёзд: ${stars}` : `Поставить звезду, звёзд: ${stars}`}>
      <svg viewBox="0 0 24 24" width={18} height={18} aria-hidden="true" fill={starred ? 'currentColor' : 'none'}
        stroke="currentColor" strokeWidth={1.7} strokeLinejoin="round"><path d={STAR} /></svg>
      {stars}
    </button>
  )
}

const EYE = 'M2.5 12s3.5-6.5 9.5-6.5 9.5 6.5 9.5 6.5-3.5 6.5-9.5 6.5S2.5 12 2.5 12z'

// Просмотры сайта: уникальные посетители, как считает сервер. Только показ, не кнопка.
export const Views: React.FC<{ count: number; className?: string }> = ({ count, className = '' }) => (
  <span className={`pm-views ${className}`} title="Уникальные посетители" aria-label={`Просмотров: ${count}`}>
    <svg viewBox="0 0 24 24" width={18} height={18} aria-hidden="true" fill="none" stroke="currentColor"
      strokeWidth={1.7} strokeLinejoin="round"><path d={EYE} /><circle cx="12" cy="12" r="3" /></svg>
    {count}
  </span>
)
