import { configureStore } from "@reduxjs/toolkit"
import { baseApi } from "./api"
import uiReducer from "./uiSlice"

export const store = configureStore({
  reducer: {
    [baseApi.reducerPath]: baseApi.reducer,
    ui: uiReducer,
  },
  middleware: (getDefault) => getDefault().concat(baseApi.middleware),
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
