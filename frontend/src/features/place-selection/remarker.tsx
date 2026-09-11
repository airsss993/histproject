import React, { useState } from 'react'
import { Marker, Popup, useMapEvents } from 'react-leaflet'
import L from 'leaflet'
import { battleIcon, tragedyIcon } from '../marker-location/eventmarkers'  // @ts-ignore
import '../marker-location/marker.scss'

const iconMapping: Record<number, L.Icon> = {
  1: tragedyIcon,
  2: tragedyIcon,
  3: battleIcon,
  4: battleIcon,
  5: battleIcon,
}

const MapClickHandler: React.FC<{
  onMapClick: (latlng: L.LatLng) => void
}> = ({ onMapClick }) => {
  useMapEvents({
    click(e) {
      onMapClick(e.latlng)
    },
  })
  return null
}

interface ReMarkerProps {
  eventType?: number
  onPositionChange?: (pos: L.LatLng | null) => void
}

export const ReMarker: React.FC<ReMarkerProps> = ({ eventType = 1, onPositionChange }) => {
  const [position, setPosition] = useState<L.LatLng | null>(null)

  const icon = iconMapping[eventType] ?? battleIcon

  const handleSetPosition = (latlng: L.LatLng) => {
    setPosition(latlng)
    onPositionChange?.(latlng)
  }

  return (
    <>
      <MapClickHandler onMapClick={handleSetPosition} />
      {position && (
        <Marker position={position} icon={icon}>
          <Popup className="event-marker-popup">
            <div style={{ fontFamily: 'Cruinn, sans-serif', color: '#fff' }}>
              <div style={{ marginBottom: 6, fontWeight: 600 }}>
                Маркер
              </div>
              <div style={{ fontSize: 13, opacity: 0.85 }}>
                Ш: {position.lat.toFixed(4)}
              </div>
              <div style={{ fontSize: 13, opacity: 0.85 }}>
                Д: {position.lng.toFixed(4)}
              </div>
              <button
                onClick={() => { setPosition(null); onPositionChange?.(null) }}
                style={{
                  marginTop: 10,
                  padding: '4px 10px',
                  background: '#c0392b',
                  color: '#fff',
                  border: 'none',
                  borderRadius: 4,
                  cursor: 'pointer',
                  fontSize: 12,
                  fontFamily: 'Cruinn, sans-serif',
                }}
              >
                Удалить
              </button>
            </div>
          </Popup>
        </Marker>
      )}
    </>
  )
}
