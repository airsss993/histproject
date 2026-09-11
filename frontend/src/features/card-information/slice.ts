import { createSlice, PayloadAction } from '@reduxjs/toolkit'

export enum EventType {
    Execution = 1,
    MilitaryOperation = 2,
    BorderConflict = 3,
    Uprising = 4
}
export interface EventObject {
    id: number;
    title: string;
    description: string;
    latitude: string;
    longitude: string;
    eventDate: string;
    eventType: EventType;
    previewUrlImage: string;
    siteUrl: string | null;
}

interface EventsState {
  events: EventObject[];
  isLoading: boolean;
  error: string | null;
  cardData: EventObject | null;
}

const initialState: EventsState = {
  events: [],
  isLoading: false,
  error: null,
  cardData: null,
};

const eventsSlice = createSlice({
  name: 'events',
  initialState,
  reducers: {
    fetchAllEvents: (state) => {
      state.isLoading = true;
      state.error = null;
    },
    fetchAllEventsSuccess: (state, action: PayloadAction<EventObject[]>) => {
      state.events = action.payload;
      state.isLoading = false;
    },
    fetchAllEventsFailure: (state, action: PayloadAction<string>) => {
      state.isLoading = false;
      state.error = action.payload;
    },
    setCardData: (state, action: PayloadAction<EventObject | null>) => {
      state.cardData = action.payload;
    },
    clearCardData: (state) => {
      state.cardData = null;
    },
  },
});

export const {
  fetchAllEvents,
  fetchAllEventsSuccess,
  fetchAllEventsFailure,
  setCardData,
  clearCardData,
} = eventsSlice.actions;

export default eventsSlice.reducer;