import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Logo, useIsMobile } from '../public/PublicMap.tsx'
import { MapView, type MapHandle } from '../public/MapView.tsx'
import { TYPES, TYPE_LABEL, formatDate, type ApiResult, type EventType } from '../public/api.ts'
import { Field } from '../../shared/ui/Field.tsx'
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

// Кончилась сессия - любой 401 возвращает на форму входа.
const ExpiredContext = React.createContext<() => void>(noop)

export const Panel: React.FC = () => {
  const [me, setMe] = useState<Me | null | undefined>(undefined)
  const [notice, setNotice] = useState('')
  useEffect(() => { api.getSession().then(r => setMe(r.ok ? r.data.admin : null)) }, [])
  if (me === undefined) return <div className="pm-root mod-page" />
  if (!me) return <Login notice={notice} onDone={m => { setNotice(''); setMe(m) }} />
  return (
    <ExpiredContext.Provider value={() => setMe(null)}>
      <Shell me={me} onMe={setMe} onLogout={() => { api.logout(); setMe(null) }}
        onRelogin={() => { setNotice('Пароль изменён. Войдите с новым паролем.'); setMe(null) }} />
    </ExpiredContext.Provider>
  )
}

const Login: React.FC<{ notice: string; onDone: (me: Me) => void }> = ({ notice, onDone }) => {
  const navigate = useNavigate()
  const [form, setForm] = useState({ login: '', password: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    const next: Record<string, string> = {}
    if (!form.login.trim()) next.login = 'Введите логин'
    if (!form.password) next.password = 'Введите пароль'
    setErrors(next)
    if (Object.keys(next).length || busy) return
    setBusy(true)
    const res = await api.login(form.login.trim(), form.password)
    setBusy(false)
    if (res.ok) return onDone(res.data.admin)
    if (res.fields) setErrors(res.fields)
    else setErrors({ form: res.status === 401 ? 'Неверный логин или пароль' : api.problem(res) })
  }

  return (
    <div className="pm-root mod-page mod-login">
      <header className="mod-top"><Logo onClick={() => navigate('/map')} /></header>
      <form className="mod-card mod-login-card" onSubmit={submit} noValidate>
        <h1 className="mod-title">Модерация</h1>
        <p className="mod-dim">Вход для модераторов</p>
        {notice && <p className="mod-note" role="status">{notice}</p>}
        <Field label="Логин" error={errors.login}>
          <input value={form.login} autoFocus autoComplete="username" maxLength={100}
            onChange={e => setForm(f => ({ ...f, login: e.target.value }))} />
        </Field>
        <Field label="Пароль" error={errors.password}>
          <input type="password" value={form.password} autoComplete="current-password" maxLength={128}
            onChange={e => setForm(f => ({ ...f, password: e.target.value }))} />
        </Field>
        {errors.form && <p className="mod-error" role="alert">{errors.form}</p>}
        <button className="pm-btn mod-wide" disabled={busy}>{busy ? 'Входим...' : 'Войти'}</button>
      </form>
    </div>
  )
}

type Tab = 'submissions' | 'history' | 'moderators' | 'bot' | 'mail' | 'profile'

// Иконки меню: контур 24x24, stroke задаёт .pm-menu-item svg.
const ICON: Record<Tab | 'map' | 'logout', string> = {
  submissions: 'M4 13h4l1.5 3h5L16 13h4 M5.5 5h13L20 13v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-5z',
  history: 'M12 7v5l3 2 M3.5 12a8.5 8.5 0 1 0 2.5-6 M3 4v4h4',
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

const Shell: React.FC<{ me: Me; onMe: (me: Me) => void; onLogout: () => void; onRelogin: () => void }> = ({ me, onMe, onLogout, onRelogin }) => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const superAdmin = me.role === 'super_admin'
  const account: [string, [Tab, string][]] = ['Аккаунт', [['profile', 'Профиль']]]
  const groups: [string, [Tab, string][]][] = superAdmin
    ? [['Модерация', [['submissions', 'Заявки'], ['history', 'История']]],
        ['Управление', [['moderators', 'Модераторы'], ['bot', 'Бот'], ['mail', 'Почта']]], account]
    : [['Модерация', [['submissions', 'Заявки']]], account]
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
      {tab === 'moderators' && <Moderators me={me} onProfile={() => open('profile')} />}
      {tab === 'bot' && <Bot />}
      {tab === 'mail' && <Mail />}
      {tab === 'profile' && <Profile onSaved={a => onMe({ ...me, name: a.name })} onRelogin={onRelogin} />}
    </main>
  )

  // На телефоне профиль - кнопкой в шапке: в нижней полосе места на пять вкладок.
  const barTabs = tabs.filter(([t]) => t !== 'profile')
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
        <button type="button" className={`mod-side-back${tab === 'profile' ? ' on' : ''}`} title="Профиль" aria-label="Профиль"
          aria-current={tab === 'profile' ? 'page' : undefined} onClick={() => open('profile')}><Icon name="profile" /></button>
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
  if (state.error) return <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
  if (!state.data) return <p className="mod-dim">Загружаем...</p>
  if (!state.data.items.length) return <p className="mod-dim mod-empty">{empty}</p>
  return <div className={state.loading ? 'mod-stale' : ''}>{children(state.data)}</div>
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
      <dl className="mod-facts">
        <dt>Номер</dt><dd>{s.publicId}</dd>
        <dt>Тип</dt><dd>{TYPE_LABEL[s.eventType]}</dd>
        <dt>Дата события</dt><dd>{formatDate(s.eventDate)}</dd>
        <dt>Точка автора</dt><dd>{s.latitude !== undefined ? `${s.latitude.toFixed(5)}, ${s.longitude!.toFixed(5)}` : 'не указана'}</dd>
        <dt>Почта</dt><dd><a href={`mailto:${s.email}`}>{s.email}</a></dd>
        <dt>Telegram</dt><dd><a href={`https://t.me/${s.telegramUsername}`} target="_blank" rel="noopener noreferrer">@{s.telegramUsername}</a></dd>
        <dt>Прислана</dt><dd>{api.dateTime(s.createdAt)}</dd>
        <dt>Изменена</dt><dd>{api.dateTime(s.updatedAt)}</dd>
      </dl>
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

const History: React.FC = () => {
  const [q, setQ] = useState('')
  const query = useDebounced(q.trim())
  const [page, setPage] = useState(1)
  const list = useLoad(() => api.listAudit({ q: query, page }), [query, page])
  useEffect(() => setPage(1), [query])
  return (
    <section className="mod-narrow">
      <h1 className="mod-title">История действий</h1>
      <p className="mod-dim">Каждое действие модератора. Записи нельзя изменить или удалить.</p>
      <Field><input type="search" aria-label="Поиск по истории" placeholder="Поиск по заявке, логину или действию" value={q} onChange={e => setQ(e.target.value)} /></Field>
      <Rows state={list} empty="Действий пока нет">
        {(data: Page<AuditEntry>) => (
          <>
            <ul className="mod-rows">
              {data.items.map(a => (
                <li key={a.id} className="mod-row mod-log">
                  <span className="mod-row-head"><b>{a.adminLogin} <span className={`mod-act ${a.action}`}>{ACTION_LABEL[a.action].toLowerCase()}</span> {a.submissionPublicId} {a.submissionTitle}</b></span>
                  <span className="mod-dim">{api.dateTime(a.createdAt)} · {api.ROLE_LABEL[a.adminRole] ?? a.adminRole}</span>
                  {a.comment && <span className="mod-log-comment">{a.comment}</span>}
                </li>
              ))}
            </ul>
            <Pager page={page} pageSize={data.pageSize} total={data.total} onPage={setPage} />
          </>
        )}
      </Rows>
    </section>
  )
}

const EMPTY = { login: '', name: '', password: '', email: '', role: 'admin' as Role }

// Пароль модератору: 14 символов, в нём есть заглавная, строчная, цифра и знак из _-!@#$%.
// Без похожих друг на друга I, O, l, 0, 1: пароль часто диктуют или переписывают.
const PASS_SETS = ['ABCDEFGHJKLMNPQRSTUVWXYZ', 'abcdefghijkmnopqrstuvwxyz', '23456789', '_-!@#$%']
const PASS_ALL = PASS_SETS.join('')
const randomIndex = (n: number) => crypto.getRandomValues(new Uint32Array(1))[0] % n
function generatePassword(length = 14) {
  const chars = PASS_SETS.map(set => set[randomIndex(set.length)])
  while (chars.length < length) chars.push(PASS_ALL[randomIndex(PASS_ALL.length)])
  for (let i = chars.length - 1; i > 0; i--) {
    const j = randomIndex(i + 1);
    [chars[i], chars[j]] = [chars[j], chars[i]]
  }
  return chars.join('')
}

const Moderators: React.FC<{ me: Me; onProfile: () => void }> = ({ me, onProfile }) => {
  const expired = React.useContext(ExpiredContext)
  const list = useLoad(() => api.listModerators(), [])
  const [form, setForm] = useState(EMPTY)
  const [generated, setGenerated] = useState('')
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [editing, setEditing] = useState<number | null>(null)

  const act = async (action: () => Promise<ApiResult>, onOk?: () => void) => {
    setBusy(true)
    setNote('')
    const res = await action()
    setBusy(false)
    if (res.ok) { onOk?.(); list.reload(); return true }
    if (res.status === 401) expired()
    else if (res.fields) setErrors(res.fields)
    else setNote(res.message ?? api.problem(res))
    return false
  }

  const create = (e: React.FormEvent) => {
    e.preventDefault()
    setErrors({})
    act(() => api.createModerator({ ...form, login: form.login.trim(), name: form.name.trim(), email: form.email.trim() }), () => setForm(EMPTY))
  }
  const set = (k: keyof typeof EMPTY) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setForm(f => ({ ...f, [k]: e.target.value }))
    setErrors(er => ({ ...er, [k]: '' }))
  }

  return (
    <section className="mod-narrow">
      <h1 className="mod-title">Модераторы</h1>
      <p className="mod-dim">Модератор проверяет заявки. Супер-админ ещё видит историю и управляет модераторами.</p>
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
                  <b>{m.name || m.login}{m.id === me.id && ' (вы)'}</b>
                  <span className="mod-dim">{m.name && `${m.login} · `}{m.email || 'почта не указана'} · с {formatDate(m.createdAt)}</span>
                </span>
                <Field className="mod-select"><select value={m.role} disabled={busy || m.id === me.id} aria-label={`Роль ${m.login}`}
                  onChange={e => act(() => api.changeRole(m.id, e.target.value as Role))}>
                  <option value="admin">Модератор</option>
                  <option value="super_admin">Супер-админ</option>
                </select></Field>
                <button type="button" className="pm-btn-text" disabled={busy}
                  onClick={() => m.id === me.id ? onProfile() : setEditing(m.id)}>Изменить</button>
                {m.id !== me.id && (
                  <button type="button" className="pm-btn-text danger" disabled={busy}
                    onClick={() => confirm(`Удалить ${m.login}? Войти он больше не сможет.`) && act(() => api.deleteModerator(m.id))}>Удалить</button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Rows>

      <form className="mod-card mod-form" onSubmit={create} noValidate>
        <h3>Добавить модератора</h3>
        <div className="mod-pair">
          <Field label="Имя" error={errors.name}><input value={form.name} onChange={set('name')} autoComplete="off" maxLength={100} placeholder="Иван Иванов" /></Field>
          <Field label="Логин" error={errors.login}><input value={form.login} onChange={set('login')} autoComplete="off" maxLength={100} placeholder="ivanov" /></Field>
        </div>
        <div className="mod-pair">
          <Field label="Пароль" error={errors.password} after={
            <button type="button" className="pm-field-btn" title="Сгенерировать пароль" aria-label="Сгенерировать пароль" onClick={() => {
              const password = generatePassword()
              setGenerated(password)
              setForm(f => ({ ...f, password }))
              setErrors(er => ({ ...er, password: '' }))
            }}>
              <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true"><path d="M20 12a8 8 0 0 1-14.3 4.9 M4 12a8 8 0 0 1 14.3-4.9 M18.5 3v4.5H14 M5.5 21v-4.5H10" /></svg>
            </button>
          }>
            {/* Сгенерированный пароль виден, чтобы его можно было скопировать и передать модератору. */}
            <input type={form.password && form.password === generated ? 'text' : 'password'} value={form.password} onChange={set('password')}
              autoComplete="new-password" spellCheck={false} placeholder="не короче 12 символов" />
          </Field>
          <Field label="Почта для уведомлений" error={errors.email}><input type="email" value={form.email} onChange={set('email')} maxLength={254} placeholder="можно пусто" /></Field>
        </div>
        <div className="mod-pair">
          <Field label="Роль" error={errors.role}>
            <select value={form.role} onChange={set('role')}>
              <option value="admin">Модератор</option>
              <option value="super_admin">Супер-админ</option>
            </select>
          </Field>
        </div>
        <div className="mod-actions"><button className="pm-btn" disabled={busy}>Добавить</button></div>
      </form>
    </section>
  )
}

// Правка чужой учётки. Свою супер-админ правит в профиле: там пароль меняется только с текущим.
const ModeratorEdit: React.FC<{ moderator: Moderator; onCancel: () => void; onSaved: () => void }> = ({ moderator: m, onCancel, onSaved }) => {
  const [form, setForm] = useState({ name: m.name, email: m.email, password: '' })
  const s = useSubmit()
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => { setForm(f => ({ ...f, [k]: e.target.value })); s.clear(k) }
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name.trim()) return s.setErrors({ name: 'Введите имя' })
    if (await s.submit(() => api.updateModerator(m.id, { name: form.name.trim(), email: form.email.trim(), password: form.password }))) onSaved()
  }
  return (
    <form className="mod-edit" onSubmit={save} noValidate>
      <b>Изменить {m.login}</b>
      <div className="mod-pair">
        <Field label="Имя" error={s.errors.name}><input value={form.name} onChange={set('name')} autoComplete="off" maxLength={100} autoFocus /></Field>
        <Field label="Почта для уведомлений" error={s.errors.email}><input type="email" value={form.email} onChange={set('email')} maxLength={254} placeholder="можно пусто" /></Field>
      </div>
      <div className="mod-pair">
        <Field label="Новый пароль" error={s.errors.password}>
          <input type="password" value={form.password} onChange={set('password')} autoComplete="new-password" placeholder="пусто - пароль прежний" />
        </Field>
      </div>
      {form.password && <p className="mod-dim">После смены пароля модератора выкинет из панели на всех устройствах.</p>}
      {s.noteLine}
      <div className="mod-actions">
        <button className="pm-btn" disabled={s.busy}>{s.busy ? 'Сохраняем...' : 'Сохранить'}</button>
        <button type="button" className="pm-btn-text" onClick={onCancel}>Отмена</button>
      </div>
    </form>
  )
}

const Profile: React.FC<{ onSaved: (m: Moderator) => void; onRelogin: () => void }> = ({ onSaved, onRelogin }) => {
  const state = useLoad(() => api.getProfile(), [])
  return (
    <section className="mod-narrow">
      <h1 className="mod-title">Профиль</h1>
      {state.data
        ? <ProfileForm admin={state.data.admin} onSaved={onSaved} />
        : state.error
          ? <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
          : <p className="mod-dim">Загружаем...</p>}
      <PasswordForm onDone={onRelogin} />
    </section>
  )
}

const ProfileForm: React.FC<{ admin: Moderator; onSaved: (m: Moderator) => void }> = ({ admin, onSaved }) => {
  const [form, setForm] = useState({ name: admin.name, email: admin.email })
  const s = useSubmit()
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => { setForm(f => ({ ...f, [k]: e.target.value })); s.clear(k) }
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name.trim()) return s.setErrors({ name: 'Введите имя' })
    const res = await s.submit(() => api.updateProfile({ name: form.name.trim(), email: form.email.trim() }), 'Сохранено')
    if (res) { setForm({ name: res.admin.name, email: res.admin.email }); onSaved(res.admin) }
  }
  return (
    <form className="mod-card mod-form" onSubmit={save} noValidate>
      <h3>Данные</h3>
      <p className="mod-dim">Логин {admin.login}, {api.ROLE_LABEL[admin.role].toLowerCase()}. Логин и роль меняет супер-админ.</p>
      <div className="mod-pair">
        <Field label="Имя" error={s.errors.name}><input value={form.name} onChange={set('name')} autoComplete="name" maxLength={100} placeholder="Иван Иванов" /></Field>
        <Field label="Почта для уведомлений" error={s.errors.email}><input type="email" value={form.email} onChange={set('email')} autoComplete="email" maxLength={254} placeholder="можно пусто" /></Field>
      </div>
      <p className="mod-dim">На почту приходят письма о новых заявках. Пустое поле - писем не будет.</p>
      {s.noteLine}
      <div className="mod-actions"><button className="pm-btn" disabled={s.busy}>{s.busy ? 'Сохраняем...' : 'Сохранить'}</button></div>
    </form>
  )
}

const PasswordForm: React.FC<{ onDone: () => void }> = ({ onDone }) => {
  const [form, setForm] = useState({ currentPassword: '', password: '', repeat: '' })
  const s = useSubmit()
  const set = (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) => { setForm(f => ({ ...f, [k]: e.target.value })); s.clear(k) }
  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    const next: Record<string, string> = {}
    if (!form.currentPassword) next.currentPassword = 'Введите текущий пароль'
    if (!form.password) next.password = 'Введите новый пароль'
    else if (form.repeat !== form.password) next.repeat = 'Пароли не совпадают'
    if (Object.keys(next).length) return s.setErrors(next)
    if (await s.submit(() => api.changePassword(form.currentPassword, form.password))) onDone()
  }
  return (
    <form className="mod-card mod-form" onSubmit={save} noValidate>
      <h3>Пароль</h3>
      <p className="mod-dim">После смены пароля панель попросит войти заново, на остальных устройствах сессии закроются.</p>
      <Field label="Текущий пароль" error={s.errors.currentPassword}>
        <input type="password" value={form.currentPassword} onChange={set('currentPassword')} autoComplete="current-password" />
      </Field>
      <div className="mod-pair">
        <Field label="Новый пароль" error={s.errors.password}>
          <input type="password" value={form.password} onChange={set('password')} autoComplete="new-password" placeholder="не короче 12 символов" />
        </Field>
        <Field label="Повторите пароль" error={s.errors.repeat}>
          <input type="password" value={form.repeat} onChange={set('repeat')} autoComplete="new-password" />
        </Field>
      </div>
      {s.noteLine}
      <div className="mod-actions"><button className="pm-btn" disabled={s.busy}>{s.busy ? 'Меняем...' : 'Сменить пароль'}</button></div>
    </form>
  )
}

// Ключи Postbox задаются на сервере. Здесь статус, отправитель писем и проверочное письмо.
const Mail: React.FC = () => {
  const state = useLoad(() => api.getMail(), [])
  const mail = state.data?.mail
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
        ? state.error
          ? <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
          : <p className="mod-dim">Загружаем...</p>
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
  const bot = state.data?.bot
  const candidates = useLoad(bot?.configured ? api.listBotCandidates : NO_CANDIDATES, [bot?.configured])
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
        {state.error
          ? <p className="mod-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
          : <p className="mod-dim">Загружаем...</p>}
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
