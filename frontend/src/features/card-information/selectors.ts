import { RootState } from '../../app/store'

export const selectAllEvents = (state: RootState) => state.card?.events || [];
export const selectEventsLoading = (state: RootState) => state.card?.isLoading || false;
export const selectEventsError = (state: RootState) => state.card?.error || null;
export const selectCardData = (state: RootState) => state.card?.cardData || null;

export const selectEventById = (state: RootState, id: number) => 
  state.card?.events?.find(event => event.id === id);