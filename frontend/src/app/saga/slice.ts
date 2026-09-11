import { createSlice } from '@reduxjs/toolkit'
// import { reducer } from '../../features/login/slice'

const initialState = {
  rootappsite: [],
  header: [],
  index: [],
  main: [],
  mapp: [],
  layer: [],
  marker: [],
  filter: [],
  form: [],
  card: [],
  panel: [],
  login: [],
  admin: [],
  hislog: [],
  moder: [],
  addmapp: []
};

const markersSlice = createSlice({
  name: 'marker',
  initialState,
  reducers: {
    setEvents(state, action) {
      state.marker = action.payload;
    },
  // adminLogin: reducer.reducers,
    // другие редьюсеры
  }
});

export const { setEvents } = markersSlice.actions;
export default markersSlice.reducer;