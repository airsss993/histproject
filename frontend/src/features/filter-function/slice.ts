import { createSlice, createSelector, PayloadAction } from '@reduxjs/toolkit'
import { EventObject } from '../marker-location/evenPositions'
import { EventTypeItem, mapSelectedOptionsToEventTypeIds } from './ui/typeven'

export interface FilterState {
  selectedOptions: { [key: string]: boolean };
  periodRange: { min: number; max: number };
  selectedPeriod: string | null;
}

export interface ApplyFiltersPayload extends FilterState {
  hideAllMarkers?: boolean;
}

interface FilterSliceState extends FilterState {
  isOpen: boolean;
  activeFilter: number | null;
  events: EventObject[];
  eventTypes: EventTypeItem[];
  isLoading: boolean;
  error: string | null;
}

const initialState: FilterSliceState = {
  isOpen: false,
  activeFilter: null,
  selectedOptions: {},
  periodRange: { min: 862, max: 2026 },
  selectedPeriod: null,
  events: [],
  eventTypes: [],
  isLoading: false,
  error: null,
};

const filterSlice = createSlice({
  name: 'filter',
  initialState,
  reducers: {
    setIsOpen(state, action: PayloadAction<boolean>) {
      state.isOpen = action.payload;
    },
    setActiveFilter(state, action: PayloadAction<number | null>) {
      state.activeFilter = action.payload;
    },
    toggleOption(state, action: PayloadAction<string>) {
      const option = action.payload;
      state.selectedOptions[option] = !state.selectedOptions[option];
    },
    setPeriodRange(state, action: PayloadAction<{ min: number; max: number }>) {
      state.periodRange = action.payload;
    },
    setSelectedPeriod(state, action: PayloadAction<string | null>) {
      state.selectedPeriod = action.payload;
    },
    setEvents(state, action: PayloadAction<EventObject[]>) {
      state.events = action.payload;
    },
    setIsLoading(state, action: PayloadAction<boolean>) {
      state.isLoading = action.payload;
    },
    setError(state, action: PayloadAction<string | null>) {
      state.error = action.payload;
    },
    resetFilterState(state) {
      state.selectedOptions = {};
      state.periodRange = { min: 862, max: 2026 };
      state.selectedPeriod = null;
    },
    // Saga trigger actions — payload handled only in sagas
    fetchEventsRequest() {},
    applyFiltersRequest(_state, _action: PayloadAction<ApplyFiltersPayload>) {},
    resetAndReloadRequest() {},
  },
});

export const {
  setIsOpen,
  setActiveFilter,
  toggleOption,
  setPeriodRange,
  setSelectedPeriod,
  setEvents,
  setIsLoading,
  setError,
  resetFilterState,
  fetchEventsRequest,
  applyFiltersRequest,
  resetAndReloadRequest,
} = filterSlice.actions;

// ---- Selectors ----

const selectFilterSlice = (state: { filter: FilterSliceState }) => state.filter;

const parseEventYear = (dateString: string): number | null => {
  if (!dateString) return null;
  const parsed = new Date(dateString);
  if (!isNaN(parsed.getTime())) return parsed.getFullYear();
  const matches = dateString.match(/\d{3,4}/g);
  if (!matches || matches.length === 0) return null;
  const year = parseInt(matches[matches.length - 1], 10);
  return Number.isNaN(year) ? null : year;
};

export const selectFilteredEvents = createSelector(
  selectFilterSlice,
  ({ events, selectedOptions, periodRange, selectedPeriod, eventTypes }) => {
    const selectedTypeIds = mapSelectedOptionsToEventTypeIds(selectedOptions, eventTypes);
    return events.filter(event => {
      const year = parseEventYear(event.eventDate);
      if (year === null) return false;

      const dateMatch = year >= periodRange.min && year <= periodRange.max;
      const typeMatch =
        selectedTypeIds.length === 0 ||
        selectedTypeIds.includes(event.eventType as unknown as number);

      let periodMatch = true;
      if (selectedPeriod) {
        const m = selectedPeriod.match(/(\d+)\D+(\d+)/);
        if (m) periodMatch = year >= parseInt(m[1]) && year <= parseInt(m[2]);
      }

      return dateMatch && typeMatch && periodMatch;
    });
  }
);

export default filterSlice.reducer;
