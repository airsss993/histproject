import React from 'react'
import { Link } from 'react-router-dom'

export const Butomap: React.FC = () => {
  return(
    <Link to="/map">
      <button>
        Перейти к карте →
      </button>
    </Link>
  )
}