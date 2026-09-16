import { forwardRef, useEffect, useImperativeHandle, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { geoAlbers, geoGraticule, geoPath } from 'd3-geo'
import { select } from 'd3-selection'
import { zoom as d3zoom, zoomIdentity, type ZoomBehavior, type ZoomTransform } from 'd3-zoom'
import { feature } from 'topojson-client'
import type { GeometryCollection, Topology } from 'topojson-specification'
import type { Feature } from 'geojson'
import world from 'world-atlas/countries-110m.json'
import { TYPE_LABEL, yearOf, type EventType, type HistEvent } from './api.ts'

const topo = world as unknown as Topology<{ countries: GeometryCollection<{ name: string }> }>
const COUNTRIES = feature(topo, topo.objects.countries).features as Feature[]
const RUSSIA = COUNTRIES.find(f => f.properties?.name === 'Russia')!

const NEIGHBOURS = new Set(['Kazakhstan', 'China', 'Mongolia', 'Ukraine', 'Belarus', 'Finland', 'Norway', 'Sweden', 'Poland',
  'Estonia', 'Latvia', 'Lithuania', 'Turkey', 'Japan', 'Iran', 'Georgia', 'Azerbaijan', 'Armenia', 'Uzbekistan',
  'Kyrgyzstan', 'Turkmenistan', 'Tajikistan', 'North Korea', 'South Korea', 'Afghanistan', 'Moldova', 'Romania',
  'Germany', 'Iraq', 'Slovakia', 'Hungary', 'Czechia', 'India', 'Pakistan', 'Nepal', 'Denmark', 'Bulgaria'])
const LABELS: Record<string, string> = { Kazakhstan: 'КАЗАХСТАН', China: 'КИТАЙ', Mongolia: 'МОНГОЛИЯ', Finland: 'ФИНЛЯНДИЯ', Ukraine: 'УКРАИНА', Japan: 'ЯПОНИЯ' }
const OTHERS = COUNTRIES.filter(f => NEIGHBOURS.has(f.properties?.name))

export const TYPE_COLOR: Record<EventType, string> = { political: '#8FB4E8', military: '#F0B62A' }
const CELL = 56
const INK = '#060D2E'
const MUTED = '#4E5C86'

// Метка из "Логотип и метки": капля 64x64 с остриём в (32, 58), глиф типа в окне.
const DROP = 'M32 58C32 58 13 37 13 25A19 19 0 0 1 51 25C51 37 32 58 32 58Z'
const GLYPH: Record<EventType, string[]> = {
  military: ['M32 11V19', 'M24.5 19H39.5', 'M28.5 19V29L32 36L35.5 29V19'],
  political: ['M24 16H40', 'M27 16V32', 'M37 16V32', 'M23 32H41'],
}
const alpha = (hex: string, a: number) => hex + Math.round(a * 255).toString(16).padStart(2, '0')

// Размер метки в px по зуму: 16 - залитая капля, 24 - капля с точкой, от 32 - с глифом.
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
      {selected && <circle cx={32} cy={28} r={33} stroke={alpha(color, 0.34)} strokeWidth={1.6} />}
      <path d={DROP} fill={fill} />
      {size < 32
        ? <circle cx={32} cy={25} r={4} fill={selected ? INK : color} stroke="none" />
        : GLYPH[type].map(d => <path key={d} d={d} stroke={selected ? INK : color} />)}
    </g>
  )
}

export interface MapHandle { zoomBy(k: number): void }

interface Props {
  events: HistEvent[]
  isActive: (e: HistEvent) => boolean
  selectedId: number | null
  onSelect: (id: number) => void
  pad: number
  labels: boolean
  sea?: string
  interactive?: boolean
}

/** Метка на карте: остриё в (0, 0), наведение увеличивает в 1.14 раза. */
function Marker({ type, state, size }: { type: EventType; state: PinState; size: number }) {
  const px = state === 'selected' ? Math.max(size, 32) * 1.1 : state === 'hover' ? size * 1.14 : size
  return (
    <g transform={`scale(${px / 64}) translate(-32 -58)`}>
      <circle cx={32} cy={30} r={32 * Math.max(1, 28 / px)} fill="transparent" />
      <PinShape type={type} size={px} state={state} />
    </g>
  )
}

export const MapView = forwardRef<MapHandle, Props>(function MapView(
  { events, isActive, selectedId, onSelect, pad, labels, sea = '#060D2E', interactive = true }, ref) {
  const wrap = useRef<HTMLDivElement>(null)
  const svg = useRef<SVGSVGElement>(null)
  const zb = useRef<ZoomBehavior<SVGSVGElement, unknown>>()
  const [size, setSize] = useState({ w: 0, h: 0 })
  const [t, setT] = useState<ZoomTransform>(zoomIdentity)
  const [hover, setHover] = useState<number | null>(null)

  useLayoutEffect(() => {
    const el = wrap.current!
    const ro = new ResizeObserver(() => setSize({ w: el.clientWidth, h: el.clientHeight }))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const geo = useMemo(() => {
    if (!size.w) return null
    const proj = geoAlbers().rotate([-105, 0]).center([0, 62]).parallels([50, 70])
      .fitExtent([[pad, pad], [size.w - pad, size.h - pad]], RUSSIA)
    const path = geoPath(proj)
    return {
      proj,
      russia: path(RUSSIA) ?? '',
      others: path({ type: 'FeatureCollection', features: OTHERS }) ?? '',
      grat: path(geoGraticule().step([15, 15])()) ?? '',
      labels: OTHERS.filter(f => LABELS[f.properties?.name]).map(f => ({ name: LABELS[f.properties?.name], c: path.centroid(f) })),
    }
  }, [size, pad])

  useEffect(() => {
    if (!svg.current || !size.w || !interactive) return
    const z = d3zoom<SVGSVGElement, unknown>()
      .scaleExtent([1, 12])
      .translateExtent([[0, 0], [size.w, size.h]])
      .on('zoom', ev => setT(ev.transform))
    zb.current = z
    select(svg.current).call(z)
    return () => { select(svg.current!).on('.zoom', null) }
  }, [size, interactive])

  const setTransform = (next: ZoomTransform) => { if (svg.current && zb.current) select(svg.current).call(zb.current.transform, next) }
  useImperativeHandle(ref, () => ({
    zoomBy: k => { if (svg.current && zb.current) select(svg.current).call(zb.current.scaleBy, k) },
  }), [])

  // Точки в экранных координатах; кластеризуем только активные, по сетке CELL.
  // ponytail: сеточная кластеризация O(n), на десятках тысяч меток понадобится supercluster.
  const view = useMemo(() => {
    if (!geo) return { singles: [], clusters: [], inactive: [] }
    type P = { e: HistEvent; x: number; y: number; px: number; py: number }
    const inactive: P[] = [], cells = new Map<string, P[]>()
    for (const e of events) {
      const p = geo.proj([e.longitude, e.latitude])
      if (!p) continue
      const [x, y] = t.apply(p)
      const pt = { e, x, y, px: p[0], py: p[1] }
      if (!isActive(e)) { inactive.push(pt); continue }
      const key = e.id === selectedId ? `sel` : `${Math.floor(x / CELL)}:${Math.floor(y / CELL)}`
      cells.set(key, [...(cells.get(key) ?? []), pt])
    }
    const singles: P[] = [], clusters: { x: number; y: number; members: P[] }[] = []
    for (const m of cells.values()) {
      if (m.length === 1) singles.push(m[0])
      else clusters.push({ x: m.reduce((s, p) => s + p.x, 0) / m.length, y: m.reduce((s, p) => s + p.y, 0) / m.length, members: m })
    }
    return { singles, clusters, inactive }
  }, [geo, events, isActive, t, selectedId])

  const zoomToCluster = (members: { px: number; py: number }[]) => {
    const xs = members.map(m => m.px), ys = members.map(m => m.py)
    const [x0, x1, y0, y1] = [Math.min(...xs), Math.max(...xs), Math.min(...ys), Math.max(...ys)]
    const k = Math.min(12, 0.6 / Math.max((x1 - x0) / size.w, (y1 - y0) / size.h, 0.01))
    setTransform(zoomIdentity.translate(size.w / 2, size.h / 2).scale(k).translate(-(x0 + x1) / 2, -(y0 + y1) / 2))
  }

  const hovered = view.singles.find(p => p.e.id === hover && p.e.id !== selectedId)
  const pin = pinSize(t.k)

  return (
    <div ref={wrap} className="pm-map">
      <svg ref={svg} width={size.w} height={size.h} style={{ display: 'block', background: sea }}>
        {geo && (
          <>
            <g transform={t.toString()}>
              <path d={geo.grat} fill="none" stroke="rgba(50,64,110,0.4)" strokeWidth={0.6} vectorEffect="non-scaling-stroke" />
              <path d={geo.others} fill="#080E28" stroke="#2C4272" strokeWidth={0.7} vectorEffect="non-scaling-stroke" />
              <path d={geo.russia} fill="#0B1234" stroke="#32406E" strokeWidth={1} vectorEffect="non-scaling-stroke" />
            </g>
            {labels && geo.labels.map(l => {
              const [x, y] = t.apply(l.c)
              return <text key={l.name} x={x} y={y} textAnchor="middle" className="pm-map-label">{l.name}</text>
            })}
            {view.inactive.map(p => (
              <g key={p.e.id} transform={`translate(${p.x},${p.y})`} style={{ pointerEvents: 'none' }}>
                <Marker type={p.e.eventType} state="inactive" size={pin} />
              </g>
            ))}
            {view.clusters.map(c => (
              <g key={c.members[0].e.id} transform={`translate(${c.x},${c.y})`} className="pm-hit"
                role="button" aria-label={`${c.members.length} событий, приблизить`} onClick={() => zoomToCluster(c.members)}>
                <circle r={16.5} fill={alpha(TYPE_COLOR.military, 0.12)} stroke={TYPE_COLOR.military} strokeWidth={1.6} />
                <text textAnchor="middle" dy={4.5} className="pm-cluster-n">{c.members.length}</text>
              </g>
            ))}
            {view.singles.map(p => (
              <g key={p.e.id} transform={`translate(${p.x},${p.y})`} className="pm-hit" role="button" aria-label={p.e.title}
                onMouseEnter={() => setHover(p.e.id)} onMouseLeave={() => setHover(null)} onClick={() => onSelect(p.e.id)}>
                <Marker type={p.e.eventType} size={pin} state={p.e.id === selectedId ? 'selected' : p.e.id === hover ? 'hover' : 'normal'} />
              </g>
            ))}
          </>
        )}
      </svg>
      {hovered && (
        <div className="pm-popup" style={{ left: hovered.x, top: hovered.y - pin * 1.14 - 8 }}>
          <div className="pm-popup-title">{hovered.e.title}</div>
          <div className="pm-popup-meta">{yearOf(hovered.e)} · {TYPE_LABEL[hovered.e.eventType]}</div>
        </div>
      )}
    </div>
  )
})
