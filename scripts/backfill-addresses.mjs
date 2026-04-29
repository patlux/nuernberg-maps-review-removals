#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';

const RESULTS_JSON = 'output/places.json';
const RESULTS_CSV = 'output/places.csv';

function parseArgs(argv) {
  const args = { input: RESULTS_JSON, headless: true, concurrency: 4, limit: 0, onlyMissing: true };
  for (let i = 0; i < argv.length; i += 1) {
    const [key, inline] = argv[i].split('=');
    const next = inline ?? argv[i + 1];
    const consume = inline === undefined && next && !next.startsWith('--');
    if (key === '--input') { args.input = next; if (consume) i += 1; }
    else if (key === '--headless') { args.headless = next === 'true'; if (consume) i += 1; }
    else if (key === '--concurrency') { args.concurrency = Math.max(1, Number(next) || 1); if (consume) i += 1; }
    else if (key === '--limit') { args.limit = Number(next) || 0; if (consume) i += 1; }
    else if (key === '--all') args.onlyMissing = false;
    else throw new Error(`Unknown argument: ${argv[i]}`);
  }
  return args;
}

function normalizeUrl(url) {
  try { const u = new URL(url); u.searchParams.set('hl', 'de'); return u.toString(); }
  catch { return url; }
}

function csvEscape(value) {
  if (value === null || value === undefined) return '';
  const s = String(value);
  return /[",\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
}

function writeCsv(file, rows) {
  const columns = ['id', 'name', 'postcode', 'address', 'rating', 'reviewCount', 'category', 'hasDefamationNotice', 'removedMin', 'removedMax', 'removedEstimate', 'deletionRatioPct', 'realRatingAdjusted', 'removedText', 'url', 'readAt', 'status', 'error'];
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, `${[columns.join(','), ...rows.map(row => columns.map(column => csvEscape(row[column])).join(','))].join('\n')}\n`);
}

function extractAddress(text) {
  const match = text.match(/Adresse:\s*([^\n]*\b9\d{4}\s+[^\n]*)/i)
    || text.match(/\n([^\n]*,\s*9\d{4}\s+[^\n]*)\n/i);
  return match?.[1]?.replace(/^Adresse:\s*/i, '').trim() || null;
}

async function acceptConsent(page) {
  for (const name of [/Alle akzeptieren/i, /Accept all/i, /Ich stimme zu/i]) {
    try { await page.getByRole('button', { name }).click({ timeout: 1500 }); return; } catch {}
  }
}

async function readAddress(page, url) {
  await page.goto(normalizeUrl(url), { waitUntil: 'domcontentloaded', timeout: 25000 });
  await acceptConsent(page);
  let address = null;
  for (let i = 0; i < 14 && !address; i += 1) {
    await page.waitForTimeout(i === 0 ? 1800 : 500);
    address = await page.evaluate(() => {
      const attrTexts = Array.from(document.querySelectorAll('[aria-label], [alt], [data-tooltip]'))
        .flatMap(el => [el.getAttribute('aria-label'), el.getAttribute('alt'), el.getAttribute('data-tooltip')])
        .filter(Boolean);
      const text = [document.body.innerText, ...attrTexts].join('\n');
      const match = text.match(/Adresse:\s*([^\n]*\b9\d{4}\s+[^\n]*)/i)
        || text.match(/\n([^\n]*,\s*9\d{4}\s+[^\n]*)\n/i);
      return match?.[1]?.replace(/^Adresse:\s*/i, '').trim() || null;
    });
  }
  return address;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const rows = JSON.parse(fs.readFileSync(args.input, 'utf8'));
  let todo = rows.filter(row => row.url && (!args.onlyMissing || !row.address));
  todo.sort((a, b) => Number(Boolean(b.hasDefamationNotice)) - Number(Boolean(a.hasDefamationNotice))
    || Number(b.removedEstimate || 0) - Number(a.removedEstimate || 0)
    || String(a.name || '').localeCompare(String(b.name || ''), 'de'));
  if (args.limit) todo = todo.slice(0, args.limit);
  console.log(`Backfilling addresses: ${todo.length} rows`);

  const save = () => {
    rows.sort((a, b) => String(a.postcode || '').localeCompare(String(b.postcode || '')) || String(a.name || '').localeCompare(String(b.name || '')));
    fs.writeFileSync(args.input, `${JSON.stringify(rows, null, 2)}\n`);
    writeCsv(RESULTS_CSV, rows);
  };

  const browser = await chromium.launch({ headless: args.headless });
  const context = await browser.newContext({
    locale: 'de-DE',
    timezoneId: 'Europe/Berlin',
    viewport: { width: 1440, height: 1100 },
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36'
  });

  let next = 0;
  let done = 0;
  let found = 0;
  async function worker(workerId) {
    const page = await context.newPage();
    while (next < todo.length) {
      const row = todo[next++];
      try {
        const address = await readAddress(page, row.url);
        if (address) {
          row.address = address;
          row.postcode = address.match(/\b9\d{4}\b/)?.[0] || row.postcode || null;
          found += 1;
        }
        done += 1;
        if (done % 10 === 0 || address) save();
        if (done % 25 === 0 || address) console.log(`[${done}/${todo.length}] ${address ? '✓' : '–'} ${row.name}${address ? ` — ${address}` : ''}`);
      } catch (error) {
        done += 1;
        console.warn(`[${done}/${todo.length}] ERROR ${row.name}: ${error?.message || error}`);
      }
    }
    await page.close();
  }

  await Promise.all(Array.from({ length: Math.min(args.concurrency, todo.length) }, (_, i) => worker(i + 1)));
  await browser.close();

  save();
  console.log(`Done. Found ${found} addresses. Total with address: ${rows.filter(row => row.address).length}`);
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
