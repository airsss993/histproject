import React from 'react'
import { NavLink } from 'react-router-dom'

export const ButReq: React.FC = () => {
  return (
    <NavLink to="/adm">
      <button>
        Заявки
      </button>
    </NavLink>
  )
}