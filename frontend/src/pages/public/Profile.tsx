import React, { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import { HeaderNav, Logo, ROMAN, useIsMobile } from './PublicMap.tsx'
import { PinShape } from './MapView.tsx'
import { NotFound } from './NotFound.tsx'
import { StarButton, Views } from './Star.tsx'
import { Avatar } from '../../shared/ui/Avatar.tsx'
import { Loading, useSettle } from '../../shared/ui/Loader.tsx'
import { Check } from '../../shared/ui/Field.tsx'
import {
  deleteAccount, eventPath, fetchMe, fetchProfile, profilePath, formatDate, logoutUser, setShowLastName, TYPE_LABEL,
  type Profile, type StarState, type User, type Work, type WorkStatus,
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
          <span key={work.publicId} className="pf-pin" style={{ left: `${((year - RAIL_FROM) / (RAIL_TO - RAIL_FROM)) * 100}%` }}
            title={`${work.title} · ${formatDate(work.eventDate)}`}>
            <svg viewBox="0 0 64 64" width="16" height="16"><PinShape type={work.eventType} size={16} /></svg>
          </span>
        )
      })}
    </div>
    <div className="pf-rail-scale">{ROMAN.map(n => <span key={n}>{n}</span>)}</div>
  </div>
)

// "3 мин 20 с", "45 с", "2 ч 5 мин".
const duration = (seconds: number) => {
  const s = Math.round(seconds), h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60)
  if (h) return m ? `${h} ч ${m} мин` : `${h} ч`
  if (m) return s % 60 ? `${m} мин ${s % 60} с` : `${m} мин`
  return `${s} с`
}

// Статистика просмотров сайта. Сервер отдаёт её только владельцу профиля.
const WorkStats: React.FC<{ stats: NonNullable<Work['stats']> }> = ({ stats: { views, visitors, activeSeconds } }) => (
  <span className="pf-tile-stats">
    <span>{views} {plural(views, 'просмотр', 'просмотра', 'просмотров')}</span>
    <span>{visitors} {plural(visitors, 'посетитель', 'посетителя', 'посетителей')}</span>
    <span>всего {duration(activeSeconds)}</span>
    {views > 0 && <span>в среднем {duration(activeSeconds / views)}</span>}
  </span>
)

// Опубликованная работа - плитка-витрина: главное в ней превью сайта, а не строка с датой.
// Звезда - кнопка, поэтому плитка не ссылка целиком: ссылка растянута под звездой.
const Tile: React.FC<{ work: Work; i: number; onStar: (star: StarState) => void }> = ({ work, i, onStar }) => (
  <div className="pf-tile pm-glass" style={{ '--i': i } as React.CSSProperties}>
    {work.eventId !== undefined && (
      <StarButton id={work.eventId} stars={work.stars} starred={work.starred} onChange={onStar} className="pm-btn-ghost pm-pill pf-tile-star" />
    )}
    <Link className="pf-tile-link" to={work.eventId !== undefined ? eventPath(work.eventId) : '/map'}>
      <span className="pf-tile-frame">
        {work.screenshotUrl
          ? <img className="pf-tile-shot" src={work.screenshotUrl} alt="" loading="lazy" />
          : <span className="pf-tile-shot pm-hatch" />}
      </span>
      <span className="pf-tile-body">
        <span className="pf-tile-title">{work.title}</span>
        <span className="pm-dim-text pf-tile-meta">
          {TYPE_LABEL[work.eventType]} · {formatDate(work.eventDate)}
          {work.eventId !== undefined && <Views count={work.visitors} />}
        </span>
      </span>
      {work.stats && <WorkStats stats={work.stats} />}
      <span className="pf-tile-go">Открыть сайт</span>
    </Link>
  </div>
)

// Незавершённая заявка - строка с путём, как в референсе отслеживания заказа. Видит только автор.
const WorkCard: React.FC<{ work: Work; i: number }> = ({ work, i }) => {
  const status = STATUS[work.status]
  return (
    <article className="pf-card pm-glass" style={{ '--i': i } as React.CSSProperties}>
      {work.screenshotUrl
        ? <img className="pf-shot" src={work.screenshotUrl} alt="" loading="lazy" />
        : <span className="pf-shot pm-hatch" />}
      <div className="pf-card-body">
        <h3>{work.title}</h3>
        <p className="pm-dim-text">
          {TYPE_LABEL[work.eventType]} &middot; {formatDate(work.eventDate)} &middot; <span className="pf-nowrap">заявка {work.publicId}</span>
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

// Настройки видит только владелец: показ фамилии (согласие на распространение) и удаление учётки.
const Settings: React.FC<{ me: User; onChange: (me: User) => void }> = ({ me, onChange }) => {
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  const [confirm, setConfirm] = useState(false)
  const [error, setError] = useState('')
  const toggle = async (on: boolean) => {
    setBusy(true)
    setError('')
    const res = await setShowLastName(on)
    setBusy(false)
    if (res.ok) onChange(res.data.user)
    else setError('Не удалось сохранить. Попробуйте ещё раз.')
  }
  const remove = async () => {
    setBusy(true)
    setError('')
    const res = await deleteAccount()
    if (res.ok) return navigate('/map', { replace: true })
    setBusy(false)
    setError('Не удалось удалить аккаунт. Попробуйте ещё раз.')
  }
  return (
    <section className="pf-settings pm-glass">
      <h2 className="pf-title">Настройки</h2>
      <Check checked={me.showLastName} disabled={busy} onChange={v => { void toggle(v) }}>
        Показывать мою фамилию целиком в профиле и в авторстве событий
        (<Link to="/consent/distribution" target="_blank">условия</Link>). Без этого видна только первая буква.
      </Check>
      <div className="pf-settings-row">
        {confirm ? (
          <>
            <p>Удалим учётную запись, звёзды и просмотры. Опубликованные события останутся на карте без имени автора. Отменить нельзя.</p>
            <div className="pf-actions">
              <button type="button" className="pm-btn danger pm-pill" disabled={busy} onClick={() => { void remove() }}>Удалить навсегда</button>
              <button type="button" className="pm-btn-ghost pm-pill" disabled={busy} onClick={() => setConfirm(false)}>Отмена</button>
            </div>
          </>
        ) : (
          <button type="button" className="pm-btn-ghost danger pm-pill" onClick={() => setConfirm(true)}>Удалить аккаунт</button>
        )}
        <Link to="/privacy" className="pf-settings-link">Как мы храним данные</Link>
      </div>
      {error && <p className="pm-field-error" role="alert">{error}</p>}
    </section>
  )
}

const Title: React.FC<{ count: number; children: React.ReactNode }> = ({ count, children }) => (
  <h2 className="pf-title">{children}<span className="pf-count">{count}</span></h2>
)

export const ProfilePage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const mobile = useIsMobile()
  // Старые ссылки /u/RHU-7K3F9Q переписываем на /user/rhu-7k3f9q.
  const { pathname } = useLocation()
  useEffect(() => { if (id && pathname !== profilePath(id)) navigate(profilePath(id), { replace: true }) }, [id, pathname, navigate])
  const [state, setState] = useState<{ k: 'load' } | { k: 'ready'; profile: Profile } | { k: 'none' } | { k: 'error' }>({ k: 'load' })
  const [me, setMe] = useState<User | null>(null)
  const [filter, setFilter] = useState<Filter>('all')
  const [retry, setRetry] = useState(0)

  useEffect(() => {
    const ac = new AbortController()
    fetchMe(ac.signal).then(setMe, () => {})
    return () => ac.abort()
  }, [])

  useEffect(() => {
    if (!id) return setState({ k: 'none' })
    const ac = new AbortController()
    setState({ k: 'load' })
    fetchProfile(id, ac.signal).then(
      profile => setState(profile ? { k: 'ready', profile } : { k: 'none' }),
      (e: Error) => { if (e.name !== 'AbortError') setState({ k: 'error' }) },
    )
    return () => ac.abort()
  }, [id, retry])

  const phase = useSettle(state.k === 'load', state.k === 'ready')
  const onStar = (publicId: string, star: StarState) => setState(st => st.k !== 'ready' ? st : {
    ...st, profile: { ...st.profile, works: st.profile.works.map(w => w.publicId === publicId ? { ...w, ...star } : w) },
  })

  if (state.k === 'none') return <NotFound path={profilePath(id ?? '')} kind="profile" />

  const profile = state.k === 'ready' && phase === 'idle' ? state.profile : null
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
        <HeaderNav me={me} mobile={mobile} onLogout={() => { void logoutUser().then(() => navigate('/map')) }} onPropose={() => navigate('/map?propose=1')} />
      </div>

      <main className="pf-main">
        {phase !== 'idle' && <Loading phase={phase} label="Загружаем профиль" done="Профиль загружен" className="pf-note pm-glass" />}
        {state.k === 'error' && phase === 'idle' && (
          <div className="pf-note pm-glass">
            Нет связи с сервером
            <button type="button" className="pm-btn-ghost pm-pill" onClick={() => setRetry(n => n + 1)}>Повторить</button>
          </div>
        )}

        {profile && (
          <>
            <section className="pf-hero pm-glass">
              <Avatar seed={profile.user.publicId} size={mobile ? 72 : 104} className="pf-avatar" />
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
                <div className="pf-list">{shownPending.map((w, i) => <WorkCard key={w.publicId} work={w} i={i} />)}</div>
              </>
            )}

            {shownPublished.length > 0 && (
              <>
                <Title count={shownPublished.length}>{owner ? 'Опубликовано' : 'Сайты на карте'}</Title>
                <div className="pf-gallery">{shownPublished.map((w, i) => <Tile key={w.publicId} work={w} i={i} onStar={star => onStar(w.publicId, star)} />)}</div>
              </>
            )}

            {works.length === 0 && (
              <div className="pf-note pm-glass">
                {owner ? 'Пока ни одного сайта на карте. Предложите событие - и оно появится здесь.' : 'У автора пока нет опубликованных сайтов.'}
                {owner && <Link to="/map?propose=1" className="pm-btn pm-pill">Предложить событие</Link>}
              </div>
            )}

            {owner && me && (
              <Settings me={me} onChange={next => {
                setMe(next)
                // Владелец видит в шапке то же, что гости: без согласия - первую букву фамилии.
                setState(st => st.k !== 'ready' ? st : {
                  ...st, profile: { ...st.profile, user: { ...st.profile.user, lastName: next.showLastName ? next.lastName : `${next.lastName.slice(0, 1)}.` } },
                })
              }} />
            )}
          </>
        )}
      </main>
    </div>
  )
}
