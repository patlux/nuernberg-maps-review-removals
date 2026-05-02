const DATA = JSON.parse(document.getElementById('placesData').textContent);
const BEZIRKE = JSON.parse(document.getElementById('bezirkData').textContent);
const valid = DATA.filter(row => Number.isFinite(row.rating) && Number.isFinite(row.reviewCount));
const fmt = new Intl.NumberFormat('de-DE');
const fmt1 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 1, minimumFractionDigits: 1 });
const fmt2 = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2, minimumFractionDigits: 2 });
const fmtDateTime = new Intl.DateTimeFormat('de-DE', { dateStyle: 'short', timeStyle: 'short' });
const state = { mode: 'ratio', sortKey: 'deletionRatioPct', sortDir: 'desc', userLocation: null };
const els = {
  themeToggle: document.getElementById('themeToggle'), controls: document.getElementById('dashboardFilterControls'), filterToggle: document.getElementById('filterToggle'), filterSummary: document.getElementById('filterSummary'), search: document.getElementById('searchInput'), postcode: document.getElementById('postcodeFilter'), bezirk: document.getElementById('bezirkFilter'), banner: document.getElementById('bannerFilter'), range: document.getElementById('rangeFilter'), minReviews: document.getElementById('minReviews'), reset: document.getElementById('resetFilters'), tbody: document.querySelector('#placesTable tbody'), resultCount: document.getElementById('resultCount'), tableTitle: document.getElementById('tableTitle'), mapCount: document.getElementById('mapCount'), nearbyStatus: document.getElementById('nearbyStatus')
};
const titles = { all: 'Alle Orte', removed: 'Meiste entfernte Bewertungen', ratio: 'Höchste Lösch-Quote', worst: 'Schlechtestes Worst-Case-Rating', clean: 'Orte ohne Löschbanner', nearby: 'In meiner Nähe' };
let placesMap = null;
let bezirkLayer = null;
let markerLayer = null;
let tileLayer = null;
let mapUnavailable = false;
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
function tileURL() { return 'https://{s}.basemaps.cartocdn.com/' + (currentTheme() === 'dark' ? 'dark_all' : 'light_all') + '/{z}/{x}/{y}{r}.png'; }
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
  const timestamp = Date.parse(value || '');
  return Number.isFinite(timestamp) ? fmtDateTime.format(new Date(timestamp)) : '–';
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
function searchText(row) { return [row.name, row.postcode, row.bezirkLabel, row.category, row.removedRange, row.removedText, row.address].join(' ').toLowerCase(); }
function matches(row) {
  const q = els.search.value.trim().toLowerCase();
  if (q && !searchText(row).includes(q)) return false;
  if (els.postcode.value && row.postcode !== els.postcode.value) return false;
  if (els.bezirk.value === '__none__' && row.bezirkLabel) return false;
  if (els.bezirk.value && els.bezirk.value !== '__none__' && row.bezirkLabel !== els.bezirk.value) return false;
  if (els.banner.value === 'banner' && !row.hasBanner) return false;
  if (els.banner.value === 'clean' && row.hasBanner) return false;
  if (els.range.value && row.removedRange !== els.range.value) return false;
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
  return 'Standort gesetzt. Orte werden nach Entfernung sortiert.' + accuracy;
}
function requestUserLocation() {
  if (!navigator.geolocation) {
    setNearbyStatus('Standortzugriff ist nur per HTTPS oder localhost verfügbar.', true);
    return;
  }
  setNearbyPending(true);
  setNearbyStatus('Standort wird angefragt und nur lokal im Browser verwendet …');
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
  if (mode === 'worst') return ['realRatingAdjusted', 'asc'];
  if (mode === 'clean') return ['rating', 'desc'];
  if (mode === 'nearby') return ['distanceKm', 'asc'];
  return ['rank', 'asc'];
}
function modeRows(rows) {
  if (state.mode === 'removed') return bannerRows(rows);
  if (state.mode === 'ratio') return bannerRows(rows).filter(row => Number.isFinite(row.deletionRatioPct));
  if (state.mode === 'worst') return bannerRows(rows).filter(row => Number.isFinite(row.realRatingAdjusted));
  if (state.mode === 'clean') return cleanRows(rows);
  if (state.mode === 'nearby') return rows.filter(hasCoords);
  return rows;
}
function value(row, key, index) {
  if (key === 'rank') return index + 1;
  if (key === 'hasBanner') return row.hasBanner ? 1 : 0;
  if (key === 'distanceKm') return rowDistanceKm(row);
  if (key === 'readAt') return Date.parse(row.readAt || '') || 0;
  const v = row[key];
  return typeof v === 'string' ? v.toLowerCase() : (Number.isFinite(v) ? v : -Infinity);
}
function sortRows(rows) {
  return rows.map((row, index) => ({ row, index })).sort((a, b) => {
    const av = value(a.row, state.sortKey, a.index);
    const bv = value(b.row, state.sortKey, b.index);
    const result = typeof av === 'string' ? av.localeCompare(bv, 'de') : av - bv;
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
  if (!row.hasBanner) return colorVar('--green', '#2d7b3f');
  if ((row.deletionRatioPct || 0) >= 10) return colorVar('--red', '#c9332c');
  return colorVar('--orange', '#ef7d16');
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
function initMap() {
  if (placesMap || mapUnavailable) return Boolean(placesMap);
  const root = document.getElementById('placesMap');
  if (typeof L === 'undefined') {
    mapUnavailable = true;
    root.innerHTML = '<div class="map-empty">Karte konnte nicht geladen werden. Internetzugriff auf Leaflet/Kartenkacheln prüfen.</div>';
    return false;
  }
  root.innerHTML = '';
  placesMap = L.map(root, { scrollWheelZoom: false, touchZoom: true }).setView([49.4521, 11.0767], 12);
  placesMap.createPane('bezirkPane');
  placesMap.getPane('bezirkPane').style.zIndex = 350;
  placesMap.createPane('placeMarkerPane');
  placesMap.getPane('placeMarkerPane').style.zIndex = 650;
  tileLayer = L.tileLayer(tileURL(), { maxZoom: 20, subdomains: 'abcd', attribution: '&copy; OpenStreetMap-Mitwirkende &copy; CARTO' }).addTo(placesMap);
  bezirkLayer = L.layerGroup().addTo(placesMap);
  markerLayer = L.layerGroup().addTo(placesMap);
  setupMapGestureGate(root);
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
  const district = BEZIRKE.find(item => item.label === label);
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
  for (const district of BEZIRKE) {
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
    const tooltip = '<strong>' + esc(district.label) + '</strong><br>' + n(stat.rows) + ' Orte · ' + n(stat.banners) + ' Banner · ' + pct(ratio);
    for (const polygon of district.polygons) {
      const layer = L.polygon(polygon, style);
      layer.bindTooltip(tooltip, { sticky: true, direction: 'top' });
      layer.on('mouseover', () => layer.setStyle({ weight: Math.max(style.weight, 3), color: red, fillOpacity: Math.max(style.fillOpacity, .14) }));
      layer.on('mouseout', () => layer.setStyle(style));
      layer.on('click', () => {
        els.bezirk.value = district.label;
        render();
      });
      layer.addTo(bezirkLayer);
    }
  }
}
function renderMap(rows, allFilteredRows) {
  if (!initMap()) return;
  renderBezirkLayer(allFilteredRows || rows);
  markerLayer.clearLayers();
  const mapped = rows.filter(hasCoords);
  if (state.mode === 'nearby') mapped.sort((a, b) => rowDistanceKm(a) - rowDistanceKm(b));
  els.mapCount.textContent = n(mapped.length) + ' von ' + n(rows.length);
  for (const row of mapped) {
    const marker = L.circleMarker([row.lat, row.lng], { pane: 'placeMarkerPane', radius: row.hasBanner ? 7 : 5, color: '#fff', weight: 1.5, fillColor: markerColor(row), fillOpacity: .9 });
    const distance = state.mode === 'nearby' ? ' · ' + distanceLabel(rowDistanceKm(row)) : '';
    marker.bindTooltip(row.name + distance + (row.bezirkLabel ? ' · ' + row.bezirkLabel : '') + (row.hasBanner ? ' · ' + row.removedRange : ''), { direction: 'top' });
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
  document.getElementById('distribution').innerHTML = counts.map(item => '<div class="dist-row"><strong>' + esc(item.bin) + '</strong><div class="dist-track"><div class="dist-fill" style="width:' + (item.count / max * 100) + '%"></div></div><span>' + n(item.count) + '</span></div>').join('') || '<p>Keine Banner im Filter.</p>';
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
  document.getElementById('bezirkSummary').innerHTML = items.map(item => '<button type="button" class="bezirk-row" data-bezirk="' + esc(item.label) + '"><strong>' + esc(item.label) + '</strong><span>' + n(item.rows) + ' Orte</span><span>' + n(item.banners) + ' Banner</span><span>' + pct(item.banners / Math.max(item.rows, 1) * 100) + '</span></button>').join('') || '<p>Keine Bezirksdaten im Filter.</p>';
}
function updatePanels(rows) {
  const banners = bannerRows(rows);
  renderBars('barsRemoved', 'removed', [...banners].sort((a,b) => b.removedEstimate - a.removedEstimate), row => row.removedEstimate, row => row.removedRange + ' · ' + n(row.removedEstimate), '', 300);
  renderBars('barsRatio', 'ratio', [...banners].sort((a,b) => (b.deletionRatioPct ?? -1) - (a.deletionRatioPct ?? -1)), row => row.deletionRatioPct || 0, row => pct(row.deletionRatioPct), '', Math.max(10, ...banners.map(row => row.deletionRatioPct || 0)));
  renderBars('barsWorst', 'worst', [...banners].filter(row => Number.isFinite(row.realRatingAdjusted)).sort((a,b) => a.realRatingAdjusted - b.realRatingAdjusted), row => 5 - row.realRatingAdjusted, row => rating(row.rating) + '★ → ' + rating(row.realRatingAdjusted, 2) + '★', 'orange', 4);
  renderBars('barsClean', 'clean', [...cleanRankingRows(rows)].sort((a,b) => b.rating - a.rating || b.reviewCount - a.reviewCount), row => row.rating, row => rating(row.rating) + '★ · ' + n(row.reviewCount) + ' Rezensionen', 'green', 5);
  renderDistribution(rows);
}
function renderTable(rows) {
  const scoped = modeRows(rows);
  const sorted = sortRows(scoped);
  if (state.mode === 'nearby' && state.userLocation) {
    const accuracy = Number.isFinite(state.userLocation.accuracy) ? ' · Standortgenauigkeit ca. ' + n(Math.round(state.userLocation.accuracy)) + ' m' : '';
    const sortText = state.sortKey === 'distanceKm' ? ' · sortiert nach Entfernung' : '';
    els.resultCount.textContent = n(sorted.length) + ' Orte mit Koordinaten im aktuellen Filter' + sortText + accuracy;
  } else {
    els.resultCount.textContent = n(sorted.length) + ' von ' + n(rows.length) + ' Orten im aktuellen Filter';
  }
  els.tableTitle.textContent = titles[state.mode];
  els.tbody.innerHTML = sorted.map((row, index) => {
    const distance = state.mode === 'nearby' ? '<span class="entry-address">Entfernung: ' + esc(distanceLabel(rowDistanceKm(row))) + '</span>' : '';
    const address = row.address ? '<span class="entry-address">' + esc(row.address) + '</span>' : '';
    return '<tr data-entry-id="' + esc(row.id) + '"><td class="rank">' + (index + 1) + '</td><td class="name"><a href="' + esc(row.url) + '" target="_blank" rel="noopener noreferrer">' + esc(row.name) + '</a>' + distance + address + '</td><td>' + esc(row.bezirkLabel || '–') + '</td><td>' + esc(row.postcode) + '</td><td class="num">' + rating(row.rating) + '</td><td class="num">' + n(row.reviewCount) + '</td><td>' + (row.hasBanner ? '<span class="pill bad">Löschbanner</span>' : '<span class="pill">kein Löschbanner</span>') + '</td><td class="num">' + (row.hasBanner ? esc(row.removedRange) : '–') + '</td><td class="num">' + (row.hasBanner ? rating(row.removedEstimate) : '–') + '</td><td class="num">' + pct(row.deletionRatioPct) + '</td><td class="num">' + rating(row.realRatingAdjusted, 2) + '</td><td>' + esc(readAtLabel(row.readAt)) + '</td><td>' + esc(row.category) + '</td></tr>';
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
  renderMap(modeRows(rows), rows);
  renderTable(rows);
  updateFilterToggle();
}
function activateMode(mode) {
  state.mode = mode;
  [state.sortKey, state.sortDir] = defaultSortFor(mode);
  if (mode !== 'nearby' && !els.nearbyStatus.classList.contains('error')) setNearbyStatus('');
  document.querySelectorAll('.tab').forEach(tab => tab.classList.toggle('active', tab.dataset.mode === mode));
}
function focusEntry(entryId) {
  const row = Array.from(els.tbody.rows).find(tr => tr.dataset.entryId === entryId);
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
  render();
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
  renderTable(filtered());
}));
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
[els.search, els.postcode, els.bezirk, els.banner, els.range, els.minReviews].forEach(input => {
  input.addEventListener('input', render);
  input.addEventListener('change', render);
});
els.reset.addEventListener('click', () => {
  els.search.value = ''; els.postcode.value = ''; els.bezirk.value = ''; els.banner.value = 'all'; els.range.value = ''; els.minReviews.value = 0;
  activateMode('ratio');
  render();
});
updateThemeToggle();
render();
