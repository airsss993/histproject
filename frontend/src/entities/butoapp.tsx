import React from 'react'
import { Link } from 'react-router-dom' // @ts-ignore
import RussianHeroes from '../assets/RussianHeroesLogo.png' // ../../public/RussianHeroesLogo.png

export const Butoapp: React.FC = () => {
  return (
    <Link to="/app">
      <img src={RussianHeroes} alt="Логотип Россия - страна Героев" />
    </Link>
  )
}