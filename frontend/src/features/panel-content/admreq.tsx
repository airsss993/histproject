import React, { useState } from 'react'
import { RequestModalContent } from '../admin-connection/admin-function/reqmodal'
import { ApproveModal } from '../admin-connection/admin-function/apprej'
import { rejectRequest, reviewRequest } from '../admin-connection/moder-function/modreques'
import { deleteObject } from '../admin-connection/admin-function/cordel'

export interface Request {
  id: string
  title: string
  status: string
  createdAt?: string
  date: string
  description: string
  eventDate: string
  eventType: string
  telegram: string
  email: string
}

interface RequestModalProps {
  request: Request
  onClose: () => void
}

export const RequestModal: React.FC<RequestModalProps> = ({ request, onClose }) => {
  const [isClosing, setIsClosing] = useState(false)
  const [isVerified, setIsVerified] = useState(false)
  const [showRejectConfirm, setShowRejectConfirm] = useState(false)
  const [rejectComment, setRejectComment] = useState('')
  const [showApproveModal, setShowApproveModal] = useState(false)
  const [isRequestFading, setIsRequestFading] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [isDeleteClosing, setIsDeleteClosing] = useState(false)

  const handleClose = () => {
    setIsClosing(true)
    setTimeout(onClose, 300)
  }

  const handleDeleteOpen = () => { setIsDeleteClosing(false); setShowDeleteConfirm(true) }
  const handleDeleteClose = () => {
    setIsDeleteClosing(true)
    setTimeout(() => { setShowDeleteConfirm(false); setIsDeleteClosing(false) }, 250)
  }
  const handleDeleteConfirm = async () => {
    setIsDeleteClosing(true)
    setTimeout(async () => {
      setShowDeleteConfirm(false)
      setIsDeleteClosing(false)
      await handleRejectConfirm()
    }, 250)
  }

  const handleApproveOpen = () => {
    setIsRequestFading(true)
    setShowApproveModal(true)
  }

  const handleApproveClose = () => {
    setShowApproveModal(false)
    setIsRequestFading(false)
  }

  const handleApproveConfirm = () => {
    setShowApproveModal(false)
    setIsRequestFading(false)
    handleClose()
  }

  const handleVerify = async () => {
    try {
      await reviewRequest(request.id)
    } catch (err: any) {
      console.error('Ошибка взятия в работу:', err?.message)
    }
    setIsVerified(true)
  }

  const PUBLISHED_STATUSES = new Set(['published', 'Опубликовано', 'Опубликована'])

  const handleRejectConfirm = async () => {
    try {
      if (PUBLISHED_STATUSES.has(request.status)) {
        await deleteObject(Number(request.id))
      } else {
        await rejectRequest(request.id, rejectComment)
      }
    } catch (err: any) {
      console.error('Ошибка:', err?.message)
    }
    handleClose()
  }

  return (
    <>
      <div className={`request-modal__overlay${isClosing ? ' request-modal__overlay--closing' : ''}`}>
        <div
          className={`request-modal${isClosing ? ' request-modal--closing' : ''}${isRequestFading ? ' request-modal--fading-for-approve' : ''}`}
          onClick={(e) => e.stopPropagation()}
        >
          <button className="request-modal__close" onClick={handleClose}>
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M15 5L5 15M5 5L15 15" stroke="#1a1a1a" strokeWidth="1.8" strokeLinecap="round"/>
            </svg>
          </button>

          <RequestModalContent // @ts-ignore
            request={request}
            isVerified={isVerified}
            showRejectConfirm={showRejectConfirm}
            rejectComment={rejectComment}
            onCommentChange={setRejectComment}
            onVerify={handleVerify}
            onReject={() => setShowRejectConfirm(true)}
            onRejectConfirm={handleRejectConfirm}
            onApprove={handleApproveOpen}
            onDeleteClick={handleDeleteOpen}
          />

          {showDeleteConfirm && (
            <div className={`delete-confirm${isDeleteClosing ? ' delete-confirm--closing' : ''}`}>
              <p className="delete-confirm__text">Вы уверены что хотите удалить?</p>
              <div className="delete-confirm__actions">
                <button className="delete-confirm__no" onClick={handleDeleteClose}>Нет</button>
                <button className="delete-confirm__yes" onClick={handleDeleteConfirm}>Да</button>
              </div>
            </div>
          )}
        </div>
      </div>

      {showApproveModal && (
        <ApproveModal // @ts-ignore
          request={request}
          onClose={handleApproveClose}
          onConfirm={handleApproveConfirm}
        />
      )}
    </>
  )
}
