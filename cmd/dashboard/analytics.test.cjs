const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { runInNewContext } = require('node:vm');

const source = readFileSync(join(__dirname, 'analytics.js'), 'utf8');
function visit(enabled = false, options = {}) {
  const storage = new Map(enabled ? [['dashboardAnalyticsConsent', 'yes']] : []);
  const requests = [];
  const events = {};
  const button = { addEventListener: (name, fn) => { events[name] = fn; } };
  const status = {};
  let ready;
  let reloads = 0;
  const config = { src: 'https://analytics.example.test/script.js', domain: options.domain ?? 'example.test' };
  const context = {
    document: {
      getElementById: id => id === 'analyticsConfig' ? { textContent: JSON.stringify(config) } : id === 'analyticsConsent' ? button : status,
      createElement: () => ({ dataset: {} }),
      head: { appendChild: script => requests.push(script) },
      addEventListener: (name, fn) => { assert.equal(name, 'DOMContentLoaded'); ready = fn; },
    },
    sessionStorage: {
      getItem: key => { if (options.storageBlocked) throw new Error('blocked'); return storage.get(key); },
      setItem: (key, value) => { if (options.storageBlocked) throw new Error('blocked'); storage.set(key, value); },
      removeItem: key => storage.delete(key),
    },
    location: { reload: () => { reloads++; } },
    window: {},
  };
  runInNewContext(source, context);
  ready();
  return { requests, storage, button, status, context, click: () => events.click(), reloads: () => reloads };
}

test('no analytics request before consent; explicit opt-in persists for reload', () => {
  const page = visit();
  assert.equal(page.requests.length, 0);
  assert.equal(page.button.textContent, 'Analytics erlauben');
  page.click();
  assert.equal(page.requests.length, 0);
  assert.equal(page.storage.get('dashboardAnalyticsConsent'), 'yes');
  assert.equal(page.reloads(), 1);
});

test('consented visit loads one script; revocation reloads without consent', () => {
  const page = visit(true);
  assert.equal(page.requests.length, 1);
  assert.equal(page.requests[0].dataset.domain, 'example.test');
  assert.equal(page.button.textContent, 'Analytics deaktivieren');
  page.click();
  assert.equal(page.storage.has('dashboardAnalyticsConsent'), false);
  assert.equal(page.reloads(), 1);
  assert.equal(visit(page.storage.has('dashboardAnalyticsConsent')).requests.length, 0);
});

test('unavailable session storage fails closed', () => {
  const page = visit(false, { storageBlocked: true });
  page.click();
  assert.equal(page.requests.length, 0);
  assert.equal(page.reloads(), 0);
  assert.match(page.status.textContent, /konnte nicht gespeichert/);
});

test('domainless Plausible API is only initialized after consent', () => {
  assert.equal(visit(false, { domain: '' }).context.window.plausible, undefined);
  const page = visit(true, { domain: '' });
  assert.equal(typeof page.context.window.plausible, 'function');
  assert.equal(page.requests.length, 1);
});
