import React, { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Logo, ROMAN, useIsMobile } from './PublicMap.tsx'
import { PinShape } from './MapView.tsx'
import { NotFound } from './NotFound.tsx'
import { Avatar } from '../../shared/ui/Avatar.tsx'
import {
  fetchMe, fetchProfile, formatDate, logoutUser, TYPE_LABEL,
  type Profile, type User, type Work, type WorkStatus,
} from './api.ts'
import './public.css'
import './profile.css'

// Путь заявки: доля пройденного показывается полосой, как у доставки в референсе.
const STATUS: Record<WorkStatus, { label: string; tone: string; step: number; note: string }> = {
  processing: { label: 'Собираем сайт', tone: 'wait', step: 0.2, note: 'Разбираем архив и снимаем превью' },
  new: { label: 'Ждёт проверки', tone: 'wait', step: 0.45, note: 'Заявка в очереди к модератору' },
  review: { label: 'На проверке', tone: 'work', step: 0.75, note: 'Модератор смотрит событие и архив' },
  published: { label: 'Опубликовано', tone: 'ok', step: 1, note: 'Событие на карте' },
  rejected: { label: 'Отклонена', tone: 'bad', step: 1, note: 'Исправьте замечание и отправьте заново' },
}

const ICON: Record<WorkStatus, string> = {
  processing: 'M12 3a9 9 0 1 0 9 9M12 7v5l3 2',
  new: 'M3 7l9 6 9-6M3 7h18v10H3z',
  review: 'M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6-10-6-10-6zM12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z',
  published: 'M12 3a9 9 0 1 0 9 9 9 9 0 0 0-9-9zM8 12.5l2.5 2.5L16 9.5',
  rejected: 'M12 3a9 9 0 1 0 9 9 9 9 0 0 0-9-9zM9 9l6 6M15 9l-6 6',
}

const Glyph: React.FC<{ d: string }> = ({ d }) => (
  <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor"
    strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d={d} />
  </svg>
)

const StatusChip: React.FC<{ status: WorkStatus }> = ({ status }) => (
  <span className={`pf-status ${STATUS[status].tone}`}><Glyph d={ICON[status]} />{STATUS[status].label}</span>
)

// Фильтры заявок автора. Пустые группы не показываем, так что "ничего не найдено" недостижимо.
type Filter = 'all' | 'published' | 'work' | 'rejected'
const FILTERS: { key: Filter; label: string; icon: string; has: (work: Work) => boolean }[] = [
  { key: 'all', label: 'Все', icon: 'M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z', has: () => true },
  { key: 'published', label: 'Опубликованы', icon: ICON.published, has: w => w.status === 'published' },
  { key: 'work', label: 'В работе', icon: ICON.review, has: w => w.status !== 'published' && w.status !== 'rejected' },
  { key: 'rejected', label: 'Отклонены', icon: ICON.rejected, has: w => w.status === 'rejected' },
]

// "20 марта 2026": месяц с числом склоняется, без числа Intl даёт именительный ("март 2026").
const joined = (iso: string) =>
  new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }).replace(' г.', '')

const plural = (n: number, one: string, few: string, many: string) => {
  const d = n % 10, h = n % 100
  return d === 1 && h !== 11 ? one : d >= 2 && d <= 4 && (h < 12 || h > 14) ? few : many
}

// Шкала веков IX-XXI, та же, что на карте: 801 год - левый край, 2100 - правый.
const RAIL_FROM = 801, RAIL_TO = 2100
const yearOfWork = (work: Work) => parseInt(work.eventDate.slice(0, 4), 10)
const roman = (year: number) => ROMAN[Math.ceil(year / 100) - 9] ?? ''

const span = (works: Work[]) => {
  if (works.length === 0) return ''
  const years = works.map(yearOfWork)
  const from = roman(Math.min(...years)), to = roman(Math.max(...years))
  return from === to ? `${from} век` : `${from}-${to} век`
}

// События автора на шкале веков: та же капля, что стоит на карте, на своём годе.
const Timeline: React.FC<{ works: Work[] }> = ({ works }) => (
  <div className="pf-rail" aria-hidden="true">
    <div className="pf-rail-pins">
      {works.map(work => {
        const year = Math.min(RAIL_TO, Math.max(RAIL_FROM, yearOfWork(work)))
        return (
          <span key={work.id} className="pf-pin" style={{ left: `${((year - RAIL_FROM) / (RAIL_TO - RAIL_FROM)) * 100}%` }}
            title={`${work.title} · ${formatDate(work.eventDate)}`}>
            <svg viewBox="0 0 64 64" width="16" height="16"><PinShape type={work.eventType} size={16} /></svg>
          </span>
        )
      })}
    </div>
    <div className="pf-rail-scale">{ROMAN.map(n => <span key={n}>{n}</span>)}</div>
  </div>
)

// Опубликованная работа - плитка-витрина: главное в ней превью сайта, а не строка с датой.
const Tile: React.FC<{ work: Work }> = ({ work }) => (
  <a className="pf-tile pm-glass" href={work.siteUrl} target="_blank" rel="noreferrer">
    <span className="pf-tile-frame">
      {work.screenshotUrl
        ? <img className="pf-tile-shot" src={work.screenshotUrl} alt="" loading="lazy" />
        : <span className="pf-tile-shot pm-hatch" />}
    </span>
    <span className="pf-tile-body">
      <span className="pf-tile-title">{work.title}</span>
      <span className="pm-dim-text">{TYPE_LABEL[work.eventType]} · {formatDate(work.eventDate)}</span>
    </span>
    <span className="pf-tile-go">Открыть сайт</span>
  </a>
)

// Незавершённая заявка - строка с путём, как в референсе отслеживания заказа. Видит только автор.
const WorkCard: React.FC<{ work: Work }> = ({ work }) => {
  const status = STATUS[work.status]
  return (
    <article className="pf-card pm-glass">
      {work.screenshotUrl
        ? <img className="pf-shot" src={work.screenshotUrl} alt="" loading="lazy" />
        : <span className="pf-shot pm-hatch" />}
      <div className="pf-card-body">
        <h3>{work.title}</h3>
        <p className="pm-dim-text">
          {TYPE_LABEL[work.eventType]} &middot; {formatDate(work.eventDate)} &middot; заявка №{work.id}
        </p>
        {work.comment && <p className="pf-comment">{work.comment}</p>}
        <div className="pf-actions">
          {work.status === 'rejected' && (
            <Link className="pm-btn-ghost pm-pill" to="/map?propose=1">Отправить заново</Link>
          )}
        </div>
      </div>
      <div className="pf-card-side">
        <StatusChip status={work.status} />
        <span className="pm-dim-text">{formatDate(work.createdAt)}</span>
      </div>
      {/* Полоса пути - только пока заявка идёт: у отклонённой всё сказано чипом и замечанием. */}
      {status.step < 1 && (
        <div className={`pf-track ${status.tone}`} style={{ '--p': `${status.step * 100}%` } as React.CSSProperties}>
          <i />
          <span className="pm-dim-text">{status.note}</span>
        </div>
      )}
    </article>
  )
}

const Title: React.FC<{ count: number; children: React.ReactNode }> = ({ count, children }) => (
  <h2 className="pf-title">{children}<span className="pf-count">{count}</span></h2>
)

export const ProfilePage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const userId = Number(id)
  const [state, setState] = useState<{ k: 'load' } | { k: 'ready'; profile: Profile } | { k: 'none' } | { k: 'error' }>({ k: 'load' })
  const [me, setMe] = useState<User | null>(null)
  const [filter, setFilter] = useState<Filter>('all')

  useEffect(() => {
    const ac = new AbortController()
    fetchMe(ac.signal).then(setMe, () => {})
    return () => ac.abort()
  }, [])

  useEffect(() => {
    if (!Number.isInteger(userId) || userId <= 0) return setState({ k: 'none' })
    const ac = new AbortController()
    setState({ k: 'load' })
    fetchProfile(userId, ac.signal).then(
      profile => setState(profile ? { k: 'ready', profile } : { k: 'none' }),
      (e: Error) => { if (e.name !== 'AbortError') setState({ k: 'error' }) },
    )
    return () => ac.abort()
  }, [userId])

  if (state.k === 'none') return <NotFound path={`/u/${id}`} kind="profile" />

  const profile = state.k === 'ready' ? state.profile : null
  const works = profile?.works ?? []
  const owner = !!profile?.owner
  const published = works.filter(w => w.status === 'published')
  const inWork = works.filter(w => w.status !== 'published' && w.status !== 'rejected').length
  // Разделы сами исчезают, когда фильтр их опустошает: отдельной ветки на каждый фильтр не нужно.
  const shown = works.filter(FILTERS.find(f => f.key === filter)!.has)
  const shownPending = shown.filter(w => w.status !== 'published')
  const shownPublished = shown.filter(w => w.status === 'published')

  return (
    <div className="pm-root pf-page">
      <div role="banner" className="pf-header">
        <Logo short={mobile} onClick={() => navigate('/map')} />
        <div className="pm-grow" />
        {!mobile && <Link to="/about" className="pm-btn-text">О проекте</Link>}
        {me
          ? <span className="pm-user">
              <Link to={`/u/${me.id}`} className="pm-user-link" title="Мой профиль">
                <Avatar seed={me.id} size={32} />
                {!mobile && <span className="pm-user-name">{me.firstName} {me.lastName[0]}.</span>}
              </Link>
              <button type="button" className="pm-btn-text" onClick={() => { logoutUser(); setMe(null) }}>Выйти</button>
            </span>
          : <Link to="/login" className="pm-btn-text">Войти</Link>}
        {!mobile && <Link to="/map?propose=1" className="pm-btn-ghost pm-pill">Предложить событие</Link>}
      </div>

      <main className="pf-main">
        {state.k === 'load' && <div className="pf-note pm-glass">Загружаем профиль</div>}
        {state.k === 'error' && (
          <div className="pf-note pm-glass">
            Нет связи с сервером
            <button type="button" className="pm-btn-ghost pm-pill" onClick={() => setState({ k: 'load' })}>Повторить</button>
          </div>
        )}

        {profile && (
          <>
            <section className="pf-hero pm-glass">
              <Avatar seed={profile.user.id} size={mobile ? 72 : 104} className="pf-avatar" />
              <div className="pf-hero-name">
                <h1>{profile.user.firstName} {profile.user.lastName}</h1>
                <p className="pm-dim-text">
                  На карте с {joined(profile.user.createdAt)}
                  {published.length > 0 && <> &middot; <span className="pf-nowrap">{span(published)}</span></>}
                </p>
              </div>
              <dl className="pf-stats">
                <div><dt>Опубликовано</dt><dd>{published.length}</dd></div>
                {owner && <div><dt>В работе</dt><dd>{inWork}</dd></div>}
                {owner && <div><dt>Всего заявок</dt><dd>{works.length}</dd></div>}
              </dl>
              {published.length > 0 && <Timeline works={published} />}
            </section>

            {owner && inWork > 0 && (
              <div className="pf-strip">
                <Glyph d={ICON.review} />
                {inWork} {plural(inWork, 'заявка ждёт', 'заявки ждут', 'заявок ждут')} модератора. Ответ придёт на почту.
                <div className="pm-grow" />
                <Link to="/map?propose=1" className="pm-btn pm-pill">Предложить ещё</Link>
              </div>
            )}

            {/* Гостю фильтровать нечего: у него в профиле только опубликованное. */}
            {owner && works.length > 0 && (
              <div className="pm-chips pf-filters" aria-label="Фильтр заявок">
                {FILTERS.map(({ key, label, icon, has }) => {
                  const count = works.filter(has).length
                  if (count === 0 && key !== 'all') return null
                  return (
                    <button type="button" key={key} className={filter === key ? 'on' : ''}
                      aria-pressed={filter === key} onClick={() => setFilter(key)}>
                      <span className="pf-chip-i"><Glyph d={icon} /></span>{label}<span className="pf-chip-n">{count}</span>
                    </button>
                  )
                })}
              </div>
            )}

            {shownPending.length > 0 && (
              <>
                <Title count={shownPending.length}>Заявки</Title>
                <div className="pf-list">{shownPending.map(w => <WorkCard key={w.id} work={w} />)}</div>
              </>
            )}

            {shownPublished.length > 0 && (
              <>
                <Title count={shownPublished.length}>{owner ? 'Опубликовано' : 'Сайты на карте'}</Title>
                <div className="pf-gallery">{shownPublished.map(w => <Tile key={w.id} work={w} />)}</div>
              </>
            )}

            {works.length === 0 && (
              <div className="pf-note pm-glass">
                {owner ? 'Пока ни одного сайта на карте. Предложите событие - и оно появится здесь.' : 'У автора пока нет опубликованных сайтов.'}
                {owner && <Link to="/map?propose=1" className="pm-btn pm-pill">Предложить событие</Link>}
              </div>
            )}
          </>
        )}
      </main>
    </div>
  )
}
