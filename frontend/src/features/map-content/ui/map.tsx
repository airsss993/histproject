import React from 'react'
import { EventFilterProvider } from '../../filter-function/ui/evenFilterProvider/evenFilterProvider.tsx'
import { MapContent } from './map-content.tsx'
import { MapProvider } from '../providecons.tsx'
import './map.scss'

export const Map: React.FC = () => {
  return (
    <EventFilterProvider>
      <MapProvider>
        <MapContent />
      </MapProvider>
    </EventFilterProvider>
  )
}
