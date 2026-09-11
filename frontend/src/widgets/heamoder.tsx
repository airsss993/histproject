import React from 'react'
import { Butoapp } from '../entities/butoapp'
import { Buterm } from '../entities/buterm'
import { ButoAcc } from '../entities/butacc'

export const Heamoder: React.FC = () => {
  return (
    <div className="Heamoder">
      <Butoapp />
      <Buterm />
      <ButoAcc />
    </div>
  );
};