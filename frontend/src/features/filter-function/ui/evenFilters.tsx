import { useState, useEffect, useCallback, useMemo } from 'react'
import { EventObject } from '../../marker-location/evenPositions'
import { EventTypeItem, mapSelectedOptionsToEventTypeIds } from './typeven'

export interface FilterState {
  selectedOptions: {[key: string]: boolean};
  periodRange: { min: number; max: number };
  selectedPeriod: string | null;
}

interface UseEventFiltersProps {
  events: EventObject[];
  eventTypes?: EventTypeItem[];
  onFilteredEventsChange?: (filteredEvents: EventObject[]) => void;
}

export const useEventFilters = ({ events, eventTypes, onFilteredEventsChange }: UseEventFiltersProps) => {
  const [filterState, setFilterState] = useState<FilterState>({
    selectedOptions: {},
    periodRange: { min: 862, max: 2026 },
    selectedPeriod: null
  });

  const parseEventYear = (dateString: string): number | null => {
    if (!dateString) return null;
    const parsed = new Date(dateString);
    if (!isNaN(parsed.getTime())) return parsed.getFullYear();

    const matches = dateString.match(/\d{3,4}/g);
    if (!matches || matches.length === 0) return null;
    const year = parseInt(matches[matches.length - 1], 10);
    return Number.isNaN(year) ? null : year;
  };

  const selectedEventTypes = useMemo(() => {
    return mapSelectedOptionsToEventTypeIds(filterState.selectedOptions, eventTypes ?? []);
  }, [filterState.selectedOptions, eventTypes]);

  const filteredEvents = useMemo(() => {
    return events.filter(event => {
      const eventYear = parseEventYear(event.eventDate);
      if (eventYear === null) return false;

      const dateMatch = eventYear >= filterState.periodRange.min && 
        eventYear <= filterState.periodRange.max;
      
      const typeMatch = selectedEventTypes.length === 0 || 
        selectedEventTypes.includes(event.eventType);
      
      let periodMatch = true;
      if (filterState.selectedPeriod) {
        const matches = filterState.selectedPeriod.match(/(\d+)\D+(\d+)/);
        if (matches) {
          const startYear = parseInt(matches[1]);
          const endYear = parseInt(matches[2]);
          periodMatch = eventYear >= startYear && eventYear <= endYear;
        }
      }
      
      return dateMatch && typeMatch && periodMatch;
    });
  }, [events, filterState, selectedEventTypes]);

  useEffect(() => {
    onFilteredEventsChange?.(filteredEvents);
  }, [filteredEvents, onFilteredEventsChange]);

  const toggleOption = useCallback((option: string) => {
    setFilterState(prev => ({
      ...prev,
      selectedOptions: {
        ...prev.selectedOptions,
        [option]: !prev.selectedOptions[option]
      }
    }));
  }, []);

  const setPeriodRange = useCallback((min: number, max: number) => {
    setFilterState(prev => ({
      ...prev,
      periodRange: { min, max }
    }));
  }, []);

  const setSelectedPeriod = useCallback((periodLabel: string | null) => {
    setFilterState(prev => ({
      ...prev,
      selectedPeriod: periodLabel
    }));
  }, []);

  const resetFilters = useCallback(() => {
    setFilterState({
      selectedOptions: {},
      periodRange: { min: 862, max: 2026 },
      selectedPeriod: null
    });
  }, []);

  const setFilterStateDirect = useCallback((nextState: FilterState) => {
    setFilterState(nextState);
  }, []);

  return {
    filterState,
    filteredEvents,
    toggleOption,
    setPeriodRange,
    setSelectedPeriod,
    resetFilters,
    setFilterState: setFilterStateDirect
  };
};