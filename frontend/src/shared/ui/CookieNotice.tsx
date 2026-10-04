import React, { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'

// Уведомление о cookie входа. Согласие на него не нужно, только уведомить;
// закрытие запоминается в localStorage. Хранилище может быть недоступно - тогда плашка просто покажется снова.
const KEY = 'cookie-notice'
const seen = () => { try { return localStorage.getItem(KEY) === '1' } catch { return false } }

export const CookieNotice: React.FC = () => {
  const [open, setOpen] = useState(() => !seen())
  const { pathname } = useLocation()
  if (!open || pathname.startsWith('/admin')) return null
  const close = () => {
    try { localStorage.setItem(KEY, '1') } catch { /* приватный режим */ }
    setOpen(false)
  }
  return (
    <div className="pm-root pm-cookie pm-glass" role="region" aria-label="Уведомление о cookie">
      <p>
        Сайт хранит в браузере cookie для входа в аккаунт.
        Подробнее - в <Link to="/privacy">Политике обработки персональных данных</Link>.
      </p>
      <button type="button" className="pm-btn pm-pill" onClick={close}>Понятно</button>
    </div>
  )
}
