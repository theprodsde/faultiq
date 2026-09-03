import { createSlice, PayloadAction } from "@reduxjs/toolkit"

interface UIState {
  sidebarOpen: boolean
  activeModal: string | null
  toasts: { id: string; message: string; type: "success" | "error" | "info" }[]
}

const initialState: UIState = {
  sidebarOpen: false,
  activeModal: null,
  toasts: [],
}

export const uiSlice = createSlice({
  name: "ui",
  initialState,
  reducers: {
    setSidebarOpen(state, action: PayloadAction<boolean>) {
      state.sidebarOpen = action.payload
    },
    openModal(state, action: PayloadAction<string>) {
      state.activeModal = action.payload
    },
    closeModal(state) {
      state.activeModal = null
    },
    addToast(state, action: PayloadAction<{ message: string; type: UIState["toasts"][0]["type"] }>) {
      state.toasts.push({ id: Date.now().toString(), ...action.payload })
    },
    removeToast(state, action: PayloadAction<string>) {
      state.toasts = state.toasts.filter((t) => t.id !== action.payload)
    },
  },
})

export const { setSidebarOpen, openModal, closeModal, addToast, removeToast } = uiSlice.actions
export default uiSlice.reducer
