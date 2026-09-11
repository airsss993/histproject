import L from 'leaflet' //@ts-ignore
import './countrie.scss'

export interface CountryLabelItem {
  marker: L.Marker;
  englishName: string;
  russianName: string;
  bounds: L.LatLngBounds;
  sizeCategory: 'large' | 'medium' | 'small';
}

const visZoom = 8;
const larZoom = 3;
const medZoom = 4;
const smaZoom = 5;

export const getCountrySizeCategory = (bounds: L.LatLngBounds): 'large' | 'medium' | 'small' => {
  const area = bounds.getNorth() - bounds.getSouth();
  if (area > 10) return 'large';
  if (area > 3) return 'medium'; 
  return 'small';
};

export const shouldShowLabel = (sizeCategory: 'large' | 'medium' | 'small', zoom: number): boolean => {
  if (zoom >= visZoom) return true;
  
  switch(sizeCategory) {
    case 'large':
      return zoom >= larZoom;
    case 'medium':
      return zoom >= medZoom;
    case 'small':
      return zoom >= smaZoom;
    default:
      return false;
  }
};