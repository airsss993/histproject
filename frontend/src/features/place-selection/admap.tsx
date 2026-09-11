import React, { useRef } from 'react'
import { MapContainer, TileLayer } from 'react-leaflet'
import L from 'leaflet'
import { tiLayer } from '../../entities/cons.ts'
import { LayerLabels } from './layer.tsx'
import { CitiesLabels } from './cities.tsx'
import { ReMarker } from './remarker.tsx'
import { EventFilterProvider } from '../filter-function/ui/evenFilterProvider/evenFilterProvider.tsx' // @ts-ignore
import '../map-content/ui/map.scss' // @ts-ignore
import './admap.scss'

const centmap: [number, number] = [68.751244, 8.618423]
const zoom = 2
const minZoom = 2
const maxZoom = 8
const maxMapBounds: [number, number][] = [[-112, -169], [84, 192]]

interface AdMapProps {
  eventType?: number
  onPositionChange?: (pos: L.LatLng | null) => void
}

export const AdMap: React.FC<AdMapProps> = ({ eventType = 1, onPositionChange }) => {
  const mapRef = useRef<L.Map | null>(null)
  // https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png
  return (
    <EventFilterProvider>
      <div id="admap-wrapper">
        <MapContainer
          id="admap"
          center={centmap}
          zoom={zoom}
          minZoom={minZoom}
          maxZoom={maxZoom}
          scrollWheelZoom={true}
          maxBounds={maxMapBounds}
          maxBoundsViscosity={1.0}
          ref={mapRef}
        >
          <TileLayer url={tiLayer} noWrap={false} opacity={1} maxNativeZoom={6} />
          <LayerLabels />
          <CitiesLabels />
          <ReMarker eventType={eventType} onPositionChange={onPositionChange} />
        </MapContainer>
      </div>
    </EventFilterProvider>
  )
}