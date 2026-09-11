import React, { useState } from 'react'

interface RejectConfirmModalProps {
  comment: string
  onCommentChange: (comment: string) => void
  onConfirm: () => void
  onClose: () => void
}

export const RejectConfirmModal: React.FC<RejectConfirmModalProps> = ({
  comment,
  onCommentChange,
  onConfirm,
  onClose,
}) => {
  const [isClosing, setIsClosing] = useState(false)

  const handleConfirm = () => {
    setIsClosing(true)
    setTimeout(() => {
      onConfirm()
    }, 250)
  }

  const handleClose = () => {
    setIsClosing(true)
    setTimeout(onClose, 250)
  }

  return (
    <div className={`reject-modal__overlay${isClosing ? ' reject-modal__overlay--closing' : ''}`}>
      <div className={`reject-modal${isClosing ? ' reject-modal--closing' : ''}`} onClick={(e) => e.stopPropagation()}>
        <button className="reject-modal__close" onClick={handleClose}>
          <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M15 5L5 15M5 5L15 15" stroke="#1a1a1a" strokeWidth="1.8" strokeLinecap="round"/>
          </svg>
        </button>
        <h3 className="reject-modal__title">Отклонить заявку</h3>
        <textarea
          className="reject-modal__comment"
          placeholder="Введите комментарий"
          value={comment}
          onChange={(e) => onCommentChange(e.target.value)}
          required
        />
        <button className="reject-modal__confirm" onClick={handleConfirm}>
          Подтвердить
        </button>
      </div>
    </div>
  )
}