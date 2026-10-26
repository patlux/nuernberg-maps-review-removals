const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { runInNewContext } = require('node:vm');

const app = readFileSync(join(__dirname, 'app.js'), 'utf8');
const start = app.indexOf('function loadLeaflet()');
const end = app.indexOf('function selectedBezirkLabel()');
assert.ok(start > 0 && end > start);
const source = app.slice(start, end);

function mapHarness(enabled) {
  const requests = [];
  const handlers = {};
  const root = { innerHTML: 'consent-placeholder', classList: { remove() {} }, addEventListener: (name, fn) => { handlers.load = fn; }, querySelector: () => ({ focus() {} }) };
  const revoke = { hidden: true, addEventListener: (name, fn) => { handlers.revoke = fn; } };
  let removed = 0;
  let gestureBindings = 0;
  const map = { setView() { return this; }, createPane() {}, getPane: () => ({ style: {} }), remove: () => { removed++; } };
  const layer = () => ({ addTo() { return this; } });
  const context = {
    placesMap: null, bezirkLayer: null, markerLayer: null, tileLayer: null,
    mapUnavailable: false, mapLoadStarted: false, mapConsentGranted: false,
    pendingMapRows: [], pendingMapAllRows: [],
    CONFIG: { cartoEnabled: String(enabled) },
    tileURL: () => 'https://tiles.example.test/test.png',
    setupMapGestureGate: () => { gestureBindings++; },
    document: { getElementById: id => id === 'placesMap' ? root : revoke },
    L: { map: () => map, layerGroup: layer, tileLayer: (url, options) => { requests.push({ url, options }); return layer(); } },
  };
  runInNewContext(source, context);
  context.renderMap = () => context.initMap();
  context.setupMapConsent();
  return { context, root, revoke, requests, removed: () => removed, bindings: () => gestureBindings, load: () => handlers.load({ target: { closest: () => ({}) } }), disable: () => handlers.revoke() };
}

test('local map does not request external tiles, including after reactivation', async () => {
  const h = mapHarness(false);
  assert.equal(h.context.placesMap, null);
  h.load();
  await Promise.resolve();
  assert.notEqual(h.context.placesMap, null);
  assert.equal(h.requests.length, 0);
  assert.equal(h.revoke.hidden, false);
  h.disable();
  assert.equal(h.context.placesMap, null);
  assert.equal(h.context.mapConsentGranted, false);
  assert.equal(h.root.innerHTML, 'consent-placeholder');
  assert.equal(h.removed(), 1);
  h.load();
  await Promise.resolve();
  assert.equal(h.requests.length, 0);
  assert.equal(h.bindings(), 1);
});

test('configured external map requests tiles only after activation and tears down on withdrawal', async () => {
  const h = mapHarness(true);
  assert.equal(h.requests.length, 0);
  h.load();
  await Promise.resolve();
  assert.equal(h.requests.length, 1);
  assert.match(h.requests[0].options.attribution, /openstreetmap.org\/copyright/);
  h.disable();
  assert.equal(h.removed(), 1);
  assert.equal(h.context.tileLayer, null);
  assert.equal(h.context.mapLoadStarted, false);
});
