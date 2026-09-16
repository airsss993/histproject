import React, { forwardRef, useEffect, useImperativeHandle, useLayoutEffect, useMemo, useRef, useState } from 'react'
import maplibregl from 'maplibre-gl'
import type { ExpressionSpecification, StyleSpecification } from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import type { FeatureCollection, Position } from 'geojson'
import { TYPE_LABEL, yearOf, type EventType, type HistEvent } from './api.ts'
import { LEAVE_MS, Presence } from './presence.tsx'

export const TYPE_COLOR: Record<EventType, string> = { political: '#8FB4E8', military: '#F0B62A' }
const CELL = 56
const INK = '#060D2E'
const MUTED = '#4E5C86'
const GOLD = '#F0B62A'

// Метка из "Логотип и метки": капля 64x64 с остриём в (32, 58), глиф типа в окне.
const DROP = 'M32 58C32 58 13 37 13 25A19 19 0 0 1 51 25C51 37 32 58 32 58Z'
const GLYPH: Record<EventType, string[]> = {
  military: ['M32 11V19', 'M24.5 19H39.5', 'M28.5 19V29L32 36L35.5 29V19'],
  political: ['M24 16H40', 'M27 16V32', 'M37 16V32', 'M23 32H41'],
}
const alpha = (hex: string, a: number) => hex + Math.round(a * 255).toString(16).padStart(2, '0')

// Размер метки в px по приближению k относительно начального вида.
const pinSize = (k: number) => (k < 1.5 ? 16 : k < 2.5 ? 24 : k < 4 ? 32 : 44)

export type PinState = 'normal' | 'hover' | 'selected' | 'inactive'

/** Капля в координатах viewBox 0 0 64 64. */
export function PinShape({ type, size, state = 'normal' }: { type: EventType; size: number; state?: PinState }) {
  const color = state === 'inactive' ? MUTED : TYPE_COLOR[type]
  if (size < 24) return <path d={DROP} fill={color} />
  const selected = state === 'selected'
  const stroke = size < 32 ? 3.4 : size < 44 ? 2.8 : state === 'inactive' ? 2 : 2.4
  const fill = selected ? color : state === 'hover' ? alpha(color, 0.14) : INK
  return (
    <g fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round">
      {selected && <circle className="pm-ring" cx={32} cy={28} r={33} stroke={alpha(color, 0.34)} strokeWidth={1.6} />}
      <path d={DROP} fill={fill} />
      {size < 32
        ? <circle cx={32} cy={25} r={4} fill={selected ? INK : color} stroke="none" />
        : GLYPH[type].map(d => <path key={d} d={d} stroke={selected ? INK : color} />)}
    </g>
  )
}

// Тёмная подложка OpenFreeMap (схема OpenMapTiles). Россия и регионы - из russia-regions.geojson (OSM).
const OMF = 'https://tiles.openfreemap.org'
const NAME: ExpressionSpecification = ['coalesce', ['get', 'name:ru'], ['get', 'name']]
const HALO = { 'text-halo-color': INK, 'text-halo-width': 1.2 }
const LABEL_LAYERS = ['country-label', 'city-label', 'village-label']
const style = (sea: string): StyleSpecification => ({
  version: 8,
  glyphs: `${OMF}/fonts/{fontstack}/{range}.pbf`,
  sources: {
    omf: {
      type: 'vector', url: `${OMF}/planet`,
      attribution: '<a href="https://openfreemap.org">OpenFreeMap</a> <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
    },
  },
  layers: [
    { id: 'land', type: 'background', paint: { 'background-color': '#070C24' } },
    { id: 'water', type: 'fill', source: 'omf', 'source-layer': 'water', paint: { 'fill-color': sea } },
    { id: 'roads', type: 'line', source: 'omf', 'source-layer': 'transportation', minzoom: 6,
      filter: ['in', ['get', 'class'], ['literal', ['motorway', 'trunk', 'primary', 'secondary']]],
      paint: { 'line-color': '#1E2A58', 'line-width': ['interpolate', ['linear'], ['zoom'], 6, 0.5, 12, 2] } },
    { id: 'borders', type: 'line', source: 'omf', 'source-layer': 'boundary',
      filter: ['all', ['==', ['get', 'admin_level'], 2], ['==', ['get', 'maritime'], 0], ['==', ['get', 'disputed'], 0]],
      paint: { 'line-color': '#2C4272', 'line-width': 0.8 } },
    { id: 'country-label', type: 'symbol', source: 'omf', 'source-layer': 'place', filter: ['==', ['get', 'class'], 'country'],
      layout: { 'text-field': NAME, 'text-font': ['Noto Sans Regular'], 'text-size': 10, 'text-transform': 'uppercase', 'text-letter-spacing': 0.14 },
      paint: { 'text-color': MUTED, ...HALO } },
    { id: 'city-label', type: 'symbol', source: 'omf', 'source-layer': 'place', minzoom: 4,
      filter: ['in', ['get', 'class'], ['literal', ['city', 'town']]],
      layout: { 'text-field': NAME, 'text-font': ['Noto Sans Regular'], 'text-size': ['interpolate', ['linear'], ['zoom'], 4, 10, 10, 14] },
      paint: { 'text-color': '#93A8D0', ...HALO } },
    { id: 'village-label', type: 'symbol', source: 'omf', 'source-layer': 'place', minzoom: 9,
      filter: ['in', ['get', 'class'], ['literal', ['village', 'hamlet']]],
      layout: { 'text-field': NAME, 'text-font': ['Noto Sans Regular'], 'text-size': 11 },
      paint: { 'text-color': MUTED, ...HALO } },
  ],
})

// Чукотка пересекает 180-й меридиан: сдвигаем долготы кольца, чтобы полигон не растягивался через весь мир.
const unwrap = (ring: Position[]) => ring.reduce<Position[]>((out, [lng, lat]) => {
  const prev = out.length ? out[out.length - 1][0] : lng
  out.push([lng + 360 * Math.round((prev - lng) / 360), lat])
  return out
}, [])
// Грузим при первой карте; неудачную загрузку не кешируем, следующая карта попробует снова.
let russia: Promise<FeatureCollection> | undefined
const loadRussia = () => russia ??= fetch('/russia-regions.geojson')
  .then(r => { if (!r.ok) throw new Error(`russia-regions: ${r.status}`); return r.json() as Promise<FeatureCollection> })
  .then(fc => {
    for (const f of fc.features) {
      const g = f.geometry
      if (g.type === 'Polygon') g.coordinates = g.coordinates.map(unwrap)
      else if (g.type === 'MultiPolygon') g.coordinates = g.coordinates.map(p => p.map(unwrap))
    }
    return fc
  })
  .catch(err => { russia = undefined; throw err })

const RUSSIA_BOUNDS: [number, number, number, number] = [19, 41, 191, 82]
const PICK_PIN = `<svg width="40" height="40" viewBox="0 0 64 64" fill="none" stroke="${GOLD}" stroke-width="2.8" style="display:block">`
  + `<path d="${DROP}" fill="${INK}"/><circle cx="32" cy="25" r="6" fill="${GOLD}" stroke="none"/></svg>`

export interface MapHandle {
  zoomBy(k: number): void
  flyTo(lat: number, lng: number): void
}

interface Props {
  events: HistEvent[]
  isActive: (e: HistEvent) => boolean
  selectedId: number | null
  onSelect: (id: number) => void
  pad: number
  labels: boolean
  sea?: string
  interactive?: boolean
  /** Режим выбора места: клик по карте и перетаскивание метки вызывают onPick. */
  pick?: { lat: number; lng: number } | null
  onPick?: (lat: number, lng: number) => void
  /** Карточка выбранного события: встаёт рядом с его меткой и ездит вместе с картой. */
  callout?: React.ReactNode
}

const selectedPx = (size: number) => Math.max(size, 32) * 1.1

/** Метка на карте: остриё в (0, 0), наведение увеличивает в 1.14 раза. Масштаб в CSS, чтобы он анимировался. */
function Marker({ type, state, size }: { type: EventType; state: PinState; size: number }) {
  const px = state === 'selected' ? selectedPx(size) : state === 'hover' ? size * 1.14 : size
  return (
    <g className={state === 'selected' ? 'pm-pin sel' : 'pm-pin'} style={{ '--s': px / 64 } as React.CSSProperties}>
      <g transform="translate(-32 -58)">
        <circle cx={32} cy={30} r={32 * Math.max(1, 28 / px)} fill="transparent" />
        <PinShape type={type} size={px} state={state} />
      </g>
    </g>
  )
}

// Поля под шапку, рейл и шкалу веков: в них карточка не заходит, пока метка на экране.
const EDGE = { top: 104, right: 110, bottom: 32, left: 100 }
// При lo > hi побеждает lo.
const clamp = (v: number, lo: number, hi: number) => Math.max(Math.min(v, hi), lo)

/** Выноска у метки (x, y - остриё): справа, если влезает, иначе слева; по вертикали прижата к экрану. */
function Callout({ x, y, pin, width, height, out, children }:
  { x: number; y: number; pin: number; width: number; height: number; out: boolean; children: React.ReactNode }) {
  const ref = useRef<HTMLDivElement>(null)
  const [[w, h], setSize] = useState([0, 0])
  useLayoutEffect(() => {
    const el = ref.current!
    const measure = () => setSize([el.offsetWidth, el.offsetHeight])
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  const gap = pin * 0.35 + 14
  const cy = y - pin / 2
  const right = x + gap + w <= width - EDGE.right || x - gap - w < EDGE.left
  // Уехавшую за край метку карточка догоняет, а не остаётся прижатой к краю.
  const top = clamp(clamp(cy - h / 2, EDGE.top, height - EDGE.bottom - h), cy - h + 28, cy - 28)
  const arrow = clamp(cy - top, 20, h - 20)
  return (
    <div ref={ref} className={`pm-callout ${right ? 'right' : 'left'}${out ? ' out' : ''}`}
      style={{ left: right ? x + gap : x - gap - w, top, '--ay': `${arrow}px` } as React.CSSProperties}>
      <i className="pm-callout-arrow" />
      {children}
    </div>
  )
}

export const MapView = forwardRef<MapHandle, Props>(function MapView(
  { events, isActive, selectedId, onSelect, pad, labels, sea = INK, interactive = true, pick, onPick, callout }, ref) {
  const wrap = useRef<HTMLDivElement>(null)
  const hasCallout = !!callout
  const mapRef = useRef<maplibregl.Map>()
  const pickRef = useRef<maplibregl.Marker>()
  const onPickRef = useRef(onPick)
  onPickRef.current = onPick
  const labelsRef = useRef(labels)
  labelsRef.current = labels
  const baseZoom = useRef(0)
  const [tick, setTick] = useState(0)
  const [hover, setHover] = useState<number | null>(null)

  useEffect(() => {
    const map = new maplibregl.Map({
      container: wrap.current!, style: style(sea), bounds: RUSSIA_BOUNDS, fitBoundsOptions: { padding: pad },
      interactive, maxZoom: 14, renderWorldCopies: true, dragRotate: false, pitchWithRotate: false,
      attributionControl: { compact: true },
    })
    map.touchZoomRotate.disableRotation()
    mapRef.current = map
    baseZoom.current = map.getZoom()
    const redraw = () => setTick(t => t + 1)
    map.on('move', redraw)
    map.on('resize', redraw)
    map.on('click', e => onPickRef.current?.(e.lngLat.lat, e.lngLat.wrap().lng))
    map.on('load', () => loadRussia().then(fc => {
      if (!map.getStyle()) return
      map.addSource('russia', { type: 'geojson', data: fc })
      map.addLayer({ id: 'russia', type: 'fill', source: 'russia', paint: { 'fill-color': '#101A48' } }, 'water')
      map.addLayer({ id: 'regions', type: 'line', source: 'russia', paint: { 'line-color': '#32406E', 'line-width': ['interpolate', ['linear'], ['zoom'], 2, 0.4, 6, 1] } }, 'borders')
    }).catch(() => undefined))
    redraw()
    return () => { map.remove(); mapRef.current = undefined; pickRef.current = undefined }
    // Карта создаётся один раз; pad и sea на лету не меняются.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    const map = mapRef.current!
    const apply = () => LABEL_LAYERS.forEach(id => map.setLayoutProperty(id, 'visibility', labelsRef.current ? 'visible' : 'none'))
    // isStyleLoaded() ложно, пока грузятся тайлы, поэтому проверяем сами слои.
    if (map.getLayer(LABEL_LAYERS[0])) apply()
    else map.once('styledata', apply)
  }, [labels])

  useEffect(() => {
    const map = mapRef.current!
    map.getCanvas().style.cursor = onPick ? 'crosshair' : ''
    if (!pick) { pickRef.current?.remove(); pickRef.current = undefined; return }
    if (!pickRef.current) {
      const el = document.createElement('div')
      el.innerHTML = PICK_PIN
      el.className = 'pm-pick'
      el.setAttribute('aria-label', 'Место события, можно перетащить')
      const m = new maplibregl.Marker({ element: el, anchor: 'bottom', offset: [0, 4], draggable: true })
      m.on('dragend', () => { const p = m.getLngLat(); onPickRef.current?.(p.lat, p.wrap().lng) })
      pickRef.current = m.setLngLat([pick.lng, pick.lat]).addTo(map)
    } else pickRef.current.setLngLat([pick.lng, pick.lat])
  }, [pick, onPick])

  useImperativeHandle(ref, () => ({
    zoomBy: k => { const map = mapRef.current; if (map) map.zoomTo(map.getZoom() + Math.log2(k)) },
    flyTo: (lat, lng) => mapRef.current?.flyTo({ center: [lng, lat], zoom: 10, duration: 800 }),
  }), [])

  // Ушедшую карточку (закрыли или выбрали другую) держим, пока играет анимация ухода
  // (LEAVE_MS); новая в это время уже выезжает у своей метки.
  type Shown = { key: number | null; node: React.ReactNode }
  const liveKey = callout ? selectedId : undefined
  const live = useRef<Shown>()
  const [leaving, setLeaving] = useState<Shown[]>([])
  useEffect(() => {
    const gone = live.current && live.current.key !== liveKey ? live.current : undefined
    setLeaving(l => {
      const rest = l.filter(x => x.key !== liveKey)
      return gone ? [...rest, gone] : rest.length === l.length ? l : rest
    })
    // Без очистки: таймер должен сработать, даже если карточку успели сменить ещё раз.
    if (gone) setTimeout(() => setLeaving(l => l.filter(x => x !== gone)), LEAVE_MS)
  }, [liveKey])
  // После эффекта выше: он должен видеть карточку прошлого рендера.
  useEffect(() => { live.current = liveKey === undefined ? undefined : { key: liveKey, node: callout } })
  const cards = [
    ...leaving.map(c => ({ ...c, out: true })),
    ...(liveKey === undefined ? [] : [{ key: liveKey, node: callout, out: false }]),
  ]
  const anchorIds = cards.map(c => c.key).join()

  // Выбранное за краем экрана (из поиска, ленты, ссылки) подвозим в центр, иначе карточку не видно.
  useEffect(() => {
    const map = mapRef.current!
    const e = events.find(ev => ev.id === selectedId)
    if (!e || !hasCallout) return
    const lng = e.longitude + 360 * Math.round((map.getCenter().lng - e.longitude) / 360)
    const { x, y } = map.project([lng, e.latitude])
    const { width, height } = map.getContainer().getBoundingClientRect()
    if (x < EDGE.left || y < EDGE.top || x > width - EDGE.right || y > height - EDGE.bottom)
      map.easeTo({ center: [lng, e.latitude], duration: 600 })
  }, [selectedId, events, hasCallout])

  // Точки в экранных координатах; кластеризуем только активные, по сетке CELL.
  // ponytail: сеточная кластеризация O(n), на десятках тысяч меток понадобится supercluster.
  const view = useMemo(() => {
    type P = { e: HistEvent; x: number; y: number; lng: number }
    const map = mapRef.current
    const pts: (P & { active: boolean })[] = [], clusters: { x: number; y: number; members: P[] }[] = []
    const anchors = new Map<number, { x: number; y: number }>()
    if (!map) return { pins: pts, clusters, anchors, k: 1, width: 0, height: 0 }
    const wanted = new Set(anchorIds.split(',').map(Number))
    const { width, height } = map.getContainer().getBoundingClientRect()
    const center = map.getCenter().lng
    const cells = new Map<string, P[]>()
    for (const e of events) {
      // Берём копию мира, ближайшую к центру, иначе метки у 180-го меридиана уезжают за экран.
      const lng = e.longitude + 360 * Math.round((center - e.longitude) / 360)
      const { x, y } = map.project([lng, e.latitude])
      if (wanted.has(e.id)) anchors.set(e.id, { x, y })
      if (x < -CELL || y < -CELL || x > width + CELL || y > height + CELL) continue
      const pt = { e, x, y, lng, active: isActive(e) }
      pts.push(pt)
      if (!pt.active) continue
      const key = e.id === selectedId ? 'sel' : `${Math.floor(x / CELL)}:${Math.floor(y / CELL)}`
      cells.set(key, [...(cells.get(key) ?? []), pt])
    }
    const grouped = new Set<number>()
    for (const m of cells.values()) {
      if (m.length === 1) continue
      m.forEach(p => grouped.add(p.e.id))
      clusters.push({ x: m.reduce((s, p) => s + p.x, 0) / m.length, y: m.reduce((s, p) => s + p.y, 0) / m.length, members: m })
    }
    // Активные и неактивные одним списком в порядке events: при смене фильтра метка остаётся тем же
    // DOM-узлом и перекрашивается плавно, а не пересоздаётся.
    const pins = pts.filter(p => !grouped.has(p.e.id))
    return { pins, clusters, anchors, k: 2 ** (map.getZoom() - baseZoom.current), width, height }
    // tick - перерисовка при движении карты.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [events, isActive, tick, selectedId, anchorIds])

  // Долготы уже подогнаны к видимой копии мира, иначе группа через 180-й меридиан растянется на весь мир.
  const zoomToCluster = (members: { e: HistEvent; lng: number }[]) => {
    const b = new maplibregl.LngLatBounds()
    members.forEach(({ e, lng }) => b.extend([lng, e.latitude]))
    mapRef.current?.fitBounds(b, { padding: 80, maxZoom: 12, duration: 600 })
  }

  const button = (run: () => void) => ({
    className: 'pm-hit', role: 'button', tabIndex: 0, onClick: run,
    onKeyDown: (ev: React.KeyboardEvent) => { if (ev.key === 'Enter' || ev.key === ' ') { ev.preventDefault(); run() } },
  })
  const hovered = view.pins.find(p => p.active && p.e.id === hover && p.e.id !== selectedId)
  const pin = pinSize(view.k)

  return (
    <div className="pm-map" style={{ background: sea }}>
      <div ref={wrap} className="pm-map-canvas" />
      <svg className="pm-map-overlay">
        {view.clusters.map(c => (
          <g key={`c${c.members[0].e.id}`} transform={`translate(${c.x},${c.y})`} {...button(() => zoomToCluster(c.members))}
            className="pm-hit pm-appear" aria-label={`${c.members.length} событий, приблизить`}>
            <circle className="pm-cluster" r={16.5} fill={alpha(GOLD, 0.12)} stroke={GOLD} strokeWidth={1.6} />
            <text textAnchor="middle" dy={4.5} className="pm-cluster-n">{c.members.length}</text>
          </g>
        ))}
        {view.pins.map(p => p.active ? (
          <g key={p.e.id} transform={`translate(${p.x},${p.y})`} {...button(() => onSelect(p.e.id))} aria-label={p.e.title}
            className="pm-hit pm-appear"
            onMouseEnter={() => setHover(p.e.id)} onMouseLeave={() => setHover(null)}
            onFocus={() => setHover(p.e.id)} onBlur={() => setHover(null)}>
            <Marker type={p.e.eventType} size={pin} state={p.e.id === selectedId ? 'selected' : p.e.id === hover ? 'hover' : 'normal'} />
          </g>
        ) : (
          <g key={p.e.id} transform={`translate(${p.x},${p.y})`} className="pm-appear">
            <Marker type={p.e.eventType} state="inactive" size={pin} />
          </g>
        ))}
      </svg>
      <Presence>{hovered && (
        <div className="pm-popup" style={{ left: hovered.x, top: hovered.y - pin * 1.14 - 8 }}>
          <div className="pm-popup-title">{hovered.e.title}</div>
          <div className="pm-popup-meta">{yearOf(hovered.e)} · {TYPE_LABEL[hovered.e.eventType]}</div>
        </div>
      )}</Presence>
      {cards.map(c => {
        const at = c.key === null ? undefined : view.anchors.get(c.key)
        return at
          ? <Callout key={c.key} x={at.x} y={at.y} pin={selectedPx(pin)} width={view.width} height={view.height} out={c.out}>{c.node}</Callout>
          // Событие не в списке (ссылка на снятое, ещё грузится): метки нет, карточка в углу.
          : <div key={c.key} className={`pm-callout fixed${c.out ? ' out' : ''}`}>{c.node}</div>
      })}
    </div>
  )
})
