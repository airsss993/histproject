import React, { useCallback, useRef } from 'react'
import { MapContainer, TileLayer } from 'react-leaflet'
import L from 'leaflet'
import { tiLayer } from '../../../entities/cons.ts'
import { CityLabels } from '../../layer-position/city.tsx'
import { LayersLabels } from '../../layer-position/layers.tsx'
import { MarkerWithPopup } from '../../marker-location/markers.tsx'
import { FilterButtonList } from '../../filter-function/ui/filters.tsx'
import { CardOnMap } from '../../card-information/ui/cardPosition'
import { EventObject } from '../../marker-location/evenPositions.ts'
import { useMapContext } from '../providecons.tsx'

const centmap: [number, number] = [68.751244, 98.618423]
const zoom = 3
const minZoom = 3
const maxZoom = 12
const maxMapBounds: [number, number][] = [[-112, -169], [84, 192]]

export const MapContent: React.FC = () => {
  const mapRef = useRef<L.Map | null>(null)
  const { activeEvent, activeMarkerKey, dispatch } = useMapContext()

  const handleMarkerOpen = (event: EventObject, position: L.LatLng, markerKey: string) => {
    dispatch({ type: 'SET_ACTIVE_EVENT', payload: { event, position, markerKey } })
  }

  const handleCardClose = useCallback(() => {
    dispatch({ type: 'SET_ACTIVE_EVENT', payload: null })
  }, [dispatch])

  return (
    <div id="map-wrapper">
      <FilterButtonList />
      {activeEvent && (
        <CardOnMap
          isVisible={true}
          position={activeEvent.position}
          event={activeEvent.event}
          onClose={handleCardClose}
        />
      )}
      <MapContainer
        id="map"
        center={centmap}
        zoom={zoom}
        minZoom={minZoom}
        maxZoom={maxZoom}
        scrollWheelZoom={true}
        maxBounds={maxMapBounds}
        maxBoundsViscosity={1.0}
        ref={mapRef}
      >
        <TileLayer url={tiLayer} noWrap={false} opacity={0} maxNativeZoom={6} />
        <LayersLabels />
        <CityLabels />
        <MarkerWithPopup
          onMarkerOpen={handleMarkerOpen}
          activeMarkerKey={activeMarkerKey}
        />
      </MapContainer>
    </div>
  )
}