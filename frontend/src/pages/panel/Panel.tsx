import React, { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Logo, useIsMobile } from '../public/PublicMap.tsx'
import { MapView, type MapHandle } from '../public/MapView.tsx'
import { TYPES, TYPE_LABEL, formatDate, type ApiResult, type EventType } from '../public/api.ts'
import * as api from './api.ts'
import type { AuditEntry, Me, Moderator, Page, Role, Status, Submission } from './api.ts'
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
  useEffect(() => { api.getSession().then(r => setMe(r.ok ? r.data.admin : null)) }, [])
  if (me === undefined) return <div className="pm-root ad-page" />
  if (!me) return <Login onDone={setMe} />
  return (
    <ExpiredContext.Provider value={() => setMe(null)}>
      <Shell me={me} onLogout={() => { api.logout(); setMe(null) }} />
    </ExpiredContext.Provider>
  )
}

const Login: React.FC<{ onDone: (me: Me) => void }> = ({ onDone }) => {
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
    <div className="pm-root ad-page ad-login">
      <header className="ad-top"><Logo onClick={() => navigate('/map')} /></header>
      <form className="ad-card ad-login-card" onSubmit={submit} noValidate>
        <h1 className="ad-title">Модерация</h1>
        <p className="ad-dim">Вход для модераторов</p>
        <Field label="Логин" error={errors.login}>
          <input value={form.login} autoFocus autoComplete="username" maxLength={100}
            onChange={e => setForm(f => ({ ...f, login: e.target.value }))} />
        </Field>
        <Field label="Пароль" error={errors.password}>
          <input type="password" value={form.password} autoComplete="current-password" maxLength={128}
            onChange={e => setForm(f => ({ ...f, password: e.target.value }))} />
        </Field>
        {errors.form && <p className="ad-error" role="alert">{errors.form}</p>}
        <button className="pm-btn ad-wide" disabled={busy}>{busy ? 'Входим...' : 'Войти'}</button>
      </form>
    </div>
  )
}

type Tab = 'submissions' | 'history' | 'moderators'

const Shell: React.FC<{ me: Me; onLogout: () => void }> = ({ me, onLogout }) => {
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const superAdmin = me.role === 'super_admin'
  const tabs: [Tab, string][] = superAdmin
    ? [['submissions', 'Заявки'], ['history', 'История'], ['moderators', 'Модераторы']]
    : [['submissions', 'Заявки']]
  const tab = tabs.find(([t]) => t === params.get('tab'))?.[0] ?? 'submissions'

  return (
    <div className={`pm-root ad-page${mobile ? ' mobile' : ''}`}>
      <header className="ad-top">
        <Logo short={mobile} onClick={() => navigate('/map')} />
        {tabs.length > 1 && (
          <nav className="ad-tabs">
            {tabs.map(([t, label]) => (
              <button key={t} type="button" className={t === tab ? 'on' : ''} onClick={() => setParams(t === 'submissions' ? {} : { tab: t })}>{label}</button>
            ))}
          </nav>
        )}
        <div className="pm-grow" />
        <span className="ad-me">{me.login}<small>{api.ROLE_LABEL[me.role]}</small></span>
        <button type="button" className="pm-btn-text" onClick={onLogout}>Выйти</button>
      </header>
      <main className="ad-main">
        {tab === 'submissions' && <Submissions mobile={mobile} />}
        {tab === 'history' && <History />}
        {tab === 'moderators' && <Moderators me={me} />}
      </main>
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
    <div className="ad-split">
      <section className="ad-list">
        <div className="ad-filters" role="tablist">
          {FILTERS.map(f => (
            <button key={f.label} type="button" role="tab" aria-selected={f.status === status} className={f.status === status ? 'on' : ''}
              onClick={() => { setParams(f.status === 'new' ? {} : { status: f.status || 'all' }); setSelected(null) }}>{f.label}</button>
          ))}
        </div>
        <input className="ad-input" type="search" placeholder="Поиск по названию или номеру" value={q} onChange={e => setQ(e.target.value)} />
        <Rows state={list} empty="Заявок нет">
          {(data: Page<Submission>) => (
            <>
              <ul className="ad-rows">
                {data.items.map(s => (
                  <li key={s.id}>
                    <button type="button" className={`ad-row${selected?.id === s.id ? ' on' : ''}`} onClick={() => setSelected(s)}>
                      <span className="ad-row-head"><b>{s.title}</b><Badge status={s.status} /></span>
                      <span className="ad-dim">#{s.id} · {TYPE_LABEL[s.eventType]} · {formatDate(s.eventDate)} · прислана {api.dateTime(s.createdAt)}</span>
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
        <section className="ad-detail-col">
          {selected
            ? <Detail key={selected.id} submission={selected} onChanged={changed} />
            : <div className="ad-card ad-placeholder ad-dim">Выберите заявку в списке</div>}
        </section>
      )}
    </div>
  )
}

// Список с загрузкой, ошибкой и пустым состоянием.
function Rows<T extends { items: unknown[] }>({ state, empty, children }: {
  state: { data?: T; error?: string; loading: boolean; reload: () => void }; empty: string; children: (data: T) => React.ReactNode
}) {
  if (state.error) return <p className="ad-error">{state.error} <button type="button" className="pm-btn-text" onClick={state.reload}>Повторить</button></p>
  if (!state.data) return <p className="ad-dim">Загружаем...</p>
  if (!state.data.items.length) return <p className="ad-dim ad-empty">{empty}</p>
  return <div className={state.loading ? 'ad-stale' : ''}>{children(state.data)}</div>
}

const Pager: React.FC<{ page: number; pageSize: number; total: number; onPage: (p: number) => void }> = ({ page, pageSize, total, onPage }) => {
  const pages = Math.ceil(total / pageSize)
  if (pages <= 1) return null
  return (
    <div className="ad-pager">
      <button type="button" className="pm-btn-ghost" disabled={page <= 1} onClick={() => onPage(page - 1)}>Назад</button>
      <span className="ad-dim">{page} из {pages}</span>
      <button type="button" className="pm-btn-ghost" disabled={page >= pages} onClick={() => onPage(page + 1)}>Дальше</button>
    </div>
  )
}

const Badge: React.FC<{ status: Status }> = ({ status }) => <span className={`ad-badge ${status}`}>{STATUS_LABEL[status]}</span>

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
    <article className="ad-card ad-detail">
      {onBack && <button type="button" className="pm-btn-text ad-back" onClick={onBack}>К списку</button>}
      <div className="ad-row-head"><h2 className="ad-title">{s.title}</h2><Badge status={s.status} /></div>
      {s.screenshotUrl && (
        <a href={s.siteUrl} target="_blank" rel="noopener noreferrer" className="ad-shot"><img src={s.screenshotUrl} alt="Снимок присланного сайта" /></a>
      )}
      <p className="ad-desc">{s.description}</p>
      <dl className="ad-facts">
        <dt>Номер</dt><dd>#{s.id}</dd>
        <dt>Тип</dt><dd>{TYPE_LABEL[s.eventType]}</dd>
        <dt>Дата события</dt><dd>{formatDate(s.eventDate)}</dd>
        <dt>Точка автора</dt><dd>{s.latitude !== undefined ? `${s.latitude.toFixed(5)}, ${s.longitude!.toFixed(5)}` : 'не указана'}</dd>
        <dt>Почта</dt><dd><a href={`mailto:${s.email}`}>{s.email}</a></dd>
        <dt>Telegram</dt><dd><a href={`https://t.me/${s.telegramUsername}`} target="_blank" rel="noopener noreferrer">@{s.telegramUsername}</a></dd>
        <dt>Прислана</dt><dd>{api.dateTime(s.createdAt)}</dd>
        <dt>Изменена</dt><dd>{api.dateTime(s.updatedAt)}</dd>
      </dl>
      {s.status === 'rejected' && s.adminComment && <p className="ad-reason"><b>Причина отказа.</b> {s.adminComment}</p>}
      {s.status === 'processing' && <p className="ad-dim">Архив ещё распаковывается, предпросмотр появится через минуту.</p>}

      {mode === 'view' && (
        <div className="ad-actions">
          {s.status === 'new' && <button type="button" className="pm-btn" disabled={busy} onClick={() => run(() => api.reviewSubmission(s.id))}>Проверить</button>}
          {preview}
          {s.status === 'review' && <>
            <button type="button" className="pm-btn" onClick={() => { setMode('approve'); setErrors({}) }}>Одобрить</button>
            <button type="button" className="pm-btn-ghost ad-danger" onClick={() => { setMode('reject'); setErrors({}) }}>Отклонить</button>
          </>}
        </div>
      )}

      {mode === 'approve' && (
        <form className="ad-form" onSubmit={approve} noValidate>
          <h3>Публикация на карте</h3>
          <p className="ad-dim">Проверьте точку: кликните по карте или перетащите метку. Координаты можно ввести вручную.</p>
          <div className="ad-map"><MapView ref={mapRef} events={[]} isActive={always} selectedId={null} onSelect={noop} pad={12} labels
            pick={pick} onPick={busy ? undefined : (a, b) => { setLat(a.toFixed(6)); setLng(b.toFixed(6)); setErrors({}) }} /></div>
          <div className="ad-pair">
            <Field label="Широта" error={errors.latitude}><input inputMode="decimal" value={lat} onChange={e => setLat(e.target.value)} placeholder="55.751244" /></Field>
            <Field label="Долгота" error={errors.longitude}><input inputMode="decimal" value={lng} onChange={e => setLng(e.target.value)} placeholder="37.618423" /></Field>
          </div>
          <div className="ad-label">Тип события</div>
          <div className="pm-types">
            {TYPES.map(t => <button key={t} type="button" className={t === type ? 'on' : ''} aria-pressed={t === type} onClick={() => setType(t)}>{TYPE_LABEL[t]}</button>)}
          </div>
          {errors.eventType && <p className="ad-error">{errors.eventType}</p>}
          {note && <p className="ad-error" role="alert">{note}</p>}
          <div className="ad-actions">
            <button className="pm-btn" disabled={busy}>{busy ? 'Публикуем...' : 'Опубликовать'}</button>
            <button type="button" className="pm-btn-text" onClick={() => setMode('view')}>Отмена</button>
          </div>
        </form>
      )}

      {mode === 'reject' && (
        <form className="ad-form" onSubmit={reject} noValidate>
          <h3>Отклонить заявку</h3>
          <Field label="Причина для автора" error={errors.comment}>
            <textarea rows={4} maxLength={MAX_REASON} autoFocus value={reason} onChange={e => { setReason(e.target.value); setErrors({}) }} />
          </Field>
          <p className="ad-dim">{reason.length} / {MAX_REASON}</p>
          {note && <p className="ad-error" role="alert">{note}</p>}
          <div className="ad-actions">
            <button className="pm-btn ad-danger-fill" disabled={busy}>{busy ? 'Отклоняем...' : 'Отклонить'}</button>
            <button type="button" className="pm-btn-text" onClick={() => setMode('view')}>Отмена</button>
          </div>
        </form>
      )}
      {mode === 'view' && note && <p className="ad-error" role="alert">{note}</p>}
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
    <section className="ad-narrow">
      <h1 className="ad-title">История действий</h1>
      <p className="ad-dim">Каждое действие модератора. Записи нельзя изменить или удалить.</p>
      <input className="ad-input" type="search" placeholder="Поиск по заявке, логину или действию" value={q} onChange={e => setQ(e.target.value)} />
      <Rows state={list} empty="Действий пока нет">
        {(data: Page<AuditEntry>) => (
          <>
            <ul className="ad-rows">
              {data.items.map(a => (
                <li key={a.id} className="ad-row ad-log">
                  <span className="ad-row-head"><b>{a.adminLogin} <span className={`ad-act ${a.action}`}>{ACTION_LABEL[a.action].toLowerCase()}</span> #{a.submissionId} {a.submissionTitle}</b></span>
                  <span className="ad-dim">{api.dateTime(a.createdAt)} · {api.ROLE_LABEL[a.adminRole] ?? a.adminRole}</span>
                  {a.comment && <span className="ad-log-comment">{a.comment}</span>}
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

const EMPTY = { login: '', password: '', email: '', role: 'admin' as Role }

const Moderators: React.FC<{ me: Me }> = ({ me }) => {
  const expired = React.useContext(ExpiredContext)
  const list = useLoad(() => api.listModerators(), [])
  const [form, setForm] = useState(EMPTY)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)

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
    act(() => api.createModerator({ ...form, login: form.login.trim(), email: form.email.trim() }), () => setForm(EMPTY))
  }
  const set = (k: keyof typeof EMPTY) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    setForm(f => ({ ...f, [k]: e.target.value }))
    setErrors(er => ({ ...er, [k]: '' }))
  }

  return (
    <section className="ad-narrow">
      <h1 className="ad-title">Модераторы</h1>
      <p className="ad-dim">Модератор проверяет заявки. Супер-админ ещё видит историю и управляет модераторами.</p>
      {note && <p className="ad-error" role="alert">{note}</p>}
      <Rows state={list} empty="Никого нет">
        {(data: { items: Moderator[] }) => (
          <ul className="ad-rows">
            {data.items.map(m => (
              <li key={m.id} className="ad-row ad-mod">
                <span className="ad-grow-col">
                  <b>{m.login}{m.id === me.id && ' (вы)'}</b>
                  <span className="ad-dim">{m.email || 'почта не указана'} · с {formatDate(m.createdAt)}</span>
                </span>
                <select className="ad-input ad-select" value={m.role} disabled={busy || m.id === me.id} aria-label={`Роль ${m.login}`}
                  onChange={e => act(() => api.changeRole(m.id, e.target.value as Role))}>
                  <option value="admin">Модератор</option>
                  <option value="super_admin">Супер-админ</option>
                </select>
                {m.id !== me.id && (
                  <button type="button" className="pm-btn-text ad-danger" disabled={busy}
                    onClick={() => confirm(`Удалить ${m.login}? Войти он больше не сможет.`) && act(() => api.deleteModerator(m.id))}>Удалить</button>
                )}
              </li>
            ))}
          </ul>
        )}
      </Rows>

      <form className="ad-card ad-form" onSubmit={create} noValidate>
        <h3>Добавить модератора</h3>
        <div className="ad-pair">
          <Field label="Логин" error={errors.login}><input value={form.login} onChange={set('login')} autoComplete="off" maxLength={100} placeholder="ivanov" /></Field>
          <Field label="Пароль" error={errors.password}><input type="password" value={form.password} onChange={set('password')} autoComplete="new-password" placeholder="не короче 12 символов" /></Field>
        </div>
        <div className="ad-pair">
          <Field label="Почта для уведомлений" error={errors.email}><input type="email" value={form.email} onChange={set('email')} maxLength={254} placeholder="можно пусто" /></Field>
          <Field label="Роль" error={errors.role}>
            <select value={form.role} onChange={set('role')}>
              <option value="admin">Модератор</option>
              <option value="super_admin">Супер-админ</option>
            </select>
          </Field>
        </div>
        <div className="ad-actions"><button className="pm-btn" disabled={busy}>Добавить</button></div>
      </form>
    </section>
  )
}

const Field: React.FC<{ label: string; error?: string; children: React.ReactElement }> = ({ label, error, children }) => {
  const id = React.useId()
  return (
    <div className="ad-field">
      <label className="ad-label" htmlFor={id}>{label}</label>
      {React.cloneElement(children as React.ReactElement<Record<string, unknown>>, {
        id, className: `ad-input${error ? ' bad' : ''}`, 'aria-invalid': !!error, 'aria-describedby': error ? `${id}-e` : undefined,
      })}
      {error && <p className="ad-error" id={`${id}-e`}>{error}</p>}
    </div>
  )
}
