import React from 'react'

interface SuccessButtonProps {
  isFormValid: boolean
  isSubmitting?: boolean
  label?: string
  loadingLabel?: string
  onClick?: () => void
}

export const BuToSuc: React.FC<SuccessButtonProps> = ({
  isFormValid,
  isSubmitting = false,
  label = 'Далее',
  loadingLabel = 'Отправка...',
  onClick
}) => {
  return (
    <button
      type="submit"
      className={`submit-btn final-submit-btn ${isFormValid ? 'with-background' : ''}`}
      disabled={!isFormValid || isSubmitting}
      onClick={onClick}
    >
      {isSubmitting ? loadingLabel : label}
    </button>
  )
}