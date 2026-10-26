const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { runInNewContext } = require('node:vm');

const source = readFileSync(join(__dirname, 'app.js'), 'utf8');
function section(start, end) {
  const from = source.indexOf(start);
  const to = source.indexOf(end, from);
  assert.ok(from >= 0 && to > from);
  return source.slice(from, to);
}
function harness() {
  const context = { COORD_SCALE: 100000, DASHBOARD_CITY: 'Nürnberg', state: { mode: 'scenario', sortKey: 'oneStarScenario', sortDir: 'asc' } };
  runInNewContext(section('function finiteNumber(', 'function getBezirke(') + section('function bannerRows(', 'function updateKpis('), context);
  return context;
}
const dictionaries = [['90402'], ['01 Altstadt'], ['Restaurant'], ['Gastronomie'], ['21–50']];
const observed = ['', 'Example', 0, 45000, 8000, 0, 4.5, 100, 0, 0, '', 0, 0, 35.5, 26.2];

test('scenario uses visible rating/count and the declared estimate, without claiming the lower bound', () => {
  const h = harness();
  const row = h.decodePlaceRow(observed, 0, dictionaries);
  assert.equal(typeof row.oneStarScenario, 'number');
  assert.ok(Math.abs(row.oneStarScenario - (4.5 * 100 + 35.5) / 135.5) < 1e-12);
  assert.equal(row.oneStarScenario.toFixed(2), '3.58');
  const upper = h.decodePlaceRow([...observed.slice(0, 13), 50, 33.3], 1, dictionaries);
  assert.equal(upper.oneStarScenario.toFixed(2), '3.33');
  assert.ok(upper.oneStarScenario < row.oneStarScenario);
});

test('open-ended range uses the declared 300 input, not an old stored rating', () => {
  const h = harness();
  const row = h.decodePlaceRow([...observed.slice(0, 13), 300, 75, 1.111], 0, [...dictionaries.slice(0, 4), ['über 250']]);
  assert.equal(row.oneStarScenario, 1.875);
});

test('no scenario for absent notices or unavailable/invalid calculation inputs', () => {
  const h = harness();
  const rows = [observed.slice(0, 12)];
  for (const [index, value] of [[6, null], [6, 6], [7, 0], [7, null], [13, null], [13, 0], [13, -1]]) {
    const row = [...observed]; row[index] = value; rows.push(row);
  }
  for (const row of rows) assert.equal(h.decodePlaceRow(row, 0, dictionaries).oneStarScenario, null);
  const unknownRange = [...dictionaries.slice(0, 4), ['']];
  assert.equal(h.decodePlaceRow(observed, 0, unknownRange).oneStarScenario, null);
});

test('scenario mode excludes missing values and sorts numeric results ascending', () => {
  const h = harness();
  assert.equal(JSON.stringify(h.defaultSortFor('scenario')), JSON.stringify(['oneStarScenario', 'asc']));
  const rows = [
    { name: 'B', hasBanner: true, removedEstimate: 20, oneStarScenario: 3.8 },
    { name: 'A', hasBanner: true, removedEstimate: 20, oneStarScenario: 2.9 },
    { name: 'C', hasBanner: false, removedEstimate: 0, oneStarScenario: null },
    { name: 'D', hasBanner: true, removedEstimate: 20, oneStarScenario: null },
  ];
  assert.equal(h.sortRows(h.modeRows(rows)).map(row => row.name).join(','), 'A,B');
  h.state.mode = 'all';
  assert.equal(h.sortRows(rows).map(row => row.name).join(','), 'A,B,C,D');
  h.state.sortDir = 'desc';
  assert.equal(h.sortRows(rows).map(row => row.name).join(','), 'B,A,C,D');
});

test('overview uses scenario mode and table renders the model value', () => {
  const h = harness();
  const calls = [];
  h.renderBars = (...args) => calls.push(args);
  h.renderDistribution = h.renderRatioHistogram = () => {};
  h.rating = (value, digits = 1) => Number.isFinite(value) ? value.toFixed(digits) : '–';
  h.esc = String;
  h.pct = String;
  h.n = String;
  h.MIN_CLEAN_RANKING_REVIEWS = 100;
  runInNewContext(section('function updatePanels(', 'function renderTable('), h);
  const row = h.decodePlaceRow(observed, 0, dictionaries);
  h.updatePanels([row]);
  const scenario = calls.find(([id]) => id === 'barsScenario');
  assert.ok(scenario);
  assert.equal(scenario[1], 'scenario');
  assert.equal(scenario[4](row), 'Google 4.5★ · Szenario 3.58★');

  h.tableLimit = 500;
  h.TABLE_PAGE_SIZE = 500;
  h.lastSortedRows = [];
  h.titles = { scenario: '1-Stern-Szenario' };
  h.els = { resultCount: {}, tableTitle: {}, showMore: {}, tbody: {} };
  h.readAtLabel = () => '13.05.2026';
  h.document = { querySelectorAll: () => [] };
  runInNewContext(section('function renderTable(', 'function render()'), h);
  h.renderTable([row]);
  assert.equal(h.els.tableTitle.textContent, '1-Stern-Szenario');
  assert.match(h.els.tbody.innerHTML, /3\.58/);
  assert.equal((h.els.tbody.innerHTML.match(/<td\b/g) || []).length, 13);
});
