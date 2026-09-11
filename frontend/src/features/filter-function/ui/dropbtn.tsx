import React from 'react'

interface SubDropdownBtnProps {
  label: string;
  isSelected: boolean;
  isInRange: boolean;
  onClick: () => void;
  onCheckboxChange: () => void;
  // disabled?: boolean;
}

export const SubDropdownBtn: React.FC<SubDropdownBtnProps> = ({
  label,
  isSelected,
  isInRange,
  onClick,
  onCheckboxChange,
}) => {
  const handleClick = () => {
    onClick()
  }

  const handleCheckboxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    e.stopPropagation()
    onCheckboxChange()
  }

  const handleCheckboxClick = (e: React.MouseEvent) => {
    e.stopPropagation()
  }

  const buttonClasses = [
    'SubDropdownBtn',
    isSelected ? 'selected' : '',
    !isInRange ? 'out-of-range' : ''
  ].filter(Boolean).join(' ')

  return (
    <div
      className={buttonClasses}
      onClick={handleClick}
    >
      <span>{label}</span>
      <input 
        type="checkbox" 
        checked={isSelected}
        onChange={handleCheckboxChange}
        onClick={handleCheckboxClick}
        // disabled={!isInRange} // НЕ блокируем чекбокс
      />
    </div>
  )
}