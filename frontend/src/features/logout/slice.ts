import { createSlice } from '@reduxjs/toolkit'
import { AuthState } from '../login/types'

export const name = 'logout'

const initialState: AuthState = {
  login: '',
  password: '',
  loading: false,
  error: null,
  isAuthenticated: false,
  role: null,
  username: localStorage.getItem('username'),
  logoutLoading: false,
  logoutCompleted: false,
  refreshReady: false,
  refreshUnauthorized: false,
  navigateTo: null,
  auth: null,
  navigateToLogin: undefined
};

export const { reducer, actions } = createSlice({
  name,
  initialState,
  reducers: {
    logout: (state) => {
      state.isAuthenticated = false;
      state.role = null;
      state.username = null;
      state.login = '';
      state.password = '';
      localStorage.removeItem('username');
    },
    clearError: (state) => {
      state.error = null;
    },
    logoutRequest: (state) => {
      state.logoutLoading = true;
      state.logoutCompleted = false;
    },
    logoutSuccess: (state) => {
      state.logoutLoading = false;
      state.logoutCompleted = true;
    },
    logoutReset: (state) => {
      state.logoutCompleted = false;
    },
  },
});