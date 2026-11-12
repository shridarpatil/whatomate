/** @type {import('tailwindcss').Config} */
const defaultTheme = require('tailwindcss/defaultTheme')

module.exports = {
  darkMode: ["class"],
  corePlugins: { preflight: false },
  // Add light mode variant for dark-first design
  plugins: [
    require("tailwindcss-animate"),
    function({ addVariant }) {
      addVariant('light', '.light &')
    }
  ],
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
  	container: {
  		center: true,
  		padding: '2rem',
  		screens: {
  			'2xl': '1400px'
  		}
  	},
  	extend: {
  		fontFamily: {
  			sans: ['Inter', ...defaultTheme.fontFamily.sans],
  		},
  		colors: {
  			border: 'hsl(var(--sh-border))',
  			input: 'hsl(var(--sh-input))',
  			ring: 'hsl(var(--sh-ring))',
  			background: 'hsl(var(--sh-background))',
  			foreground: 'hsl(var(--sh-foreground))',
  			primary: {
  				DEFAULT: 'hsl(var(--sh-primary))',
  				foreground: 'hsl(var(--sh-primary-foreground))'
  			},
  			secondary: {
  				DEFAULT: 'hsl(var(--sh-secondary))',
  				foreground: 'hsl(var(--sh-secondary-foreground))'
  			},
  			destructive: {
  				DEFAULT: 'hsl(var(--sh-destructive))',
  				foreground: 'hsl(var(--sh-destructive-foreground))'
  			},
  			muted: {
  				DEFAULT: 'hsl(var(--sh-muted))',
  				foreground: 'hsl(var(--sh-muted-foreground))'
  			},
  			accent: {
  				DEFAULT: 'hsl(var(--sh-accent))',
  				foreground: 'hsl(var(--sh-accent-foreground))'
  			},
  			popover: {
  				DEFAULT: 'hsl(var(--sh-popover))',
  				foreground: 'hsl(var(--sh-popover-foreground))'
  			},
  			card: {
  				DEFAULT: 'hsl(var(--sh-card))',
  				foreground: 'hsl(var(--sh-card-foreground))'
  			},
  			whatsapp: {
  				green: '#25D366',
  				teal: '#128C7E',
  				'teal-dark': '#075E54',
  				light: '#DCF8C6'
  			},
  			facebook: {
  				DEFAULT: '#1877F2',
  				dark: '#0C5DC7',
  				hover: '#166FE5',
  				hoverDark: '#0A4DAD'
  			},
  			// Violet color palette (primary accent)
  			violet: {
  				50: '#f5f3ff',
  				100: '#ede9fe',
  				200: '#ddd6fe',
  				300: '#c4b5fd',
  				400: '#a78bfa',
  				500: '#8b5cf6',
  				600: '#7c3aed',
  				700: '#6d28d9',
  				800: '#5b21b6',
  				900: '#4c1d95',
  				950: '#2e1065'
  			},
  			// Glass effect colors
  			glass: {
  				bg: 'var(--glass-bg)',
  				'bg-hover': 'var(--glass-bg-hover)',
  				border: 'var(--glass-border)'
  			}
  		},
  		borderRadius: {
  			lg: 'var(--sh-radius)',
  			md: 'calc(var(--sh-radius) - 2px)',
  			sm: 'calc(var(--sh-radius) - 4px)'
  		},
  		keyframes: {
  			'accordion-down': {
  				from: {
  					height: '0'
  				},
  				to: {
  					height: 'var(--reka-accordion-content-height)'
  				}
  			},
  			'accordion-up': {
  				from: {
  					height: 'var(--reka-accordion-content-height)'
  				},
  				to: {
  					height: '0'
  				}
  			}
  		},
  		animation: {
  			'accordion-down': 'accordion-down 0.2s ease-out',
  			'accordion-up': 'accordion-up 0.2s ease-out'
  		},
  		// Glass effect backdrop blur
  		backdropBlur: {
  			xs: '2px'
  		}
  	}
  },
}
