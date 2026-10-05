import React, { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { NotFound } from './NotFound.tsx'
import { StarButton, Views } from './Star.tsx'
import { Avatar } from '../../shared/ui/Avatar.tsx'
import { fetchEvent, formatDate, profilePath, startView, touchView, TYPE_LABEL, type HistEvent } from './api.ts'
import './public.css'

// Через сколько секунд активного просмотра предложить звезду.
const STAR_PROMPT_SECONDS = 45
// Длительность дописывается и по ходу просмотра: мобильный браузер может убить вкладку без pagehide.
const FLUSH_EVERY_SECONDS = 30

const storage = {
  get: (key: string) => { try { return localStorage.getItem(key) } catch { return null } },
  set: (key: string, value: string) => { try { localStorage.setItem(key, value) } catch { /* приватное окно */ } },
}

// Случайный id браузера: по нему считаются уникальные гости. Без localStorage живёт до перезагрузки.
let memoryVisitor = ''
function visitorId() {
  const saved = storage.get('hp-visitor')
  if (saved) return saved
  memoryVisitor ||= crypto.randomUUID()
  storage.set('hp-visitor', memoryVisitor)
  return memoryVisitor
}

const dismissedKey = (id: number) => `hp-star-prompt-dismissed:${id}`

// Сайт события на весь первый экран, под ним строка с названием и звездой, ниже - о событии. Считает активное время просмотра: секунда
// идёт, только пока вкладка видна и окно в фокусе. Фокус внутри iframe - тоже фокус документа.
export const Viewer: React.FC = () => {
  const { id: param } = useParams()
  const id = param && /^\d+$/.test(param) ? Number(param) : null
  const [event, setEvent] = useState<HistEvent | null | undefined>(undefined)
  const [failed, setFailed] = useState(false)
  // Просмотр записан (не автор) и набрал порог - можно предложить звезду.
  const [promptDue, setPromptDue] = useState(false)
  const [dismissed, setDismissed] = useState(() => id !== null && storage.get(dismissedKey(id)) !== null)

  useEffect(() => {
    if (id === null) return
    const ac = new AbortController()
    fetchEvent(id, ac.signal).then(setEvent, (e: Error) => { if (e.name !== 'AbortError') setFailed(true) })
    return () => ac.abort()
  }, [id])

  const ready = !!event
  useEffect(() => {
    if (!ready || id === null) return
    const visitor = visitorId()
    let viewId: number | null = null
    let active = 0, sent = 0
    const flush = () => {
      if (viewId === null || active === sent) return
      touchView(id, viewId, visitor, active)
      sent = active
    }
    // Старт через таймер: StrictMode в dev сразу снимает первый эффект, и лишний просмотр не пишется.
    const start = setTimeout(() => {
      void startView(id, visitor).then(v => { viewId = v })
    }, 0)
    const tick = setInterval(() => {
      if (document.visibilityState !== 'visible' || !document.hasFocus()) return
      active++
      if (active === STAR_PROMPT_SECONDS && viewId !== null) setPromptDue(true)
      if (active % FLUSH_EVERY_SECONDS === 0) flush()
    }, 1000)
    const onVisibility = () => { if (document.visibilityState === 'hidden') flush() }
    document.addEventListener('visibilitychange', onVisibility)
    window.addEventListener('pagehide', flush)
    return () => {
      clearTimeout(start)
      clearInterval(tick)
      document.removeEventListener('visibilitychange', onVisibility)
      window.removeEventListener('pagehide', flush)
      flush()
    }
  }, [ready, id])

  if (id === null || event === null) return <NotFound path={`/event/${param}`} kind="event" />

  const dismiss = () => {
    storage.set(dismissedKey(id), '1')
    setDismissed(true)
  }
  const showPrompt = promptDue && !dismissed && !!event && !event.starred

  return (
    <div className="pm-root sv-page">
      {/* Первый экран: сайт и строка под ним. Остальное о событии - ниже, прокруткой. */}
      <div className="sv-screen">
        <div className="sv-window pm-glass">
          {event
            ? <iframe className="sv-frame" src={event.siteUrl} title={event.title} sandbox="allow-scripts allow-forms allow-popups" />
            : <div className="sv-frame sv-frame-empty">{failed ? 'Нет связи с сервером' : 'Загружаем...'}</div>}
          <div className="sv-bar">
            <Link className="pm-close-sm sv-back" to={`/map?event=${id}`} aria-label="Назад к карте">
              <svg viewBox="0 0 24 24" width={16} height={16} aria-hidden="true" fill="none" stroke="currentColor"
                strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round"><path d="M15 5l-7 7 7 7" /></svg>
            </Link>
            <h1 className="sv-title">{event?.title}</h1>
            {showPrompt && (
              <span className="sv-prompt" role="status">
                Понравился сайт? Поставьте звезду
                <button type="button" className="pm-close-sm" onClick={dismiss} aria-label="Скрыть подсказку">×</button>
              </span>
            )}
            {event && <Views count={event.visitors} />}
            {event && (
              <StarButton id={event.id} stars={event.stars} starred={event.starred}
                onChange={s => setEvent({ ...event, ...s })} className="pm-btn-ghost pm-pill sv-star" />
            )}
          </div>
        </div>
      </div>
      {event && (
        <section className="sv-info" id="sv-info">
          <div className="pm-card-sub">
            <span className={`pm-card-type ${event.eventType}`}>{TYPE_LABEL[event.eventType]}</span>
            <i />
            <span>{formatDate(event.eventDate)}</span>
            {event.author && (<>
              <i />
              <Link className="sv-author" to={profilePath(event.author.publicId)} aria-label={`Автор сайта: ${event.author.firstName} ${event.author.lastName}`}>
                <Avatar seed={event.author.publicId} size={24} />
                {event.author.firstName} {event.author.lastName}
              </Link>
            </>)}
          </div>
          {event.description && <p className="sv-text">{event.description}</p>}
        </section>
      )}
    </div>
  )
}
