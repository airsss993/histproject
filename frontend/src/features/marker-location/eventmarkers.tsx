import React, { useRef, useEffect, useMemo } from 'react'
import { Marker, Popup } from 'react-leaflet'
import L from 'leaflet'
import { useDispatch } from 'react-redux'
import { EventObject } from './evenPositions' // @ts-ignore
import MarkerPolitTarget from '../../assets/MarkerPolitTarget.png' // @ts-ignore
import MarkerSwordTarget from '../../assets/MarkerSwordTarget.png' // @ts-ignore
import iconShadow from '../../assets/marker-shadow.png'
import { FETCH_CARD_DATA } from '../../app/saga/saga'
// import { EventsDataService } from '../card-information/ui/reurlcard'

type EventMarkerProps = {
  event: EventObject
  markerKey: string
  isActive: boolean // @ts-ignore
  data: EventsDataService
  onOpen: (markerKey: string, event: EventObject, position: L.LatLng) => void
}

const popupTimeOut = 100
const popupFadeDuration = 300

export const battleIcon = L.icon({
  iconUrl: MarkerPolitTarget,
  shadowUrl: iconShadow,
  iconSize: [32, 48],
  iconAnchor: [16, 32],
  popupAnchor: [0, 4],
})

export const tragedyIcon = L.icon({
  iconUrl: MarkerSwordTarget,
  shadowUrl: iconShadow,
  iconSize: [32, 48],
  iconAnchor: [16, 32],
  popupAnchor: [0, 4],
})

const iconMapping: Record<number, L.Icon> = {
  1: battleIcon,
  2: tragedyIcon,
  3: battleIcon,
  4: battleIcon,
  5: battleIcon,
}

const getIconByEventType = (eventType: number): L.Icon => {
  return iconMapping[eventType] || tragedyIcon
}

export const EventMarker: React.FC<EventMarkerProps> = ({
  event,
  markerKey,
  isActive,
  onOpen,
}) => {
  const closeTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const fadeCloseTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const markerRef = useRef<L.Marker | null>(null)
  const isPinned = useRef(false)
  const lat = parseFloat(event.latitude)
  const lng = parseFloat(event.longitude)
  const position = !isNaN(lat) && !isNaN(lng) ? ([lat, lng] as [number, number]) : null
  const markerLatLng = useMemo(() => (position ? L.latLng(position[0], position[1]) : null), [position])
  const dispatch = useDispatch()

  if (!position || !markerLatLng) return null

  const formatDate = (dateString: string) => {
    try {
      return new Date(dateString).toLocaleDateString('ru-RU', { 
        year: 'numeric', 
        month: 'long', 
        day: 'numeric' 
      })
    } catch {
      return dateString
    }
  }

  useEffect(() => {
    if (!isActive) {
      isPinned.current = false
      markerRef.current?.closePopup()
    }
  }, [isActive])

  useEffect(() => {
  const el = markerRef.current?.getElement()
  if (!el) return
  if (isActive) {
    el.classList.add('clicked')
  } else {
    el.classList.remove('clicked')
  }
}, [isActive])


  const clearPopupCloseTimers = () => {
    if (closeTimeoutRef.current) {
      clearTimeout(closeTimeoutRef.current);
      closeTimeoutRef.current = null;
    }
    if (fadeCloseTimeoutRef.current) {
      clearTimeout(fadeCloseTimeoutRef.current);
      fadeCloseTimeoutRef.current = null;
    }
  };

  const removeFadeClass = (marker: L.Marker) => {
    const popupElement = marker.getPopup()?.getElement();
    if (popupElement) {
      popupElement.classList.remove('event-marker-popup-fade-out');
    }
  };

  const closePopupWithFade = (marker: L.Marker) => {
    const popupElement = marker.getPopup()?.getElement();
    if (!popupElement) {
      marker.closePopup();
      return;
    }

    if (fadeCloseTimeoutRef.current) {
      clearTimeout(fadeCloseTimeoutRef.current);
    }

    popupElement.classList.add('event-marker-popup-fade-out');
    fadeCloseTimeoutRef.current = setTimeout(() => {
      marker.closePopup();
      popupElement.classList.remove('event-marker-popup-fade-out');
      fadeCloseTimeoutRef.current = null;
    }, popupFadeDuration);
  };

  const eventHandlers = {
    mouseover: () => {
      if (markerRef.current) {
        clearPopupCloseTimers();
        removeFadeClass(markerRef.current);
        markerRef.current.openPopup();
      }
    },
    mouseout: () => {
      if (isPinned.current) return

      if (closeTimeoutRef.current) {
        clearTimeout(closeTimeoutRef.current);
      }

      closeTimeoutRef.current = setTimeout(() => {
        if (markerRef.current) {
          closePopupWithFade(markerRef.current);
        }
      }, popupTimeOut);
    },
    click: () => {
      if (markerRef.current && markerLatLng) {
        clearPopupCloseTimers();
        isPinned.current = true
        markerRef.current.openPopup();
        console.log(`Объекты события: {
          Айдшник события: ${event.id},
          Тип события: ${event.eventType},
          Координаты: (Ш: ${event.latitude}, Д: ${event.longitude}),
          Портрет: ${event.previewUrlImage},
          Заголовок: "${event.title}",
          Дата события: "${event.eventDate}",
          Описание события: "${event.description}",
          Сайт: ${event.siteUrl || 'не обнаружен'}
        }`);
        markerRef.current.getElement()?.classList.add('clicked')
        // Вызываем onOpen и диспатчим запрос
        onOpen(markerKey, event, markerLatLng);
        dispatch({ type: FETCH_CARD_DATA, payload: event.id });
      }
    },
  };

  return (
    <>
      <Marker
        position={position}
        icon={getIconByEventType(event.eventType)}
        ref={markerRef}
        eventHandlers={eventHandlers}
      >
        <Popup className="event-marker-popup">
          <div
            onMouseEnter={() => {
              if (markerRef.current) {
                clearPopupCloseTimers()
                removeFadeClass(markerRef.current)
              }
            }}
          >
            <h3 style={{ margin: '0 0 6px 0', color: '#FFFFFF' }}>{event.title}</h3>
            <small style={{ color: '#FFFFFF', display: 'block', fontSize: '14px', marginTop: '0.2em' }}>
              🗓️ {formatDate(event.eventDate)}
            </small>
          </div>
        </Popup>
      </Marker>
    </>
  )
}