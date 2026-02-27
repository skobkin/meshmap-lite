import { render } from 'preact'
import '@picocss/pico/css/pico.min.css'
import 'leaflet/dist/leaflet.css'
import 'leaflet.markercluster/dist/MarkerCluster.css'
import 'leaflet.markercluster/dist/MarkerCluster.Default.css'
import { App } from './App'
import './styles.css'

render(<App />, document.getElementById('app')!)
