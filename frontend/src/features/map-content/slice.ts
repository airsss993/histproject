import L from 'leaflet'
import { EventObject } from '../marker-location/evenPositions'

export interface ActiveEventData {
  event: EventObject
  position: L.LatLng
  markerKey: string
}

export interface MapState {
  activeEvent: ActiveEventData | null
  filteredEventTypes: number[]
}

export const initialState: MapState = {
  activeEvent: null,
  filteredEventTypes: [],
}

export type MapAction =
  | { type: 'SET_ACTIVE_EVENT'; payload: ActiveEventData | null }
  | { type: 'TOGGLE_FILTER'; payload: number }
