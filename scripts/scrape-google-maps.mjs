#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';

const NUREMBERG_POSTCODES = [
  '90402', '90403', '90408', '90409', '90411', '90419', '90425', '90427',
  '90429', '90431', '90439', '90441', '90443', '90449', '90451', '90453',
  '90455', '90459', '90461', '90469', '90471', '90473', '90475', '90478',
  '90480', '90482', '90489', '90491'
];

const DEFAULT_QUERIES = ['restaurant', 'café', 'imbiss', 'pizzeria', 'bäckerei'];
const OUTPUT_DIR = 'output';
const RESULTS_JSON = path.join(OUTPUT_DIR, 'places.json');
const RESULTS_CSV = path.join(OUTPUT_DIR, 'places.csv');

function parseArgs(argv) {
  const args = {
    postcodes: 'all',
    queries: DEFAULT_QUERIES.join(','),
    maxResults: 0,
    headless: false,
    discoveryOnly: false,
    scrapeOnly: false,
    delayMin: 2500,
    delayMax: 6000,
    out: RESULTS_JSON
  };

  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    const [key, inline] = arg.split('=');
    const next = inline ?? argv[i + 1];
    const consume = inline === undefined && next && !next.startsWith('--');

    switch (key) {
      case '--postcodes': args.postcodes = next; if (consume) i += 1; break;
      case '--queries': args.queries = next; if (consume) i += 1; break;
      case '--max-results': args.maxResults = Number(next); if (consume) i += 1; break;
      case '--delay-min': args.delayMin = Number(next); if (consume) i += 1; break;
      case '--delay-max': args.delayMax = Number(next); if (consume) i += 1; break;
      case '--out': args.out = next; if (consume) i += 1; break;
      case '--headless': args.headless = next === 'true'; if (consume) i += 1; break;
      case '--discovery-only': args.discoveryOnly = true; break;
      case '--scrape-only': args.scrapeOnly = true; break;
      case '--help': printHelpAndExit(); break;
      default:
        throw new Error(`Unknown argument: ${arg}`);
    }
  }

  args.postcodes = args.postcodes === 'all'
    ? NUREMBERG_POSTCODES
    : args.postcodes.split(',').map(s => s.trim()).filter(Boolean);
  args.queries = args.queries.split(',').map(s => s.trim()).filter(Boolean);
  args.out = args.out || RESULTS_JSON;
  return args;
}

function printHelpAndExit() {
  console.log(`Usage:
  npm run scrape -- --postcodes all --headless false
  npm run scrape -- --postcodes 90402,90403 --queries restaurant,café,imbiss

Options:
  --postcodes <all|csv>     Nürnberg PLZ list. Default: all known Nürnberg PLZ.
  --queries <csv>           Google Maps search terms. Default: ${DEFAULT_QUERIES.join(',')}.
  --max-results <n>         Stop after n discovered places. 0 = unlimited.
  --headless <true|false>   Headless browser. Default: false; safer for consent/CAPTCHA.
  --discovery-only          Only create/update output/discovery.json.
  --scrape-only             Skip discovery; scrape output/discovery.json.
  --delay-min <ms>          Minimum delay between place pages. Default: 2500.
  --delay-max <ms>          Maximum delay between place pages. Default: 6000.
  --out <path>              Results JSON path. Default: output/places.json.
`);
  process.exit(0);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function randomDelay(min, max) {
  if (!max || max < min) return min;
  return min + Math.floor(Math.random() * (max - min + 1));
}

function ensureDir(fileOrDir) {
  const dir = path.extname(fileOrDir) ? path.dirname(fileOrDir) : fileOrDir;
  fs.mkdirSync(dir, { recursive: true });
}

function readJson(file, fallback) {
  if (!fs.existsSync(file)) return fallback;
  return JSON.parse(fs.readFileSync(file, 'utf8'));
}

function writeJson(file, value) {
  ensureDir(file);
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}

function csvEscape(value) {
  if (value === null || value === undefined) return '';
  const s = String(value);
  return /[",\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
}

function writeCsv(file, rows) {
  const columns = [
    'id', 'name', 'postcode', 'address', 'rating', 'reviewCount', 'category',
    'hasDefamationNotice', 'removedMin', 'removedMax', 'removedEstimate',
    'deletionRatioPct', 'realRatingAdjusted', 'removedText', 'url', 'readAt',
    'status', 'error'
  ];
  const lines = [columns.join(',')];
  for (const row of rows) {
    lines.push(columns.map(column => csvEscape(row[column])).join(','));
  }
  ensureDir(file);
  fs.writeFileSync(file, `${lines.join('\n')}\n`);
}

function normalizeUrl(url) {
  try {
    const u = new URL(url);
    u.searchParams.set('hl', 'de');
    return u.toString();
  } catch {
    return url;
  }
}

function placeIdFromUrl(url) {
  const decoded = decodeURIComponent(url);
  const id = decoded.match(/!1s([^!]+)/)?.[1];
  if (id) return id;
  const place = decoded.match(/\/maps\/place\/([^/@?]+)/)?.[1];
  const coords = decoded.match(/@([-0-9.]+,[-0-9.]+)/)?.[1];
  return `${place || 'place'}:${coords || url}`;
}

function parseGermanNumber(value) {
  if (value === null || value === undefined) return null;
  const s = String(value).trim().replace(/\./g, '').replace(',', '.');
  const n = Number(s);
  return Number.isFinite(n) ? n : null;
}

function parseNotice(text) {
  const compact = text.replace(/\s+/g, ' ').trim();
  if (!/diffamierung/i.test(compact) || !/entfernt/i.test(compact)) return null;

  const words = new Map([
    ['eine', 1], ['ein', 1], ['einer', 1], ['zwei', 2], ['drei', 3], ['vier', 4],
    ['fünf', 5], ['fuenf', 5], ['sechs', 6], ['sieben', 7], ['acht', 8],
    ['neun', 9], ['zehn', 10]
  ]);
  const number = token => {
    if (!token) return null;
    const normalized = token.toLowerCase().replaceAll('ü', 'ue');
    if (words.has(token.toLowerCase())) return words.get(token.toLowerCase());
    if (words.has(normalized)) return words.get(normalized);
    return parseGermanNumber(token);
  };

  const notice = compact.match(/((?:über|mehr als)\s+\d+[.]?\d*\s+Bewertung(?:en)?.{0,160}?Diffamierung.{0,80}?entfernt\.?)/i)?.[1]
    || compact.match(/((?:eine|ein|einer|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn|\d+[.]?\d*)\s+(?:bis\s+(?:eine|ein|einer|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn|\d+[.]?\d*)\s+)?Bewertung(?:en)?.{0,160}?Diffamierung.{0,80}?entfernt\.?)/i)?.[1]
    || compact.match(/(.{0,80}Diffamierung.{0,80}entfernt\.?)/i)?.[1]
    || '';

  let min = null;
  let max = null;

  let m = notice.match(/(?:über|mehr als)\s+(\d+[.]?\d*)\s+Bewertung(?:en)?/i);
  if (m) {
    min = parseGermanNumber(m[1]);
    max = null;
  }

  m = notice.match(/((?:eine|ein|einer|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn|\d+[.]?\d*))\s+bis\s+((?:eine|ein|einer|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn|\d+[.]?\d*))\s+Bewertung(?:en)?/i);
  if (m) {
    min = number(m[1]);
    max = number(m[2]);
  }

  if (min === null) {
    m = notice.match(/(eine|ein|einer|zwei|drei|vier|fünf|fuenf|sechs|sieben|acht|neun|zehn|\d+[.]?\d*)\s+Bewertung(?:en)?/i);
    if (m) {
      min = number(m[1]);
      max = min;
    }
  }

  if (min === null && /eine\s+bewertung/i.test(notice)) {
    min = 1;
    max = 1;
  }

  if (min === null) return null;

  const estimate = max === null ? min + 50 : (min + max) / 2;
  return { text: notice.trim(), min, max, estimate };
}

function computeMetrics(row) {
  row.deletionRatioPct = null;
  row.realRatingAdjusted = null;
  if (!row.removedEstimate || !row.reviewCount || !row.rating) return row;
  const total = row.reviewCount + row.removedEstimate;
  row.deletionRatioPct = round((row.removedEstimate / total) * 100, 2);
  row.realRatingAdjusted = round(((row.rating * row.reviewCount) + row.removedEstimate) / total, 3);
  return row;
}

function parsePlaceStats(text) {
  const ratingMatch = text.match(/(?:^|\n|\s)([1-5][,.][0-9])\s*(?:Sterne|stars|★)/i)
    || text.match(/(?:^|\n|\s)([1-5][,.][0-9])\s*\n\s*(?:\(?[\d.]+\)?\s*)?(?:Rezensionen|Berichte)/i)
    || text.match(/\b([1-5][,.][0-9])\b/i);

  const reviewMatches = [...text.matchAll(/(?:^|\n|\s|\()([0-9][0-9.]*)\)?\s*(?:Rezensionen|Berichte)\b/gi)]
    .map(match => parseGermanNumber(match[1]))
    .filter(value => value !== null && value >= 0);

  return {
    rating: parseGermanNumber(ratingMatch?.[1]),
    reviewCount: reviewMatches[0] ?? null
  };
}

function extractAddress(text) {
  const match = text.match(/Adresse:\s*([^\n]*\b9\d{4}\s+[^\n]*)/i)
    || text.match(/\n([^\n]*,\s*9\d{4}\s+[^\n]*)\n/i);
  return match?.[1]?.replace(/^Adresse:\s*/i, '').trim() || null;
}

async function readMapText(page) {
  return page.evaluate(() => {
    const attrTexts = Array.from(document.querySelectorAll('[aria-label], [alt], [data-tooltip]'))
      .flatMap(el => [el.getAttribute('aria-label'), el.getAttribute('alt'), el.getAttribute('data-tooltip')])
      .filter(Boolean);
    return {
      title: document.title,
      h1: document.querySelector('h1')?.textContent?.trim() || '',
      text: [document.body.innerText, ...attrTexts].join('\n')
    };
  });
}

function round(value, digits) {
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
}

async function acceptConsent(page) {
  const buttons = [
    /Alle akzeptieren/i,
    /Ich stimme zu/i,
    /Akzeptieren/i,
    /Accept all/i,
    /I agree/i
  ];
  for (const name of buttons) {
    try {
      const button = page.getByRole('button', { name }).first();
      if (await button.isVisible({ timeout: 1200 })) {
        await button.click({ timeout: 3000 });
        await page.waitForTimeout(1000);
        return;
      }
    } catch {
      // Try next consent label.
    }
  }
}

async function discoverPlaces(page, args) {
  const discoveryFile = path.join(OUTPUT_DIR, 'discovery.json');
  const existing = readJson(discoveryFile, []);
  const seen = new Map(existing.map(place => [place.id, place]));

  for (const postcode of args.postcodes) {
    for (const query of args.queries) {
      if (args.maxResults && seen.size >= args.maxResults) break;
      const search = `${query} ${postcode} Nürnberg`;
      const url = `https://www.google.com/maps/search/${encodeURIComponent(search)}?hl=de`;
      console.log(`\nDiscover: ${search}`);
      await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 });
      await acceptConsent(page);
      await page.waitForTimeout(3000);

      for (let pass = 0, stagnant = 0, lastCount = -1; pass < 45; pass += 1) {
        const places = await page.evaluate(() => {
          const anchors = Array.from(document.querySelectorAll('a[href*="/maps/place/"]'));
          return anchors.map(a => ({
            url: a.href,
            name: (a.getAttribute('aria-label') || a.textContent || '').split('\n')[0].trim()
          })).filter(place => place.url);
        });

        for (const place of places) {
          const id = placeIdFromUrl(place.url);
          if (!seen.has(id)) {
            seen.set(id, {
              id,
              name: place.name,
              url: normalizeUrl(place.url),
              discoveredPostcode: postcode,
              discoveredQuery: query
            });
          }
        }

        if (seen.size === lastCount) stagnant += 1;
        else stagnant = 0;
        lastCount = seen.size;
        process.stdout.write(`\r  places: ${seen.size}   `);

        if (args.maxResults && seen.size >= args.maxResults) break;
        if (stagnant >= 5) break;

        await page.evaluate(() => {
          const feed = document.querySelector('div[role="feed"]') || document.querySelector('[aria-label*="Ergebnisse"]') || document.scrollingElement;
          feed?.scrollBy?.(0, 2400);
        });
        await page.mouse.wheel(0, 2400);
        await page.waitForTimeout(1400);
      }
      writeJson(discoveryFile, [...seen.values()]);
      console.log(`\n  saved ${seen.size} discoveries`);
    }
  }

  return [...seen.values()].slice(0, args.maxResults || undefined);
}

async function clickReviewsTab(page) {
  const candidates = [
    page.getByRole('tab', { name: /Rezensionen|Reviews/i }).first(),
    page.getByRole('button', { name: /Rezensionen|Reviews/i }).first(),
    page.locator('button:has-text("Rezensionen")').first(),
    page.locator('[role="tab"]:has-text("Rezensionen")').first()
  ];

  for (const candidate of candidates) {
    try {
      if (await candidate.isVisible({ timeout: 1500 })) {
        await candidate.click({ timeout: 3000 });
        await page.waitForTimeout(2000);
        return true;
      }
    } catch {
      // Try next selector.
    }
  }
  return false;
}

async function extractPlace(page, discovery) {
  await page.goto(normalizeUrl(discovery.url), { waitUntil: 'domcontentloaded', timeout: 60000 });
  await acceptConsent(page);
  await page.waitForTimeout(2500);
  const overview = await readMapText(page);
  await clickReviewsTab(page);
  const reviews = await readMapText(page);

  const raw = {
    title: reviews.title || overview.title,
    h1: reviews.h1 || overview.h1,
    text: `${overview.text}\n${reviews.text}`
  };
  const topText = overview.text.slice(0, 2500);
  const name = raw.h1 || discovery.name || raw.title.replace(/ - Google Maps.*/i, '').trim();
  const stats = parsePlaceStats(raw.text);
  const address = extractAddress(overview.text);
  const categoryMatch = topText.match(/\n([^\n]*(?:Restaurant|Café|Cafe|Bäckerei|Imbiss|Pizzeria|Bar|Küche|Döner)[^\n]*)\n/i);
  const notice = parseNotice(raw.text);

  const row = {
    id: discovery.id,
    name,
    postcode: address?.match(/\b9\d{4}\b/)?.[0] || discovery.discoveredPostcode || null,
    address,
    rating: stats.rating,
    reviewCount: stats.reviewCount,
    category: categoryMatch?.[1]?.trim() || null,
    hasDefamationNotice: Boolean(notice),
    removedMin: notice?.min ?? null,
    removedMax: notice?.max ?? null,
    removedEstimate: notice?.estimate ?? null,
    deletionRatioPct: null,
    realRatingAdjusted: null,
    removedText: notice?.text ?? null,
    url: normalizeUrl(discovery.url),
    readAt: new Date().toISOString(),
    status: 'success',
    error: null
  };

  return computeMetrics(row);
}

async function scrapePlaces(page, discoveries, args) {
  const previous = readJson(args.out, []);
  const rows = new Map(previous.map(row => [row.id, row]));
  const todo = discoveries.filter(place => !rows.has(place.id) || rows.get(place.id).status !== 'success');
  console.log(`\nScrape: ${todo.length} remaining / ${discoveries.length} discovered`);

  for (let i = 0; i < todo.length; i += 1) {
    const place = todo[i];
    console.log(`[${i + 1}/${todo.length}] ${place.name || place.id}`);
    try {
      const row = await extractPlace(page, place);
      rows.set(row.id, row);
      console.log(`  ${row.rating ?? '?'}★ ${row.reviewCount ?? '?'} reviews; removed=${row.removedText || 'none'}`);
    } catch (error) {
      rows.set(place.id, {
        id: place.id,
        name: place.name || null,
        postcode: place.discoveredPostcode || null,
        address: null,
        rating: null,
        reviewCount: null,
        category: null,
        hasDefamationNotice: false,
        removedMin: null,
        removedMax: null,
        removedEstimate: null,
        deletionRatioPct: null,
        realRatingAdjusted: null,
        removedText: null,
        url: normalizeUrl(place.url),
        readAt: new Date().toISOString(),
        status: 'error',
        error: error?.message || String(error)
      });
      console.warn(`  ERROR: ${error?.message || error}`);
      try {
        await page.screenshot({ path: path.join('debug', `${place.id.replace(/[^a-z0-9_-]/gi, '_')}.png`), fullPage: true });
      } catch {
        // Ignore debug screenshot failures.
      }
    }

    const sorted = [...rows.values()].sort((a, b) => String(a.postcode || '').localeCompare(String(b.postcode || '')) || String(a.name || '').localeCompare(String(b.name || '')));
    writeJson(args.out, sorted);
    writeCsv(RESULTS_CSV, sorted);
    await sleep(randomDelay(args.delayMin, args.delayMax));
  }

  return [...rows.values()];
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  ensureDir(OUTPUT_DIR);
  ensureDir('debug');

  const browser = await chromium.launch({ headless: args.headless });
  const context = await browser.newContext({
    locale: 'de-DE',
    timezoneId: 'Europe/Berlin',
    viewport: { width: 1440, height: 1100 },
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36'
  });
  const page = await context.newPage();

  try {
    const discoveryFile = path.join(OUTPUT_DIR, 'discovery.json');
    const discoveries = args.scrapeOnly
      ? readJson(discoveryFile, [])
      : await discoverPlaces(page, args);

    if (args.discoveryOnly) {
      console.log(`Discovery complete: ${discoveries.length} places`);
      return;
    }

    await scrapePlaces(page, discoveries, args);
    console.log(`\nDone. Results: ${args.out} and ${RESULTS_CSV}`);
  } finally {
    await browser.close();
  }
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
