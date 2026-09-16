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

export const TYPE_COLOR: Record<EventType, string> = { political: '#50749F', military: '#F0B62A' }
const CELL = 56

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

function Marker({ type, state }: { type: EventType; state: 'normal' | 'hover' | 'selected' | 'inactive' }) {
  const selected = state === 'selected'
  const r = selected ? 13 : 9
  const glyph = type === 'military'
    ? <path d="M -3.2 3.4 L 0 -3.8 L 3.2 3.4" fill="none" stroke="#010011" strokeWidth={1.6} strokeLinejoin="round" />
    : <rect x={-3} y={-3} width={6} height={6} fill="#010011" transform="rotate(45)" />
  return (
    <g opacity={state === 'inactive' ? 0.28 : 1}>
      {(selected || state === 'hover') && (
        <circle r={selected ? 24 : 18} fill="none" stroke={selected ? '#F0B62A' : 'rgba(240,182,42,0.45)'} strokeDasharray={selected ? '3 4' : undefined} />
      )}
      <circle r={r + 12} fill="transparent" />
      <circle r={r + 3} fill="rgba(1,0,17,0.55)" />
      <circle r={r} fill={TYPE_COLOR[type]} stroke={selected ? '#FFFFFF' : 'rgba(1,0,17,0.6)'} strokeWidth={1.2} />
      {glyph}
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
                <Marker type={p.e.eventType} state="inactive" />
              </g>
            ))}
            {view.clusters.map(c => (
              <g key={c.members[0].e.id} transform={`translate(${c.x},${c.y})`} className="pm-hit"
                role="button" aria-label={`${c.members.length} событий, приблизить`} onClick={() => zoomToCluster(c.members)}>
                <circle r={22} fill="rgba(19,28,64,0.55)" stroke="#32406E" />
                <circle r={15} fill="#131C40" stroke="#8A6A1C" />
                <text textAnchor="middle" dy={5} className="pm-cluster-n">{c.members.length}</text>
              </g>
            ))}
            {view.singles.map(p => (
              <g key={p.e.id} transform={`translate(${p.x},${p.y})`} className="pm-hit" role="button" aria-label={p.e.title}
                onMouseEnter={() => setHover(p.e.id)} onMouseLeave={() => setHover(null)} onClick={() => onSelect(p.e.id)}>
                <Marker type={p.e.eventType} state={p.e.id === selectedId ? 'selected' : p.e.id === hover ? 'hover' : 'normal'} />
              </g>
            ))}
          </>
        )}
      </svg>
      {hovered && (
        <div className="pm-popup" style={{ left: hovered.x, top: hovered.y - 26 }}>
          <div className="pm-popup-title">{hovered.e.title}</div>
          <div className="pm-popup-meta">{yearOf(hovered.e)} · {TYPE_LABEL[hovered.e.eventType]}</div>
        </div>
      )}
    </div>
  )
})
