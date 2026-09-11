import React from 'react'
import { NavLink } from 'react-router-dom'

export const ButoMods: React.FC = () => {
  return (
    <NavLink to="/mod">
      <button>
        Модераторы
      </button>
    </NavLink>
  )
}