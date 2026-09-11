import React from 'react'
import { Butoapp } from '../entities/butoapp'
import { Buterm } from '../entities/buterm'
import { Butoform } from '../entities/butoform'

export interface HeaderProps {
  onOpenModal: () => void
}

export const Header: React.FC<HeaderProps> = ({ onOpenModal }) => {
  return (
    <header>
      <Butoapp />
      <Buterm />
      <Butoform onOpenModal={onOpenModal} />
    </header>
  );
};