import React, { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Logo, useIsMobile } from './PublicMap.tsx'
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

const Glyph: React.FC<{ status: WorkStatus }> = ({ status }) => (
  <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor"
    strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d={ICON[status]} />
  </svg>
)

const StatusChip: React.FC<{ status: WorkStatus }> = ({ status }) => (
  <span className={`pf-status ${STATUS[status].tone}`}><Glyph status={status} />{STATUS[status].label}</span>
)

// "20 марта 2026": месяц с числом склоняется, без числа Intl даёт именительный ("март 2026").
const joined = (iso: string) =>
  new Date(iso).toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' }).replace(' г.', '')

const plural = (n: number, one: string, few: string, many: string) => {
  const d = n % 10, h = n % 100
  return d === 1 && h !== 11 ? one : d >= 2 && d <= 4 && (h < 12 || h > 14) ? few : many
}

const WorkCard: React.FC<{ work: Work; owner: boolean }> = ({ work, owner }) => {
  const status = STATUS[work.status]
  return (
    <article className="pf-card pm-glass">
      {work.screenshotUrl
        ? <img className="pf-shot" src={work.screenshotUrl} alt="" loading="lazy" />
        : <span className="pf-shot pm-hatch" />}
      <div className="pf-card-body">
        <h3>{work.title}</h3>
        <p className="pm-dim-text">
          {TYPE_LABEL[work.eventType]} &middot; {formatDate(work.eventDate)}{owner && ` · заявка №${work.id}`}
        </p>
        {owner && work.comment && <p className="pf-comment">{work.comment}</p>}
        <div className="pf-actions">
          {work.status === 'published' && work.siteUrl && (
            <a className="pm-btn-ghost pm-pill" href={work.siteUrl} target="_blank" rel="noreferrer">Открыть сайт</a>
          )}
          {owner && work.status === 'rejected' && (
            <Link className="pm-btn-ghost pm-pill" to="/map?propose=1">Отправить заново</Link>
          )}
        </div>
      </div>
      <div className="pf-card-side">
        {/* Гостю статус не нужен: у него в списке только опубликованное. */}
        {owner && <StatusChip status={work.status} />}
        <span className="pm-dim-text">{formatDate(work.createdAt)}</span>
      </div>
      {/* Полоса пути - только пока заявка идёт: у опубликованной и отклонённой всё сказано чипом. */}
      {owner && status.step < 1 && (
        <div className={`pf-track ${status.tone}`} style={{ '--p': `${status.step * 100}%` } as React.CSSProperties}>
          <i />
          <span className="pm-dim-text">{status.note}</span>
        </div>
      )}
    </article>
  )
}

export const ProfilePage: React.FC = () => {
  const { id } = useParams()
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const userId = Number(id)
  const [state, setState] = useState<{ k: 'load' } | { k: 'ready'; profile: Profile } | { k: 'none' } | { k: 'error' }>({ k: 'load' })
  const [me, setMe] = useState<User | null>(null)

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
  const published = works.filter(w => w.status === 'published').length
  const inWork = works.filter(w => w.status === 'processing' || w.status === 'new' || w.status === 'review').length

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
              <Avatar seed={profile.user.id} size={mobile ? 72 : 96} className="pf-avatar" />
              <div className="pf-hero-name">
                <h1>{profile.user.firstName} {profile.user.lastName}</h1>
                <p className="pm-dim-text">На карте с {joined(profile.user.createdAt)}</p>
              </div>
              <dl className="pf-stats">
                <div><dt>Опубликовано</dt><dd>{published}</dd></div>
                {owner && <div><dt>В работе</dt><dd>{inWork}</dd></div>}
                {owner && <div><dt>Всего заявок</dt><dd>{works.length}</dd></div>}
              </dl>
            </section>

            {owner && inWork > 0 && (
              <div className="pf-strip">
                <Glyph status="review" />
                {inWork} {plural(inWork, 'заявка ждёт', 'заявки ждут', 'заявок ждут')} модератора. Ответ придёт на почту.
                <div className="pm-grow" />
                <Link to="/map?propose=1" className="pm-btn pm-pill">Предложить ещё</Link>
              </div>
            )}

            <h2 className="pf-title">
              {owner ? 'Мои заявки' : 'Опубликованные сайты'}
              <span className="pf-count">{works.length}</span>
            </h2>

            {works.length === 0
              ? <div className="pf-note pm-glass">
                  {owner ? 'Вы ещё не предлагали события.' : 'У автора пока нет опубликованных сайтов.'}
                  {owner && <Link to="/map?propose=1" className="pm-btn pm-pill">Предложить событие</Link>}
                </div>
              : <div className="pf-list">{works.map(w => <WorkCard key={w.id} work={w} owner={owner} />)}</div>}
          </>
        )}
      </main>
    </div>
  )
}
