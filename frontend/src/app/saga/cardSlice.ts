import { createSlice } from '@reduxjs/toolkit';
import { FETCH_CARD_DATA_SUCCESS, FETCH_CARD_DATA } from './saga';

const cardSlice = createSlice({
  name: 'card',
  initialState: {
    cardData: null as any,
    events: [] as any[],
    isLoading: false,
    error: null as string | null,
  },
  reducers: {},
  extraReducers: (builder) => {
    builder.addCase(FETCH_CARD_DATA, (state) => {
      state.cardData = null;
    });
    builder.addCase(FETCH_CARD_DATA_SUCCESS, (state, action) => {// @ts-ignore
      state.cardData = action.payload;
    });
  },
});

export default cardSlice.reducer;