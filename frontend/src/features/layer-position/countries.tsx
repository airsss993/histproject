import React, { useCallback, useEffect, useRef } from 'react'
import L from 'leaflet'
import { countriesPosition } from './countriesPosition.ts'
import { countriesTranslation } from './countriesTranslation.ts'
import { getCountrySizeCategory, shouldShowLabel } from './zoom.tsx'

export const CountryLabels: React.FC<{
  geoData: any,
  map: L.Map,
  currentZoom: number
}> = ({ geoData, map, currentZoom }) => {
  const labelsRef = useRef<{ marker: L.Marker; englishName: string; russianName: string; bounds: any; sizeCategory: string }[]>([]);

  const updateLabelsVisibility = useCallback((zoom: number) => {
    labelsRef.current.forEach(item => {
      const { marker, sizeCategory } = item; // @ts-ignore
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
    if (!geoData || !map) return;

    labelsRef.current.forEach(item => {
      if (map.hasLayer(item.marker)) {
        map.removeLayer(item.marker);
      }
    });
    labelsRef.current = [];

    geoData.features.forEach((feature: any) => {
      const englishName = feature.properties.name || feature.properties.NAME || feature.properties.ADMIN || 'Неизвестная страна';
      const russianName = countriesTranslation[englishName] || englishName;

      try {
        const tempLayer = L.geoJSON(feature);
        const bounds = tempLayer.getBounds();
        const sizeCategory = getCountrySizeCategory(bounds);
        const labelPosition = getCountryLabelPosition(englishName, bounds);

        const label = L.marker(labelPosition, {
          icon: L.divIcon({
            className: `country-label country-label-${sizeCategory} is-hidden`,
            html: `<div class="country-name">${russianName}</div>`,
            iconSize: [100, 20],
            iconAnchor: [50, 10]
          }),
          interactive: false,
          zIndexOffset: 1000
        });

        label.addTo(map);
        labelsRef.current.push({ marker: label, englishName, russianName, bounds, sizeCategory });
      } catch (error) {
        console.log('Ошибка при создании метки:', russianName, error);
      }
    });

    updateLabelsVisibility(currentZoom);
  }, [geoData, map]);

  useEffect(() => {
    if (!geoData || labelsRef.current.length === 0) return;
    updateLabelsVisibility(currentZoom);
  }, [currentZoom, geoData, updateLabelsVisibility]);

  return null;
};

export const centerRussiaOnMap: React.FC = (geoData: any): any => {
  if (!geoData) return geoData;
  
  const adjustedData = JSON.parse(JSON.stringify(geoData));
  
  adjustedData.features = adjustedData.features.map((feature: any) => {
    const englishName = feature.properties.name || feature.properties.NAME || feature.properties.ADMIN || '';
    const isRussia = englishName === 'Russia' || englishName === 'Russian Federation' || countriesTranslation[englishName] === 'Россия';
    
    if (isRussia && feature.geometry.type === 'MultiPolygon') {
      feature.geometry.coordinates = feature.geometry.coordinates.map((polygon: any[][][]) => {
        return polygon.map((ring: any[][]) => {
          return ring.map((coord: any[]) => {
            let [lng, lat] = coord;
            if (lng < 0) {
              return [lng + 360, lat];
            }
            return coord;
          });
        });
      });
    }
    return feature;
  });
  return adjustedData;
};

export const getCountryLabelPosition = (countryName: string, bounds: L.LatLngBounds): L.LatLng => {
  if (countriesPosition[countryName]) {
    const [lat, lng] = countriesPosition[countryName];
    return L.latLng(lat, lng);
  }
  return bounds.getCenter();
};