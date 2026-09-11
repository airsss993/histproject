import React from 'react'
import { Butomap } from '../../../entities/butomap'

export const MainContentComponent: React.FC = () => {
  return (
    <div className="RussianFon">
      <div className="RussianText">
        <h4>
          Россия — 
            <br /> 
              страна героев
        </h4>
        <h5>
          Интерактивная карта исторических
            <br />
              событий
        </h5>
        <h6>
          Исследуйте историю. Сохраняйте память. Добавляйте события на общую карту подвигов.
        </h6>
      </div>
      <Butomap />
    </div>
  );
};