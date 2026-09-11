import { EventType } from '../../marker-location/evenPositions'

export interface FilterConfig {
  options: any;
  id: string;
  label: string;
  type: 'period' | 'options';
}

export interface HistoricalPeriod {
  label: string;
  startYear: number;
  endYear: number;
}

export interface EventDates {
    id: number;
    title: string;
    description: string;
    latitude: string;
    longitude: string;
    eventDate: string;
    eventType: EventType;
    previewUrlImage: string;
}

export interface EventTypeItem {
    id: number;
    name: string;
    description: string;
}

export interface DateRange {
    dateFrom: string;
    dateTo: string;
}

export interface ApiResponse {
    objects: EventDates[];
}

export interface FilterRequestData {
    eventTypeIds?: number[]; // int[]
    dateFrom?: string;
    dateTo?: string;
    // periodLabel: string | null;
}

const OPTION_TO_EVENT_TYPE_NAME: Record<string, string> = {
    'Битвы': 'Военная операция',
    'Войны': 'Войны',
    'Революции': 'Расстрел/Расправа',
    'Восстания': 'Восстание/Бунт',
    'Перевороты': 'Пограничный конфликт'
};

const FALLBACK_NAME_TO_ID: Record<string, number> = {
    'Расстрел/Расправа': 1,
    'Военная операция': 2,
    'Пограничный конфликт': 3,
    'Восстание/Бунт': 4,
    'Войны': 5
};

export const mapSelectedOptionsToEventTypeIds = (
    selectedOptions: { [key: string]: boolean },
    eventTypes: EventTypeItem[]
): number[] => {
    const selectedNames = new Set<string>();
    Object.entries(selectedOptions).forEach(([option, isSelected]) => {
        if (!isSelected) return;
        const mappedName = OPTION_TO_EVENT_TYPE_NAME[option] ?? option;
        selectedNames.add(mappedName);
    });

    if (eventTypes.length > 0) {
        return eventTypes
            .filter((eventType) => selectedNames.has(eventType.name))
            .map((eventType) => eventType.id);
    }

    return Array.from(selectedNames)
        .map((name) => FALLBACK_NAME_TO_ID[name])
        .filter((id): id is number => typeof id === 'number');
};

const formatYearForApi = (year: number): string => {
    return year.toString().padStart(4, '0');
};

export const buildFilterRequestData = (params: {
    selectedOptions: { [key: string]: boolean };
    periodRange: { min: number; max: number };
    selectedPeriod: string | null;
    eventTypes?: EventTypeItem[];
}): FilterRequestData => {
    const eventTypeIds = mapSelectedOptionsToEventTypeIds(
        params.selectedOptions,
        params.eventTypes ?? []
    );

    const formattedMinYear = formatYearForApi(params.periodRange.min);
    const formattedMaxYear = formatYearForApi(params.periodRange.max);

    return {
        eventTypeIds,
        dateFrom: `${formattedMinYear}-01-01`,
        dateTo: `${formattedMaxYear}-12-31`,
        // periodLabel: params.selectedPeriod,
    };
};