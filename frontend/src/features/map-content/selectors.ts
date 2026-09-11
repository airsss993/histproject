import { MapState, ActiveEventData } from './slice'

export const selectActiveEvent = (state: MapState): ActiveEventData | null =>
  state.activeEvent

export const selectActiveMarkerKey = (state: MapState): string | null =>
  state.activeEvent?.markerKey ?? null

export const selectFilteredEventTypes = (state: MapState): number[] =>
  state.filteredEventTypes

export const selectIsEventActive = (state: MapState): boolean =>
  state.activeEvent !== null
