import { render } from 'preact'
import '@picocss/pico/css/pico.min.css'
import 'leaflet/dist/leaflet.css'
import { App } from './App'
import './styles.css'

render(<App />, document.getElementById('app')!)
