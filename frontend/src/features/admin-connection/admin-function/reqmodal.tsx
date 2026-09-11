import React from 'react'
import { Request } from './reques'
import { statusClass } from './reqard'

export const formatEventDate = (dateString: string): string => {
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString
  const parts = date.toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  }).replace(' г.', '').split(' ')
  if (parts.length === 3) {
    parts[1] = parts[1].charAt(0).toUpperCase() + parts[1].slice(1)
  }
  return parts.join(' ')
}

export const formatFetchDate = (dateString: string): string => {
  const date = new Date(dateString)
  if (isNaN(date.getTime())) return dateString
  const datePart = date.toLocaleDateString('ru-RU', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  }).replace(' г.', '')
  const timePart = date.toLocaleTimeString('ru-RU', {
    hour: '2-digit',
    minute: '2-digit',
  })
  return `${datePart}, ${timePart}`
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

const OPTION_TO_EVENT_TYPE_NAME: Record<string, number> = {
    'Битвы': 2, // 1
    'Войны': 5, // 2
    'Революции': 1, // 3
    'Восстания': 4,
    'Перевороты': 3 // 5
};

// const FALLBACK_NAME_TO_ID: Record<string, number> = {
//     'Расстрел/Расправа': 1,
//     'Военная операция': 2,
//     'Пограничный конфликт': 3,
//     'Восстание/Бунт': 4,
//     'Войны': 5
// };

const EVENT_TYPE_ID_TO_NAME: Record<number, string> = Object.fromEntries(
    Object.entries(OPTION_TO_EVENT_TYPE_NAME).map(([name, id]) => [id, name])
);

interface RequestModalContentProps {
  request: Request
  isVerified: boolean
  showRejectConfirm: boolean
  rejectComment: string
  onCommentChange: (value: string) => void
  onVerify: () => void
  onReject: () => void
  onRejectConfirm: () => void
  onApprove: () => void
  onDeleteClick: () => void
}

export const RequestModalContent: React.FC<RequestModalContentProps> = ({
  request,
  isVerified,
  showRejectConfirm,
  rejectComment,
  onCommentChange,
  onVerify,
  onReject,
  onRejectConfirm,
  onApprove,
  onDeleteClick,
}) => {
  const rawSiteUrl = request.siteUrl || null;

  const actualSiteUrl = rawSiteUrl
    ? rawSiteUrl.startsWith('http://') || rawSiteUrl.startsWith('https://')
      ? rawSiteUrl
      : `https://${rawSiteUrl}`
    : null;

  const handleLearnMoreClick = () => {
    if (actualSiteUrl) {
      window.open(actualSiteUrl, '_blank', 'noopener,noreferrer');
    }
  };

  const renderRequestFields = () => (
    <>
      <h3 className="request-modal__section-heading">Описание события</h3>
      <p className="request-modal__description">{request.description}</p>

      <div className="request-modal__grid">
        <div className="request-modal__field">
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3" y="4" width="14" height="13" rx="2" stroke="#555" strokeWidth="1.5" />
            <path d="M3 8H17" stroke="#555" strokeWidth="1.5" />
            <path d="M7 2V5M13 2V5" stroke="#555" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          <span className="request-modal__label">Дата события</span>
          <span className="request-modal__value">{formatEventDate(request.eventDate)}</span>
        </div>
        
        <div className="request-modal__field">
          <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M10 2C7.24 2 5 4.24 5 7C5 10.75 10 17 10 17C10 17 15 10.75 15 7C15 4.24 12.76 2 10 2Z" stroke="#555" strokeWidth="1.5"/>
            <circle cx="10" cy="7" r="2" stroke="#555" strokeWidth="1.5"/>
          </svg>
          <span className="request-modal__label">Тип события</span>
          <span className="request-modal__value">{EVENT_TYPE_ID_TO_NAME[Number(request.eventTypeId)] ?? request.eventTypeId}</span>
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

  const isNewStatus = statusClass[request.status] === 'new'

  const badgeClass = showRejectConfirm
    ? 'rejected'
    : (isVerified && isNewStatus)
      ? 'review'
      : (statusClass[request.status] ?? request.status)

  const badgeLabel = showRejectConfirm
    ? STATUS_LABEL['rejected']
    : (isVerified && isNewStatus)
      ? STATUS_LABEL['review']
      : (STATUS_LABEL[request.status] ?? request.status)

  return (
    <>
      <div className="request-modal__header-row">
        <h2 className="request-modal__title">{request.title}</h2>
          {(isVerified || showRejectConfirm || statusClass[request.status] === 'rejected' || statusClass[request.status] === 'published' || statusClass[request.status] === 'review') && (
            <span className={`request-modal__status-badge request-modal__status-badge--${badgeClass}`}>
              <span className="request-modal__status-dot" />
              {badgeLabel}
            </span>
          )}
      </div>
      <span className="request-modal__id">{request.id}</span>

      {renderRequestFields()}

      {statusClass[request.status] === 'rejected' && (
        <div className="request-modal__rejection-reason">
          <span className="request-modal__rejection-reason-label">
            Причина отклонения:&nbsp;
            <span className="request-modal__rejection-reason-text">
              {request.comment || request.adminComment || 'Не указана'}
            </span>
          </span>
        </div>
      )}

      {!isVerified && !showRejectConfirm && statusClass[request.status] !== 'rejected' && statusClass[request.status] !== 'published' && statusClass[request.status] !== 'review' && (
        <button className="request-modal__verify" onClick={onVerify}>Проверить</button>
      )}

      {statusClass[request.status] === 'published' && !showRejectConfirm && (
        <div className="request-modal__actions">
          <button className="request-modal__reject"
              onClick={onDeleteClick}>
            <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.6"/>
              <path d="M13 7L7 13M7 7L13 13" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"/>
            </svg>
            Удалить
          </button>
          <button className="request-modal__approve" onClick={onApprove}>
            <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.6"/>
              <path d="M6.5 10.5L9 13L13.5 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"/>
            </svg>
            Редактировать
          </button>
        </div>
      )}

      {(isVerified || statusClass[request.status] === 'review') && !showRejectConfirm && statusClass[request.status] !== 'published' && (
        <>
          {statusClass[request.status] !== 'published' && (
            <button className="request-modal__preview" onClick={handleLearnMoreClick}>
              Предпросмотр
            </button>
          )}
          <div className="request-modal__actions">
            <button className="request-modal__reject" onClick={onReject}>
              <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.6" />
                <path d="M13 7L7 13M7 7L13 13" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
              </svg>
              {statusClass[request.status] === 'published' ? 'Удалить' : 'Отклонить'}
            </button>
            <button className="request-modal__approve" onClick={onApprove}>
              <svg width="18" height="18" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.6" />
                <path d="M6.5 10.5L9 13L13.5 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              {statusClass[request.status] === 'published' ? 'Редактировать' : 'Одобрить'}
            </button>
          </div>
        </>
      )}

      {showRejectConfirm && (
        <>
          <textarea
            className="request-modal__comment"
            placeholder="Введите комментарий"
            value={rejectComment}
            onChange={(e) => onCommentChange(e.target.value)}
            required
          />
            <button 
              className="request-modal__confirm" 
              onClick={onRejectConfirm}
              disabled={!rejectComment.trim()}
            >
              Подтвердить
            </button>
        </>
      )}
    </>
  )
}