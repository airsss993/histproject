import type { FeatureCollection, Position } from 'geojson'
import type { HistEvent } from './api.ts'

// Чукотка пересекает 180-й меридиан: сдвигаем долготы кольца, чтобы полигон не растягивался через весь мир.
const unwrap = (ring: Position[]) => ring.reduce<Position[]>((out, [lng, lat]) => {
  const prev = out.length ? out[out.length - 1][0] : lng
  out.push([lng + 360 * Math.round((prev - lng) / 360), lat])
  return out
}, [])
// Грузим при первой карте; неудачную загрузку не кешируем, следующая карта попробует снова.
let russia: Promise<FeatureCollection> | undefined
export const loadRussia = () => russia ??= fetch('/russia-regions.geojson')
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

/** [запад, юг, восток, север]; у Чукотки восток больше 180. */
export type BBox = [number, number, number, number]
export interface RegionGroup { iso: string | null; name: string; bbox: BBox; events: HistEvent[] }

const bboxOf = (pts: Position[]): BBox => pts.reduce<BBox>(
  ([w, s, e, n], [x, y]) => [Math.min(w, x), Math.min(s, y), Math.max(e, x), Math.max(n, y)],
  [Infinity, Infinity, -Infinity, -Infinity])

const inRing = (ring: Position[], x: number, y: number) => {
  let inside = false
  for (let i = 0, j = ring.length - 1; i < ring.length; j = i++) {
    const [xi, yi] = ring[i], [xj, yj] = ring[j]
    if ((yi > y) !== (yj > y) && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi) inside = !inside
  }
  return inside
}

type Region = { iso: string; name: string; bbox: BBox; polys: Position[][][] }
const cache = new WeakMap<FeatureCollection, Region[]>()
const regionsOf = (fc: FeatureCollection) => {
  let rs = cache.get(fc)
  if (rs) return rs
  rs = fc.features.flatMap(f => {
    const g = f.geometry
    const polys = g.type === 'Polygon' ? [g.coordinates] : g.type === 'MultiPolygon' ? g.coordinates : []
    if (!polys.length) return []
    return [{ iso: String(f.properties?.iso), name: String(f.properties?.name), bbox: bboxOf(polys.flatMap(p => p[0])), polys }]
  })
  cache.set(fc, rs)
  return rs
}

export const OUTSIDE = 'За пределами России'

/** Регион точки: внутри внешнего кольца и вне дыр. У Чукотки долготы за 180, поэтому пробуем и lng + 360. */
export function regionOf(fc: FeatureCollection, lng: number, lat: number): Region | undefined {
  for (const r of regionsOf(fc)) {
    for (const x of [lng, lng + 360]) {
      if (x < r.bbox[0] || x > r.bbox[2] || lat < r.bbox[1] || lat > r.bbox[3]) continue
      if (r.polys.some(p => inRing(p[0], x, lat) && !p.slice(1).some(h => inRing(h, x, lat)))) return r
    }
  }
}

/**
 * События по регионам с запада на восток, чтобы шар при прокрутке крутился в одну сторону.
 * Внутри региона - по дате. События вне России - последней группой.
 * ponytail: перебор полигонов на каждый вызов, O(событий x вершин в bbox); на тысячах событий кешировать регион по id.
 */
export function groupByRegion(events: HistEvent[], fc: FeatureCollection): RegionGroup[] {
  const groups = new Map<string | null, RegionGroup>()
  for (const e of [...events].sort((a, b) => a.eventDate.localeCompare(b.eventDate))) {
    const r = regionOf(fc, e.longitude, e.latitude)
    const key = r?.iso ?? null
    let g = groups.get(key)
    if (!g) groups.set(key, g = { iso: key, name: r?.name ?? OUTSIDE, bbox: r?.bbox ?? [e.longitude, e.latitude, e.longitude, e.latitude], events: [] })
    g.events.push(e)
  }
  const out = groups.get(null)
  if (out) out.bbox = bboxOf(out.events.map(e => [e.longitude, e.latitude]))
  const mid = (b: BBox) => (b[0] + b[2]) / 2
  return [...groups.values()].sort((a, b) => (a.iso === null ? 1 : b.iso === null ? -1 : mid(a.bbox) - mid(b.bbox)))
}
