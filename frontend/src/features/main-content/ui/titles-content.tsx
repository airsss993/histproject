import React, { useEffect } from 'react'
import classNames from 'classnames'
import { useDispatch, useSelector } from 'react-redux'
import { MainContent } from '../index'

export const TitlesContentComponent: React.FC = () => {
  const dispatch = useDispatch()
  const titleH1Text = useSelector(MainContent.selectors.selectH1);
  const titleH2Text = useSelector(MainContent.selectors.selectH2);
  const titleH3Text = useSelector(MainContent.selectors.selectH3);
  const needAnimated = useSelector(MainContent.selectors.selectNeedAnimated);

  useEffect(() => {
    console.log('call 1');
    
    return () => {
      console.log('call');
      
      dispatch(MainContent.actions.disableAnimated())
    }
  }, [])
  return (
    <div className="RussianText">
      <div className={classNames('RussianFons', { animated: needAnimated })}>
        {titleH1Text}
      </div>
      <div
        style={{ animationDelay: `${titleH2Text.length * 0.6}s` }}
      >
        {titleH2Text}
      </div>
      <div
        style={{ animationDelay: `${titleH2Text.length * 1.2}s` }}
      >
        {titleH3Text}
      </div>
    </div>
  );
};