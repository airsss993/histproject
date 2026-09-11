import React, { useEffect, useState, useRef } from 'react'
import L from 'leaflet'
import { useMap } from 'react-leaflet'
import { landGeoJSon } from '../../entities/cons.ts'
import { centerRussiaOnMap, CountryLabels } from './countries.tsx' // @ts-ignore
import './countrie.scss'

export const LayersLabels: React.FC = () => {
  const map = useMap();
  const [geoData, setGeoData] = useState<any>(null);
  const [currentZoom, setCurrentZoom] = useState(map.getZoom());
  const countriesLayerRef = useRef<L.GeoJSON | null>(null);
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

  useEffect(() => {
    if (isDataLoadedRef.current) return; // https://raw.githubusercontent.com/datasets/geo-boundaries-world-110m/master/countries.geojson
    fetch(landGeoJSon)
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
      if (countriesLayerRef.current && map.hasLayer(countriesLayerRef.current)) {
        map.removeLayer(countriesLayerRef.current);
      }
    };
  }, [map]);

  useEffect(() => {
    if (!geoData) return;

    if (countriesLayerRef.current && map.hasLayer(countriesLayerRef.current)) {
      map.removeLayer(countriesLayerRef.current);
    }

    const countriesLayer = L.geoJSON(geoData, {
      style: {
        fillColor: "#2C4672",
        weight: 1.2,
        color: "#2F3B54",
        fillOpacity: 1,
        opacity: 1
      },
    }).addTo(map);

    countriesLayerRef.current = countriesLayer;
  }, [geoData, map]);

  return <CountryLabels geoData={geoData} map={map} currentZoom={currentZoom} />;
};