import React, { createContext, useReducer, ReactNode } from 'react'
import { MapState, MapAction, initialState } from './slice'
import { mapReducer } from './reducer'
import { selectActiveEvent, selectActiveMarkerKey, selectFilteredEventTypes, selectIsEventActive } from './selectors'

interface MapContextValue {
  state: MapState
  dispatch: React.Dispatch<MapAction>
  activeEvent: ReturnType<typeof selectActiveEvent>
  activeMarkerKey: ReturnType<typeof selectActiveMarkerKey>
  filteredEventTypes: ReturnType<typeof selectFilteredEventTypes>
  isEventActive: ReturnType<typeof selectIsEventActive>
}

const MapContext = createContext<MapContextValue | undefined>(undefined)

export const MapProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(mapReducer, initialState)

  const value: MapContextValue = {
    state,
    dispatch,
    activeEvent: selectActiveEvent(state),
    activeMarkerKey: selectActiveMarkerKey(state),
    filteredEventTypes: selectFilteredEventTypes(state),
    isEventActive: selectIsEventActive(state),
  }

  return <MapContext.Provider value={value}>{children}</MapContext.Provider>
}

export const useMapContext = () => {
  const ctx = React.useContext(MapContext)
  if (!ctx) throw new Error('useMapContext must be used within MapProvider')
  return ctx
}
