import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Logo, useIsMobile } from '../public/PublicMap.tsx'
import { MapView, type MapHandle } from '../public/MapView.tsx'
import { TYPES, TYPE_LABEL, formatDate, type ApiResult, type EventType } from '../public/api.ts'
import { Field } from '../../shared/ui/Field.tsx'
import { Loading, useSettle } from '../../shared/ui/Loader.tsx'
import { fetchMe, logoutUser, profilePath as profileUrl } from '../public/api.ts'
import * as api from './api.ts'
import type { AuditEntry, Me, Moderator, Page, Role, Status, Submission, TelegramChat } from './api.ts'
import '../public/public.css'
import './panel.css'

type Fail = Extract<ApiResult, { ok: false }>

const STATUS_LABEL: Record<Status, string> = {
  processing: 'Обрабатывается', new: 'Новая', review: 'На проверке', rejected: 'Отклонена', published: 'Опубликована',
}
const FILTERS: { status: Status | ''; label: string }[] = [
  { status: 'new', label: 'Новые' }, { status: 'review', label: 'На проверке' },
  { status: 'rejected', label: 'Отклонено' }, { status: 'published', label: 'Опубликовано' }, { status: '', label: 'Все' },
]
const ACTION_LABEL: Record<AuditEntry['action'], string> = { reviewed: 'Взял на проверку', rejected: 'Отклонил', published: 'Опубликовал' }
const MAX_REASON = 1000
const noop = () => {}
const always = () => true

// Кончилась общая сессия - любой 401 возвращает на общий вход.
const ExpiredContext = React.createContext<() => void>(noop)

export const Panel: React.FC = () => {
  const [me, setMe] = useState<Me | null | undefined>(undefined)
  const [forbidden, setForbidden] = useState(false)
  const navigate = useNavigate()
  useEffect(() => {
    api.getSession().then(r => {
      if (r.ok) setMe(r.data.admin)
      else if (r.status === 401) navigate('/login', { replace: true, state: { back: '/admin' } })
      else { setForbidden(r.status === 403); setMe(null) }
    })
  }, [navigate])
  if (me === undefined) return <div className="pm-root mod-page" />
  if (!me) return <div className="pm-root mod-page mod-login"><header className="mod-top"><Logo onClick={() => navigate('/map')} /></header><div className="mod-card mod-login-card"><h1 className="mod-title">{forbidden ? 'Нет доступа к панели' : 'Не удалось открыть панель'}</h1><p className="mod-dim">{forbidden ? 'Этот аккаунт не имеет прав модератора.' : 'Проверьте соединение и попробуйте ещё раз.'}</p><button className="pm-btn" onClick={() => navigate('/map')}>На карту</button></div></div>
  return (
    <ExpiredContext.Provider value={() => navigate('/login', { replace: true, state: { back: '/admin' } })}>
      <Shell me={me} onLogout={async () => { await logoutUser(); navigate('/map', { replace: true }) }} />
    </ExpiredContext.Provider>
  )
}

type Tab = 'submissions' | 'history' | 'metrics' | 'moderators' | 'bot' | 'mail'

// Иконки меню: контур 24x24, stroke задаёт .pm-menu-item svg.
const ICON: Record<Tab | 'map' | 'logout' | 'profile', string> = {
  submissions: 'M4 13h4l1.5 3h5L16 13h4 M5.5 5h13L20 13v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-5z',
  history: 'M12 7v5l3 2 M3.5 12a8.5 8.5 0 1 0 2.5-6 M3 4v4h4',
  metrics: 'M4 20V10 M10 20V4 M16 20v-7 M21 20H3',
  moderators: 'M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z M2.5 20a6.5 6.5 0 0 1 13 0 M16 4.5a3.5 3.5 0 0 1 0 6.5 M18.5 14.5A6.5 6.5 0 0 1 21.5 20',
  bot: 'M21 4 3 11l6.5 2.5L18 7l-6.5 7.5L17 20z',
  mail: 'M4 6h16a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1z M3.5 7l8.5 6 8.5-6',
  profile: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8z M4.5 20a7.5 7.5 0 0 1 15 0',
  map: 'M15 6l-6 6 6 6',
  logout: 'M10 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4 M20 12H9 M16 8l4 4-4 4',
}
const Icon: React.FC<{ name: keyof typeof ICON }> = ({ name }) => (
  <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path d={ICON[name]} /></svg>
)

const Shell: React.FC<{ me: Me; onLogout: () => void }> = ({ me, onLogout }) => {
  const navigate = useNavigate()
  const [profilePath, setProfilePath] = useState('/map')
  useEffect(() => { const ac = new AbortController(); fetchMe(ac.signal).then(user => { if (user) setProfilePath(profileUrl(user.publicId)) }).catch(() => {}); return () => ac.abort() }, [])
  const mobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const superAdmin = me.role === 'super_admin'
  const groups: [string, [Tab, string][]][] = superAdmin
    ? [['Модерация', [['submissions', 'Заявки'], ['history', 'История'], ['metrics', 'Метрики']]],
        ['Управление', [['moderators', 'Модераторы'], ['bot', 'Бот'], ['mail', 'Почта']]]]
    : [['Модерация', [['submissions', 'Заявки'], ['metrics', 'Метрики']]]]
  const tabs = groups.flatMap(([, items]) => items)
  const tab = tabs.find(([t]) => t === params.get('tab'))?.[0] ?? 'submissions'
  const open = (t: Tab) => setParams(t === 'submissions' ? {} : { tab: t })

  // Плашка выбранного пункта одна на всё меню и переезжает к новому пункту. Первая расстановка
  // и перестройка раскладки (ширина окна) идут без анимации, переезд - только при смене пункта.
  const navRef = useRef<HTMLElement>(null)
  const [pill, setPill] = useState<{ style: React.CSSProperties; tab: Tab }>()
  useLayoutEffect(() => {
    const nav = navRef.current
    if (!nav) return
    const place = (animate: boolean) => {
      const on = nav.querySelector<HTMLElement>('.pm-menu-item.on')
      if (on) setPill({ tab, style: {
        width: on.offsetWidth, height: on.offsetHeight, translate: `${on.offsetLeft}px ${on.offsetTop}px`,
        ...(animate ? {} : { transition: 'none' }),
      } })
    }
    place(pill !== undefined && pill.tab !== tab)
    // Первый вызов ResizeObserver приходит сразу при подписке - он не перестройка, пропускаем.
    let first = true
    const ro = new ResizeObserver(() => { if (!first) place(false); first = false })
    ro.observe(nav)
    return () => ro.disconnect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, mobile])

  const main = (
    <main key={tab} className="mod-main">
      {tab === 'submissions' && <Submissions mobile={mobile} />}
      {tab === 'history' && <History />}
      {tab === 'metrics' && <MetricsTab />}
      {tab === 'moderators' && <Moderators me={me} />}
      {tab === 'bot' && <Bot />}
      {tab === 'mail' && <Mail />}
    </main>
  )

  const barTabs = tabs
  if (mobile) return (
    <div className="pm-root mod-page mobile">
      <header className="mod-top">
        <Logo short onClick={() => navigate('/map')} />
        {barTabs.length > 1 && (
          <nav className="mod-tabs">
            {barTabs.map(([t, label]) => (
              <button key={t} type="button" className={t === tab ? 'on' : ''} aria-current={t === tab ? 'page' : undefined}
                onClick={() => open(t)}><Icon name={t} /><span>{label}</span></button>
            ))}
          </nav>
        )}
        <div className="pm-grow" />
        <button type="button" className="mod-side-back" title="Мой профиль" aria-label="Мой профиль"
          onClick={() => navigate(profilePath)}><Icon name="profile" /></button>
        <button type="button" className="pm-btn-text" onClick={onLogout}>Выйти</button>
      </header>
      {main}
    </div>
  )

  return (
    <div className="pm-root mod-page mod-shell">
      <aside className="mod-side">
        <div className="mod-side-brand">
          <Logo short onClick={() => navigate('/map')} />
          <button type="button" className="mod-side-back" title="На карту" aria-label="На карту" onClick={() => navigate('/map')}><Icon name="map" /></button>
        </div>
        <div className="mod-side-hello">
          <h2>Здравствуйте,<br />{me.name || me.login}</h2>
          <span className="mod-dim">{api.ROLE_LABEL[me.role]}</span>
        </div>
        <nav ref={navRef} className="mod-side-nav pm-menu">
          {pill && <span className="pm-menu-pill" style={pill.style} aria-hidden="true" />}
          {groups.map(([title, items]) => (
            <React.Fragment key={title}>
              <span className="pm-menu-label">{title}</span>
              {items.map(([t, label]) => (
                <button key={t} type="button" className={`pm-menu-item${t === tab ? ' on' : ''}`} aria-current={t === tab ? 'page' : undefined}
                  title={label} onClick={() => open(t)}><Icon name={t} /><span>{label}</span></button>
              ))}
            </React.Fragment>
          ))}
        </nav>
        <button type="button" className="pm-menu-item" onClick={() => navigate(profilePath)}><Icon name="profile" /><span>Мой профиль</span></button>
        <button type="button" className="pm-menu-item mod-side-out" title="Выйти" onClick={onLogout}><Icon name="logout" /><span>Выйти</span></button>
      </aside>
      <div className="mod-scroll">{main}</div>
    </div>
  )
}

// Загрузка с отменой устаревших ответов и выходом на форму входа по 401.
function useLoad<T>(load: () => Promise<ApiResult<T>>, deps: React.DependencyList) {
  const expired = React.useContext(ExpiredContext)
  const [state, setState] = useState<{ data?: T; error?: string; loading: boolean }>({ loading: true })
  const [tick, setTick] = useState(0)
  useEffect(() => {
    let live = true
    setState(s => ({ ...s, loading: true }))
    load().then(res => {
      if (!live) return
      if (res.ok) setState({ data: res.data, loading: false })
      else if (res.status === 401) expired()
      else setState({ error: api.problem(res), loading: false })
    })
    return () => { live = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick])
  return { ...state, reload: useCallback(() => setTick(t => t + 1), []) }
}

function useDebounced<T>(value: T, ms = 300) {
  const [v, setV] = useState(value)
  useEffect(() => { const t = setTimeout(() => setV(value), ms); return () => clearTimeout(t) }, [value, ms])
  return v
}

// Отправка формы: ошибки полей - к полям, остальное - строкой под формой, 401 - на вход.
function useSubmit() {
  const expired = React.useContext(ExpiredContext)
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [note, setNote] = useState<{ text: string; bad: boolean } | null>(null)
  const submit = async <T,>(action: () => Promise<ApiResult<T>>, done?: string): Promise<T | undefined> => {
    setBusy(true)
    setNote(null)
    setErrors({})
    const res = await action()
    setBusy(false)
    if (res.ok) {
      if (done) setNote({ text: done, bad: false })
      return res.data
    }
    if (res.status === 401) expired()
    else if (res.fields) setErrors(res.fields)
    else setNote({ text: api.problem(res), bad: true })
  }
  const clear = (k: string) => { setErrors(e => ({ ...e, [k]: '' })); setNote(null) }
  const noteLine = note && <p className={note.bad ? 'mod-error' : 'mod-note'} role={note.bad ? 'alert' : 'status'}>{note.text}</p>
  return { busy, errors, setErrors, noteLine, submit, clear }
}

const Submissions: React.FC<{ mobile: boolean }> = ({ mobile }) => {
  const [params, setParams] = useSearchParams()
  const raw = params.get('status') ?? 'new'
  const status = (raw === 'all' ? '' : raw) as Status | ''
  const [q, setQ] = useState('')
  const query = useDebounced(q.trim())
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Submission | null>(null)
  const list = useLoad(() => api.listSubmissions({ status, q: query, page }), [status, query, page])

  useEffect(() => setPage(1), [status, query])
  const changed = (s: Submission) => { setSelected(s); list.reload() }

  if (mobile && selected) {
    return <Detail key={selected.id} submission={selected} onBack={() => setSelected(null)} onChanged={changed} />
  }
  return (
    <div className="mod-split">
      <section className="mod-list">
        <div className="pm-chips" role="tablist">
          {FILTERS.map(f => (
            <button key={f.label} type="button" role="tab" aria-selected={f.status === status} className={f.status === status ? 'on' : ''}
              onClick={() => { setParams(f.status === 'new' ? {} : { status: f.status || 'all' }); setSelected(null) }}>{f.label}</button>
          ))}
        </div>
        <Field><input type="search" aria-label="Поиск заявок" placeholder="Поиск по названию или номеру" value={q} onChange={e => setQ(e.target.value)} /></Field>
        <Rows state={list} empty="Заявок нет">
          {(data: Page<Submission>) => (
            <>
              <ul className="mod-rows">
                {data.items.map(s => (
                  <li key={s.id}>
                    <button type="button" className={`mod-row${selected?.id === s.id ? ' on' : ''}`} onClick={() => setSelected(s)}>
                      <span className="mod-row-head"><b>{s.title}</b><Badge status={s.status} /></span>
                      <span className="mod-dim">{s.publicId} · {TYPE_LABEL[s.eventType]} · {formatDate(s.eventDate)} · прислана {api.dateTime(s.createdAt)}</span>
                    </button>
                  </li>
                ))}
              </ul>
              <Pager page={page} pageSize={data.pageSize} total={data.total} onPage={setPage} />
            </>
          )}
        </Rows>
      </section>
      {!mobile && (
        <section className="mod-detail-col">
          {selected
            ? <Detail key={selected.id} submission={selected} onChanged={changed} />
            : <div className="mod-card mod-placeholder mod-dim">Выберите заявку в списке</div>}
        </section>
      )}
    </div>
  )
}

// Список с загрузкой, ошибкой и пустым состоянием.
function Rows<T extends { items: unknown[] }>({ state, empty, children }: {
  state: { data?: T; error?: string; loading: boolean; reload: () => void }; empty: string; children: (data: T) => React.ReactNode
}) {
  // Индикатор только на первую загрузку: при смене страницы или фильтра старый список просто тускнеет.
  const phase = useSettle(!state.data && !state.error, !!state.data?.items.length)
  if (phase !== 'idle') return <Loading phase={phase} label="Загружаем" done="Загружено" className="mod-dim" />
  if (state.error) return <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
  if (!state.data) return null
  if (!state.data.items.length) return <p className="mod-dim mod-empty">{empty}</p>
  return <div className={`mod-reveal${state.loading ? ' mod-stale' : ''}`}>{children(state.data)}</div>
}

const Pager: React.FC<{ page: number; pageSize: number; total: number; onPage: (p: number) => void }> = ({ page, pageSize, total, onPage }) => {
  const pages = Math.ceil(total / pageSize)
  if (pages <= 1) return null
  return (
    <div className="mod-pager">
      <button type="button" className="pm-btn-ghost" disabled={page <= 1} onClick={() => onPage(page - 1)}>Назад</button>
      <span className="mod-dim">{page} из {pages}</span>
      <button type="button" className="pm-btn-ghost" disabled={page >= pages} onClick={() => onPage(page + 1)}>Дальше</button>
    </div>
  )
}

const Badge: React.FC<{ status: Status }> = ({ status }) => <span className={`mod-badge ${status}`}>{STATUS_LABEL[status]}</span>

type Mode = 'view' | 'approve' | 'reject'

const Detail: React.FC<{ submission: Submission; onChanged: (s: Submission) => void; onBack?: () => void }> = ({ submission: s, onChanged, onBack }) => {
  const expired = React.useContext(ExpiredContext)
  const [mode, setMode] = useState<Mode>('view')
  const mapRef = useRef<MapHandle>(null)
  // Карта создаётся вместе с формой одобрения, эффект родителя идёт после её инициализации.
  useEffect(() => {
    if (mode === 'approve' && s.latitude != null && s.longitude != null) mapRef.current?.flyTo(s.latitude, s.longitude, 5)
  }, [mode, s.latitude, s.longitude])
  const [busy, setBusy] = useState(false)
  const [note, setNote] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [reason, setReason] = useState('')
  const [lat, setLat] = useState(s.latitude?.toFixed(6) ?? '')
  const [lng, setLng] = useState(s.longitude?.toFixed(6) ?? '')
  const [type, setType] = useState<EventType>(s.eventType)

  const run = async (action: () => Promise<ApiResult<{ submission: Submission }>>) => {
    setBusy(true)
    setNote('')
    const res = await action()
    setBusy(false)
    if (res.ok) { setMode('view'); onChanged(res.data.submission); return }
    if (res.status === 401) return expired()
    if (res.fields) setErrors(res.fields)
    else setNote(res.status === 409 ? 'Статус заявки уже изменился: её обработал другой модератор. Обновите список.' : api.problem(res as Fail))
  }

  const approve = (e: React.FormEvent) => {
    e.preventDefault()
    const la = parseFloat(lat.replace(',', '.')), lo = parseFloat(lng.replace(',', '.'))
    const next: Record<string, string> = {}
    if (!(la >= -90 && la <= 90)) next.latitude = 'Широта от -90 до 90'
    if (!(lo >= -180 && lo <= 180)) next.longitude = 'Долгота от -180 до 180'
    setErrors(next)
    if (!Object.keys(next).length) run(() => api.publishSubmission(s.id, la, lo, type))
  }

  const reject = (e: React.FormEvent) => {
    e.preventDefault()
    if (!reason.trim()) return setErrors({ comment: 'Напишите причину: её получит автор' })
    run(() => api.rejectSubmission(s.id, reason.trim()))
  }

  const la = parseFloat(lat.replace(',', '.')), lo = parseFloat(lng.replace(',', '.'))
  const pick = Math.abs(la) <= 90 && Math.abs(lo) <= 180 ? { lat: la, lng: lo } : null
  const preview = s.siteUrl && (
    <a className="pm-btn-ghost" href={s.siteUrl} target="_blank" rel="noopener noreferrer">
      {s.status === 'published' ? 'Открыть сайт' : 'Предпросмотр'}
    </a>
  )

  return (
    <article className="mod-card mod-detail">
      {onBack && <button type="button" className="pm-btn-text mod-back" onClick={onBack}>К списку</button>}
      <div className="mod-row-head"><h2 className="mod-title">{s.title}</h2><Badge status={s.status} /></div>
      {s.screenshotUrl && (
        <a href={s.siteUrl} target="_blank" rel="noopener noreferrer" className="mod-shot"><img src={s.screenshotUrl} alt="Снимок присланного сайта" /></a>
      )}
      <p className="mod-desc">{s.description}</p>
      <div className="mod-facts">
        <section>
          <h4 className="pm-label">Событие</h4>
          <dl>
            <div><dt>Номер</dt><dd className="mod-code">{s.publicId}</dd></div>
            <div><dt>Тип</dt><dd>{TYPE_LABEL[s.eventType]}</dd></div>
            <div><dt>Дата</dt><dd>{formatDate(s.eventDate)}</dd></div>
            <div><dt>Точка автора</dt><dd>{s.latitude !== undefined ? `${s.latitude.toFixed(5)}, ${s.longitude!.toFixed(5)}` : <span className="mod-dim">не указана</span>}</dd></div>
          </dl>
        </section>
        <section>
          <h4 className="pm-label">Автор</h4>
          <div className="mod-contacts">
            <a className="pm-btn-ghost pm-pill" href={`mailto:${s.email}`} title={s.email}><Icon name="mail" /><span>{s.email}</span></a>
            <a className="pm-btn-ghost pm-pill" href={`https://t.me/${s.telegramUsername}`} target="_blank" rel="noopener noreferrer" title="Открыть в Telegram"><Icon name="bot" /><span>@{s.telegramUsername}</span></a>
          </div>
        </section>
      </div>
      <p className="mod-dim">Прислана {api.dateTime(s.createdAt)}{s.updatedAt !== s.createdAt && <>, изменена {api.dateTime(s.updatedAt)}</>}</p>
      {s.status === 'rejected' && s.adminComment && <p className="mod-reason"><b>Причина отказа.</b> {s.adminComment}</p>}
      {s.status === 'processing' && <p className="mod-dim">Архив ещё распаковывается, предпросмотр появится через минуту.</p>}

      {mode === 'view' && (
        <div className="mod-actions">
          {s.status === 'new' && <button type="button" className="pm-btn" disabled={busy} onClick={() => run(() => api.reviewSubmission(s.id))}>Проверить</button>}
          {preview}
          {s.status === 'review' && <>
            <button type="button" className="pm-btn" onClick={() => { setMode('approve'); setErrors({}) }}>Одобрить</button>
            <button type="button" className="pm-btn-ghost danger" onClick={() => { setMode('reject'); setErrors({}) }}>Отклонить</button>
          </>}
        </div>
      )}

      {mode === 'approve' && (
        <form className="mod-form" onSubmit={approve} noValidate>
          <h3>Публикация на карте</h3>
          <p className="mod-dim">Проверьте точку: кликните по карте или перетащите метку. Координаты можно ввести вручную.</p>
          <div className="mod-map"><MapView ref={mapRef} events={[]} isActive={always} selectedId={null} onSelect={noop} pad={12} labels
            pick={pick} onPick={busy ? undefined : (a, b) => { setLat(a.toFixed(6)); setLng(b.toFixed(6)); setErrors({}) }} /></div>
          <div className="mod-pair">
            <Field label="Широта" error={errors.latitude}><input inputMode="decimal" value={lat} onChange={e => setLat(e.target.value)} placeholder="55.751244" /></Field>
            <Field label="Долгота" error={errors.longitude}><input inputMode="decimal" value={lng} onChange={e => setLng(e.target.value)} placeholder="37.618423" /></Field>
          </div>
          <div className="pm-label">Тип события</div>
          <div className="pm-types">
            {TYPES.map(t => <button key={t} type="button" className={t === type ? 'on' : ''} aria-pressed={t === type} onClick={() => setType(t)}>{TYPE_LABEL[t]}</button>)}
          </div>
          {errors.eventType && <p className="mod-error">{errors.eventType}</p>}
          {note && <p className="mod-error" role="alert">{note}</p>}
          <div className="mod-actions">
            <button className="pm-btn" disabled={busy}>{busy ? 'Публикуем...' : 'Опубликовать'}</button>
            <button type="button" className="pm-btn-text" onClick={() => setMode('view')}>Отмена</button>
          </div>
        </form>
      )}

      {mode === 'reject' && (
        <form className="mod-form" onSubmit={reject} noValidate>
          <h3>Отклонить заявку</h3>
          <Field label="Причина для автора" error={errors.comment}>
            <textarea rows={4} maxLength={MAX_REASON} autoFocus value={reason} onChange={e => { setReason(e.target.value); setErrors({}) }} />
          </Field>
          <p className="mod-dim">{reason.length} / {MAX_REASON}</p>
          {note && <p className="mod-error" role="alert">{note}</p>}
          <div className="mod-actions">
            <button className="pm-btn danger" disabled={busy}>{busy ? 'Отклоняем...' : 'Отклонить'}</button>
            <button type="button" className="pm-btn-text" onClick={() => setMode('view')}>Отмена</button>
          </div>
        </form>
      )}
      {mode === 'view' && note && <p className="mod-error" role="alert">{note}</p>}
    </article>
  )
}

type Period = 'day' | 'week' | 'month'
const PERIODS: [Period, string][] = [['day', 'День'], ['week', 'Неделя'], ['month', 'Месяц']]
const DAY = 86400000

// Начало дня, недели (с понедельника) или месяца - ключ группы на таймлайне.
const periodStart = (d: Date, p: Period) => {
  const s = new Date(d.getFullYear(), d.getMonth(), p === 'month' ? 1 : d.getDate())
  if (p === 'week') s.setDate(s.getDate() - (s.getDay() + 6) % 7)
  return s
}

const periodLabel = (start: Date, p: Period) => {
  const now = new Date()
  const ago = Math.round((periodStart(now, p).getTime() - start.getTime()) / DAY)
  const year = start.getFullYear() !== now.getFullYear() ? { year: 'numeric' as const } : {}
  if (p === 'day') {
    if (ago === 0) return 'Сегодня'
    if (ago === 1) return 'Вчера'
    return start.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', ...year })
  }
  if (p === 'week') {
    if (ago === 0) return 'Эта неделя'
    if (ago === 7) return 'Прошлая неделя'
    const end = new Date(start.getTime() + 6 * DAY)
    const from = start.getMonth() === end.getMonth() ? String(start.getDate()) : start.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' })
    return `${from} - ${end.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', ...year })}`
  }
  const month = start.toLocaleDateString('ru-RU', { month: 'long', ...year })
  return month[0].toUpperCase() + month.slice(1)
}

const History: React.FC = () => {
  const [q, setQ] = useState('')
  const query = useDebounced(q.trim())
  const [page, setPage] = useState(1)
  const [period, setPeriod] = useState<Period>('day')
  const list = useLoad(() => api.listAudit({ q: query, page }), [query, page])
  useEffect(() => setPage(1), [query])

  // Записи приходят от новых к старым, поэтому соседние записи одного периода идут подряд.
  const groups = (items: AuditEntry[]) => items.reduce<{ key: number; label: string; items: AuditEntry[] }[]>((acc, a) => {
    const start = periodStart(new Date(a.createdAt), period)
    const last = acc[acc.length - 1]
    if (last?.key === start.getTime()) last.items.push(a)
    else acc.push({ key: start.getTime(), label: periodLabel(start, period).replace(' г.', ''), items: [a] })
    return acc
  }, [])
  const time = (iso: string) => new Date(iso).toLocaleString('ru-RU', period === 'day'
    ? { hour: '2-digit', minute: '2-digit' }
    : { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

  return (
    <section className="mod-narrow">
      <div className="mod-tl-head">
        <h1 className="mod-title">История действий</h1>
        <div className="pm-chips" role="tablist" aria-label="Группировка">
          {PERIODS.map(([p, label]) => (
            <button key={p} type="button" role="tab" aria-selected={p === period} className={p === period ? 'on' : ''} onClick={() => setPeriod(p)}>{label}</button>
          ))}
        </div>
      </div>
      <p className="mod-dim">Каждое действие модератора. Записи нельзя изменить или удалить.</p>
      <Field><input type="search" aria-label="Поиск по истории" placeholder="Поиск по заявке, логину или действию" value={q} onChange={e => setQ(e.target.value)} /></Field>
      <Rows state={list} empty="Действий пока нет">
        {(data: Page<AuditEntry>) => (
          <>
            <ol className="mod-tl">
              {groups(data.items).map(g => (
                <li key={g.key}>
                  <h2 className="mod-tl-day">{g.label}</h2>
                  <ol>
                    {g.items.map(a => (
                      <li key={a.id} className="mod-tl-item">
                        <span className="mod-row-head">
                          <b>{a.adminLogin} <span className={`mod-act ${a.action}`}>{ACTION_LABEL[a.action].toLowerCase()}</span> {a.submissionTitle}</b>
                          <time className="mod-dim" dateTime={a.createdAt} title={api.dateTime(a.createdAt)}>{time(a.createdAt)}</time>
                        </span>
                        <span className="mod-dim">{a.submissionPublicId} · {api.ROLE_LABEL[a.adminRole] ?? a.adminRole}</span>
                        {a.comment && <span className="mod-log-comment">{a.comment}</span>}
                      </li>
                    ))}
                  </ol>
                </li>
              ))}
            </ol>
            <Pager page={page} pageSize={data.pageSize} total={data.total} onPage={setPage} />
          </>
        )}
      </Rows>
    </section>
  )
}

const METRIC_PERIODS: [number, string][] = [[7, '7 дней'], [30, '30 дней'], [90, '90 дней'], [365, 'Год']]
// Шаги воронки формы, как их шлёт track('form_step') в Propose.tsx.
const AUTH_FUNNEL_LABEL = ['Открыли вход', 'Запросили код', 'Вошли']
const FUNNEL_LABEL = ['Открыли форму', 'Тип события', 'О событии', 'Место', 'Сайт', 'Контакты', 'Отправили заявку']

const pct = (part: number, whole: number) => whole > 0 ? `${Math.round(part / whole * 100)}%` : '-'
const duration = (seconds: number) => {
  const s = Math.round(seconds)
  return s < 60 ? `${s} с` : `${Math.floor(s / 60)} мин ${s % 60} с`
}
const num = (n: number) => n.toLocaleString('ru-RU')
const count = (n: number, one: string, few: string, many: string) => {
  const m10 = n % 10, m100 = n % 100
  return `${num(n)} ${m10 === 1 && m100 !== 11 ? one : m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14) ? few : many}`
}

// Дни без просмотров сервер не отдаёт, а на графике они нужны нулями.
function fillDays(daily: api.Metrics['daily'], days: number) {
  const byDate = new Map(daily.map(d => [d.date, d]))
  const out: api.Metrics['daily'] = []
  const today = new Date()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() - i)
    const date = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    out.push(byDate.get(date) ?? { date, views: 0, visitors: 0 })
  }
  return out
}

const MetricsTab: React.FC = () => {
  const [days, setDays] = useState(30)
  const state = useLoad(() => api.getMetrics(days), [days])
  // Rows ждёт items: у отчёта список - это события.
  const rows = { ...state, data: state.data && { ...state.data, items: [state.data] } }
  return (
    <section className="mod-metrics">
      <div className="mod-tl-head">
        <h1 className="mod-title">Метрики</h1>
        <div className="pm-chips" role="tablist" aria-label="Период">
          {METRIC_PERIODS.map(([d, label]) => (
            <button key={d} type="button" role="tab" aria-selected={d === days} className={d === days ? 'on' : ''} onClick={() => setDays(d)}>{label}</button>
          ))}
        </div>
      </div>
      <Rows state={rows} empty="Данных пока нет">{({ items: [m] }) => <MetricsBody m={m} />}</Rows>
    </section>
  )
}

// Изменение к прошлому периоду: у счётчиков в процентах, у долей в процентных пунктах.
// Без данных за прошлый период сравнивать не с чем, плашки нет.
type Delta = { text: string; dir: -1 | 0 | 1 } | null
const sign = (d: number): -1 | 0 | 1 => d > 0 ? 1 : d < 0 ? -1 : 0
const relDelta = (now: number, before: number): Delta => {
  if (before <= 0) return null
  const d = Math.round((now - before) / before * 100)
  return { text: `${Math.abs(d)}%`, dir: sign(d) }
}
const ratio = (part: number, whole: number) => whole > 0 ? part / whole : null
const ppDelta = (now: number | null, before: number | null): Delta => {
  if (now === null || before === null) return null
  const d = Math.round((now - before) * 100)
  return { text: `${Math.abs(d)} п.п.`, dir: sign(d) }
}

const DeltaPill: React.FC<{ d: Delta; days: number }> = ({ d, days }) => d && (
  <span className={`mod-delta ${d.dir > 0 ? 'up' : d.dir < 0 ? 'down' : ''}`} title={`К предыдущим ${days} дн.`}>
    {d.dir !== 0 && <svg viewBox="0 0 12 12" aria-hidden="true"><path d={d.dir > 0 ? 'M6 9.5V2.5 M2.5 6 6 2.5 9.5 6' : 'M6 2.5v7 M2.5 6 6 9.5 9.5 6'} /></svg>}
    {d.dir > 0 ? '+' : d.dir < 0 ? '-' : ''}{d.text}
  </span>
)

// Иконки плиток: контур 24x24, как в меню.
const KPI_ICON = {
  submissions: ICON.submissions,
  published: 'M12 21s-7-6.2-7-11.5a7 7 0 0 1 14 0C19 14.8 12 21 12 21z M12 12a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z',
  approved: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z M8 12.5l2.7 2.7L16 9.8',
  stars: 'M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z',
  time: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z M12 7.5V12l3 2',
  ctr: 'M6 4l5.5 14 2-5.5 5.5-2z M14 14l5 5',
  scroll: 'M12 4v16 M8 8l4-4 4 4 M8 16l4 4 4-4',
  read: 'M4 5.5A1.5 1.5 0 0 1 5.5 4H11v15H5.5A1.5 1.5 0 0 0 4 20.5z M20 5.5A1.5 1.5 0 0 0 18.5 4H13v15h5.5a1.5 1.5 0 0 1 1.5 1.5z',
}

interface Kpi { icon: keyof typeof KPI_ICON; label: string; value: string; sub: string; delta: Delta; fill?: number | null }

const KpiTile: React.FC<{ k: Kpi; days: number }> = ({ k, days }) => (
  <div className="mod-kpi">
    <div className="mod-kpi-head">
      <span className="mod-kpi-icon"><svg viewBox="0 0 24 24" aria-hidden="true"><path d={KPI_ICON[k.icon]} /></svg></span>
      <span className="mod-dim">{k.label}</span>
    </div>
    <div className="mod-kpi-value"><b>{k.value}</b><DeltaPill d={k.delta} days={days} /></div>
    <span className="mod-kpi-fill" hidden={k.fill === undefined}><i style={{ width: `${(k.fill ?? 0) * 100}%` }} /></span>
    <span className="mod-dim mod-kpi-sub">{k.sub}</span>
  </div>
)

// Сглаженная линия через точки: кривые Катмулла-Рома, контрольные точки зажаты по высоте,
// чтобы линия не уходила ниже нуля между нулевыми днями.
function smoothPath(points: [number, number][], top: number, bottom: number) {
  if (points.length === 1) return `M${points[0][0]},${points[0][1]}`
  const clamp = (y: number) => Math.min(bottom, Math.max(top, y))
  let d = `M${points[0][0]},${points[0][1]}`
  for (let i = 0; i < points.length - 1; i++) {
    const [p0, p1, p2, p3] = [points[i - 1] ?? points[i], points[i], points[i + 1], points[i + 2] ?? points[i + 1]]
    const c1 = [p1[0] + (p2[0] - p0[0]) / 6, clamp(p1[1] + (p2[1] - p0[1]) / 6)]
    const c2 = [p2[0] - (p3[0] - p1[0]) / 6, clamp(p2[1] - (p3[1] - p1[1]) / 6)]
    d += ` C${c1[0]},${c1[1]} ${c2[0]},${c2[1]} ${p2[0]},${p2[1]}`
  }
  return d
}

const CHART_W = 1000, CHART_H = 220, CHART_TOP = 12

const VisitorsChart: React.FC<{ series: api.Metrics['daily'] }> = ({ series }) => {
  const [hover, setHover] = useState<number | null>(null)
  const peak = Math.max(1, ...series.map(d => d.visitors))
  const avg = series.reduce((s, d) => s + d.visitors, 0) / series.length
  const x = (i: number) => series.length > 1 ? i / (series.length - 1) * CHART_W : CHART_W / 2
  const y = (v: number) => CHART_H - v / peak * (CHART_H - CHART_TOP)
  const points = series.map((d, i) => [x(i), y(d.visitors)] as [number, number])
  const line = smoothPath(points, CHART_TOP, CHART_H)
  const area = `${line} L${x(series.length - 1)},${CHART_H} L${x(0)},${CHART_H} Z`
  const pick = (e: React.PointerEvent<HTMLDivElement>) => {
    const box = e.currentTarget.getBoundingClientRect()
    setHover(Math.round(Math.min(1, Math.max(0, (e.clientX - box.left) / box.width)) * (series.length - 1)))
  }
  const h = hover === null ? null : series[hover]
  const left = (i: number) => `${x(i) / CHART_W * 100}%`
  const mid = series[Math.floor(series.length / 2)]
  return (
    <div className="mod-chart">
      <div className="mod-chart-plot" onPointerMove={pick} onPointerDown={pick} onPointerLeave={() => setHover(null)}
        role="img" aria-label={`Уникальные посетители по дням, максимум ${peak}, в среднем ${avg.toFixed(1)}`}>
        <svg viewBox={`0 0 ${CHART_W} ${CHART_H}`} preserveAspectRatio="none" aria-hidden="true">
          <defs>
            <linearGradient id="mod-chart-fill" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" stopColor="var(--gold)" stopOpacity="0.32" />
              <stop offset="1" stopColor="var(--gold)" stopOpacity="0" />
            </linearGradient>
          </defs>
          {[0, 0.5, 1].map(f => <line key={f} className="mod-chart-grid" x1="0" x2={CHART_W} y1={y(peak * f)} y2={y(peak * f)} />)}
          <path d={area} fill="url(#mod-chart-fill)" />
          <path d={line} className="mod-chart-line" />
          {avg > 0 && <line className="mod-chart-avg" x1="0" x2={CHART_W} y1={y(avg)} y2={y(avg)} />}
        </svg>
        <span className="mod-chart-tick" style={{ top: `${y(peak) / CHART_H * 100}%` }}>{num(peak)}</span>
        <span className="mod-chart-tick" style={{ top: `${y(peak / 2) / CHART_H * 100}%` }}>{num(Math.round(peak / 2))}</span>
        {avg > 0 && <span className="mod-chart-tick avg" style={{ top: `${y(avg) / CHART_H * 100}%` }}>ср. {avg.toFixed(1).replace('.', ',')}</span>}
        {h && hover !== null && <>
          <span className="mod-chart-cross" style={{ left: left(hover) }} />
          <span className="mod-chart-dot" style={{ left: left(hover), top: `${y(h.visitors) / CHART_H * 100}%` }} />
          <span className={`mod-chart-tip ${hover > series.length * 0.7 ? 'flip' : ''}`} style={{ left: left(hover) }}>
            <span className="mod-dim">{formatDate(h.date)}</span>
            <b>{count(h.visitors, 'посетитель', 'посетителя', 'посетителей')}</b>
            <span className="mod-dim">{count(h.views, 'просмотр', 'просмотра', 'просмотров')}</span>
          </span>
        </>}
      </div>
      <div className="mod-chart-axis mod-dim">
        <span>{formatDate(series[0].date)}</span><span>{formatDate(mid.date)}</span><span>{formatDate(series[series.length - 1].date)}</span>
      </div>
    </div>
  )
}

// Воронка "трубой": высота полосы - доля от открывших форму, между шагами плавный переход.
const FLOW_W = 100, FLOW_H = 120
function flowPath(shares: number[]) {
  const n = shares.length
  const half = shares.map(v => Math.max(1.5, v * (FLOW_H - 8) / 2))
  const top = half.map(h => FLOW_H / 2 - h), bottom = half.map(h => FLOW_H / 2 + h)
  const flatEnd = (i: number) => i * FLOW_W + FLOW_W * 0.7, flatStart = (i: number) => i * FLOW_W + FLOW_W * 0.3
  let d = `M0,${top[0]}`
  for (let i = 0; i < n; i++) {
    d += ` L${flatEnd(i)},${top[i]}`
    if (i < n - 1) {
      const m = (flatEnd(i) + flatStart(i + 1)) / 2
      d += ` C${m},${top[i]} ${m},${top[i + 1]} ${flatStart(i + 1)},${top[i + 1]}`
    }
  }
  d += ` L${n * FLOW_W},${top[n - 1]} L${n * FLOW_W},${bottom[n - 1]}`
  for (let i = n - 1; i >= 0; i--) {
    d += ` L${flatStart(i)},${bottom[i]}`
    if (i > 0) {
      const m = (flatStart(i) + flatEnd(i - 1)) / 2
      d += ` C${m},${bottom[i]} ${m},${bottom[i - 1]} ${flatEnd(i - 1)},${bottom[i - 1]}`
    }
  }
  return `${d} L0,${bottom[0]} Z`
}

const Funnel: React.FC<{ steps: { label: string; visitors: number }[] }> = ({ steps }) => {
  const base = Math.max(1, ...steps.map(s => s.visitors))
  const n = steps.length
  return (
    <div className="mod-flow-wrap">
      <div className="mod-flow" style={{ gridTemplateColumns: `repeat(${n}, minmax(0, 1fr))` }}>
        {steps.map(s => <span key={s.label} className="mod-flow-label">{s.label}</span>)}
        <div className="mod-flow-band" style={{ gridColumn: `1 / span ${n}` }}>
          <svg viewBox={`0 0 ${n * FLOW_W} ${FLOW_H}`} preserveAspectRatio="none" aria-hidden="true">
            <defs>
              <linearGradient id="mod-flow-fill" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0" stopColor="var(--gold)" stopOpacity="0.5" />
                <stop offset="1" stopColor="var(--gold)" stopOpacity="0.18" />
              </linearGradient>
            </defs>
            <path d={flowPath(steps.map(s => s.visitors / base))} fill="url(#mod-flow-fill)" />
          </svg>
          {steps.map((s, i) => (
            <span key={s.label} className="mod-flow-pill" style={{ left: `${(i + 0.5) / n * 100}%` }}>{pct(s.visitors, steps[0].visitors)}</span>
          ))}
        </div>
        {steps.map((s, i) => (
          <span key={s.label} className="mod-flow-foot">
            <b>{num(s.visitors)}</b>
            {i > 0 && steps[i - 1].visitors > s.visitors && (
              <span className="mod-flow-drop" title="Отвал от предыдущего шага">-{pct(steps[i - 1].visitors - s.visitors, steps[i - 1].visitors)}</span>
            )}
          </span>
        ))}
      </div>
    </div>
  )
}

const hasData = (e: api.MetricsEvent) => e.views + e.visitors + e.stars + e.cardOpens > 0

const EventsRating: React.FC<{ events: api.MetricsEvent[] }> = ({ events }) => {
  const [all, setAll] = useState(false)
  const active = events.filter(hasData)
  const shown = all ? events : active
  const peak = Math.max(1, ...events.map(e => e.visitors))
  return (
    <>
      <div className="mod-table-wrap">
        <table className="mod-table">
          <thead><tr>
            <th>#</th><th>Событие</th><th>Посетители</th><th>Просмотры</th><th>Время</th><th>Прокрутка</th>
            <th>Дочитали</th><th>Звёзды</th><th>CTR</th>
          </tr></thead>
          <tbody>
            {shown.map((e, i) => (
              <tr key={e.id}>
                <td className="mod-dim">{i + 1}</td>
                <td>
                  <a href={`/event/${e.id}`} target="_blank" rel="noreferrer">{e.title}</a>
                  <span className="mod-rank-bar"><i style={{ width: `${e.visitors / peak * 100}%` }} /></span>
                </td>
                <td><b>{num(e.visitors)}</b></td><td>{num(e.views)}</td><td>{e.views ? duration(e.avgActiveSeconds) : '-'}</td>
                <td>{e.scrolledViews ? `${Math.round(e.avgScroll)}%` : '-'}</td><td>{pct(e.readThrough, e.scrolledViews)}</td>
                <td>{num(e.stars)}</td><td>{pct(e.cardClicks, e.cardOpens)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {!active.length && !all && <p className="mod-dim">За период у событий нет просмотров и звёзд.</p>}
      {events.length > active.length && (
        <button type="button" className="pm-btn-text mod-more" onClick={() => setAll(v => !v)}>
          {all ? 'Скрыть события без данных' : `Показать ещё ${count(events.length - active.length, 'событие', 'события', 'событий')} без данных`}
        </button>
      )}
    </>
  )
}

const MetricsBody: React.FC<{ m: api.Metrics }> = ({ m }) => {
  const p = m.previous
  const decided = m.approved + m.rejected, decidedBefore = p.approved + p.rejected
  const approval = ratio(m.approved, decided), ctr = ratio(m.cardClicks, m.cardOpens)
  const read = ratio(m.readThrough, m.scrolledViews)
  const tiles: Kpi[] = [
    { icon: 'submissions', label: 'Заявки', value: num(m.submissions), sub: 'пришло за период', delta: relDelta(m.submissions, p.submissions) },
    { icon: 'published', label: 'Опубликовано событий', value: num(m.published), sub: `всего на карте ${num(m.eventsTotal)}`, delta: relDelta(m.published, p.published) },
    {
      icon: 'approved', label: 'Доля одобренных', value: pct(m.approved, decided), fill: approval,
      sub: decided ? `${num(m.approved)} из ${count(decided, 'решённой', 'решённых', 'решённых')}` : 'решений пока нет',
      delta: ppDelta(approval, ratio(p.approved, decidedBefore)),
    },
    { icon: 'stars', label: 'Звёзды', value: `+${num(m.stars)}`, sub: `всего ${num(m.starsTotal)}`, delta: relDelta(m.stars, p.stars) },
    { icon: 'time', label: 'Активное время', value: duration(m.avgActiveSeconds), sub: 'в среднем на просмотр', delta: relDelta(m.avgActiveSeconds, p.avgActiveSeconds) },
    {
      icon: 'ctr', label: 'CTR карточка -> сайт', value: pct(m.cardClicks, m.cardOpens), fill: ctr,
      sub: `${count(m.cardClicks, 'переход', 'перехода', 'переходов')} из ${count(m.cardOpens, 'показа', 'показов', 'показов')}`,
      delta: ppDelta(ctr, ratio(p.cardClicks, p.cardOpens)),
    },
    {
      icon: 'scroll', label: 'Глубина прокрутки', value: m.scrolledViews ? `${Math.round(m.avgScroll)}%` : '-',
      fill: m.scrolledViews ? m.avgScroll / 100 : null, sub: 'в среднем по сайту',
      delta: ppDelta(m.scrolledViews ? m.avgScroll / 100 : null, p.scrolledViews ? p.avgScroll / 100 : null),
    },
    {
      icon: 'read', label: 'Дочитываемость', value: pct(m.readThrough, m.scrolledViews), fill: read, sub: 'долистали до 90%',
      delta: ppDelta(read, ratio(p.readThrough, p.scrolledViews)),
    },
  ]
  const steps = (labels: string[], rows: api.Metrics['funnel']) =>
    labels.map((label, step) => ({ label, visitors: rows.find(f => f.step === step)?.visitors ?? 0 }))
  const funnel = steps(FUNNEL_LABEL, m.funnel)
  const sent = funnel[funnel.length - 1].visitors
  const auth = steps(AUTH_FUNNEL_LABEL, m.authFunnel)
  return (
    <div className="mod-metrics-body">
      <section className="mod-panel mod-hero">
        <div className="mod-hero-head">
          <div>
            <span className="mod-dim">Уникальные посетители сайтов</span>
            <div className="mod-hero-value"><b>{num(m.visitors)}</b><DeltaPill d={relDelta(m.visitors, p.visitors)} days={m.days} /></div>
            <span className="mod-dim">{p.visitors ? `к предыдущим ${count(m.days, 'дню', 'дням', 'дням')}` : 'за прошлый период данных нет'}</span>
          </div>
          <dl className="mod-hero-side">
            <div><dt className="mod-dim">Просмотры сайтов</dt><dd><b>{num(m.views)}</b><DeltaPill d={relDelta(m.views, p.views)} days={m.days} /></dd></div>
            <div><dt className="mod-dim">Открыли карточку</dt><dd><b>{num(m.cardOpens)}</b><DeltaPill d={relDelta(m.cardOpens, p.cardOpens)} days={m.days} /></dd></div>
          </dl>
        </div>
        <VisitorsChart series={fillDays(m.daily, m.days)} />
      </section>

      <div className="mod-kpis">{tiles.map(k => <KpiTile key={k.label} k={k} days={m.days} />)}</div>

      <section className="mod-panel">
        <div className="mod-panel-head">
          <h2>Конверсия шагов формы</h2>
          <span className="mod-dim">{funnel[0].visitors ? `до отправки дошли ${pct(sent, funnel[0].visitors)}` : 'форму за период не открывали'}</span>
        </div>
        <Funnel steps={funnel} />
      </section>

      <section className="mod-panel">
        <div className="mod-panel-head">
          <h2>Вход перед заявкой</h2>
          <span className="mod-dim">{auth[0].visitors
            ? `гости из "Предложить событие": ушли до входа ${pct(auth[0].visitors - auth[2].visitors, auth[0].visitors)}`
            : 'гости за период не нажимали "Предложить событие"'}</span>
        </div>
        <Funnel steps={auth} />
      </section>

      <section className="mod-panel">
        <div className="mod-panel-head"><h2>Рейтинг событий</h2><span className="mod-dim">по уникальным посетителям</span></div>
        <EventsRating events={m.events} />
      </section>
      <p className="mod-dim mod-metrics-note">
        Посетитель - аккаунт, у гостя - браузер. Изменение считается к такому же числу дней перед периодом.
        Время, прокрутка и CTR считаются с 14 октября 2026, когда начался сбор.
        CAC, LTV, выручка и ROMI не считаются: на сайте нет платежей и рекламного бюджета.
      </p>
    </div>
  )
}

const Moderators: React.FC<{ me: Me }> = ({ me }) => {
  const expired = React.useContext(ExpiredContext)
  const list = useLoad(() => api.listModerators(), [])
  const [form, setForm] = useState({ name: '', email: '', role: 'admin' as Role })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [editing, setEditing] = useState<number | null>(null)
  const act = async (action: () => Promise<ApiResult>, onOk?: () => void) => {
    setBusy(true)
    setNote('')
    const res = await action()
    setBusy(false)
    if (res.ok) { onOk?.(); list.reload(); return }
    if (res.status === 401) expired()
    else if (res.fields) setErrors(res.fields)
    else setNote(api.problem(res))
  }
  const create = (e: React.FormEvent) => {
    e.preventDefault()
    setErrors({})
    act(() => api.createModerator({ ...form, name: form.name.trim(), email: form.email.trim() }),
      () => setForm({ name: '', email: '', role: 'admin' }))
  }
  return (
    <section className="mod-narrow">
      <h1 className="mod-title">Модераторы</h1>
      <p className="mod-dim">Назначайте роль зарегистрированному пользователю по его почте. Он входит по обычному коду.</p>
      {note && <p className="mod-error" role="alert">{note}</p>}
      <Rows state={list} empty="Никого нет">
        {(data: { items: Moderator[] }) => (
          <ul className="mod-rows">
            {data.items.map(m => m.id === editing ? (
              <li key={m.id} className="mod-row">
                <ModeratorEdit moderator={m} onCancel={() => setEditing(null)} onSaved={() => { setEditing(null); list.reload() }} />
              </li>
            ) : (
              <li key={m.id} className="mod-row mod-mod">
                <span className="mod-grow-col">
                  <b>{m.name || m.email}{m.id === me.id && ' (вы)'}</b>
                  <span className="mod-dim">{m.email} · с {formatDate(m.createdAt)}</span>
                </span>
                <Field className="mod-select"><select value={m.role} disabled={busy || m.id === me.id} aria-label={`Роль ${m.email}`}
                  onChange={e => act(() => api.changeRole(m.id, e.target.value as Role))}>
                  <option value="admin">Модератор</option><option value="super_admin">Супер-админ</option>
                </select></Field>
                {m.id !== me.id && <>
                  <button type="button" className="pm-btn-text" disabled={busy} onClick={() => setEditing(m.id)}>Изменить</button>
                  <button type="button" className="pm-btn-text danger" disabled={busy}
                    onClick={() => confirm(`Снять роль с ${m.email}?`) && act(() => api.deleteModerator(m.id))}>Снять роль</button>
                </>}
              </li>
            ))}
          </ul>
        )}
      </Rows>
      <form className="mod-card mod-form" onSubmit={create} noValidate>
        <h3>Назначить роль</h3>
        <div className="mod-pair">
          <Field label="Имя" error={errors.name}><input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} maxLength={100} /></Field>
          <Field label="Почта пользователя" error={errors.email}><input type="email" value={form.email} onChange={e => setForm(f => ({ ...f, email: e.target.value }))} maxLength={254} required /></Field>
        </div>
        <Field label="Роль" error={errors.role}><select value={form.role} onChange={e => setForm(f => ({ ...f, role: e.target.value as Role }))}>
          <option value="admin">Модератор</option><option value="super_admin">Супер-админ</option>
        </select></Field>
        <div className="mod-actions"><button className="pm-btn" disabled={busy}>Назначить</button></div>
      </form>
    </section>
  )
}

const ModeratorEdit: React.FC<{ moderator: Moderator; onCancel: () => void; onSaved: () => void }> = ({ moderator: m, onCancel, onSaved }) => {
  const [form, setForm] = useState({ name: m.name, email: m.email })
  const s = useSubmit()
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name.trim()) return s.setErrors({ name: 'Введите имя' })
    if (await s.submit(() => api.updateModerator(m.id, { name: form.name.trim(), email: form.email.trim() }))) onSaved()
  }
  return (
    <form className="mod-edit" onSubmit={save} noValidate>
      <b>Изменить {m.email}</b>
      <div className="mod-pair">
        <Field label="Имя" error={s.errors.name}><input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} maxLength={100} autoFocus /></Field>
        <Field label="Почта пользователя" error={s.errors.email}><input type="email" value={form.email} onChange={e => setForm(f => ({ ...f, email: e.target.value }))} maxLength={254} /></Field>
      </div>
      {s.noteLine}
      <div className="mod-actions"><button className="pm-btn" disabled={s.busy}>Сохранить</button><button type="button" className="pm-btn-text" onClick={onCancel}>Отмена</button></div>
    </form>
  )
}

// Ключи Postbox задаются на сервере. Здесь статус, отправитель писем и проверочное письмо.
const Mail: React.FC = () => {
  const state = useLoad(() => api.getMail(), [])
  const phase = useSettle(!state.data && !state.error, !!state.data)
  const mail = phase === 'idle' ? state.data?.mail : undefined
  const [email, setEmail] = useState('')
  const s = useSubmit()
  const send = (e: React.FormEvent) => {
    e.preventDefault()
    if (!email.trim()) return s.setErrors({ email: 'Введите адрес' })
    s.submit(() => api.testMail(email.trim()), `Письмо для ${email.trim()} отправлено. Проверьте входящие и папку "Спам".`)
  }
  return (
    <section className="mod-narrow">
      <h1 className="mod-title">Почта</h1>
      <p className="mod-dim">Сайт отправляет письма: коды входа и решения по заявкам авторам, уведомления о новых заявках модераторам.</p>
      {!mail
        ? phase === 'idle'
          ? <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
          : <Loading phase={phase} label="Загружаем настройки почты" done="Загружено" className="mod-dim" />
        : <>
          <div className="mod-card mod-status">
            <span className={`mod-dot${mail.configured ? ' on' : ''}`} aria-hidden="true" />
            {mail.configured
              ? <span>Почта подключена</span>
              : <span>Почта не подключена: письма не уходят</span>}
          </div>
          {mail.configured && <MailSender mail={mail} onSaved={state.reload} />}
          <form className="mod-card mod-form" onSubmit={send} noValidate>
            <h3>Проверка</h3>
            <p className="mod-dim">Отправьте письмо себе и убедитесь, что оно дошло и не попало в спам.</p>
            <Field label="Адрес" error={s.errors.email}>
              <input type="email" value={email} onChange={e => { setEmail(e.target.value); s.clear('email') }} autoComplete="email" maxLength={254} placeholder="you@example.ru" />
            </Field>
            {s.noteLine}
            <div className="mod-actions"><button className="pm-btn" disabled={s.busy || !mail.configured}>{s.busy ? 'Отправляем...' : 'Отправить письмо'}</button></div>
          </form>
        </>}
    </section>
  )
}

const MailSender: React.FC<{ mail: api.MailStatus; onSaved: () => void }> = ({ mail, onSaved }) => {
  const [form, setForm] = useState({ name: mail.name ?? '', address: mail.address ?? '' })
  const s = useSubmit()
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => { setForm(f => ({ ...f, [k]: e.target.value })); s.clear(k) }
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.address.trim()) return s.setErrors({ address: 'Введите адрес' })
    if (await s.submit(() => api.setMailSender({ name: form.name.trim(), address: form.address.trim() }), 'Сохранено')) onSaved()
  }
  return (
    <form className="mod-card mod-form" onSubmit={save} noValidate>
      <h3>Отправитель</h3>
      <p className="mod-dim">Так письма подписаны в поле "От" у получателя. Меняется сразу для всех писем сайта.</p>
      <div className="mod-pair">
        <Field label="Имя" error={s.errors.name}><input value={form.name} onChange={set('name')} maxLength={100} placeholder="Россия - страна Героев" /></Field>
        <Field label="Адрес" error={s.errors.address}><input type="email" value={form.address} onChange={set('address')} maxLength={254} placeholder={`info@${mail.domain}`} /></Field>
      </div>
      <p className="mod-dim">Адрес только на домене {mail.domain}: с другого адреса письма не уйдут.</p>
      {s.noteLine}
      <div className="mod-actions"><button className="pm-btn" disabled={s.busy}>{s.busy ? 'Сохраняем...' : 'Сохранить'}</button></div>
    </form>
  )
}

const NO_CANDIDATES = () => Promise.resolve<ApiResult<{ items: TelegramChat[] }>>({ ok: true, data: { items: [] } })

// Кто получает уведомления бота: доступ выдаётся тем, кто сам написал боту.
const Bot: React.FC = () => {
  const expired = React.useContext(ExpiredContext)
  const state = useLoad(() => api.getBot(), [])
  const phase = useSettle(!state.data && !state.error, !!state.data)
  const bot = phase === 'idle' ? state.data?.bot : undefined
  // Кандидатов просим сразу по ответу, не дожидаясь, пока доиграет индикатор.
  const configured = state.data?.bot.configured
  const candidates = useLoad(configured ? api.listBotCandidates : NO_CANDIDATES, [configured])
  const [token, setToken] = useState('')
  const [tokenError, setTokenError] = useState('')
  // Итог действия показывается у блока, где его сделали: у токена или у списка получателей.
  const [note, setNote] = useState<{ text: string; bad: boolean; at: 'token' | 'list' } | null>(null)
  const [busy, setBusy] = useState(false)

  const act = async (at: 'token' | 'list', action: () => Promise<ApiResult>, done?: string) => {
    setBusy(true)
    setNote(null)
    const res = await action()
    setBusy(false)
    if (res.ok) {
      state.reload()
      candidates.reload()
      if (done) setNote({ text: done, bad: false, at })
      return true
    }
    if (res.status === 401) expired()
    else if (res.fields?.token) setTokenError(res.fields.token)
    else setNote({ text: api.problem(res), bad: true, at })
    return false
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    setTokenError('')
    if (!token.trim()) return setTokenError('Вставьте токен')
    if (await act('token', () => api.setBotToken(token.trim()), 'Токен сохранён')) setToken('')
  }

  if (!bot) {
    return (
      <section className="mod-narrow">
        <h1 className="mod-title">Бот</h1>
        {phase === 'idle'
          ? <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
          : <Loading phase={phase} label="Загружаем настройки бота" done="Загружено" className="mod-dim" />}
      </section>
    )
  }
  const noteLine = note && <p className={note.bad ? 'mod-error' : 'mod-dim'} role={note.bad ? 'alert' : 'status'}>{note.text}</p>
  const link = bot.username && <a href={`https://t.me/${bot.username}`} target="_blank" rel="noopener noreferrer">@{bot.username}</a>

  return (
    <section className="mod-narrow">
      <h1 className="mod-title">Бот</h1>
      <p className="mod-dim">Telegram-бот пишет о новых заявках и о заявках, готовых к проверке. Сообщения получают только чаты из списка ниже.</p>
      <form className="mod-card mod-form" onSubmit={save} noValidate>
        <h3>Токен</h3>
        <p className="mod-dim">
          {bot.configured
            ? <>Подключён {link}. Новый токен заменит текущий, получатели сохранятся.</>
            : 'Бот не подключён. Создайте бота у @BotFather командой /newbot и вставьте токен, который он пришлёт.'}
        </p>
        <Field label="Токен из @BotFather" error={tokenError}>
          <input type="password" value={token} autoComplete="off" spellCheck={false} maxLength={200} placeholder="123456789:AA..."
            onChange={e => { setToken(e.target.value); setTokenError('') }} />
        </Field>
        {note?.at === 'token' && noteLine}
        <div className="mod-actions">
          <button className="pm-btn" disabled={busy}>Сохранить</button>
          {bot.configured && (
            <button type="button" className="pm-btn-text danger" disabled={busy}
              onClick={() => confirm('Отключить бота? Уведомления в Telegram перестанут приходить.') && act('token', () => api.setBotToken(''), 'Бот отключён')}>Отключить</button>
          )}
        </div>
      </form>

      {bot.configured && (
        <>
          <h3>Получают уведомления</h3>
          {bot.recipients.length ? (
            <ul className="mod-rows">
              {bot.recipients.map(c => (
                <li key={c.chatId} className="mod-row mod-mod">
                  <ChatName chat={c} />
                  <button type="button" className="pm-btn-text danger" disabled={busy}
                    onClick={() => confirm(`Закрыть доступ для ${c.title}? Уведомления туда больше не придут.`) && act('list', () => api.revokeBot(c.chatId))}>Закрыть доступ</button>
                </li>
              ))}
            </ul>
          ) : <p className="mod-dim">Пока никто. Выдайте доступ тем, кто написал боту.</p>}
          {note?.at === 'list' && noteLine}
          {bot.recipients.length > 0 && (
            <div className="mod-actions">
              <button type="button" className="pm-btn-ghost" disabled={busy} onClick={() => act('list', api.testBot, 'Проверка дошла до всех получателей')}>Отправить проверку</button>
            </div>
          )}

          <h3>Написали боту</h3>
          <p className="mod-dim">
            Человек открывает {link} в Telegram и нажимает Start. Чтобы уведомления шли в группу, добавьте бота в неё.
            Потом обновите список: Telegram помнит обращения к боту сутки.
          </p>
          <Rows state={candidates} empty="За последние сутки боту никто не писал">
            {(data: { items: TelegramChat[] }) => (
              <ul className="mod-rows">
                {data.items.map(c => (
                  <li key={c.chatId} className="mod-row mod-mod">
                    <ChatName chat={c} />
                    <button type="button" className="pm-btn-ghost" disabled={busy} onClick={() => act('list', () => api.grantBot(c.chatId), `${c.title} получает уведомления`)}>Дать доступ</button>
                  </li>
                ))}
              </ul>
            )}
          </Rows>
          <div className="mod-actions">
            <button type="button" className="pm-btn-text" disabled={candidates.loading} onClick={candidates.reload}>{candidates.loading ? 'Обновляем...' : 'Обновить список'}</button>
          </div>
        </>
      )}
    </section>
  )
}

const ChatName: React.FC<{ chat: TelegramChat }> = ({ chat }) => (
  <span className="mod-grow-col">
    <b>{chat.title}</b>
    <span className="mod-dim">{chat.username ? `@${chat.username}` : chat.chatId < 0 ? 'группа или канал' : 'без username'}</span>
  </span>
)
