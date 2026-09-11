import React, { useState, useEffect } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { BuToSuc } from '../../../entities/butosuc'
import { SuccessModal } from './success'
import { actions } from '../slice'
import { selectEmail,
  selectTelegram,
  selectEventName,
  selectIsSubmitting,
  selectSubmitDone,
  selectError,
  selectIsSuccessOpen,
  selectIsContactFormValid } from '../selectors'
import './modal.scss'

interface ContactModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess?: () => void
}

export const ContactModal: React.FC<ContactModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const dispatch = useDispatch()
  const email = useSelector(selectEmail)
  const telegram = useSelector(selectTelegram)
  const eventName = useSelector(selectEventName)
  const isSubmitting = useSelector(selectIsSubmitting)
  const submitDone = useSelector(selectSubmitDone)
  const error = useSelector(selectError)
  const isSuccessOpen = useSelector(selectIsSuccessOpen)
  const isFormValid = useSelector(selectIsContactFormValid)

  const [isClosing, setIsClosing] = useState(false)
  const [isSubmitClosing, setIsSubmitClosing] = useState(false)

  const handleClose = () => {
    setIsClosing(true)
    setTimeout(() => {
      setIsClosing(false)
      onClose()
    }, 300)
  }

  useEffect(() => {
    if (!submitDone) return
    setIsSubmitClosing(true)
    const t = setTimeout(() => {
      setIsSubmitClosing(false)
      onClose()
      dispatch(actions.openSuccessModal())
      dispatch(actions.resetSubmitDone())
      if (onSuccess) onSuccess()
    }, 300)
    return () => clearTimeout(t)
  }, [submitDone])

  useEffect(() => {
    if (error) alert(error)
  }, [error])

  const handleSend = () => {
    if (isSubmitting) return
    dispatch(actions.submitRequest())
  }

  return (
    <>
      <div
        className={`modal-overlay ${isOpen && !isClosing && !isSubmitClosing ? 'open' : ''} ${isClosing ? 'close' : ''} ${isSubmitClosing ? 'submit-closing' : ''}`}
        style={{ display: isOpen || isClosing || isSubmitClosing ? 'flex' : 'none' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="modal-content success-modal" onClick={(e) => e.stopPropagation()}>
          <button className="modal-close" onClick={handleClose}>×</button>
          <h2 className="modal-title">Контактные данные</h2>
          <p className="modal-description">
            {eventName ? (
              <>Заполните контактные данные, мы проверим корректность материалов, после модерации вы получите обратную связь</>
            ) : (
              <>Спасибо за ваше предложение! Мы рассмотрим его <br />в ближайшее время и свяжемся с вами.</>
            )}
          </p>
          <div className="form-group" id="GEMail">
            <label htmlFor="email" className="GEMail">Email</label>
            <input
              type="text"
              id="email"
              name="email"
              placeholder="имя@mail.com"
              value={email}
              onChange={(e) => dispatch(actions.setEmail(e.target.value))}
              required
            />
          </div>
          <div className="form-group" id="TG">
            <label htmlFor="tg" className="TG">Telegram</label>
            <input
              type="text"
              id="tg"
              name="tg"
              placeholder="@имя пользователя"
              value={telegram}
              onChange={(e) => dispatch(actions.setTelegram(e.target.value))}
              required
            />
          </div>
          <div className="success-actions">
            <BuToSuc
              isFormValid={isFormValid}
              isSubmitting={isSubmitting}
              label="Отправить заявку"
              loadingLabel="Отправка..."
              onClick={handleSend}
            />
          </div>
        </div>
      </div>
      <SuccessModal
        isOpen={isSuccessOpen}
        onClose={() => dispatch(actions.closeSuccessModal())}
        onReset={() => {
          dispatch(actions.resetContactForm())
          dispatch(actions.resetMainForm())
        }}
      />
    </>
  )
}