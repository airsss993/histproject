import React from 'react'
import { Butoapp } from '../entities/butoapp'
import { Buterm } from '../entities/buterm'
import { ButNav } from '../entities/butnav'
import { ButoAcc } from '../entities/butacc'

export const Headmer: React.FC = () => {
  return (
    <div className="Headmer">
      <Butoapp />
      <Buterm />
      <ButNav />
      <ButoAcc />
    </div>
  );
};