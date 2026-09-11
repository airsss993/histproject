import React, { useEffect, useState, useRef, useCallback } from 'react'
import L from 'leaflet'
import { useMap } from 'react-leaflet'
import { landGeoJSon } from '../../entities/cons.ts'
import { CountryLabelItem, getCountrySizeCategory, shouldShowLabel } from '../layer-position/zoom.tsx'
import { centerRussiaOnMap, getCountryLabelPosition } from '../layer-position/countries.tsx'
import { countriesTranslation } from '../layer-position/countriesTranslation.ts' // @ts-ignore
import '../layer-position/countrie.scss'

export const LayerLabels: React.FC = () => {
  const map = useMap();
  const [geoData, setGeoData] = useState<any>(null);
  const [currentZoom, setCurrentZoom] = useState(map.getZoom());
  const allLabelsRef = useRef<CountryLabelItem[]>([]);
  const isDataLoadedRef = useRef(false);

  useEffect(() => {
    const handleZoom = () => {
      const newZoom = map.getZoom();
      setCurrentZoom(newZoom);
    };
    
    map.on('zoomend', handleZoom);
    return () => {
      map.off('zoomend', handleZoom);
    };
  }, [map]);

  const updateLabelsVisibility = useCallback((zoom: number) => {
    allLabelsRef.current.forEach(item => {
      const { marker, sizeCategory } = item;
      const shouldBeVisible = shouldShowLabel(sizeCategory, zoom);

      const divIconElement = marker.getElement(); 
      if (divIconElement) {
        if (shouldBeVisible) {
          divIconElement.classList.add('is-visible');
          divIconElement.classList.remove('is-hidden');
        } else {
          divIconElement.classList.add('is-hidden');
          divIconElement.classList.remove('is-visible');
        }
      }
    });
  }, []);

  useEffect(() => {
    if (isDataLoadedRef.current) return; // https://raw.githubusercontent.com/datasets/geo-boundaries-world-110m/master/countries.geojson
    fetch(landGeoJSon) // ./src/features/layer-position/countries.geojson
      .then(response => response.json())
      .then(data => {
        const adjustedData = centerRussiaOnMap(data);
        setGeoData(adjustedData);
        isDataLoadedRef.current = true;
      })
      .catch(error => {
        console.error('Ошибка загрузки GeoJSon:', error);
      });
    
    return () => {
      allLabelsRef.current.forEach(item => {
        if (map.hasLayer(item.marker)) {
          map.removeLayer(item.marker);
        }
      });
      allLabelsRef.current = [];
    };
  }, [map]);

  useEffect(() => {
    if (!geoData) return;

    allLabelsRef.current.forEach(item => {
      if (map.hasLayer(item.marker)) {
        map.removeLayer(item.marker);
      }
    });
    allLabelsRef.current = [];

    geoData.features.forEach((feature: any) => {
      const englishName = feature.properties.name || feature.properties.NAME || feature.properties.ADMIN || 'Неизвестная страна';
      const russianName = countriesTranslation[englishName] || englishName;
      
      try {
        const tempLayer = L.geoJSON(feature);
        const bounds = tempLayer.getBounds();
        const sizeCategory = getCountrySizeCategory(bounds);
        const labelPosition = getCountryLabelPosition(englishName, bounds);
        
        const initialVisibility = shouldShowLabel(sizeCategory, currentZoom);
        const initialClassName = initialVisibility ? 'is-visible' : 'is-hidden';

        const label = L.marker(labelPosition, {
          icon: L.divIcon({
            className: `country-label country-label-${sizeCategory} ${initialClassName}`,
            html: `<div class="councitry-name">${russianName}</div>`,
            iconSize: [100, 20],
            iconAnchor: [50, 10]
          }),
          interactive: false,
          zIndexOffset: 1000
        });
        
        label.addTo(map);
        allLabelsRef.current.push({
          marker: label,
          englishName,
          russianName,
          bounds,
          sizeCategory,
        });
      } catch (error) {
        console.log('Ошибка при создании метки:', russianName, error);
      }
    });
    
    updateLabelsVisibility(currentZoom);
  }, [geoData, map]);

  useEffect(() => {
    if (geoData && allLabelsRef.current.length > 0) {
      updateLabelsVisibility(currentZoom);
    }
  }, [currentZoom, geoData, updateLabelsVisibility]);

  return null;
};