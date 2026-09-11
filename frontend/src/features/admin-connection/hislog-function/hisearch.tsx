import React from 'react'

interface HisSearchProps {
  value: string
  onChange: (value: string) => void
}

export const HisSearch: React.FC<HisSearchProps> = ({ value, onChange }) => (
  <div className="hislog__search-wrap">
    <svg width="16" height="16" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="9" cy="9" r="6.5" stroke="#888" strokeWidth="1.5" />
      <path d="M14 14L18 18" stroke="#888" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
    <input
      className="hislog__search"
      type="text"
      placeholder="Поиск по событию, действию или модератору"
      value={value}
      onChange={e => onChange(e.target.value)}
    />
  </div>
)
