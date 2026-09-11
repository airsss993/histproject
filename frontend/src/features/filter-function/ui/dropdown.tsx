import React, { useState, useEffect, useRef } from 'react'
import { ActiveFilterDropdown } from './activedropdown'
import { FilterConfig, HistoricalPeriod } from './typeven'
import { useEventFilterContext } from './evenFilterProvider/evenFilterProvider' // @ts-ignore
import './filter.scss'

interface FilterDropdownProps {
  label: string
  isOpen: boolean
  onClose?: () => void
}

export const FilterDropdown: React.FC<FilterDropdownProps> = ({ isOpen }) => {
  const [opacity, setOpacity] = useState(0)
  const [pointerEvents, setPointerEvents] = useState<React.CSSProperties['pointerEvents']>('none')
  const hasBeenOpenedRef = useRef(false)

  useEffect(() => {
    if (isOpen) {
      hasBeenOpenedRef.current = true
      setPointerEvents('auto')
      const frame = requestAnimationFrame(() => setOpacity(1))
      return () => cancelAnimationFrame(frame)
    }
    if (!hasBeenOpenedRef.current) return
    setOpacity(0)
    const timer = setTimeout(() => setPointerEvents('none'), 500)
    return () => clearTimeout(timer)
  }, [isOpen])

  const [activeFilter, setActiveFilter] = useState<number | null>(null)
  const [selectedOptions, setSelectedOptions] = useState<{[key: string]: boolean}>({})
  const [periodRange, setPeriodRange] = useState({ min: 862, max: 2026 })
  const [selectedPeriod, setSelectedPeriod] = useState<string | null>(null)
  const { resetAndReload, applyFiltersWith } = useEventFilterContext()

  const historicalPeriods: HistoricalPeriod[] = [
    { label: '862–988гг.', startYear: 862, endYear: 988 },
    { label: '988–1132гг.', startYear: 988, endYear: 1132 },
    { label: '1132–1237гг.', startYear: 1132, endYear: 1237 },
    { label: '1237–1480гг.', startYear: 1237, endYear: 1480 },
    { label: '1480–1547гг.', startYear: 1480, endYear: 1547 },
    { label: '1547–1584гг.', startYear: 1547, endYear: 1584 },
    { label: '1584–1613гг.', startYear: 1584, endYear: 1613 },
    { label: '1613–1682гг.', startYear: 1613, endYear: 1682 },
    { label: '1682–1721гг.', startYear: 1682, endYear: 1721 },
    { label: '1721–1801гг.', startYear: 1721, endYear: 1801 },
    { label: '1801–1917гг.', startYear: 1801, endYear: 1917 },
    { label: '1917–1922гг.', startYear: 1917, endYear: 1922 },
    { label: '1922–1991гг.', startYear: 1922, endYear: 1991 },
    { label: '1991г. – Настоящее время', startYear: 1991, endYear: 2026 }
  ]

  const filters: FilterConfig[] = [
    {// @ts-ignore
      name: 'Военные события',
      options: ['Битвы', 'Войны']
    },
    {// @ts-ignore
      name: 'Политические события',
      options: ['Революции', 'Восстания', 'Перевороты']
    },
    {// @ts-ignore
      name: 'Период',
      options: historicalPeriods.map(p => p.label)
    }
  ]

  const handleFilterClick = (index: number) => {
    setActiveFilter(activeFilter === index ? null : index)
  }

  const handlePeriodChange = (min: number, max: number) => {
    setPeriodRange({ min, max })
    console.log('Выбран период (числа):', { min, max })
  }

  const handlePeriodSelect = (periodLabel: string) => {
    setSelectedPeriod(selectedPeriod === periodLabel ? null : periodLabel)
  }

  const handlePeriodClick = (period: HistoricalPeriod) => {
    setPeriodRange({ min: period.startYear, max: period.endYear })
  }

  const handleOptionToggle = (option: string) => {
    setSelectedOptions(prev => ({
      ...prev,
      [option]: !prev[option]
    }))
  }

  const handleResetFilters = async () => {
    setSelectedOptions({})
    setSelectedPeriod(null)
    setPeriodRange({ min: 862, max: 2026 })
    await resetAndReload()
  }

  const handleApplyFilters = async () => {
  console.log('Применяются фильтры с periodRange (числа):', periodRange)
  
  const hasSelectedOptions = Object.values(selectedOptions).some(value => value === true)
  const hasSelectedPeriod = selectedPeriod !== null
  const hasCustomPeriodRange = periodRange.min !== 862 || periodRange.max !== 2026
  
  if (!hasSelectedOptions && !hasSelectedPeriod && !hasCustomPeriodRange) {
    console.log('Ни один фильтр не выбран, скрываем все маркеры')
    await applyFiltersWith({
      selectedOptions: {}, // @ts-ignore
      periodRange: { min: null, max: null },
      selectedPeriod: null,
      hideAllMarkers: true
    })
  } else {
    await applyFiltersWith({
      selectedOptions,
      periodRange,
      selectedPeriod
    })
  }
}

  return (
    <div className="FilterDropdown" style={{ opacity, pointerEvents, transition: 'opacity 0.5s ease' }}>
      {filters.map((filter, index) => (
        <div key={index} className="FilterItemWrapper">
          <div 
            className={`FilterItem ${activeFilter === index ? 'active' : ''}`}
            style={{ opacity, pointerEvents, transition: 'opacity 0.5s ease' }} 
            onClick={() => handleFilterClick(index)}
          >
            <label className="DropdownLabel">
              {filter.// @ts-ignore
              name}
              <summary className={activeFilter === index ? 'rotated' : ''} data-index={index}>
                <svg width="30" height="30" viewBox="0 0 26 26" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M4.33337 9.75L13 18.4167L21.6667 9.75" stroke="#C09139" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </summary> 
            </label>
          </div>

          {activeFilter === index && (
            <ActiveFilterDropdown
              filterIndex={index}
              filter={filter}
              periodRange={periodRange}
              selectedPeriod={selectedPeriod}
              selectedOptions={selectedOptions}
              historicalPeriods={historicalPeriods}
              onPeriodChange={handlePeriodChange}
              onPeriodSelect={handlePeriodSelect}
              onPeriodClick={handlePeriodClick}
              onOptionToggle={handleOptionToggle}
            />
          )}
        </div>
      ))}

      <div className="FilterActions">
        <button className="ApplyBtn" onClick={handleApplyFilters}>Применить</button>
        <button className="ResetBtn" onClick={handleResetFilters}>Сбросить</button>
      </div>
    </div>
  )
}