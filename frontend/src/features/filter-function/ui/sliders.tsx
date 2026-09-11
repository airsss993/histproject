import React, { useState, useRef, useEffect } from 'react'

interface DualRangeSliderProps {
  min: number;
  max: number;
  value: { min: number; max: number };
  onChange?: (min: number, max: number) => void;
}

export const DualRangeSlider: React.FC<DualRangeSliderProps> = ({ 
  min, 
  max, 
  value,
  onChange 
}) => {
  const [minVal, setMinVal] = useState(value.min);
  const [maxVal, setMaxVal] = useState(value.max);
  
  const minRangeRef = useRef<HTMLInputElement>(null);
  const maxRangeRef = useRef<HTMLInputElement>(null);
  const rangeTrackRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setMinVal(value.min);
    setMaxVal(value.max);
  }, [value]);

  useEffect(() => {
    if (minRangeRef.current && maxRangeRef.current && rangeTrackRef.current) {
      const minPercent = ((minVal - min) / (max - min)) * 100;
      const maxPercent = ((maxVal - min) / (max - min)) * 100;
      
      const thumbWidth = 40;
      const sliderWidth = rangeTrackRef.current.parentElement?.offsetWidth || 100;
      
      const thumbOffset = (thumbWidth / sliderWidth) * 20;
      
      const correctedMinPercent = Math.max(0, minPercent - thumbOffset);
      const correctedMaxPercent = Math.min(100, maxPercent + thumbOffset);
      
      rangeTrackRef.current.style.left = `${correctedMinPercent}%`;
      rangeTrackRef.current.style.width = `${correctedMaxPercent - correctedMinPercent}%`;
    }
  }, [minVal, maxVal, min, max]);

  const handleMinChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = Math.min(Number(e.target.value), maxVal - 1);
    setMinVal(value);
    onChange?.(value, maxVal);
  };

  const handleMaxChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = Math.max(Number(e.target.value), minVal + 1);
    setMaxVal(value);
    onChange?.(minVal, value);
  };

  return (
    <div className="dual-range-slider" data-index="3">
      <div className="slider-track"></div>
      <div ref={rangeTrackRef} className="slider-range"></div>
      
      <div className="slider-input-container">
        <input
          ref={minRangeRef}
          type="range"
          className="dual-range min-range"
          min={min}
          max={max}
          value={minVal}
          onChange={handleMinChange}
        />
        <input
          ref={maxRangeRef}
          type="range"
          className="dual-range max-range"
          min={min}
          max={max}
          value={maxVal}
          onChange={handleMaxChange}
        />
      </div>
    </div>
  );
};