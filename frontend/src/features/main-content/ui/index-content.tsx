import React from 'react'
import { Butomap } from '../../../entities/butomap'
import { TitlesContentComponent } from './titles-content'

export const IndexContentComponent: React.FC = () => {
  return (
    <div className="RussianFon">
      <TitlesContentComponent />
      <Butomap />
    </div>
  );
};