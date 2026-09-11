import React, { createContext, useContext, useState, ReactNode } from 'react';
import { EventType } from '../../marker-location/evenPositions'

interface FilterContextType {
  selectedTypes: Set<EventType>;
  toggleType: (type: EventType) => void;
  isTypeSelected: (type: EventType) => boolean;
}

const EventFilterContext = createContext<FilterContextType | undefined>(undefined);

export const EventFilterProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [selectedTypes, setSelectedTypes] = useState<Set<EventType>>(// @ts-ignore
    new Set([EventType.Battle, EventType.Tragedy])
  );

  const toggleType = (type: EventType) => {
    setSelectedTypes(prev => {
      const newSet = new Set(prev);
      if (newSet.has(type)) {
        newSet.delete(type);
      } else {
        newSet.add(type);
      }
      return newSet;
    });
  };

  const isTypeSelected = (type: EventType) => selectedTypes.has(type);

  return (
    <EventFilterContext.Provider value={{ selectedTypes, toggleType, isTypeSelected }}>
      {children}
    </EventFilterContext.Provider>
  );
};

export const useEventFilter = () => {
  const context = useContext(EventFilterContext);
  if (!context) {
    throw new Error('useEventFilter must be used within EventFilterProvider');
  }
  return context;
};
