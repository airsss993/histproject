import React, { createContext, useContext, ReactNode, useEffect, useMemo, useState, useCallback } from 'react'
// import { resetAndReload, loadInitialEvents } from '../../../app/saga/saga'
import { EventObject } from '../../../marker-location/evenPositions'
import { useEventFilters, FilterState } from '../evenFilters'
import { buildFilterRequestData, EventTypeItem } from '../typeven'
import { fetchEventsAPI, fetchEventsByFiltersAPI } from '../../saga'

interface EventFilterContextType {
  filteredEvents: EventObject[];
  filterState: FilterState;
  toggleOption: (option: string) => void;
  setPeriodRange: (min: number, max: number) => void;
  setSelectedPeriod: (periodLabel: string | null) => void;
  resetFilters: () => void;
  applyFilters: () => Promise<void>;
  applyFiltersWith: (nextState: FilterState) => Promise<void>;
  resetAndReload: () => Promise<void>;
  isLoading: boolean;
  error: string | null;
}

const EventFilterContext = createContext<EventFilterContextType | undefined>(undefined);

export const useEventFilterContext = () => {
  const context = useContext(EventFilterContext);
  if (!context) {
    throw new Error('useEventFilterContext must be used within EventFilterProvider');
  }
  return context;
};

interface EventFilterProviderProps {
  children: ReactNode;
}

export const EventFilterProvider: React.FC<EventFilterProviderProps> = ({ children }) => {

  const [events, setEvents] = useState<EventObject[]>([]);
  const [eventTypes] = useState<EventTypeItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const {
    filteredEvents,
    filterState,
    toggleOption,
    setPeriodRange,
    setSelectedPeriod,
    resetFilters,
    setFilterState
  } = useEventFilters({ events, eventTypes });

  const loadInitialEvents = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const fetchedEvents = await fetchEventsAPI();
      setEvents(Array.isArray(fetchedEvents) ? (fetchedEvents as EventObject[]) : []);
    } catch (err) {
      console.error('Failed to load events:', err);
      setError('Не удалось загрузить события');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadInitialEvents();
  }, [loadInitialEvents]);

  const resetAndReload = useCallback(async () => {
    resetFilters();
    await loadInitialEvents();
  }, [loadInitialEvents, resetFilters]);

  const applyFilters = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const payload = buildFilterRequestData({
        selectedOptions: filterState.selectedOptions,
        periodRange: filterState.periodRange,
        selectedPeriod: filterState.selectedPeriod,
        eventTypes
      });
      const fetchedEvents = await fetchEventsByFiltersAPI(payload);
      setEvents(Array.isArray(fetchedEvents) ? (fetchedEvents as EventObject[]) : []);
    } catch (err) {
      console.error('Failed to apply filters:', err);
      setError('Не удалось применить фильтры');
    } finally {
      setIsLoading(false);
    }
  }, [filterState, eventTypes]);

  const applyFiltersWith = useCallback(async (nextState: FilterState) => {
    try {
      setIsLoading(true);
      setError(null);
      setFilterState(nextState);
      const payload = buildFilterRequestData({
        selectedOptions: nextState.selectedOptions,
        periodRange: nextState.periodRange,
        selectedPeriod: nextState.selectedPeriod,
        eventTypes
      });
      const fetchedEvents = await fetchEventsByFiltersAPI(payload);
      setEvents(Array.isArray(fetchedEvents) ? (fetchedEvents as EventObject[]) : []);
    } catch (err) {
      console.error('Failed to apply filters:', err);
      setError('Не удалось применить фильтры');
    } finally {
      setIsLoading(false);
    }
  }, [setFilterState, eventTypes]);

  const contextValue = useMemo(() => ({
    filteredEvents,
    filterState,
    toggleOption,
    setPeriodRange,
    setSelectedPeriod,
    resetFilters,
    applyFilters,
    applyFiltersWith,
    resetAndReload,
    isLoading,
    error
  }), [
    filteredEvents,
    filterState,
    toggleOption,
    setPeriodRange,
    setSelectedPeriod,
    resetFilters,
    applyFilters,
    applyFiltersWith,
    resetAndReload,
    isLoading,
    error
  ]);

  return (
    <EventFilterContext.Provider
      value={contextValue}
    >
      {children}
    </EventFilterContext.Provider>
  );
};