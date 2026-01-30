/**
 * Centralized Chart.js setup
 * Import this module in components that need charts to ensure registration happens once
 */
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'

// Register Chart.js components once
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  BarElement,
  ArcElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

// Set default options for better tooltip behavior
// This makes tooltips show when hovering near data points, not just exactly on them
ChartJS.defaults.interaction.mode = 'index'
ChartJS.defaults.interaction.intersect = false

// Re-export chart components for convenience
export { Line, Bar, Pie, Doughnut } from 'vue-chartjs'
export { ChartJS }
