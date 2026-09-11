import React from 'react'

interface SubmitButtonProps {
  isFormValid: boolean
  label?: string
  onClick?: () => void
}

export const BuToCon: React.FC<SubmitButtonProps> = ({ 
  isFormValid, 
  label = 'Далее', 
  onClick 
}) => {
  return (
    <button
      type="submit"
      className={`submit-btn final-submit-btn ${isFormValid ? 'with-background' : ''}`}
      disabled={!isFormValid}
      onClick={onClick}
    >
      {label}
    </button>
  )
}