export const colors = {
  // Primary Blues
  primary: {
    950: '#0F3D5E',
    900: '#1B4965',
    800: '#2C699A',
    700: '#468FAF',
    600: '#5BA3C4',
    500: '#6EB8D9',
    400: '#8CC5E0',
    300: '#AAD2E7',
    200: '#C8DFF0',
    100: '#E6EDF7',
  },
  // Lime Green Accents
  accent: {
    900: '#5F7D2F',
    800: '#76C893',
    700: '#A7C957',
    600: '#B5E48C',
    500: '#C3EDBF',
    400: '#D1F6E0',
  },
  // Dark Mode
  dark: {
    950: '#081018',
    900: '#0B1724',
    800: '#132238',
    700: '#1B3050',
    600: '#243B5C',
    500: '#2D4668',
  },
  // Light Mode
  light: {
    50: '#F8FAFC',
    100: '#EEF2F7',
    200: '#E2E8F0',
    300: '#CBD5E1',
  },
  // Status Colors
  status: {
    success: '#10B981',
    warning: '#F59E0B',
    error: '#EF4444',
    info: '#3B82F6',
  },
}

export const gradients = {
  primary: 'linear-gradient(135deg, #0F3D5E 0%, #1B4965 100%)',
  accent: 'linear-gradient(135deg, #A7C957 0%, #76C893 100%)',
  glow: 'radial-gradient(circle at center, rgba(167, 201, 87, 0.1) 0%, transparent 70%)',
  dark: 'linear-gradient(135deg, #081018 0%, #132238 100%)',
}

export const shadows = {
  sm: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
  md: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
  lg: '0 10px 15px -3px rgba(0, 0, 0, 0.1)',
  xl: '0 20px 25px -5px rgba(0, 0, 0, 0.1)',
  '2xl': '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
  glow: '0 0 20px rgba(167, 201, 87, 0.3)',
  'glow-lg': '0 0 40px rgba(167, 201, 87, 0.2)',
}

export const transitions = {
  fast: '150ms cubic-bezier(0.4, 0, 0.2, 1)',
  base: '200ms cubic-bezier(0.4, 0, 0.2, 1)',
  slow: '300ms cubic-bezier(0.4, 0, 0.2, 1)',
  slower: '500ms cubic-bezier(0.4, 0, 0.2, 1)',
}

export const spacing = {
  xs: '0.25rem',
  sm: '0.5rem',
  md: '1rem',
  lg: '1.5rem',
  xl: '2rem',
  '2xl': '3rem',
  '3xl': '4rem',
  '4xl': '6rem',
}

export const borderRadius = {
  sm: '0.375rem',
  md: '0.5rem',
  lg: '0.75rem',
  xl: '1rem',
  '2xl': '1.5rem',
  full: '9999px',
}

export const zIndex = {
  dropdown: 1000,
  sticky: 1020,
  fixed: 1030,
  backdrop: 1040,
  offcanvas: 1050,
  modal: 1060,
  popover: 1070,
  tooltip: 1080,
}
