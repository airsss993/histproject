import React, { useState, useRef, useEffect } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import { useNavigate } from 'react-router-dom'
import { LogOutFeature } from '../features/logout'

export const ButoAcc: React.FC = () => {
  const dispatch = useDispatch();
  const navigate = useNavigate();
  
  const [open, setOpen] = useState(false);
  const [closing, setClosing] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const isOpenRef = useRef(false);
  const logoutCompleted = useSelector((state: any) => state.logout.logoutCompleted);

  useEffect(() => {
    if (logoutCompleted) {
      dispatch(LogOutFeature.actions.logout());
      dispatch(LogOutFeature.actions.logoutReset());
      navigate('/log');
    }
  }, [logoutCompleted, dispatch, navigate]);

  const handleLogout = () => {
    dispatch(LogOutFeature.actions.logoutRequest());
  };

  const closeDropdown = () => {
    if (!isOpenRef.current) return;
    isOpenRef.current = false;
    setClosing(true);
    setTimeout(() => {
      setOpen(false);
      setClosing(false);
    }, 300);
  };

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        closeDropdown();
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleToggle = () => {
    if (isOpenRef.current) {
      closeDropdown();
    } else {
      isOpenRef.current = true;
      setOpen(true);
    }
  };

  const username = localStorage.getItem('username') || 'UnAuthorized_AdModer';
  const parts = username.split('_');
  const firstInitial = parts[0]?.[0]?.toUpperCase() ?? 'A';
  const secondInitial = parts[1]?.[0]?.toUpperCase() ?? 'A';

  return (
    <div className="butacc-wrap" ref={ref}>
      <button className="butacc" onClick={handleToggle}>
        {firstInitial}{secondInitial}
      </button>
      {(open || closing) && (
        <div className={`butacc-dropdown${closing ? ' butacc-dropdown--closing' : ''}`}>
          <span className="butacc-dropdown__name">{username}</span>
          <button className="butacc-dropdown__logout" onClick={handleLogout}>
            Выйти
          </button>
        </div>
      )}
    </div>
  );
};