import React from 'react'
import { NavLink } from 'react-router-dom'

export const ButoHic: React.FC = () => {
  return (
    <NavLink to="/his">
      <button>
        История изменений
      </button>
    </NavLink>
  )
}