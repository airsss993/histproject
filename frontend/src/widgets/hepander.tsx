import React from 'react'
import { Butoapp } from '../entities/butoapp'
import { Buterm } from '../entities/buterm'

export const Hepander: React.FC = () => { // BuToApp, BuTerm
  return (
    <div className="Hepander">
      <Butoapp />
      <Buterm />
    </div>
  );
};