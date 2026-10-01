import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { Avatar } from '../../shared/ui/Avatar.tsx'
import { LoadMark, Splash, SPLASH_DONE_MS, SPLASH_MS, useSettle } from '../../shared/ui/Loader.tsx'
import { profilePath, fetchAllEvents, fetchEvent, fetchMe, logoutUser, type User, formatDate, TYPE_LABEL, TYPE_PLURAL, TYPES, yearOf, type EventType, type HistEvent } from './api.ts'
import { MapView, PinShape, TYPE_COLOR, type MapHandle } from './MapView.tsx'
import { NotFound } from './NotFound.tsx'
import { ProposeModal } from './Propose.tsx'
import { Presence } from './presence.tsx'
import './public.css'

export const MIN_YEAR = 862
export const MAX_YEAR = 2026
export const ROMAN = ['IX', 'X', 'XI', 'XII', 'XIII', 'XIV', 'XV', 'XVI', 'XVII', 'XVIII', 'XIX', 'XX', 'XXI']

const yearLabel = (y: number) => (y >= MAX_YEAR ? 'н.в.' : String(y))
const plural = (n: number, [one, few, many]: [string, string, string]) => {
  const m10 = n % 10, m100 = n % 100
  return m10 === 1 && m100 !== 11 ? one : m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14) ? few : many
}
const EVENTS_WORDS: [string, string, string] = ['событие', 'события', 'событий']
const clampYear = (v: number, dflt: number) => (Number.isFinite(v) ? Math.min(MAX_YEAR, Math.max(MIN_YEAR, v)) : dflt)

interface Filter { types: EventType[]; from: number; to: number }
const DEFAULT_FILTER: Filter = { types: [], from: MIN_YEAR, to: MAX_YEAR }
const matches = (f: Filter, e: HistEvent) => {
  const y = yearOf(e)
  return (f.types.length === 0 || f.types.includes(e.eventType)) && y >= f.from && y <= f.to
}
const isDefault = (f: Filter) => f.types.length === 0 && f.from === MIN_YEAR && f.to === MAX_YEAR
const filterBadge = (f: Filter) => f.types.length + (f.from !== MIN_YEAR || f.to !== MAX_YEAR ? 1 : 0)

type Layer = 'none' | 'search' | 'filters' | 'feed'
type Load = { status: 'loading' } | { status: 'error' } | { status: 'ready'; events: HistEvent[] }

export function useIsMobile() {
  const q = '(max-width: 640px)'
  const [m, setM] = useState(() => window.matchMedia(q).matches)
  useEffect(() => {
    const mq = window.matchMedia(q)
    const on = () => setM(mq.matches)
    mq.addEventListener('change', on)
    return () => mq.removeEventListener('change', on)
  }, [])
  return m
}

export const Logo: React.FC<{ short?: boolean; onClick?: () => void }> = ({ short, onClick }) => (
  <button type="button" className="pm-logo" onClick={onClick}>
    <img className="pm-logo-mark" src="/logo.svg" alt="" />
    <span className="pm-logo-text">{short ? 'Страна Героев' : 'Россия - страна Героев'}</span>
  </button>
)

// Дверь и стрелка: внутрь - войти, наружу - выйти.
const LOGIN = 'M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4 M4 12h11 M11 8l4 4-4 4'
const LOGOUT = 'M10 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h4 M9 12h11 M16 8l4 4-4 4'
const CHEVRON = 'M7 10l5 5 5-5'
const CLOSE = 'M7 7l10 10 M17 7L7 17'
const NavIcon: React.FC<{ d: string }> = ({ d }) => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.7}
    strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={d} /></svg>
)

// Меню шапки карты и профиля. На карту ведёт логотип, отдельного пункта нет.
export const HeaderNav: React.FC<{
  me: User | null; mobile: boolean; onLogout: () => void; onPropose: () => void; proposeOpen?: boolean
}> = ({ me, mobile, onLogout, onPropose, proposeOpen }) => {
  const [open, setOpen] = useState(false)
  const [adminRole, setAdminRole] = useState<'admin' | 'super_admin' | null>(null)
  const menu = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!me) { setAdminRole(null); setOpen(false); return }
    const ac = new AbortController()
    fetch(`${import.meta.env.VITE_API_BASE ?? ''}/api/v1/auth/session`, { credentials: 'include', signal: ac.signal })
      .then(async r => { const role = r.ok ? (await r.json()).admin.role : null; if (!ac.signal.aborted) setAdminRole(role) }).catch(() => {})
    return () => ac.abort()
  }, [me?.id])
  useEffect(() => {
    if (!open) return
    const outside = (e: MouseEvent) => { if (!menu.current?.contains(e.target as Node)) setOpen(false) }
    const escape = (e: KeyboardEvent) => { if (e.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', outside)
    document.addEventListener('keydown', escape)
    return () => { document.removeEventListener('mousedown', outside); document.removeEventListener('keydown', escape) }
  }, [open])
  return <nav className="pm-nav">
    <Link to="/about" className="pm-nav-item">О проекте</Link>
    {me
      ? <div className="pm-account" ref={menu}>
          <button type="button" className="pm-user" aria-label="Меню аккаунта" aria-expanded={open} aria-haspopup="true" onClick={() => setOpen(v => !v)}>
            <Avatar seed={me.publicId} size={30} />
            {!mobile && <span className="pm-user-name">{me.firstName}{me.lastName ? ` ${me.lastName[0]}.` : ''}</span>}
            <span className="pm-user-caret"><NavIcon d={CHEVRON} /></span>
          </button>
          <Presence>{open && <div className="pm-account-menu pm-glass">
            <div className="pm-account-head"><Avatar seed={me.publicId} size={34} /><span><strong>{me.firstName} {me.lastName}</strong>{adminRole && <small>{adminRole === 'super_admin' ? 'Супер-админ' : 'Модератор'}</small>}</span></div>
            <Link to={profilePath(me.publicId)} onClick={() => setOpen(false)}>Мой профиль</Link>
            {adminRole && <Link to="/admin" onClick={() => setOpen(false)}>Панель модератора</Link>}
            <button type="button" onClick={() => { setOpen(false); onLogout() }}><NavIcon d={LOGOUT} />Выйти</button>
          </div>}</Presence>
        </div>
      : <Link to="/login" className="pm-btn-ghost pm-pill pm-login"><NavIcon d={LOGIN} />Войти</Link>}
    {!mobile && <button type="button" className={`pm-btn-light pm-pill${proposeOpen ? ' on' : ''}`} onClick={onPropose}>Предложить событие</button>}
  </nav>
}

/** Заставка на весь экран - только на первую загрузку карты во вкладке, дальше обычная плашка. */
let splashSeen = false

export const PublicMap: React.FC = () => {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const mobile = useIsMobile()
  const mapRef = useRef<MapHandle>(null)

  const q = params.get('q') ?? ''
  const applied: Filter = useMemo(() => ({
    types: (params.get('types') ?? '').split(',').filter((t): t is EventType => TYPES.includes(t as EventType)),
    from: clampYear(Number(params.get('from') ?? MIN_YEAR), MIN_YEAR),
    to: clampYear(Number(params.get('to') ?? MAX_YEAR), MAX_YEAR),
  }), [params])
  const eventParam = params.get('event')
  const selectedId = eventParam && /^\d+$/.test(eventParam) ? Number(eventParam) : null

  const patch = useCallback((next: Record<string, string | null>) => {
    setParams(prev => {
      const p = new URLSearchParams(prev)
      for (const [k, v] of Object.entries(next)) v === null || v === '' ? p.delete(k) : p.set(k, v)
      return p
    }, { replace: true })
  }, [setParams])
  const applyFilter = (f: Filter) => patch({
    types: f.types.join(','),
    from: f.from === MIN_YEAR ? null : String(f.from),
    to: f.to === MAX_YEAR ? null : String(f.to),
  })

  const [layer, setLayer] = useState<Layer>('none')
  const [draft, setDraft] = useState<Filter>(applied)
  const [load, setLoad] = useState<Load>({ status: 'loading' })
  const [splash] = useState(() => !splashSeen)
  const [attempt, setAttempt] = useState(0)
  const abortRef = useRef<AbortController | undefined>(undefined)
  const [proposeOpen, setProposeOpen] = useState(false)
  const proposeParam = params.get('propose') === '1'
  // undefined - сессия ещё проверяется, null - гость.
  const [me, setMe] = useState<User | null | undefined>(undefined)
  useEffect(() => {
    const ac = new AbortController()
    fetchMe(ac.signal).then(setMe, () => { if (!ac.signal.aborted) setMe(null) })
    return () => ac.abort()
  }, [])
  // Заявку принимают только от вошедших: гостя ведём на вход, оттуда он вернётся в форму.
  // По ссылке ?propose=1 - с replace, иначе "Назад" со входа снова попадёт на этот редирект.
  const toLogin = useCallback((replace = false) => navigate('/login', { replace, state: { back: '/map?propose=1' } }), [navigate])
  useEffect(() => {
    if (!proposeParam || me === undefined) return
    if (me) setProposeOpen(true)
    else toLogin(true)
  }, [proposeParam, me, toLogin])
  // Сначала ответ сервера, потом сброс: иначе следующий fetchMe успеет вернуть ещё живую сессию.
  const logout = async () => { await logoutUser(); setMe(null) }

  useEffect(() => {
    const ac = new AbortController()
    abortRef.current = ac
    setLoad({ status: 'loading' })
    // Ответ устаревшего запуска эффекта не должен перетирать текущий; кнопка "Отмена" прерывает текущий.
    fetchAllEvents(ac.signal).then(
      events => { if (abortRef.current === ac) setLoad({ status: 'ready', events }) },
      () => { if (abortRef.current === ac) setLoad({ status: 'error' }) },
    )
    return () => ac.abort()
  }, [attempt])

  const events = load.status === 'ready' ? load.events : []
  const phase = useSettle(load.status === 'loading', events.length > 0, splash ? SPLASH_MS : undefined, splash ? SPLASH_DONE_MS : undefined)
  useEffect(() => { if (phase === 'idle') splashSeen = true }, [phase])
  const full = splash && phase !== 'idle'
  const filtered = useMemo(() => events.filter(e => matches(applied, e)), [events, applied])
  const found = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return needle ? filtered.filter(e => e.title.toLowerCase().includes(needle)) : filtered
  }, [filtered, q])
  const foundIds = useMemo(() => new Set(found.map(e => e.id)), [found])
  const isActive = useCallback((e: HistEvent) => foundIds.has(e.id), [foundIds])

  // Событие из ссылки может не попасть в список (снято, ещё грузится) - спрашиваем отдельно.
  const [extra, setExtra] = useState<{ id: number; event: HistEvent | null } | null>(null)
  const inList = events.find(e => e.id === selectedId)
  useEffect(() => {
    if (selectedId === null || inList || load.status !== 'ready' || extra?.id === selectedId) return
    const ac = new AbortController()
    fetchEvent(selectedId, ac.signal).then(event => setExtra({ id: selectedId, event }), () => {})
    return () => ac.abort()
  }, [selectedId, inList, load.status, extra])
  const selected = inList ?? (extra?.id === selectedId ? extra.event : undefined)

  if (selectedId !== null && selected === null) return <NotFound path={`/map?event=${selectedId}`} kind="event" />
  if (eventParam !== null && selectedId === null) return <NotFound path={`/map?event=${eventParam}`} kind="event" />

  const select = (id: number) => { patch({ event: String(id) }); setLayer('none') }
  const closeCard = () => patch({ event: null })
  const openLayer = (l: Layer) => {
    if (l === 'filters') setDraft(applied)
    setLayer(cur => (cur === l ? 'none' : l))
  }
  const resetFilters = () => { setDraft(DEFAULT_FILTER); applyFilter(DEFAULT_FILTER) }
  const propose = () => {
    if (me) setProposeOpen(true)
    else if (me === null) toLogin()
  }
  const closePropose = () => { setProposeOpen(false); patch({ propose: null }) }

  const highlight = !isDefault(applied)
    ? (i: number) => (i + 9) * 100 >= applied.from && (i + 8) * 100 + 1 <= applied.to
    : (i: number) => !!selected && Math.ceil(yearOf(selected) / 100) === i + 9
  const pickCentury = (i: number) => {
    const c = i + 9
    applyFilter({ ...applied, from: Math.max(MIN_YEAR, (c - 1) * 100 + 1), to: Math.min(MAX_YEAR, c * 100) })
  }

  const showCard = layer === 'none' && selectedId !== null
  const counts = Object.fromEntries(TYPES.map(t => [t, filtered.filter(e => e.eventType === t).length])) as Record<EventType, number>
  const overlay = full ? null
    : phase === 'load'
    ? { kind: 'loading', title: 'Загружаем события', text: 'Несколько секунд.', action: 'Отмена', run: () => abortRef.current?.abort() }
    : phase === 'done'
    ? { kind: 'done', title: 'События загружены', text: `${events.length} ${plural(events.length, EVENTS_WORDS)} на карте.`, action: 'Отмена', run: () => {} }
    : load.status === 'error'
      ? { kind: 'error', title: 'Нет связи с сервером', text: 'Проверьте соединение. Фильтры и период сохранены.', action: 'Повторить', run: () => setAttempt(a => a + 1) }
      : events.length === 0
        ? { kind: 'empty', title: 'Событий пока нет', text: 'На карте ещё ничего не опубликовано.', action: 'Предложить событие', run: propose }
        : filtered.length === 0
          ? { kind: 'empty', title: 'Ничего не найдено', text: `По периоду ${yearLabel(applied.from)} - ${yearLabel(applied.to)}${applied.types.length ? ` и типу ${applied.types.map(t => `"${TYPE_PLURAL[t]}"`).join(', ')}` : ''} событий нет.`, action: 'Сбросить фильтры', run: resetFilters }
          : null

  const filtersPanel = (
    <Filters mobile={mobile} draft={draft} setDraft={setDraft} events={events}
      onClose={() => setLayer('none')}
      onApply={() => { applyFilter(draft); setLayer('none') }}
      onReset={resetFilters} />
  )

  // Окно всегда вторым в фрагменте: при смене ветки React сохраняет черновик заявки.
  const modal = <ProposeModal open={proposeOpen && !!me} email={me?.email ?? ''} mobile={mobile} onClose={closePropose} />
  // На телефоне фильтры и лента - экраны поверх карты: карта остаётся смонтированной и не теряет вид.
  const screen = mobile && (layer === 'filters' || layer === 'feed')
  const feed = <Feed mobile={mobile} events={found} applied={applied} onSelect={select} onClose={() => setLayer('none')} onPropose={propose} />

  return (
    <>
    <div className={`pm-root${overlay && overlay.kind !== 'empty' ? ' pm-dim' : ''}`}>
      <MapView ref={mapRef} events={events} isActive={isActive} selectedId={showCard ? selectedId : null}
        onSelect={select} onMapClick={showCard ? closeCard : undefined} pad={mobile ? 16 : 48} labels={!mobile}
        callout={showCard && !mobile && <Card key={selectedId} event={selected ?? undefined} onClose={closeCard} />} />

      {!screen && (
      <>
        <div role="banner" className="pm-header">
          <Logo short={mobile} onClick={() => { setLayer('none'); navigate('/map') }} />
          <div className="pm-grow" />
          <HeaderNav me={me ?? null} mobile={mobile} onLogout={logout} onPropose={propose} proposeOpen={proposeOpen} />
        </div>

        <Presence>{!mobile && layer !== 'filters' && layer !== 'feed' && (
          <nav className="pm-rail pm-glass" aria-label="Управление картой">
            <button type="button" onClick={() => openLayer('filters')} title="Фильтры" className={filterBadge(applied) ? 'on' : ''} aria-label="Фильтры">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path transform="translate(3 4)" d="M14.3879,9.708 C16.1039,9.708 17.4999,11.097 17.4999,12.804 C17.4999,14.511 16.1039,15.9 14.3879,15.9 C12.6709,15.9 11.2739,14.511 11.2739,12.804 C11.2739,11.097 12.6709,9.708 14.3879,9.708 Z M14.3879,11.208 C13.4979,11.208 12.7739,11.924 12.7739,12.804 C12.7739,13.685 13.4979,14.4 14.3879,14.4 C15.2769,14.4 15.9999,13.685 15.9999,12.804 C15.9999,11.924 15.2769,11.208 14.3879,11.208 Z M7.0803,12.0928 C7.4943,12.0928 7.8303,12.4288 7.8303,12.8428 C7.8303,13.2568 7.4943,13.5928 7.0803,13.5928 L0.7793,13.5928 C0.3653,13.5928 0.0293,13.2568 0.0293,12.8428 C0.0293,12.4288 0.3653,12.0928 0.7793,12.0928 L7.0803,12.0928 Z M3.113,0 C4.83,0 6.226,1.39 6.226,3.097 C6.226,4.804 4.83,6.192 3.113,6.192 C1.397,6.192 0,4.804 0,3.097 C0,1.39 1.397,0 3.113,0 Z M3.113,1.5 C2.224,1.5 1.5,2.216 1.5,3.097 C1.5,3.977 2.224,4.692 3.113,4.692 C4.003,4.692 4.726,3.977 4.726,3.097 C4.726,2.216 4.003,1.5 3.113,1.5 Z M16.1911,2.4004 C16.6051,2.4004 16.9411,2.7364 16.9411,3.1504 C16.9411,3.5644 16.6051,3.9004 16.1911,3.9004 L9.8911,3.9004 C9.4771,3.9004 9.1411,3.5644 9.1411,3.1504 C9.1411,2.7364 9.4771,2.4004 9.8911,2.4004 L16.1911,2.4004 Z" />
              </svg>
            </button>
            <button type="button" onClick={() => mapRef.current?.zoomBy(1.6)} title="Приблизить">+</button>
            <button type="button" onClick={() => mapRef.current?.zoomBy(1 / 1.6)} title="Отдалить">-</button>
          </nav>
        )}</Presence>

        <div className={`pm-search-wrap${layer === 'filters' || layer === 'feed' ? ' pm-muted' : ''}`}>
          <label className="pm-search pm-glass pm-glow">
            <input value={q} maxLength={100} placeholder="Найти событие" aria-label="Найти событие"
              onChange={e => { patch({ q: e.target.value }); setLayer('search') }}
              onFocus={() => q && setLayer('search')}
              onKeyDown={e => { if (e.key === 'Escape') setLayer('none'); if (e.key === 'Enter' && found[0]) select(found[0].id) }} />
            {showCard && q
              ? <button type="button" className="pm-link" onClick={() => setLayer('search')}>Показать список ↓</button>
              : phase === 'idle' && events.length > 0 && <span className="pm-counter" key={q ? `${found.length}/${filtered.length}` : filtered.length}>{q ? `${found.length} из ${filtered.length}` : filtered.length}</span>}
          </label>
          <Presence>{layer === 'search' && !!q && (
            <div className="pm-results pm-glass">
              {found.length === 0 && <div className="pm-results-empty">Ничего не нашлось</div>}
              {found.slice(0, 8).map((e, i) => (
                <button type="button" key={e.id} className="pm-result" style={{ '--i': i } as React.CSSProperties} onClick={() => select(e.id)}>
                  <Thumb e={e} size={40} />
                  <span className="pm-year">{yearOf(e)}</span>
                  <span className="pm-result-title">{e.title}</span>
                  <span className="pm-grow" />
                  <span className="pm-dim-text">{TYPE_LABEL[e.eventType]}</span>
                </button>
              ))}
            </div>
          )}</Presence>
        </div>

        <Presence>{!mobile && layer === 'filters' && filtersPanel}</Presence>

        <Presence>{!mobile && phase === 'idle' && events.length > 0 && layer !== 'feed' && layer !== 'filters' && !showCard && (
          <div className="pm-legend pm-glass">
            {TYPES.map(t => (
              <span key={t}><i style={{ background: TYPE_COLOR[t] }} />{TYPE_PLURAL[t]} · {counts[t]}</span>
            ))}
          </div>
        )}</Presence>

        <div className="pm-centuries" aria-label="Шкала веков">
          {ROMAN.map((n, i) => (
            <button type="button" key={n} className={highlight(i) ? 'on' : ''} onClick={() => pickCentury(i)} title={`${n} век`}>
              <span>{n}</span><i />
            </button>
          ))}
        </div>

        {mobile && (
          <div className="pm-bottombar">
            <button type="button" className="pm-glass pm-grow" onClick={() => openLayer('filters')}>
              Фильтры {filterBadge(applied) > 0 && <span className="pm-badge">{filterBadge(applied)}</span>}
            </button>
            <button type="button" className="pm-glass pm-round" onClick={() => openLayer('feed')} aria-label="Лента событий">☰</button>
          </div>
        )}

        <Presence>{showCard && mobile && <Sheet event={selected ?? undefined} onClose={closeCard} />}</Presence>

        <Presence>{overlay && (
          <div className={`pm-state pm-glass pm-state-${overlay.kind}`} role={overlay.kind === 'error' ? 'alert' : 'status'}>
            {overlay.kind === 'loading' || overlay.kind === 'done'
              ? <LoadMark done={overlay.kind === 'done'} />
              : <div className="pm-state-icon">{overlay.kind === 'error' ? '!' : '0'}</div>}
            <div className="pm-state-title" key={`t${overlay.kind}`}>{overlay.title}</div>
            <div className="pm-state-text" key={`x${overlay.kind}`}>{overlay.text}</div>
            <button type="button" className={overlay.kind === 'loading' ? 'pm-btn-ghost' : 'pm-btn'} onClick={overlay.run}>{overlay.action}</button>
          </div>
        )}</Presence>
      </>
      )}
      {/* На телефоне фильтры и лента - экраны поверх остального; уходя, закрывают уже вернувшийся интерфейс. */}
      <Presence>{screen && (layer === 'filters' ? filtersPanel : feed)}</Presence>
      {full && <Splash phase={phase} done={`${events.length} ${plural(events.length, EVENTS_WORDS)} на карте`} onCancel={() => abortRef.current?.abort()} />}
    </div>
    {modal}
    </>
  )
}

const Thumb: React.FC<{ e: HistEvent; size: number }> = ({ e, size }) => (
  e.previewImageUrl
    ? <img className="pm-thumb" src={e.previewImageUrl} alt="" width={size} height={size} />
    : <span className="pm-thumb pm-hatch" style={{ width: size, height: size }} />
)

function useShare(e: HistEvent) {
  const [copied, setCopied] = useState(false)
  const share = async () => {
    const url = `${location.origin}/map?event=${e.id}`
    if (navigator.share) { navigator.share({ title: e.title, url }).catch(() => {}); return }
    await navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return { share, copied }
}

const Card: React.FC<{ event: HistEvent | undefined; onClose: () => void }> = ({ event: e, onClose }) => {
  if (!e) {
    return (
      <div className="pm-card pm-card-loading" aria-busy="true">
        <span className="pm-skel-circle" />
        <span className="pm-grow pm-skel-lines"><i /><i /></span>
        <button type="button" className="pm-close" onClick={onClose} aria-label="Закрыть"><NavIcon d={CLOSE} /></button>
      </div>
    )
  }
  return (
    <article className="pm-card">
      <div className={`pm-card-preview${e.previewImageUrl ? '' : ' pm-hatch'}`}>
        {e.previewImageUrl && <img src={e.previewImageUrl} alt="" />}
        <button type="button" className="pm-close" onClick={onClose} aria-label="Закрыть"><NavIcon d={CLOSE} /></button>
      </div>
      <div className="pm-card-body">
        <EventInfo e={e} />
        <EventActions e={e} />
      </div>
    </article>
  )
}

/** Общее у карточки и мобильного листа: заголовок, подпись, строки с иконками. */
const EventInfo: React.FC<{ e: HistEvent }> = ({ e }) => (
  <>
    <div className="pm-card-head">
      <h2 className="pm-card-title">{e.title}</h2>
      <span className="pm-year">{yearOf(e)}</span>
    </div>
    <div className="pm-card-sub">
      <span className={`pm-card-type ${e.eventType}`}>{TYPE_LABEL[e.eventType]}</span>
      <i />
      <span>{formatDate(e.eventDate)}</span>
    </div>
    {e.description && (
      <ul className="pm-card-info">
        <li><Icon d={ICON.text} /><p className="pm-card-text">{e.description}</p></li>
      </ul>
    )}
  </>
)

const EventActions: React.FC<{ e: HistEvent; className?: string }> = ({ e, className = 'pm-card-actions' }) => {
  const { share, copied } = useShare(e)
  return (
    <div className={className}>
      <button type="button" className="pm-card-share" onClick={share}>
        <Icon d={ICON.share} />{copied ? 'Скопировано' : 'Поделиться'}
      </button>
      <a className="pm-card-more" href={e.siteUrl} target="_blank" rel="noreferrer">
        <Icon d={ICON.open} />Узнать больше
      </a>
    </div>
  )
}

const ICON = {
  text: 'M5 6h14 M5 10h14 M5 14h14 M5 18h9',
  share: 'M18 8a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z M6 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z M18 22a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z M8.6 13.5l6.8 4 M15.4 6.5l-6.8 4',
  open: 'M14 4h6v6 M20 4l-9 9 M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5',
}
const Icon: React.FC<{ d: string }> = ({ d }) => (
  <svg className="pm-icon" viewBox="0 0 24 24" width={20} height={20} aria-hidden="true" fill="none"
    stroke="currentColor" strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round"><path d={d} /></svg>
)

const Sheet: React.FC<{ event: HistEvent | undefined; onClose: () => void; className?: string }> = ({ event: e, onClose, className = '' }) => {
  const [open, setOpen] = useState(false)
  const startY = useRef<number | null>(null)
  const onUp = (y: number) => {
    if (startY.current === null) return
    const dy = y - startY.current
    startY.current = null
    if (dy < -30) setOpen(true)
    else if (dy > 30) open ? setOpen(false) : onClose()
    else setOpen(o => !o)
  }
  return (
    <>
      <div className={`pm-sheet-shade ${className}`} onClick={onClose} />
      <section className={`pm-sheet${open ? ' open' : ''} ${className}`} aria-label="Карточка события">
        <div className="pm-sheet-handle" role="button" aria-label={open ? 'Свернуть' : 'Развернуть'}
          onPointerDown={ev => { startY.current = ev.clientY }} onPointerUp={ev => onUp(ev.clientY)}>
          <i />
        </div>
        {!e ? <div className="pm-sheet-body"><span className="pm-skel-lines"><i /><i /></span></div> : (
          <>
            <div className="pm-sheet-scroll">
              <div className={`pm-sheet-preview${e.previewImageUrl ? '' : ' pm-hatch'}`}>
                {e.previewImageUrl && <img src={e.previewImageUrl} alt="" />}
              </div>
              <div className="pm-sheet-body"><EventInfo e={e} /></div>
            </div>
            <EventActions e={e} className="pm-card-actions pm-sheet-foot" />
          </>
        )}
      </section>
    </>
  )
}

const TypeGlyph: React.FC<{ t: EventType }> = ({ t }) => (
  <svg className="pm-type-pin" width={24} height={24} viewBox="0 0 64 64" aria-hidden><PinShape type={t} size={24} /></svg>
)

const Filters: React.FC<{
  mobile: boolean; draft: Filter; setDraft: (f: Filter) => void; events: HistEvent[]
  onClose: () => void; onApply: () => void; onReset: () => void; className?: string
}> = ({ mobile, draft, setDraft, events, onClose, onApply, onReset, className = '' }) => {
  const n = events.filter(e => matches(draft, e)).length
  // Пока год вводится руками, он может выйти за диапазон: полоса не должна вылезать из трека.
  const pct = (y: number) => Math.min(100, Math.max(0, ((y - MIN_YEAR) / (MAX_YEAR - MIN_YEAR)) * 100))
  const toggle = (t: EventType) => setDraft({ ...draft, types: draft.types.includes(t) ? draft.types.filter(x => x !== t) : [...draft.types, t] })
  const setFrom = (v: number) => setDraft({ ...draft, from: Math.min(clampYear(v, MIN_YEAR), draft.to) })
  const setTo = (v: number) => setDraft({ ...draft, to: Math.max(clampYear(v, MAX_YEAR), draft.from) })
  const badge = filterBadge(draft)
  return (
    <section className={`${mobile ? 'pm-screen' : 'pm-panel pm-glass'} ${className}`} aria-label="Фильтры">
      <div className="pm-panel-head">
        <h2>Фильтры</h2>
        {badge > 0 && <span className="pm-badge">{badge}</span>}
        <span className="pm-grow" />
        <button type="button" className="pm-close-sm" onClick={onClose} aria-label="Закрыть">×</button>
      </div>
      <div className="pm-panel-sec">
        <div className="pm-label">Тип события</div>
        <div className={mobile ? 'pm-types col' : 'pm-types'}>
          {TYPES.map(t => (
            <button type="button" key={t} aria-pressed={draft.types.includes(t)} className={draft.types.includes(t) ? 'on' : ''} onClick={() => toggle(t)}>
              <TypeGlyph t={t} />
              <span>{TYPE_PLURAL[t]}</span>
              {mobile && <><span className="pm-grow" /><span className="pm-mono">{events.filter(e => e.eventType === t).length}</span></>}
            </button>
          ))}
        </div>
        {draft.types.length === 0 && <div className="pm-hint">Ничего не выбрано - показываем все типы</div>}
      </div>
      <div className="pm-panel-sec">
        <div className="pm-label">Период</div>
        <div className="pm-years">
          <input type="number" min={MIN_YEAR} max={MAX_YEAR} value={draft.from} aria-label="С года"
            onChange={e => setDraft({ ...draft, from: Number(e.target.value) })} onBlur={e => setFrom(Number(e.target.value))} />
          <span className="pm-dim-text">-</span>
          <input type="number" min={MIN_YEAR} max={MAX_YEAR} value={draft.to} aria-label="По год"
            onChange={e => setDraft({ ...draft, to: Number(e.target.value) })} onBlur={e => setTo(Number(e.target.value))} />
        </div>
        <div className="pm-range">
          <div className="pm-range-track"><i style={{ left: `${pct(draft.from)}%`, right: `${100 - pct(draft.to)}%` }} /></div>
          <input type="range" min={MIN_YEAR} max={MAX_YEAR} value={draft.from} aria-label="Начало периода" onChange={e => setFrom(Number(e.target.value))} />
          <input type="range" min={MIN_YEAR} max={MAX_YEAR} value={draft.to} aria-label="Конец периода" onChange={e => setTo(Number(e.target.value))} />
          <div className="pm-range-ends"><span>{MIN_YEAR}</span><span>н.в.</span></div>
        </div>
      </div>
      {mobile && <div className="pm-grow" />}
      <div className="pm-panel-foot">
        <button type="button" className="pm-btn pm-grow" onClick={onApply}>
          {mobile ? `Показать ${n} ${plural(n, EVENTS_WORDS)}` : `Применить · ${n}`}
        </button>
        <button type="button" className="pm-btn-text" onClick={onReset}>{mobile ? 'Сброс' : 'Сбросить'}</button>
      </div>
    </section>
  )
}

const Feed: React.FC<{
  mobile?: boolean; events: HistEvent[]; applied: Filter
  onSelect: (id: number) => void; onClose: () => void; onPropose: () => void; className?: string
}> = ({ mobile, events, applied, onSelect, onClose, onPropose, className = '' }) => {
  const sorted = useMemo(() => [...events].sort((a, b) => a.eventDate.localeCompare(b.eventDate)), [events])
  return (
    <section className={`${mobile ? 'pm-screen' : 'pm-panel pm-glass pm-feed'} ${className}`} aria-label="Лента событий">
      <div className="pm-panel-head">
        <h2>Лента событий</h2>
        <span className="pm-grow" />
        <span className="pm-dim-text">{events.length}</span>
        {!mobile && <button type="button" className="pm-close-sm" onClick={onClose} aria-label="Закрыть">×</button>}
      </div>
      {!isDefault(applied) && (
        <div className="pm-chips pm-feed-chips">
          {(applied.from !== MIN_YEAR || applied.to !== MAX_YEAR) && <span className="on">{applied.from} - {yearLabel(applied.to)}</span>}
          {applied.types.map(t => <span key={t}>{TYPE_PLURAL[t]}</span>)}
        </div>
      )}
      <div className="pm-feed-list">
        {sorted.length === 0 && <div className="pm-results-empty">Под фильтр ничего не попало</div>}
        {sorted.map(e => (
          <button type="button" key={e.id} className="pm-feed-item" onClick={() => onSelect(e.id)}>
            <Thumb e={e} size={52} />
            <span className="pm-feed-text">
              <span className="pm-row-tight"><span className="pm-year">{yearOf(e)}</span><span className="pm-feed-title">{e.title}</span></span>
              <span className="pm-feed-line">{e.description}</span>
            </span>
          </button>
        ))}
      </div>
      {mobile && (
        <div className="pm-panel-foot">
          <button type="button" className="pm-glass pm-grow pm-tall" onClick={onClose}>На карту</button>
          <button type="button" className="pm-btn-ghost pm-round" onClick={onPropose} aria-label="Предложить событие">+</button>
        </div>
      )}
    </section>
  )
}
