import React from 'react'

interface BuTGProps {
  label?: string
  onClick?: () => void
}

export const BuTG: React.FC<BuTGProps> = ({ 
  label = 'Привязать телеграм', 
  onClick 
}) => {
  return (
    <button 
      className="submit-btn-active" 
      onClick={onClick}
    >
      {label}
    </button>
  )
}