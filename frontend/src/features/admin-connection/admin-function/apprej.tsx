import React, { useState, useRef, useEffect } from 'react'
import L from 'leaflet'
import { Request } from './reques'
import { AdMap } from '../../place-selection/admap'
import { submitMarkerReview } from '../../place-selection/requiew'
import { updateObjectCoordinates } from './corup'
import { formatEventDate, formatFetchDate } from './reqmodal'
import { statusClass } from './reqard'

interface ApproveModalProps { // @ts-ignore
  request: Request
  onClose: () => void
  onConfirm: () => void
}

const EVENT_TYPES = ['Битва', 'Война', 'Революция', 'Восстания', 'Переворот']

const EVENT_TYPE_MAP: Record<string, number> = {
  'Битва': 1,
  'Война': 2,
  'Революция': 3,
  'Восстания': 4,
  'Переворот': 5,
}

const EVENT_TYPE_ID_MAP: Record<number, string> = {
  1: 'Битва',
  2: 'Война',
  3: 'Революция',
  4: 'Восстания',
  5: 'Переворот',
}

const STATUS_LABEL: Record<string, string> = {
  published:       'Опубликовано',
  review:          'На проверке',
  rejected:        'Отклонено',
  new:             'Новая',
  'Опубликовано':  'Опубликовано',
  'Опубликована':  'Опубликовано',
  'Отклонено':     'Отклонено',
  'Отклонена':     'Отклонено',
  'На проверке':   'На проверке',
  'Новая':         'Новая',
}

export const ApproveModal: React.FC<ApproveModalProps> = ({ request, onClose, onConfirm }) => {
  const [isClosing, setIsClosing] = useState(false)
  const [showEventTypeDropdown, setShowEventTypeDropdown] = useState(false)
  const [selectedEventType, setSelectedEventType] = useState(
    EVENT_TYPE_ID_MAP[Number(request.eventTypeId)] ?? request.eventTypeId
  )
  const [markerPos, setMarkerPos] = useState<L.LatLng | null>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!showEventTypeDropdown) return
    const handleOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowEventTypeDropdown(false)
      }
    }
    document.addEventListener('mousedown', handleOutside)
    return () => document.removeEventListener('mousedown', handleOutside)
  }, [showEventTypeDropdown])

  const PUBLISHED_STATUSES = new Set(['published', 'Опубликовано', 'Опубликована'])

  const handleConfirm = async () => {
    if (!markerPos) return
    try {
      if (PUBLISHED_STATUSES.has(request.status)) {
        await updateObjectCoordinates({
          id: Number(request.id),
          latitude: markerPos.lat,
          longitude: markerPos.lng,
        })
      } else {
        await submitMarkerReview(
          Number(request.id),
          markerPos.lat,
          markerPos.lng,
        )
      }
    } catch (err) {
      console.error(err)
    }
    setIsClosing(true)
    setTimeout(() => {
      onConfirm()
    }, 250)
  }

  const handleClose = () => {
    setIsClosing(true)
    setTimeout(onClose, 250)
  }

  const renderRequestFields = () => (
    <>
      <h3 className="request-modal__section-heading">Описание события</h3>
      <p className="request-modal__description">{request.description}</p>

      <div className="request-modal__grid">
        <div className="request-modal__field">
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3" y="4" width="14" height="13" rx="2" stroke="#555" strokeWidth="1.5"/>
            <path d="M3 8H17" stroke="#555" strokeWidth="1.5"/>
            <path d="M7 2V5M13 2V5" stroke="#555" strokeWidth="1.5" strokeLinecap="round"/>
          </svg>
          <span className="request-modal__label">Дата события</span>
          <span className="request-modal__value">{formatEventDate(request.eventDate)}</span>
        </div>
        
        <div className="request-modal__field request-modal__field--select" ref={dropdownRef}>
          <button
            className={`request-modal__chevron${showEventTypeDropdown ? ' request-modal__chevron--open' : ''}`}
            onClick={() => setShowEventTypeDropdown(v => !v)}
          >
            <svg width="26" height="26" viewBox="0 0 16 16" fill="none">
              <path d="M4 6L8 10L12 6" stroke="#555" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
          </button>
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M10 2C7.24 2 5 4.24 5 7C5 10.75 10 17 10 17C10 17 15 10.75 15 7C15 4.24 12.76 2 10 2Z" stroke="#555" strokeWidth="1.5"/>
            <circle cx="10" cy="7" r="2" stroke="#555" strokeWidth="1.5"/>
          </svg>
          <span className="request-modal__label">Тип события</span>
          <span className="request-modal__value">{selectedEventType}</span>
          {showEventTypeDropdown && (
            <div className="event-type-dropdown">
              {EVENT_TYPES.map(type => (
                <div
                  key={type}
                  className={`event-type-dropdown__item${selectedEventType === type ? ' event-type-dropdown__item--active' : ''}`}
                  onClick={() => { setSelectedEventType(type); setShowEventTypeDropdown(false) }}
                >
                  {type}
                </div>
              ))}
            </div>
          )}
        </div>
        
        <div className="request-modal__field">
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <circle cx="10" cy="7" r="3.5" stroke="#555" strokeWidth="1.5"/>
            <path d="M3 17C3 14 6.13 12 10 12C13.87 12 17 14 17 17" stroke="#555" strokeWidth="1.5" strokeLinecap="round"/>
          </svg>
          <span className="request-modal__label">Telegram</span>
          <span className="request-modal__value">{request.telegram}</span>
        </div>
        
        <div className="request-modal__field">
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="2" y="5" width="16" height="11" rx="2" stroke="#555" strokeWidth="1.5"/>
            <path d="M2 7L10 12L18 7" stroke="#555" strokeWidth="1.5" strokeLinecap="round"/>
          </svg>
          <span className="request-modal__label">Email</span>
          <span className="request-modal__value">{request.email}</span>
        </div>
      </div>

      <div className="request-modal__field request-modal__field--full">
        <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
          <circle cx="10" cy="10" r="7.5" stroke="#555" strokeWidth="1.5"/>
          <path d="M10 6V10.5L13 13" stroke="#555" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
        <span className="request-modal__label">Дата подачи заявки</span>
        <span className="request-modal__value">{formatFetchDate(request.date)}</span>
      </div>
    </>
  )

  return (
    <div className={`approve-modal__overlay${isClosing ? ' approve-modal__overlay--closing' : ''}`}>
      <div className={`approve-modal${isClosing ? ' approve-modal--closing' : ''}`} onClick={(e) => e.stopPropagation()}>
        <button className="approve-modal__close" onClick={handleClose}>
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M15 5L5 15M5 5L15 15" stroke="#1a1a1a" strokeWidth="1.8" strokeLinecap="round"/>
          </svg>
        </button>

        <div className="approve-modal__left">
          <h3 className="approve-modal__map-heading">Выбор места</h3>
          <AdMap eventType={EVENT_TYPE_MAP[selectedEventType] ?? 1} onPositionChange={setMarkerPos} />
        </div>

        <div className="approve-modal__right">
          <div className="request-modal__header-row">
            <h2 className="request-modal__title">{request.title}</h2>
            <span className={`request-modal__status-badge request-modal__status-badge--${statusClass[request.status] ?? request.status}`}>
              <span className="request-modal__status-dot" />
              {STATUS_LABEL[request.status] ?? request.status}
            </span>
          </div>
          <span className="request-modal__id">{request.id}</span>
          {renderRequestFields()}
            <button className="approve-modal__confirm" onClick={handleConfirm} disabled={!markerPos}>
              Подтвердить
            </button>
        </div>
      </div>
    </div>
  )
}