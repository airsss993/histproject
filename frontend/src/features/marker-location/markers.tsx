import React from 'react'
import { EventMarker } from './eventmarkers'
import { useEventFilterContext } from '../filter-function/ui/evenFilterProvider/evenFilterProvider'
// import { handleMarkerClick } from './eventmarkers'
import L from 'leaflet' // @ts-ignore
import './marker.scss'

interface MarkerWithPopupProps {
  onMarkerOpen: (event: any, position: L.LatLng, markerKey: string) => void;
  activeMarkerKey: string | null;
}

export const MarkerWithPopup: React.FC<MarkerWithPopupProps> = ({
  onMarkerOpen,
  activeMarkerKey,
}) => {
  const { filteredEvents, isLoading, error } = useEventFilterContext()

  if (isLoading || error || filteredEvents.length === 0) return null

  return (
    <>
      {filteredEvents.map((event, index) => {
        const markerKey = `${event.id}-${event.latitude}-${event.longitude}-${index}`

        return (// @ts-ignore
          <EventMarker
            key={markerKey}
            event={event}
            markerKey={markerKey}
            isActive={activeMarkerKey === markerKey}
            onOpen={(openedMarkerKey, event, position) =>
              onMarkerOpen(event, position, openedMarkerKey)
            }
          />
        )
      })}
    </>
  )
}