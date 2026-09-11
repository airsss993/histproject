import React, { useState, useEffect } from 'react'
import { DualRangeSlider } from './sliders'
import { SubDropdownBtn } from './dropbtn'
import { FilterConfig, HistoricalPeriod } from './typeven'

interface ActiveFilterDropdownProps {
  filterIndex: number
  filter: FilterConfig
  periodRange: { min: number; max: number }
  selectedPeriod: string | null
  selectedOptions: { [key: string]: boolean }
  historicalPeriods: HistoricalPeriod[]
  onPeriodChange: (min: number, max: number) => void
  onPeriodSelect: (periodLabel: string) => void
  onPeriodClick: (period: HistoricalPeriod) => void
  onOptionToggle: (option: string) => void
  onFilterType?: (type: number | null) => void
}

export const ActiveFilterDropdown: React.FC<ActiveFilterDropdownProps> = ({
  filterIndex,
  filter,
  periodRange,
  selectedPeriod,
  selectedOptions,
  historicalPeriods,
  onPeriodChange,
  onPeriodSelect,
  onPeriodClick,
  onOptionToggle
}) => {
  const [minInputValue, setMinInputValue] = useState('0862')
  const [maxInputValue, setMaxInputValue] = useState('2026')

  const formatYearWithLeadingZeros = (year: number): string => {
    return year.toString().padStart(4, '0')
  }

  const formatYearForDisplay = (year: number): string => {
    if (year === 2026) return 'н.в.'
    return formatYearWithLeadingZeros(year)
  }

  useEffect(() => {
    setMinInputValue(formatYearForDisplay(periodRange.min))
    setMaxInputValue(formatYearForDisplay(periodRange.max))
  }, [periodRange])

  const isPeriodInRange = (period: HistoricalPeriod): boolean => {
    return (period.startYear <= periodRange.max && period.endYear >= periodRange.min)
  }

  const handleMinInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setMinInputValue(e.target.value)
  }

  const handleMaxInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setMaxInputValue(e.target.value)
  }

  const handleMinInputBlur = () => {
    let value = minInputValue.replace(/[^\d]/g, '')
    let numValue = value ? parseInt(value, 10) : 862

    numValue = Math.min(Math.max(numValue, 862), periodRange.max - 1)

    const formattedForDisplay = formatYearForDisplay(numValue)
    setMinInputValue(formattedForDisplay)
    
    onPeriodChange(numValue, periodRange.max)
  }
  const handleMaxInputBlur = () => {
    let value = maxInputValue.replace(/[^\d]/g, '')
    let numValue = value ? parseInt(value, 10) : 2026

    numValue = Math.min(Math.max(numValue, periodRange.min + 1), 2026)

    const formattedForDisplay = formatYearForDisplay(numValue)
    setMaxInputValue(formattedForDisplay)
    
    onPeriodChange(periodRange.min, numValue)
  }

  const handlePeriodButtonClick = (period: HistoricalPeriod) => {
    onPeriodSelect(period.label)
    onPeriodClick(period)
  }

  const handleOptionButtonClick = (option: string) => {
    onOptionToggle(option)
  }

  return (
    <div className="FilterSubDropdown">
      {filterIndex === 2 && (
        <>
          <div className="Inputs">
            <input 
              type="text" 
              data-index="1"
              placeholder="От 862 гг."
              value={minInputValue}
              onChange={handleMinInputChange}
              onBlur={handleMinInputBlur}
            />
            <input 
              type="text"
              data-index="2" 
              placeholder="До н. в."
              value={maxInputValue}
              onChange={handleMaxInputChange}
              onBlur={handleMaxInputBlur}
            />
          </div>
          <DualRangeSlider 
            min={862}
            max={2026}
            value={periodRange}
            onChange={onPeriodChange}
          />
        </>
      )}

      {filterIndex === 2 
        ? historicalPeriods.map((period, optIndex) => {
            const isInRange = isPeriodInRange(period)
            const isSelected = selectedPeriod === period.label

            return (
              <SubDropdownBtn
                key={optIndex}
                label={period.label}
                isSelected={isSelected}
                isInRange={isInRange}
                onClick={() => handlePeriodButtonClick(period)}
                onCheckboxChange={() => handlePeriodButtonClick(period)}
              />
            )
          })
        : filter.options.map((option: string, optIndex: React.Key | null | undefined) => (
            <SubDropdownBtn
              key={optIndex}
              label={option}
              isSelected={selectedOptions[option] || false}
              isInRange={true}
              onClick={() => handleOptionButtonClick(option)}
              onCheckboxChange={() => handleOptionButtonClick(option)}
            />
          ))
      }
    </div>
  )
}