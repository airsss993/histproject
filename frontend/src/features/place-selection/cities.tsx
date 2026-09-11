import React, { useEffect, useState, useRef } from 'react'
import L from 'leaflet'
import { useMap } from 'react-leaflet'
import { citiesPosition, getCityZoomThreshold } from './citiesPosition.ts'

export const CitiesLabels: React.FC = () => {
  const map = useMap();
  const [currentZoom, setCurrentZoom] = useState(map.getZoom());
  const markersRef = useRef<{ marker: L.Marker; threshold: number }[]>([]);

  useEffect(() => {
    const handleZoom = () => setCurrentZoom(map.getZoom());
    map.on('zoomend', handleZoom);
    return () => { map.off('zoomend', handleZoom); };
  }, [map]);

  useEffect(() => {
    Object.entries(citiesPosition).forEach(([name, info]) => {
      const [lat, lng] = info.coords;
      const threshold = getCityZoomThreshold(info.type);
      const isVisible = currentZoom >= threshold;

      const marker = L.marker([lat, lng], {
        icon: L.divIcon({
          className: `city-label city-label-${info.type} ${isVisible ? 'is-visible' : 'is-hidden'}`,
          html: `<div class="city-name">${name}</div>`,
          iconSize: [120, 20],
          iconAnchor: [60, 10],
        }),
        interactive: false,
        zIndexOffset: 500,
      });

      marker.addTo(map);
      markersRef.current.push({ marker, threshold });
    });

    return () => {
      markersRef.current.forEach(({ marker }) => {
        if (map.hasLayer(marker)) map.removeLayer(marker);
      });
      markersRef.current = [];
    };
  }, [map]);

  useEffect(() => {
    markersRef.current.forEach(({ marker, threshold }) => {
      const el = marker.getElement();
      if (!el) return;
      if (currentZoom >= threshold) {
        el.classList.add('is-visible');
        el.classList.remove('is-hidden');
      } else {
        el.classList.add('is-hidden');
        el.classList.remove('is-visible');
      }
    });
  }, [currentZoom]);

  return null;
};