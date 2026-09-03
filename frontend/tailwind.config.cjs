/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/features/**/*.{js,ts,jsx,tsx,mdx}',
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        background: 'var(--background)',
        foreground: 'var(--foreground)',
        primary: {
          950: 'var(--primary-950)',
          900: 'var(--primary-900)',
          800: 'var(--primary-800)',
          700: 'var(--primary-700)',
        },
        accent: {
          900: 'var(--accent-900)',
          800: 'var(--accent-800)',
          700: 'var(--accent-700)',
        },
        card: 'var(--card-bg)',
      },
      boxShadow: {
        'elevated': '0 10px 15px -3px rgba(0,0,0,0.1)',
      },
    },
  },
  plugins: [],
}
