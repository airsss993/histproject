import React, { useState } from 'react'
import { FilterDropdown } from './dropdown' // @ts-ignore
import './filter.scss'

export const FilterButtonList: React.FC = () => {
  const [isOpen, setIsOpen] = useState(false)

  return (
    <div className="FilterContainer">
      <div className={`FilterList ${isOpen ? 'clicked' : ''}`}>
        <button 
          onClick={() => setIsOpen(!isOpen)}
          className={isOpen ? 'clicked' : ''}
        >
          Фильтры
        </button>
        <FilterDropdown 
          isOpen={isOpen}
          onClose={() => setIsOpen(false)} label={''} 
        />
      </div>
    </div>
  )
}
