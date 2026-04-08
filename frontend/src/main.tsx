import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './styles/globals.css'

// Apply saved theme synchronously before React mounts to avoid flash
const saved = localStorage.getItem('fuflogon-theme')
const theme = saved === 'light' ? 'light' : 'dark'
document.documentElement.classList.add(theme)
document.body.classList.add(theme)

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
