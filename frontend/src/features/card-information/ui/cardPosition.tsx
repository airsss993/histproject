import React, { useState, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { useSelector } from 'react-redux'
import { EventCard } from './eventcards'
import { EventObject } from '../slice'
import { selectCardData } from '../selectors'
import './eventcard.scss'

export const CardOnMap: React.FC<{
  isVisible: boolean;
  position: L.LatLng;
  event: EventObject;
  onClose: () => void;
}> = ({ isVisible, event, onClose }) => {
  const cardData = useSelector(selectCardData);
  const [show, setShow] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const hasShownRef = useRef(false);

  const handleClose = () => {
    setIsClosing(true);
    setTimeout(() => {
      setShow(false);
      setIsClosing(false);
      onClose();
    }, 300);
  };

  useEffect(() => {
    if (isVisible) {
      hasShownRef.current = true;
      setShow(true);
      setIsClosing(false);
    } else if (hasShownRef.current) {
      setIsClosing(true);
      const timer = setTimeout(() => {
        setShow(false);
        setIsClosing(false);
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [isVisible]);

  const mapWrapper = document.getElementById('map-wrapper');

  if (!mapWrapper) return null;

  return createPortal(
    <div
      className={`event-card-shell ${show ? 'is-visible' : 'is-hidden'} ${isClosing ? 'is-closing' : ''}`}
      style={{
        position: 'absolute',
        top: '0px',
        right: '0px',
        pointerEvents: show && !isClosing ? 'auto' : 'none',
        zIndex: 1000,
      }}
    >
      <EventCard
        eventTitle={event.title}
        eventDate={event.eventDate}
        eventDescription={event.description}
        imageUrl={event.previewUrlImage}
        siteUrl={event.siteUrl || undefined}
        onClose={handleClose}
        cardData={cardData || undefined}
      />
    </div>,
    mapWrapper
  );
};