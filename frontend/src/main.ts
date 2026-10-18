import './app.css'
import { mount } from 'svelte'
import App from './App.svelte'

// Mac-only styling (transparent page over the native vibrancy, room for the
// traffic lights) keys off this attribute.
if (navigator.userAgent.includes('Mac')) {
  document.documentElement.dataset.platform = 'mac'
}

mount(App, { target: document.getElementById('app')! })
