const COORD_SCALE = 100000;
const CONFIG = JSON.parse(document.getElementById('dashboardConfig').textContent);
const DASHBOARD_CITY = CONFIG.city || 'Nürnberg';
const placesPayload = JSON.parse(document.getElementById('placesData').textContent);
const DATA = placesPayload.r.map((row, index) => decodePlaceRow(row, index, placesPayload.d));
let BEZIRKE = null;
DATA.forEach(row => { row._search = buildSearchText(row); });
const valid = DATA.filter(row => Number.isFinite(row.rating) && Number.isFinite(row.reviewCount));
const fmt = new Intl.NumberFormat('de-DE');
const fmt1 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 1, minimumFractionDigits: 1 });
const fmt2 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2, minimumFractionDigits: 2 });
const fmtDateTime = new Intl.DateTimeFormat('de-DE', { dateStyle: 'short', timeStyle: 'short' });
const TABLE_PAGE_SIZE = 500;
let tableLimit = TABLE_PAGE_SIZE;
let lastSortedRows = [];
const state = { mode: 'removed', sortKey: 'removedEstimate', sortDir: 'desc', userLocation: null };
const els = {
  themeToggle: document.getElementById('themeToggle'), controls: document.getElementById('dashboardFilterControls'), filterToggle: document.getElementById('filterToggle'), filterSummary: document.getElementById('filterSummary'), search: document.getElementById('searchInput'), postcode: document.getElementById('postcodeFilter'), bezirk: document.getElementById('bezirkFilter'), banner: document.getElementById('bannerFilter'), range: document.getElementById('rangeFilter'), category: document.getElementById('categoryFilter'), minReviews: document.getElementById('minReviews'), reset: document.getElementById('resetFilters'), tbody: document.querySelector('#placesTable tbody'), resultCount: document.getElementById('resultCount'), tableTitle: document.getElementById('tableTitle'), mapCount: document.getElementById('mapCount'), nearbyStatus: document.getElementById('nearbyStatus'), showMore: document.getElementById('showMoreRows')
};
const titles = { all: 'Alle Orte', removed: 'Höchste Hinweisspannen', ratio: 'Geschätzter Modellanteil', clean: 'Orte ohne beobachteten Google-Hinweis', nearby: 'In meiner Nähe' };

function finiteNumber(value) { return Number.isFinite(value) ? value : null; }
function decodeCoord(base, offset) { return Number.isFinite(offset) ? base + offset / COORD_SCALE : null; }
function dictValue(dicts, dictIndex, valueIndex) { return dicts[dictIndex][valueIndex] ?? ''; }
function googleMapsURL(cidHex, name) {
  if (cidHex && typeof BigInt === 'function') {
    try { return 'https://www.google.com/maps?cid=' + BigInt('0x' + cidHex).toString(10); } catch (_) {}
  }
  return 'https://www.google.com/maps/search/?api=1&query=' + encodeURIComponent((name || DASHBOARD_CITY) + ' ' + DASHBOARD_CITY);
}
function decodeAddress(value, postcode) {
  if (!value) return '';
  if (value[0] === '!') return value.slice(1);
  return value + ', ' + postcode + ' ' + DASHBOARD_CITY;
}
function decodePlaceRow(row, index, dicts) {
  const postcode = dictValue(dicts, 0, row[2]);
  const hasBanner = row.length > 12;
  return {
    id: String(index), name: row[1] || '', postcode, lat: decodeCoord(49, row[3]), lng: decodeCoord(11, row[4]), bezirkLabel: dictValue(dicts, 1, row[5]), rating: finiteNumber(row[6]), reviewCount: finiteNumber(row[7]), category: dictValue(dicts, 2, row[8]), parentCategory: dictValue(dicts, 3, row[9]), hasBanner, removedRange: hasBanner ? dictValue(dicts, 4, row[12]) : '', removedEstimate: hasBanner ? finiteNumber(row[13]) || 0 : 0, deletionRatioPct: hasBanner ? finiteNumber(row[14]) : null, address: decodeAddress(row[10] || '', postcode), readAt: Number.isFinite(row[11]) ? row[11] * 60000 : 0, url: googleMapsURL(row[0], row[1])
  };
}
function getBezirke() {
  if (!BEZIRKE) BEZIRKE = JSON.parse(document.getElementById('bezirkData').textContent).map(decodeBezirkRow);
  return BEZIRKE;
}
function decodeBezirkRow(row) {
  return { label: row[0] || '', polygons: (row[1] || []).map(flat => {
    const points = [];
    let latOffset = 0;
    let lngOffset = 0;
    for (let i = 0; i + 1 < flat.length; i += 2) {
      if (i === 0) {
        latOffset = flat[i];
        lngOffset = flat[i + 1];
      } else {
        latOffset += flat[i];
        lngOffset += flat[i + 1];
      }
      points.push([decodeCoord(49, latOffset), decodeCoord(11, lngOffset)]);
    }
    return points;
  }) };
}
function buildSearchText(row) { return [row.name, row.postcode, row.bezirkLabel, row.category, row.parentCategory, row.removedRange, row.address].join(' ').toLowerCase(); }
function debounce(fn, wait) {
  let timer = 0;
  return () => {
    window.clearTimeout(timer);
    timer = window.setTimeout(fn, wait);
  };
}
function resetTableLimit() { tableLimit = TABLE_PAGE_SIZE; }
function renderReset() { resetTableLimit(); render(); }

let placesMap = null;
let bezirkLayer = null;
let markerLayer = null;
let tileLayer = null;
let mapUnavailable = false;
let mapLoadStarted = false;
let mapConsentGranted = false;
let pendingMapRows = null;
let pendingMapAllRows = null;
let mapHintTimer = null;
const activeMapKeys = new Set();
const themeStorageKey = 'dashboardTheme';
const prefersDark = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : { matches: false, addEventListener: null };

function storedTheme() {
  try {
    const theme = localStorage.getItem(themeStorageKey);
    return theme === 'dark' || theme === 'light' ? theme : '';
  } catch (_) {
    return '';
  }
}
function currentTheme() {
  const explicit = document.documentElement.dataset.theme;
  if (explicit === 'dark' || explicit === 'light') return explicit;
  return prefersDark.matches ? 'dark' : 'light';
}
function tileURL() { return 'https://{s}.basemaps.cartocdn.com/' + (currentTheme() === 'dark' ? 'dark_all' : 'light_all') + '/{z}/{x}/{y}{r}.png?key=' + encodeURIComponent(CONFIG.cartoApiKey); }
function colorVar(name, fallback) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback; }
function updateThemeToggle() {
  const isDark = currentTheme() === 'dark';
  els.themeToggle.setAttribute('aria-pressed', String(isDark));
  els.themeToggle.setAttribute('aria-label', isDark ? 'Helles Design aktivieren' : 'Dunkles Design aktivieren');
  els.themeToggle.querySelector('.theme-toggle-icon').textContent = isDark ? '☀' : '☾';
  els.themeToggle.querySelector('.theme-toggle-text').textContent = isDark ? 'Hell' : 'Dunkel';
}
function updateMapTheme() {
  if (tileLayer) tileLayer.setUrl(tileURL());
}
function setTheme(theme) {
  document.documentElement.dataset.theme = theme;
  try { localStorage.setItem(themeStorageKey, theme); } catch (_) {}
  updateThemeToggle();
  updateMapTheme();
  if (placesMap) render();
}

function pct(value) { return Number.isFinite(value) ? fmt1.format(value) + '%' : '–'; }
function rating(value, digits = 1) { return Number.isFinite(value) ? (digits === 2 ? fmt2.format(value) : fmt1.format(value)) : '–'; }
function n(value) { return Number.isFinite(value) ? fmt.format(value) : '–'; }
function distanceLabel(km) { return Number.isFinite(km) ? (km < 1 ? n(Math.round(km * 1000)) + ' m' : fmt1.format(km) + ' km') : '–'; }
function readAtLabel(value) {
  const timestamp = typeof value === 'number' ? value : Date.parse(value || '');
  return Number.isFinite(timestamp) && timestamp > 0 ? fmtDateTime.format(new Date(timestamp)) : '–';
}
function distanceKm(lat1, lng1, lat2, lng2) {
  const toRad = value => value * Math.PI / 180;
  const dLat = toRad(lat2 - lat1);
  const dLng = toRad(lng2 - lng1);
  const a = Math.sin(dLat / 2) ** 2 + Math.cos(toRad(lat1)) * Math.cos(toRad(lat2)) * Math.sin(dLng / 2) ** 2;
  return 6371 * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}
function rowDistanceKm(row) {
  if (!state.userLocation || !hasCoords(row)) return Infinity;
  return distanceKm(state.userLocation.lat, state.userLocation.lng, row.lat, row.lng);
}
function esc(s) { return String(s ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch])); }
function searchText(row) { return row._search; }
function matches(row) {
  const q = els.search.value.trim().toLowerCase();
  if (q && !searchText(row).includes(q)) return false;
  if (els.postcode.value && row.postcode !== els.postcode.value) return false;
  if (els.bezirk.value === '__none__' && row.bezirkLabel) return false;
  if (els.bezirk.value && els.bezirk.value !== '__none__' && row.bezirkLabel !== els.bezirk.value) return false;
  if (els.banner.value === 'banner' && !row.hasBanner) return false;
  if (els.banner.value === 'clean' && row.hasBanner) return false;
  if (els.range.value && row.removedRange !== els.range.value) return false;
  if (els.category.value) {
    const v = els.category.value;
    if (v.startsWith('parent:')) {
      if ((row.parentCategory || 'Sonstiges') !== v.slice(7)) return false;
    } else {
      if (row.category !== v) return false;
    }
  }
  if (Number(row.reviewCount || 0) < Number(els.minReviews.value || 0)) return false;
  return true;
}
function filtered() { return valid.filter(matches); }
function selectedOptionLabel(select) { return select.selectedOptions && select.selectedOptions[0] ? select.selectedOptions[0].textContent : select.value; }
function activeFilterSummary() {
  const parts = [];
  if (els.search.value.trim()) parts.push('Suche');
  if (els.postcode.value) parts.push(els.postcode.value);
  if (els.bezirk.value) parts.push(els.bezirk.value === '__none__' ? 'Ohne Bezirk' : selectedOptionLabel(els.bezirk));
  if (els.banner.value !== 'all') parts.push(selectedOptionLabel(els.banner));
  if (els.range.value) parts.push(els.range.value);
  if (els.category.value) parts.push(selectedOptionLabel(els.category));
  if (Number(els.minReviews.value || 0) > 0) parts.push('ab ' + n(Number(els.minReviews.value)) + ' Rezensionen');
  return parts.length ? parts.join(' · ') : 'Keine aktiven Filter';
}
function updateFilterToggle() {
  els.filterToggle.setAttribute('aria-expanded', String(!els.controls.classList.contains('is-collapsed')));
  els.filterSummary.textContent = activeFilterSummary();
}
function nearbyButton() { return document.querySelector('.tab[data-mode="nearby"]'); }
function setNearbyStatus(message, isError = false) {
  els.nearbyStatus.textContent = message;
  els.nearbyStatus.hidden = !message;
  els.nearbyStatus.classList.toggle('error', isError);
}
function setNearbyPending(pending) {
  const button = nearbyButton();
  if (!button) return;
  button.disabled = pending;
  button.textContent = pending ? 'Standort …' : 'In meiner Nähe';
}
function geolocationErrorMessage(error) {
  if (error && error.code === 1) return 'Standortfreigabe abgelehnt. Bitte im Browser erlauben, um „In meiner Nähe“ zu nutzen.';
  if (error && error.code === 2) return 'Standort konnte nicht bestimmt werden.';
  if (error && error.code === 3) return 'Standort-Anfrage hat zu lange gedauert.';
  return 'Standort konnte nicht angefragt werden.';
}
function nearbySuccessStatus() {
  const accuracy = state.userLocation && Number.isFinite(state.userLocation.accuracy) ? ' · Genauigkeit ca. ' + n(Math.round(state.userLocation.accuracy)) + ' m' : '';
  return 'Standort gesetzt. Orte werden nach Entfernung sortiert. Bei externen Kartenkacheln kann der Ausschnitt den ungefähren Standort offenlegen.' + accuracy;
}
function requestUserLocation() {
  if (!navigator.geolocation) {
    setNearbyStatus('Standortzugriff ist nur per HTTPS oder localhost verfügbar.', true);
    return;
  }
  setNearbyPending(true);
  setNearbyStatus('Standort wird angefragt. Entfernungen werden lokal berechnet; bei aktivierten externen Kacheln wird der Kartenausschnitt an CARTO übertragen …');
  navigator.geolocation.getCurrentPosition(position => {
    state.userLocation = { lat: position.coords.latitude, lng: position.coords.longitude, accuracy: position.coords.accuracy };
    setNearbyPending(false);
    setNearbyStatus(nearbySuccessStatus());
    activateMode('nearby');
    render();
    document.getElementById('placesTable').scrollIntoView({ behavior: 'smooth', block: 'start' });
  }, error => {
    setNearbyPending(false);
    setNearbyStatus(geolocationErrorMessage(error), true);
  }, { enableHighAccuracy: true, timeout: 10000, maximumAge: 60000 });
}
const MIN_CLEAN_RANKING_REVIEWS = 100;
function bannerRows(rows) { return rows.filter(row => row.hasBanner && Number.isFinite(row.removedEstimate)); }
function cleanRows(rows) { return rows.filter(row => !row.hasBanner); }
function cleanRankingRows(rows) { return cleanRows(rows).filter(row => Number.isFinite(row.rating) && Number.isFinite(row.reviewCount) && row.reviewCount >= MIN_CLEAN_RANKING_REVIEWS); }
function defaultSortFor(mode) {
  if (mode === 'removed') return ['removedEstimate', 'desc'];
  if (mode === 'ratio') return ['deletionRatioPct', 'desc'];
  if (mode === 'clean') return ['rating', 'desc'];
  if (mode === 'nearby') return ['distanceKm', 'asc'];
  return ['rank', 'asc'];
}
function modeRows(rows) {
  if (state.mode === 'removed') return bannerRows(rows);
  if (state.mode === 'ratio') return bannerRows(rows).filter(row => Number.isFinite(row.deletionRatioPct));
  if (state.mode === 'clean') return cleanRows(rows);
  if (state.mode === 'nearby') return rows.filter(hasCoords);
  return rows;
}
function value(row, key, index) {
  if (key === 'rank') return index + 1;
  if (key === 'hasBanner') return row.hasBanner ? 1 : 0;
  if (key === 'distanceKm') return rowDistanceKm(row);
  if (key === 'readAt') return typeof row.readAt === 'number' ? row.readAt : Date.parse(row.readAt || '') || 0;
  const v = row[key];
  return typeof v === 'string' ? v.toLowerCase() : (Number.isFinite(v) ? v : -Infinity);
}
function sortRows(rows) {
  return rows.map((row, index) => ({ row, index })).sort((a, b) => {
    const av = value(a.row, state.sortKey, a.index);
    const bv = value(b.row, state.sortKey, b.index);
    const result = typeof av === 'string' ? av.localeCompare(bv, 'de') : av - bv;
    if (result === 0 && state.sortKey === 'removedEstimate') return a.row.name.localeCompare(b.row.name, 'de');
    return state.sortDir === 'asc' ? result : -result;
  }).map(item => item.row);
}
function updateKpis(rows) {
  const banners = bannerRows(rows);
  const clean = cleanRows(rows);
  const removedSum = banners.reduce((sum, row) => sum + row.removedEstimate, 0);
  document.getElementById('kpiPlaces').textContent = n(rows.length);
  document.getElementById('kpiBanners').textContent = n(banners.length);
  document.getElementById('kpiBannerPct').textContent = pct((banners.length / Math.max(rows.length, 1)) * 100);
  document.getElementById('kpiRemoved').textContent = n(Math.round(removedSum));
  document.getElementById('kpiClean').textContent = n(clean.length);
}
function hasCoords(row) { return Number.isFinite(row.lat) && Number.isFinite(row.lng); }
function markerColor(row) {
  if (!row.hasBanner) return colorVar('--marker-clean', '#8a8e94');
  if ((row.deletionRatioPct || 0) >= 10) return colorVar('--marker-high', '#3c4e6b');
  return colorVar('--marker-mid', '#5b7b8e');
}
function markerMode(row) { return row.hasBanner ? 'ratio' : 'clean'; }
function isMapModifier(event) { return event.ctrlKey || event.metaKey || event.altKey; }
function setMapScrollMode(enabled) {
  if (!placesMap) return;
  const root = document.getElementById('placesMap');
  if (enabled) {
    placesMap.scrollWheelZoom.enable();
    root.classList.add('map-active');
    root.classList.remove('map-needs-key');
  } else {
    placesMap.scrollWheelZoom.disable();
    root.classList.remove('map-active');
  }
}
function flashMapHint() {
  const root = document.getElementById('placesMap');
  if (!root || root.classList.contains('map-active')) return;
  root.classList.add('map-needs-key');
  window.clearTimeout(mapHintTimer);
  mapHintTimer = window.setTimeout(() => root.classList.remove('map-needs-key'), 1300);
}
function setupMapGestureGate(root) {
  root.addEventListener('wheel', event => {
    if (isMapModifier(event)) {
      event.preventDefault();
      setMapScrollMode(true);
    } else {
      flashMapHint();
    }
  }, { capture: true, passive: false });
  root.addEventListener('touchstart', event => {
    if (event.touches && event.touches.length > 1) root.classList.add('map-active');
    else flashMapHint();
  }, { passive: true });
  root.addEventListener('touchend', () => root.classList.remove('map-active'), { passive: true });
  window.addEventListener('keydown', event => {
    if (['Control', 'Meta', 'Alt'].includes(event.key)) {
      activeMapKeys.add(event.key);
      setMapScrollMode(true);
    }
  });
  window.addEventListener('keyup', event => {
    if (['Control', 'Meta', 'Alt'].includes(event.key)) activeMapKeys.delete(event.key);
    if (activeMapKeys.size === 0) setMapScrollMode(false);
  });
  window.addEventListener('blur', () => {
    activeMapKeys.clear();
    setMapScrollMode(false);
  });
}
function loadLeaflet() {
  return typeof L !== 'undefined' ? Promise.resolve() : Promise.reject(new Error('Lokale Kartenbibliothek fehlt'));
}
function startMapLoad(rows, allFilteredRows) {
  pendingMapRows = rows;
  pendingMapAllRows = allFilteredRows || rows;
  if (placesMap || mapLoadStarted || mapUnavailable) return;
  const root = document.getElementById('placesMap');
  mapLoadStarted = true;
  root.innerHTML = '<div class="map-empty">Karte wird geladen …</div>';
  loadLeaflet().then(() => {
    if (!mapConsentGranted) return;
    renderMap(pendingMapRows || [], pendingMapAllRows || pendingMapRows || []);
    document.getElementById('mapConsentRevoke').hidden = false;
  }).catch(() => {
    mapUnavailable = true;
    root.innerHTML = '<div class="map-empty">Karte konnte nicht geladen werden. Lokale Kartenbibliothek konnte nicht initialisiert werden.</div>';
  });
}
function setupMapConsent() {
  const root = document.getElementById('placesMap');
  if (!root) return;
  const placeholder = root.innerHTML;
  setupMapGestureGate(root);
  document.getElementById('mapConsentRevoke').addEventListener('click', () => {
    mapConsentGranted = false;
    if (placesMap) placesMap.remove();
    placesMap = bezirkLayer = markerLayer = tileLayer = null;
    mapLoadStarted = mapUnavailable = false;
    root.classList.remove('map-active', 'map-needs-key');
    root.innerHTML = placeholder;
    document.getElementById('mapConsentRevoke').hidden = true;
    root.querySelector('#mapConsentLoad').focus();
  });
  root.addEventListener('click', event => {
    const btn = event.target.closest('#mapConsentLoad');
    if (!btn) return;
    if (mapConsentGranted) return;
    mapConsentGranted = true;
    startMapLoad(pendingMapRows || [], pendingMapAllRows || pendingMapRows || []);
  });
}
function initMap() {
  if (placesMap || mapUnavailable) return Boolean(placesMap);
  const root = document.getElementById('placesMap');
  if (typeof L === 'undefined') return false;
  root.innerHTML = '';
  placesMap = L.map(root, { preferCanvas: true, scrollWheelZoom: false, touchZoom: true, minZoom: 1, maxZoom: 20 }).setView([49.4521, 11.0767], 12);
  placesMap.createPane('bezirkPane');
  placesMap.getPane('bezirkPane').style.zIndex = 350;
  placesMap.createPane('placeMarkerPane');
  placesMap.getPane('placeMarkerPane').style.zIndex = 650;
  if (CONFIG.cartoEnabled === 'true') {
    tileLayer = L.tileLayer(tileURL(), { maxZoom: 20, subdomains: 'abcd', attribution: '&copy; <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions" target="_blank" rel="noopener noreferrer">CARTO</a>' }).addTo(placesMap);
  }
  bezirkLayer = L.layerGroup().addTo(placesMap);
  markerLayer = L.layerGroup().addTo(placesMap);
  return true;
}
function selectedBezirkLabel() {
  return els.bezirk.value && els.bezirk.value !== '__none__' ? els.bezirk.value : '';
}
function bezirkStats(rows) {
  const stats = new Map();
  for (const row of rows) {
    if (!row.bezirkLabel) continue;
    if (!stats.has(row.bezirkLabel)) stats.set(row.bezirkLabel, { rows: 0, banners: 0 });
    const stat = stats.get(row.bezirkLabel);
    stat.rows += 1;
    if (row.hasBanner) stat.banners += 1;
  }
  return stats;
}
function bezirkBounds(label) {
  if (!label || !placesMap) return null;
  const district = getBezirke().find(item => item.label === label);
  if (!district) return null;
  const bounds = L.latLngBounds([]);
  for (const polygon of district.polygons) {
    for (const point of polygon) bounds.extend(point);
  }
  return bounds.isValid() ? bounds : null;
}
function renderBezirkLayer(rows) {
  if (!bezirkLayer) return;
  bezirkLayer.clearLayers();
  const stats = bezirkStats(rows);
  const selected = selectedBezirkLabel();
  const red = colorVar('--red', '#cf2a1b');
  const redDark = colorVar('--red-dark', '#8c4139');
  const muted = colorVar('--muted', '#777');
  const orange = colorVar('--orange', '#ef7d16');
  const blue = colorVar('--blue', '#1f6f8b');
  for (const district of getBezirke()) {
    const stat = stats.get(district.label) || { rows: 0, banners: 0 };
    const ratio = stat.banners / Math.max(stat.rows, 1) * 100;
    const isSelected = district.label === selected;
    const hasRows = stat.rows > 0;
    const style = {
      pane: 'bezirkPane',
      color: isSelected ? red : (hasRows ? redDark : muted),
      weight: isSelected ? 3 : 1,
      opacity: isSelected ? .95 : .58,
      dashArray: hasRows ? '' : '4 4',
      fillColor: isSelected ? red : (ratio >= 15 ? red : ratio >= 8 ? orange : blue),
      fillOpacity: isSelected ? .18 : (hasRows ? Math.min(.16, .035 + ratio / 120) : .018),
      interactive: true
    };
    const tooltip = '<strong>' + esc(district.label) + '</strong><br>' + n(stat.rows) + ' Orte · ' + n(stat.banners) + ' Hinweise · ' + pct(ratio);
    for (const polygon of district.polygons) {
      const layer = L.polygon(polygon, style);
      layer.bindTooltip(tooltip, { sticky: true, direction: 'top' });
      layer.on('mouseover', () => layer.setStyle({ weight: Math.max(style.weight, 3), color: red, fillOpacity: Math.max(style.fillOpacity, .14) }));
      layer.on('mouseout', () => layer.setStyle(style));
      layer.on('click', () => {
        els.bezirk.value = district.label;
        renderReset();
      });
      layer.addTo(bezirkLayer);
    }
  }
}
function renderMap(rows, allFilteredRows) {
  const mapped = rows.filter(hasCoords);
  els.mapCount.textContent = n(mapped.length) + ' von ' + n(rows.length);
  if (!mapConsentGranted && !placesMap) {
    pendingMapRows = rows;
    pendingMapAllRows = allFilteredRows || rows;
    return;
  }
  if (typeof L === 'undefined' && !placesMap) {
    startMapLoad(rows, allFilteredRows);
    return;
  }
  if (!initMap()) return;
  renderBezirkLayer(allFilteredRows || rows);
  markerLayer.clearLayers();
  if (state.mode === 'nearby') mapped.sort((a, b) => rowDistanceKm(a) - rowDistanceKm(b));
  for (const row of mapped) {
    const marker = L.circleMarker([row.lat, row.lng], { pane: 'placeMarkerPane', radius: row.hasBanner ? 7 : 5, color: '#fff', weight: 1.5, fillColor: markerColor(row), fillOpacity: .9 });
    const distance = state.mode === 'nearby' ? ' · ' + distanceLabel(rowDistanceKm(row)) : '';
    marker.bindTooltip(esc(row.name + distance + (row.bezirkLabel ? ' · ' + row.bezirkLabel : '') + (row.hasBanner ? ' · ' + row.removedRange : '')), { direction: 'top' });
    marker.on('click', () => {
      activateMode(markerMode(row));
      render();
      requestAnimationFrame(() => focusEntry(row.id));
    });
    marker.addTo(markerLayer);
  }
  if (state.userLocation) {
    L.circleMarker([state.userLocation.lat, state.userLocation.lng], { pane: 'placeMarkerPane', radius: 9, color: '#fff', weight: 2, fillColor: '#1f6f8b', fillOpacity: .95 }).bindTooltip('Dein Standort', { direction: 'top' }).addTo(markerLayer);
  }
  if (state.mode === 'nearby' && state.userLocation) {
    const points = [[state.userLocation.lat, state.userLocation.lng], ...mapped.slice(0, 25).map(row => [row.lat, row.lng])];
    if (points.length > 1) placesMap.fitBounds(L.latLngBounds(points).pad(0.18), { maxZoom: 15, animate: false });
    else placesMap.setView([state.userLocation.lat, state.userLocation.lng], 14);
  } else if (mapped.length) {
    const bounds = L.latLngBounds(mapped.map(row => [row.lat, row.lng]));
    placesMap.fitBounds(bounds.pad(0.12), { maxZoom: 14, animate: false });
  } else {
    const districtBounds = bezirkBounds(selectedBezirkLabel());
    if (districtBounds) placesMap.fitBounds(districtBounds.pad(0.18), { maxZoom: 14, animate: false });
    else placesMap.setView([49.4521, 11.0767], 12);
  }
}
function renderBars(id, mode, rows, metric, label, color, maxValue) {
  const root = document.getElementById(id);
  const top = rows.slice(0, 8);
  const max = maxValue ?? Math.max(1, ...top.map(metric));
  root.innerHTML = top.map(row => '<a class="bar-row bar-link" href="#placesTable" data-mode="' + mode + '" data-entry-id="' + esc(row.id) + '"><div class="bar-name" title="' + esc(row.name) + '">' + esc(row.name) + '</div><div class="bar-value">' + label(row) + '</div><div class="track"><div class="fill ' + color + '" style="width:' + Math.max(2, Math.min(100, metric(row) / max * 100)) + '%"></div></div></a>').join('') || '<p>Keine Daten im Filter.</p>';
}
function renderDistribution(rows) {
  const banners = bannerRows(rows);
  const bins = ['über 250','201–250','151–200','101–150','51–100','21–50','11–20','6–10','2–5','1'];
  const counts = bins.map(bin => ({ bin, count: banners.filter(row => row.removedRange === bin).length })).filter(item => item.count > 0);
  const max = Math.max(1, ...counts.map(item => item.count));
  document.getElementById('distribution').innerHTML = counts.map(item => '<div class="dist-row"><strong>' + esc(item.bin) + '</strong><div class="dist-track"><div class="dist-fill" style="width:' + (item.count / max * 100) + '%"></div></div><span>' + n(item.count) + '</span></div>').join('') || '<p>Keine Hinweise im Filter.</p>';
}
function renderRatioHistogram(rows) {
  const root = document.getElementById('ratioHistogram');
  if (!root) return;
  const minReviews = 50;
  const scoped = bannerRows(rows).filter(row => Number.isFinite(row.deletionRatioPct) && Number.isFinite(row.reviewCount) && row.reviewCount >= minReviews);
  if (scoped.length === 0) {
    root.innerHTML = '<p>Keine passenden Orte im Filter.</p>';
    return;
  }
  const bins = [
    { label: '< 1%', min: -Infinity, max: 1 },
    { label: '1–2%', min: 1, max: 2 },
    { label: '2–5%', min: 2, max: 5 },
    { label: '5–10%', min: 5, max: 10 },
    { label: '10–20%', min: 10, max: 20 },
    { label: '≥ 20%', min: 20, max: Infinity }
  ];
  const counts = bins.map(bin => ({
    label: bin.label,
    count: scoped.filter(row => row.deletionRatioPct >= bin.min && row.deletionRatioPct < bin.max).length
  }));
  const max = Math.max(1, ...counts.map(item => item.count));
  root.innerHTML = counts.map(item => '<div class="dist-row"><strong>' + esc(item.label) + '</strong><div class="dist-track"><div class="dist-fill" style="width:' + (item.count / max * 100) + '%"></div></div><span>' + n(item.count) + '</span></div>').join('') + '<p class="dist-note">' + n(scoped.length) + ' Orte in dieser Einordnung.</p>';
}
function renderBezirkSummary(rows) {
  const groups = new Map();
  for (const row of rows) {
    const label = row.bezirkLabel || 'Ohne Bezirk';
    if (!groups.has(label)) groups.set(label, { label, rows: 0, banners: 0, removed: 0 });
    const group = groups.get(label);
    group.rows += 1;
    if (row.hasBanner) {
      group.banners += 1;
      group.removed += row.removedEstimate || 0;
    }
  }
  const items = [...groups.values()].sort((a, b) => (a.label === 'Ohne Bezirk') - (b.label === 'Ohne Bezirk') || (b.banners / Math.max(b.rows, 1)) - (a.banners / Math.max(a.rows, 1)) || b.banners - a.banners || b.rows - a.rows).slice(0, 12);
  document.getElementById('bezirkSummary').innerHTML = items.map(item => '<button type="button" class="bezirk-row" data-bezirk="' + esc(item.label) + '"><strong>' + esc(item.label) + '</strong><span>' + n(item.rows) + ' Orte</span><span>' + n(item.banners) + ' Hinweise</span><span>' + pct(item.banners / Math.max(item.rows, 1) * 100) + '</span></button>').join('') || '<p>Keine Bezirksdaten im Filter.</p>';
}
function renderParentSummary(rows) {
  const groups = new Map();
  for (const row of rows) {
    const label = row.parentCategory || 'Sonstiges';
    if (!groups.has(label)) groups.set(label, { label, rows: 0, banners: 0, removed: 0 });
    const group = groups.get(label);
    group.rows += 1;
    if (row.hasBanner) {
      group.banners += 1;
      group.removed += row.removedEstimate || 0;
    }
  }
  const items = [...groups.values()].sort((a, b) => (b.banners / Math.max(b.rows, 1)) - (a.banners / Math.max(a.rows, 1)) || b.banners - a.banners || b.rows - a.rows);
  document.getElementById('parentSummary').innerHTML = items.map(item => '<div class="bezirk-row"><strong>' + esc(item.label) + '</strong><span>' + n(item.rows) + ' Orte</span><span>' + n(item.banners) + ' Hinweise</span><span>' + pct(item.banners / Math.max(item.rows, 1) * 100) + '</span></div>').join('') || '<p>Keine Kategorie-Daten im Filter.</p>';
}
function updatePanels(rows) {
  const banners = bannerRows(rows);
  renderBars('barsRemoved', 'removed', [...banners].sort((a,b) => b.removedEstimate - a.removedEstimate || a.name.localeCompare(b.name, 'de')), row => row.removedEstimate, row => esc(row.removedRange), '', 300);
  renderBars('barsRatio', 'ratio', [...banners].sort((a,b) => (b.deletionRatioPct ?? -1) - (a.deletionRatioPct ?? -1)), row => row.deletionRatioPct || 0, row => pct(row.deletionRatioPct), '', Math.max(10, ...banners.map(row => row.deletionRatioPct || 0)));
  renderBars('barsClean', 'clean', [...cleanRankingRows(rows)].sort((a,b) => b.rating - a.rating || b.reviewCount - a.reviewCount), row => row.rating, row => rating(row.rating) + '★ · ' + n(row.reviewCount) + ' Rezensionen', 'green', 5);
  renderDistribution(rows);
  renderRatioHistogram(rows);
}
function renderTable(rows) {
  const scoped = modeRows(rows);
  const sorted = sortRows(scoped);
  const visible = sorted.slice(0, tableLimit);
  const clipped = visible.length < sorted.length;
  const shownText = clipped ? ' · ' + n(visible.length) + ' angezeigt' : '';
  lastSortedRows = sorted;
  if (state.mode === 'nearby' && state.userLocation) {
    const accuracy = Number.isFinite(state.userLocation.accuracy) ? ' · Standortgenauigkeit ca. ' + n(Math.round(state.userLocation.accuracy)) + ' m' : '';
    const sortText = state.sortKey === 'distanceKm' ? ' · sortiert nach Entfernung' : '';
    els.resultCount.textContent = n(sorted.length) + ' Orte mit Koordinaten im aktuellen Filter' + sortText + accuracy + shownText;
  } else {
    els.resultCount.textContent = n(sorted.length) + ' von ' + n(rows.length) + ' Orten im aktuellen Filter' + shownText;
  }
  els.tableTitle.textContent = titles[state.mode];
  els.showMore.hidden = !clipped;
  if (clipped) {
    const next = Math.min(TABLE_PAGE_SIZE, sorted.length - visible.length);
    els.showMore.textContent = 'Weitere ' + n(next) + ' Zeilen anzeigen (' + n(visible.length) + ' von ' + n(sorted.length) + ')';
  }
  els.tbody.innerHTML = visible.map((row, index) => {
    const rank = index + 1;
    const distance = state.mode === 'nearby' ? '<span class="entry-address">Entfernung: ' + esc(distanceLabel(rowDistanceKm(row))) + '</span>' : '';
    const address = row.address ? '<span class="entry-address">' + esc(row.address) + '</span>' : '';
    return '<tr data-entry-id="' + esc(row.id) + '"><td class="rank">' + rank + '</td><td class="name"><a href="' + esc(row.url) + '" target="_blank" rel="noopener noreferrer">' + esc(row.name) + '</a>' + distance + address + '</td><td>' + esc(row.bezirkLabel || '–') + '</td><td>' + esc(row.postcode) + '</td><td class="num">' + rating(row.rating) + '</td><td class="num">' + n(row.reviewCount) + '</td><td>' + (row.hasBanner ? '<span class="pill bad">Google-Hinweis</span>' : '<span class="pill">beim Abruf kein Hinweis</span>') + '</td><td class="num">' + (row.hasBanner ? esc(row.removedRange) : '–') + '</td><td class="num">' + (row.hasBanner ? rating(row.removedEstimate) : '–') + '</td><td class="num">' + pct(row.deletionRatioPct) + '</td><td>' + esc(readAtLabel(row.readAt)) + '</td><td>' + esc(row.category) + '</td></tr>';
  }).join('');
  document.querySelectorAll('th button[data-sort]').forEach(button => {
    const active = button.dataset.sort === state.sortKey;
    button.classList.toggle('active', active);
    button.querySelector('.arrow').textContent = active ? (state.sortDir === 'asc' ? '▲' : '▼') : '';
  });
}
function render() {
  const rows = filtered();
  updateKpis(rows);
  updatePanels(rows);
  renderBezirkSummary(rows);
  renderParentSummary(rows);
  updateSubChips(state.mode);
  renderMap(modeRows(rows), rows);
  renderTable(rows);
  updateFilterToggle();
}
function activateMode(mode) {
  resetTableLimit();
  state.mode = mode;
  [state.sortKey, state.sortDir] = defaultSortFor(mode);
  if (mode !== 'nearby' && !els.nearbyStatus.classList.contains('error')) setNearbyStatus('');
  document.querySelectorAll('.tab').forEach(tab => tab.classList.toggle('active', tab.dataset.mode === mode));
}
function focusEntry(entryId) {
  let row = Array.from(els.tbody.rows).find(tr => tr.dataset.entryId === entryId);
  if (!row) {
    const index = lastSortedRows.findIndex(item => item.id === entryId);
    if (index >= tableLimit) {
      tableLimit = Math.ceil((index + 1) / TABLE_PAGE_SIZE) * TABLE_PAGE_SIZE;
      renderTable(filtered());
      row = Array.from(els.tbody.rows).find(tr => tr.dataset.entryId === entryId);
    }
  }
  if (!row) return;
  document.querySelectorAll('tbody tr.target-row').forEach(tr => tr.classList.remove('target-row'));
  row.classList.add('target-row');
  row.scrollIntoView({ behavior: 'smooth', block: 'center' });
  window.setTimeout(() => row.classList.remove('target-row'), 2800);
}
document.querySelectorAll('.bars').forEach(root => root.addEventListener('click', event => {
  const link = event.target.closest('.bar-link');
  if (!link) return;
  event.preventDefault();
  activateMode(link.dataset.mode);
  render();
  requestAnimationFrame(() => focusEntry(link.dataset.entryId));
}));
document.getElementById('bezirkSummary').addEventListener('click', event => {
  const row = event.target.closest('.bezirk-row');
  if (!row) return;
  els.bezirk.value = row.dataset.bezirk === 'Ohne Bezirk' ? '__none__' : row.dataset.bezirk;
  renderReset();
  document.getElementById('placesTable').scrollIntoView({ behavior: 'smooth', block: 'start' });
});
document.querySelectorAll('.tab').forEach(button => button.addEventListener('click', () => {
  if (button.dataset.mode === 'nearby' && !state.userLocation) {
    requestUserLocation();
    return;
  }
  if (button.dataset.mode === 'nearby') setNearbyStatus(nearbySuccessStatus());
  activateMode(button.dataset.mode);
  render();
}));
document.querySelectorAll('th button[data-sort]').forEach(button => button.addEventListener('click', () => {
  const next = button.dataset.sort;
  if (next === state.sortKey) state.sortDir = state.sortDir === 'asc' ? 'desc' : 'asc';
  else { state.sortKey = next; state.sortDir = ['name','postcode','bezirkLabel','category','rank'].includes(next) ? 'asc' : 'desc'; }
  resetTableLimit();
  renderTable(filtered());
}));
els.showMore.addEventListener('click', () => {
  tableLimit += TABLE_PAGE_SIZE;
  renderTable(filtered());
});
els.themeToggle.addEventListener('click', () => setTheme(currentTheme() === 'dark' ? 'light' : 'dark'));
if (prefersDark.addEventListener) {
  prefersDark.addEventListener('change', () => {
    if (storedTheme() || document.documentElement.dataset.theme) return;
    updateThemeToggle();
    updateMapTheme();
    if (placesMap) render();
  });
}
els.filterToggle.addEventListener('click', () => {
  els.controls.classList.toggle('is-collapsed');
  updateFilterToggle();
});
const renderDebounced = debounce(renderReset, 140);
[els.search, els.minReviews].forEach(input => input.addEventListener('input', renderDebounced));
[els.postcode, els.bezirk, els.banner, els.range, els.category].forEach(input => input.addEventListener('change', renderReset));
els.reset.addEventListener('click', () => {
  els.search.value = ''; els.postcode.value = ''; els.bezirk.value = ''; els.banner.value = 'all'; els.range.value = ''; els.category.value = ''; els.minReviews.value = 0;
  activateMode('removed');
  document.querySelectorAll('.chip').forEach(c => c.classList.remove('active'));
  render();
});

document.querySelector('.chips').addEventListener('click', event => {
  const chip = event.target.closest('.chip');
  if (!chip) return;
  const action = chip.dataset.chip;
  const isActive = chip.classList.contains('active');
  document.querySelectorAll('.chip').forEach(c => c.classList.remove('active'));
  els.search.value = ''; els.postcode.value = ''; els.bezirk.value = ''; els.banner.value = 'all'; els.range.value = ''; els.category.value = ''; els.minReviews.value = 0;
  if (!isActive) {
    chip.classList.add('active');
    switch (action) {
      case 'banner': els.banner.value = 'banner'; break;
      case 'gastro': els.category.value = 'parent:Gastronomie'; break;
      case 'nachtleben': els.category.value = 'parent:Nachtleben'; break;
      case 'beauty': els.category.value = 'parent:Beauty & Wellness'; break;
      case 'hotels': els.category.value = 'parent:Beherbergung'; break;
      case 'gesundheit': els.category.value = 'parent:Gesundheit'; break;
      case 'altstadt': els.bezirk.value = 'Altstadt, St. Lorenz'; break;
    }
  }
  renderReset();
});

function updateSubChips(mode) {
  const configs = {
    all: [
      { label: '🔴 Mit Hinweis', test: () => els.banner.value === 'banner', apply: v => { els.banner.value = v ? 'banner' : 'all'; } },
      { label: '🗺️ Altstadt', test: () => els.bezirk.value === 'Altstadt, St. Lorenz', apply: v => { els.bezirk.value = v ? 'Altstadt, St. Lorenz' : ''; } },
    ],
    ratio: [
      { label: 'Min. 50 Rez.', test: () => Number(els.minReviews.value) === 50, apply: v => { els.minReviews.value = v ? 50 : 0; } },
      { label: 'Min. 100 Rez.', test: () => Number(els.minReviews.value) === 100, apply: v => { els.minReviews.value = v ? 100 : 0; } },
      { label: 'Min. 200 Rez.', test: () => Number(els.minReviews.value) === 200, apply: v => { els.minReviews.value = v ? 200 : 0; } },
      { label: '🍽️ Nur Gastro', test: () => els.category.value === 'parent:Gastronomie', apply: v => { els.category.value = v ? 'parent:Gastronomie' : ''; } },
      { label: '🎉 Nur Nachtleben', test: () => els.category.value === 'parent:Nachtleben', apply: v => { els.category.value = v ? 'parent:Nachtleben' : ''; } },
      { label: '💇 Nur Beauty', test: () => els.category.value === 'parent:Beauty & Wellness', apply: v => { els.category.value = v ? 'parent:Beauty & Wellness' : ''; } },
    ],
    removed: [
      { label: 'Min. 50 Rez.', test: () => Number(els.minReviews.value) === 50, apply: v => { els.minReviews.value = v ? 50 : 0; } },
      { label: 'Min. 100 Rez.', test: () => Number(els.minReviews.value) === 100, apply: v => { els.minReviews.value = v ? 100 : 0; } },
      { label: 'Min. 200 Rez.', test: () => Number(els.minReviews.value) === 200, apply: v => { els.minReviews.value = v ? 200 : 0; } },
      { label: '🍽️ Nur Gastro', test: () => els.category.value === 'parent:Gastronomie', apply: v => { els.category.value = v ? 'parent:Gastronomie' : ''; } },
      { label: '🏨 Nur Hotels', test: () => els.category.value === 'parent:Beherbergung', apply: v => { els.category.value = v ? 'parent:Beherbergung' : ''; } },
      { label: 'Spanne 51–100', test: () => els.range.value === '51–100', apply: v => { els.range.value = v ? '51–100' : ''; } },
    ],
    clean: [
      { label: 'Min. 200 Rez.', test: () => Number(els.minReviews.value) === 200, apply: v => { els.minReviews.value = v ? 200 : 0; } },
      { label: 'Min. 500 Rez.', test: () => Number(els.minReviews.value) === 500, apply: v => { els.minReviews.value = v ? 500 : 0; } },
      { label: '🍽️ Nur Gastro', test: () => els.category.value === 'parent:Gastronomie', apply: v => { els.category.value = v ? 'parent:Gastronomie' : ''; } },
      { label: '🏨 Nur Hotels', test: () => els.category.value === 'parent:Beherbergung', apply: v => { els.category.value = v ? 'parent:Beherbergung' : ''; } },
      { label: '🗺️ Altstadt', test: () => els.bezirk.value === 'Altstadt, St. Lorenz', apply: v => { els.bezirk.value = v ? 'Altstadt, St. Lorenz' : ''; } },
    ],
    nearby: [
      { label: '🔴 Nur mit Hinweis', test: () => els.banner.value === 'banner', apply: v => { els.banner.value = v ? 'banner' : 'all'; } },
      { label: '🍽️ Nur Gastro', test: () => els.category.value === 'parent:Gastronomie', apply: v => { els.category.value = v ? 'parent:Gastronomie' : ''; } },
    ],
  };
  const chips = configs[mode] || [];
  const root = document.getElementById('subChips');
  if (chips.length === 0) { root.innerHTML = ''; return; }
  root.innerHTML = chips.map((c, i) => '<button type="button" class="sub-chip' + (c.test() ? ' active' : '') + '" data-idx="' + i + '">' + c.label + '</button>').join('');
  root.querySelectorAll('.sub-chip').forEach(btn => btn.addEventListener('click', () => {
    const idx = Number(btn.dataset.idx);
    const chip = chips[idx];
    chip.apply(!chip.test());
    renderReset();
  }));
}

updateThemeToggle();
setupMapConsent();
render();
