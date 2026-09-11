import { MapState, MapAction } from './slice'

export function mapReducer(state: MapState, action: MapAction): MapState {
  switch (action.type) {
    case 'SET_ACTIVE_EVENT':
      return { ...state, activeEvent: action.payload }
    case 'TOGGLE_FILTER': {
      const index = state.filteredEventTypes.indexOf(action.payload)
      if (index >= 0) {
        return {
          ...state,
          filteredEventTypes: state.filteredEventTypes.filter(t => t !== action.payload),
        }
      }
      return {
        ...state,
        filteredEventTypes: [...state.filteredEventTypes, action.payload],
      }
    }
    default:
      return state
  }
}
