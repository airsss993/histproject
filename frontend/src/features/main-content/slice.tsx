import { createSlice, PayloadAction } from '@reduxjs/toolkit'

export interface TitleHState {
  title1: string;
  title2: string;
  title3: string;
  needAnimated: boolean;
}

const initialState: TitleHState = {// @ts-ignore
  title1: <h1>Россия —<br />страна героев</h1>,// @ts-ignore
  title2: <h2>Интерактивная карта исторических<br />событий</h2>,// @ts-ignore
  title3: <h3>Исследуйте историю. Сохраняйте память. Добавляйте события на общую карту подвигов.</h3>,
  needAnimated: true
}

export const { actions, name, reducer } = createSlice({
  name: 'main-content',
  initialState,
  reducers: {
    setTitleH1: (state, action: PayloadAction<string>) => {
      state.title1 = action.payload;
    },
    setTitleH2: (state, action: PayloadAction<string>) => {
      state.title2 = action.payload;
    },
    setTitleH3: (state, action: PayloadAction<string>) => {
      state.title3 = action.payload;
    },
    disableAnimated: (state) => {
      state.needAnimated = false;
    },
  },
});

actions.setTitleH1.type === 'main-content/setTitleH1'