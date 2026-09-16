import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { fetchAllEvents, fetchEvent, formatDate, TYPE_LABEL, TYPE_PLURAL, TYPES, yearOf, type EventType, type HistEvent } from './api.ts'
import { MapView, PinShape, TYPE_COLOR, type MapHandle } from './MapView.tsx'
import { NotFound } from './NotFound.tsx'
import { ProposeModal } from './Propose.tsx'
import './public.css'

export const MIN_YEAR = 862
export const MAX_YEAR = 2026
const ROMAN = ['IX', 'X', 'XI', 'XII', 'XIII', 'XIV', 'XV', 'XVI', 'XVII', 'XVIII', 'XIX', 'XX', 'XXI']
const PRESETS: [number, number][] = [[862, 988], [1237, 1480], [1380, 1917], [1700, 1800], [1914, 1945], [1991, MAX_YEAR]]

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

function useIsMobile() {
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

/** true ещё ms после закрытия: слой не размонтируется, пока играет анимация ухода (pm-out в public.css). */
function useLeaving(open: boolean, ms = 160) {
  const [leaving, setLeaving] = useState(false)
  const was = useRef(open)
  // layout-эффект: иначе между закрытием и уходом успевает отрисоваться кадр без панели.
  useLayoutEffect(() => {
    const closed = was.current && !open
    was.current = open
    setLeaving(closed)
    if (!closed) return
    const t = setTimeout(() => setLeaving(false), ms)
    return () => clearTimeout(t)
  }, [open, ms])
  return leaving
}

export const Logo: React.FC<{ short?: boolean; onClick?: () => void }> = ({ short, onClick }) => (
  <button type="button" className="pm-logo" onClick={onClick}>
    <img className="pm-logo-mark" src="/logo.svg" alt="" />
    <span className="pm-logo-text">{short ? 'Страна Героев' : 'Россия - страна Героев'}</span>
  </button>
)

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
  const filtersLeaving = useLeaving(layer === 'filters')
  const [draft, setDraft] = useState<Filter>(applied)
  const [load, setLoad] = useState<Load>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)
  const abortRef = useRef<AbortController>()
  const [proposeOpen, setProposeOpen] = useState(false)
  const proposeParam = params.get('propose') === '1'
  useEffect(() => { if (proposeParam) setProposeOpen(true) }, [proposeParam])

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
  const propose = () => setProposeOpen(true)
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
  const overlay = load.status === 'loading'
    ? { kind: 'loading', title: 'Загружаем события', text: 'Несколько секунд.', action: 'Отмена', run: () => abortRef.current?.abort() }
    : load.status === 'error'
      ? { kind: 'error', title: 'Нет связи с сервером', text: 'Проверьте соединение. Фильтры и период сохранены.', action: 'Повторить', run: () => setAttempt(a => a + 1) }
      : events.length === 0
        ? { kind: 'empty', title: 'Событий пока нет', text: 'На карте ещё ничего не опубликовано.', action: 'Предложить событие', run: propose }
        : filtered.length === 0
          ? { kind: 'empty', title: 'Ничего не найдено', text: `По периоду ${yearLabel(applied.from)} - ${yearLabel(applied.to)}${applied.types.length ? ` и типу ${applied.types.map(t => `"${TYPE_PLURAL[t]}"`).join(', ')}` : ''} событий нет.`, action: 'Сбросить фильтры', run: resetFilters }
          : null

  const filtersPanel = (
    <Filters mobile={mobile} leaving={layer !== 'filters'} draft={draft} setDraft={setDraft} events={events}
      onClose={() => setLayer('none')}
      onApply={() => { applyFilter(draft); setLayer('none') }}
      onReset={resetFilters} />
  )

  // Окно всегда вторым в фрагменте: при смене ветки React сохраняет черновик заявки.
  const modal = <ProposeModal open={proposeOpen} mobile={mobile} onClose={closePropose} />
  // На телефоне фильтры и лента - экраны поверх карты: карта остаётся смонтированной и не теряет вид.
  const screen = mobile && (layer === 'filters' || layer === 'feed')

  return (
    <>
    <div className={`pm-root${overlay && overlay.kind !== 'empty' ? ' pm-dim' : ''}`}>
      <MapView ref={mapRef} events={events} isActive={isActive} selectedId={showCard ? selectedId : null}
        onSelect={select} pad={mobile ? 16 : 48} labels={!mobile}
        callout={showCard && !mobile && <Card key={selectedId} event={selected ?? undefined} onClose={closeCard} />} />

      {screen ? (layer === 'filters'
        ? filtersPanel
        : <Feed mobile events={found} applied={applied} onSelect={select} onClose={() => setLayer('none')} onPropose={propose} />
      ) : (
      <>
        <div role="banner" className="pm-header">
          <Logo short={mobile} onClick={() => { setLayer('none'); navigate('/map') }} />
          <div className="pm-grow" />
          {!mobile && <button type="button" className={`pm-btn-ghost pm-pill${proposeOpen ? ' on' : ''}`} onClick={propose}>Предложить событие</button>}
        </div>

        {!mobile && layer !== 'filters' && layer !== 'feed' && (
          <nav className="pm-rail pm-glass" aria-label="Управление картой">
            <button type="button" onClick={() => openLayer('filters')} title="Фильтры" className={filterBadge(applied) ? 'on' : ''}>Ф</button>
            <button type="button" onClick={() => openLayer('feed')} title="Лента событий">Л</button>
            <button type="button" onClick={() => mapRef.current?.zoomBy(1.6)} title="Приблизить">+</button>
            <button type="button" onClick={() => mapRef.current?.zoomBy(1 / 1.6)} title="Отдалить">-</button>
            <button type="button" title="Колесо мыши - масштаб, перетаскивание - сдвиг, клик по метке - карточка события, клик по кругу с числом - приблизить группу">?</button>
          </nav>
        )}

        <div className={`pm-search-wrap${layer === 'filters' || layer === 'feed' ? ' pm-muted' : ''}`}>
          <label className="pm-search pm-glass">
            <input value={q} maxLength={100} placeholder="Найти событие" aria-label="Найти событие"
              onChange={e => { patch({ q: e.target.value }); setLayer('search') }}
              onFocus={() => q && setLayer('search')}
              onKeyDown={e => { if (e.key === 'Escape') setLayer('none'); if (e.key === 'Enter' && found[0]) select(found[0].id) }} />
            {showCard && q
              ? <button type="button" className="pm-link" onClick={() => setLayer('search')}>Показать список ↓</button>
              : load.status === 'ready' && <span className="pm-counter">{q ? `${found.length} из ${filtered.length}` : filtered.length}</span>}
          </label>
          {layer === 'search' && q && (
            <div className="pm-results pm-glass">
              {found.length === 0 && <div className="pm-results-empty">Ничего не нашлось</div>}
              {found.slice(0, 8).map(e => (
                <button type="button" key={e.id} className="pm-result" onClick={() => select(e.id)}>
                  <Thumb e={e} size={40} />
                  <span className="pm-year">{yearOf(e)}</span>
                  <span className="pm-result-title">{e.title}</span>
                  <span className="pm-grow" />
                  <span className="pm-dim-text">{TYPE_LABEL[e.eventType]}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        {!mobile && (layer === 'filters' || filtersLeaving) && filtersPanel}
        {!mobile && layer === 'feed' && <Feed events={found} applied={applied} onSelect={select} onClose={() => setLayer('none')} onPropose={propose} />}

        {!mobile && load.status === 'ready' && events.length > 0 && layer !== 'feed' && layer !== 'filters' && !showCard && (
          <div className="pm-legend pm-glass">
            {TYPES.map(t => (
              <span key={t}><i style={{ background: TYPE_COLOR[t] }} />{TYPE_PLURAL[t]} · {counts[t]}</span>
            ))}
          </div>
        )}

        <div className="pm-centuries" aria-label="Шкала веков">
          {ROMAN.map((n, i) => (
            <button type="button" key={n} className={highlight(i) ? 'on' : ''} onClick={() => pickCentury(i)} title={`${n} век`}>
              {n}<i />
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

        {showCard && mobile && <Sheet event={selected ?? undefined} onClose={closeCard} />}

        {overlay && (
          <div className={`pm-state pm-glass pm-state-${overlay.kind}`} role={overlay.kind === 'error' ? 'alert' : 'status'}>
            <div className="pm-state-icon">{overlay.kind === 'error' ? '!' : overlay.kind === 'empty' ? '0' : ''}</div>
            <div className="pm-state-title">{overlay.title}</div>
            <div className="pm-state-text">{overlay.text}</div>
            <button type="button" className={overlay.kind === 'loading' ? 'pm-btn-ghost' : 'pm-btn'} onClick={overlay.run}>{overlay.action}</button>
          </div>
        )}
      </>
      )}
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

const Badges: React.FC<{ e: HistEvent }> = ({ e }) => (
  <div className="pm-badges">
    <span className="pm-year-badge">{yearOf(e)}</span>
    <span className={`pm-type-badge ${e.eventType}`}>{TYPE_LABEL[e.eventType]}</span>
    <span className="pm-grow" />
    <span className="pm-dim-text">{formatDate(e.eventDate)}</span>
  </div>
)

function useShare(e: HistEvent | undefined) {
  const [copied, setCopied] = useState(false)
  const share = async () => {
    if (!e) return
    const url = `${location.origin}/map?event=${e.id}`
    if (navigator.share) { navigator.share({ title: e.title, url }).catch(() => {}); return }
    await navigator.clipboard.writeText(url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return { share, copied }
}

const Card: React.FC<{ event: HistEvent | undefined; onClose: () => void }> = ({ event: e, onClose }) => {
  const { share, copied } = useShare(e)
  if (!e) {
    return (
      <div className="pm-card pm-card-loading" aria-busy="true">
        <span className="pm-skel-circle" />
        <span className="pm-grow pm-skel-lines"><i /><i /></span>
        <button type="button" className="pm-close" onClick={onClose} aria-label="Закрыть">×</button>
      </div>
    )
  }
  return (
    <article className="pm-card">
      <div className={e.previewImageUrl ? 'pm-card-preview' : 'pm-card-preview none'}>
        {e.previewImageUrl && <img src={e.previewImageUrl} alt="" />}
        <button type="button" className="pm-close" onClick={onClose} aria-label="Закрыть">×</button>
      </div>
      <div className="pm-card-body">
        <Badges e={e} />
        <h2 className="pm-card-title">{e.title}</h2>
        <p className="pm-card-text">{e.description}</p>
        <div className="pm-row">
          <a className="pm-btn" href={e.siteUrl} target="_blank" rel="noreferrer">Узнать больше</a>
          <button type="button" className="pm-btn-ghost" onClick={share}>{copied ? 'Ссылка скопирована' : 'Поделиться'}</button>
        </div>
      </div>
    </article>
  )
}

const Sheet: React.FC<{ event: HistEvent | undefined; onClose: () => void }> = ({ event: e, onClose }) => {
  const [open, setOpen] = useState(false)
  const startY = useRef<number | null>(null)
  const { share } = useShare(e)
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
      <div className="pm-sheet-shade" onClick={onClose} />
      <section className={`pm-sheet${open ? ' open' : ''}`} aria-label="Карточка события">
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
              <div className="pm-sheet-body">
                <Badges e={e} />
                <h2 className="pm-card-title">{e.title}</h2>
                <p className="pm-card-text">{e.description}</p>
              </div>
            </div>
            <div className="pm-sheet-foot">
              <a className="pm-btn pm-grow" href={e.siteUrl} target="_blank" rel="noreferrer">Узнать больше</a>
              <button type="button" className="pm-btn-ghost pm-round" onClick={share} aria-label="Поделиться">↗</button>
            </div>
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
  mobile: boolean; leaving: boolean; draft: Filter; setDraft: (f: Filter) => void; events: HistEvent[]
  onClose: () => void; onApply: () => void; onReset: () => void
}> = ({ mobile, leaving, draft, setDraft, events, onClose, onApply, onReset }) => {
  const n = events.filter(e => matches(draft, e)).length
  // Пока год вводится руками, он может выйти за диапазон: полоса не должна вылезать из трека.
  const pct = (y: number) => Math.min(100, Math.max(0, ((y - MIN_YEAR) / (MAX_YEAR - MIN_YEAR)) * 100))
  const toggle = (t: EventType) => setDraft({ ...draft, types: draft.types.includes(t) ? draft.types.filter(x => x !== t) : [...draft.types, t] })
  const setFrom = (v: number) => setDraft({ ...draft, from: Math.min(clampYear(v, MIN_YEAR), draft.to) })
  const setTo = (v: number) => setDraft({ ...draft, to: Math.max(clampYear(v, MAX_YEAR), draft.from) })
  const badge = filterBadge(draft)
  return (
    <section className={mobile ? 'pm-screen' : `pm-panel pm-glass${leaving ? ' out' : ''}`} aria-label="Фильтры">
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
        {draft.types.length === 0 && <div className="pm-hint">Ничего не выбрано - показываем оба типа</div>}
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
        <div className="pm-chips">
          {PRESETS.map(([a, b]) => (
            <button type="button" key={a} className={draft.from === a && draft.to === b ? 'on' : ''} onClick={() => setDraft({ ...draft, from: a, to: b })}>
              {a} - {yearLabel(b)}
            </button>
          ))}
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
  onSelect: (id: number) => void; onClose: () => void; onPropose: () => void
}> = ({ mobile, events, applied, onSelect, onClose, onPropose }) => {
  const sorted = useMemo(() => [...events].sort((a, b) => a.eventDate.localeCompare(b.eventDate)), [events])
  return (
    <section className={mobile ? 'pm-screen' : 'pm-panel pm-glass pm-feed'} aria-label="Лента событий">
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
