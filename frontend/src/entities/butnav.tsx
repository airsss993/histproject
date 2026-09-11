import React from 'react'
import { ButReq } from '../entities/butreq'
import { ButoHic } from '../entities/butohic'
import { ButoMods } from '../entities/butomods'

export const ButNav: React.FC = () => {
  return (
    <div className="butnav">
      <ButReq />
      <ButoHic />
      <ButoMods />
    </div>
  );
};