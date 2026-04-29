#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { execFileSync } from 'node:child_process';

const WIDTH = 1800;
const HEIGHT = 2500;
const GREEN = '#2e7d32';
const RED = '#c9332c';
const ORANGE = '#ef7d16';
const GRID = '#dde3e8';
const TEXT = '#202124';
const MUTED = '#6f7377';

function parseArgs(argv) {
  const args = {
    input: 'output/places.json',
    outDir: 'output/charts',
    png: false,
    top: 30,
    minCleanReviews: 100
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    const [key, inline] = arg.split('=');
    const next = inline ?? argv[i + 1];
    const consume = inline === undefined && next && !next.startsWith('--');
    switch (key) {
      case '--input': args.input = next; if (consume) i += 1; break;
      case '--out-dir': args.outDir = next; if (consume) i += 1; break;
      case '--top': args.top = Number(next); if (consume) i += 1; break;
      case '--min-clean-reviews': args.minCleanReviews = Number(next); if (consume) i += 1; break;
      case '--png': args.png = true; break;
      case '--help': printHelpAndExit(); break;
      default: throw new Error(`Unknown argument: ${arg}`);
    }
  }
  return args;
}

function printHelpAndExit() {
  console.log(`Usage:
  npm run charts -- --png
  npm run charts -- --input output/places.json --out-dir output/charts --top 50

Options:
  --input <path>              Scrape results JSON. Default: output/places.json.
  --out-dir <path>            Chart output directory. Default: output/charts.
  --top <n>                   Rows per bar chart. Default: 30.
  --min-clean-reviews <n>     Minimum reviews for clean ranking. Default: 100.
  --png                       Export PNGs via Playwright in addition to SVG.
`);
  process.exit(0);
}

function esc(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function trunc(value, max = 38) {
  const s = String(value ?? '');
  return s.length > max ? `${s.slice(0, max - 1)}…` : s;
}

function fmtNumber(value, digits = 0) {
  if (value === null || value === undefined || Number.isNaN(value)) return '–';
  return Number(value).toLocaleString('de-DE', { minimumFractionDigits: digits, maximumFractionDigits: digits });
}

function fmtPct(value) {
  return `${fmtNumber(value, 1)}%`;
}

function fmtRating(value, digits = 1) {
  return `${fmtNumber(value, digits)}★`;
}

function removedRange(row) {
  if (!row.hasDefamationNotice) return '';
  if (row.removedMax === null || row.removedMax === undefined) return `über ${row.removedMin}`;
  if (row.removedMin === row.removedMax) return String(row.removedMin);
  return `${row.removedMin}–${row.removedMax}`;
}

function removedSortValue(row) {
  if (row.removedEstimate !== null && row.removedEstimate !== undefined) return row.removedEstimate;
  if (row.removedMax === null || row.removedMax === undefined) return (row.removedMin ?? 0) + 50;
  return ((row.removedMin ?? 0) + row.removedMax) / 2;
}

function csvEscape(value) {
  if (value === null || value === undefined) return '';
  const s = String(value);
  return /[",\n]/.test(s) ? `"${s.replaceAll('"', '""')}"` : s;
}

function validRows(rows) {
  return rows.filter(row => row.status === 'success' && row.name && row.reviewCount !== null && row.rating !== null);
}

function groupByPostcode(rows) {
  const groups = new Map();
  for (const row of rows) {
    const postcode = row.postcode || 'unknown';
    if (!groups.has(postcode)) groups.set(postcode, []);
    groups.get(postcode).push(row);
  }
  return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
}

function binFor(row) {
  if (!row.hasDefamationNotice) return 'Keine Löschung';
  const min = row.removedMin;
  const max = row.removedMax;
  if (min === 1 && max === 1) return 'Eine gelöscht';
  if (min <= 2 && max <= 5) return '2–5 gelöscht';
  if (min <= 6 && max <= 10) return '6–10 gelöscht';
  if (min <= 11 && max <= 20) return '11–20 gelöscht';
  if (min <= 21 && max <= 50) return '21–50 gelöscht';
  if (min <= 51 && max <= 100) return '51–100 gelöscht';
  if (min <= 101 && (max ?? min) <= 200) return '101–200 gelöscht';
  if (min <= 201 && (max ?? min) <= 250) return '201–250 gelöscht';
  return 'Über 250 gelöscht';
}

const BIN_ORDER = [
  'Keine Löschung', 'Eine gelöscht', '2–5 gelöscht', '6–10 gelöscht',
  '11–20 gelöscht', '21–50 gelöscht', '51–100 gelöscht', '101–200 gelöscht',
  '201–250 gelöscht', 'Über 250 gelöscht'
];

const BIN_COLORS = {
  'Keine Löschung': GREEN,
  'Eine gelöscht': '#8bc34a',
  '2–5 gelöscht': '#cddc39',
  '6–10 gelöscht': '#ffca28',
  '11–20 gelöscht': '#ff9f1c',
  '21–50 gelöscht': '#ff6d00',
  '51–100 gelöscht': '#e5391b',
  '101–200 gelöscht': '#c62828',
  '201–250 gelöscht': '#8e1919',
  'Über 250 gelöscht': '#5b0f0f'
};

function chartFrame(title, subtitle, stats) {
  return `
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#d9e8f2"/>
      <stop offset="58%" stop-color="#eef6fa"/>
      <stop offset="100%" stop-color="#d7dde2"/>
    </linearGradient>
    <filter id="softShadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="8" stdDeviation="10" flood-color="#6d7f8a" flood-opacity="0.16"/>
    </filter>
  </defs>
  <rect width="100%" height="100%" fill="url(#bg)"/>
  <circle cx="1450" cy="310" r="500" fill="#ffffff" opacity="0.18"/>
  <circle cx="260" cy="2140" r="620" fill="#ffffff" opacity="0.20"/>
  <text x="900" y="70" text-anchor="middle" font-family="Inter, Arial, sans-serif" font-size="48" font-weight="800" fill="${TEXT}">${esc(title)}</text>
  <text x="900" y="112" text-anchor="middle" font-family="Inter, Arial, sans-serif" font-size="22" fill="#4b5055">${esc(subtitle)}</text>
  <text x="900" y="160" text-anchor="middle" font-family="Inter, Arial, sans-serif" font-size="18" fill="${MUTED}">${esc(stats)}</text>`;
}

function drawBarChart({ x, y, width, height, title, subtitle, rows, color, value, maxValue, label, axisLabel }) {
  const plotX = x + 34;
  const plotY = y + 92;
  const plotW = width - 68;
  const rowH = Math.max(24, Math.min(36, (height - 126) / Math.max(rows.length, 1)));
  const actualH = rowH * Math.max(rows.length, 1) + 24;
  const max = maxValue || Math.max(1, ...rows.map(value));
  let svg = `
  <g>
    <text x="${x}" y="${y}" font-family="Inter, Arial, sans-serif" font-size="27" font-weight="800" fill="${TEXT}">${esc(title)}</text>
    <text x="${x}" y="${y + 30}" font-family="Inter, Arial, sans-serif" font-size="16" fill="${MUTED}">${esc(subtitle)}</text>
    <rect x="${x}" y="${plotY}" width="${width}" height="${actualH}" fill="#ffffff" opacity="0.96" filter="url(#softShadow)"/>
  `;

  for (let i = 0; i <= 4; i += 1) {
    const gx = plotX + (plotW * i) / 4;
    svg += `<line x1="${gx}" y1="${plotY}" x2="${gx}" y2="${plotY + actualH - 20}" stroke="${GRID}" stroke-width="1"/>`;
  }

  rows.forEach((row, index) => {
    const yy = plotY + 24 + index * rowH;
    const barW = Math.max(3, Math.min(plotW, (value(row) / max) * plotW));
    svg += `
      <text x="${x - 8}" y="${yy + 15}" text-anchor="end" font-family="Inter, Arial, sans-serif" font-size="13" fill="#83888d">${index + 1}.</text>
      <text x="${plotX}" y="${yy - 4}" font-family="Inter, Arial, sans-serif" font-size="14" fill="${TEXT}">${esc(trunc(row.name, 45))}</text>
      <rect x="${plotX}" y="${yy}" width="${barW}" height="18" fill="${color}"/>
      <text x="${plotX + barW + 8}" y="${yy + 15}" font-family="Inter, Arial, sans-serif" font-size="13" fill="${TEXT}">${esc(label(row))}</text>`;
  });

  if (axisLabel) {
    svg += `<text x="${plotX + plotW / 2}" y="${plotY + actualH + 18}" text-anchor="middle" font-family="Inter, Arial, sans-serif" font-size="13" fill="${MUTED}">${esc(axisLabel)}</text>`;
  }

  svg += '</g>';
  return svg;
}

function polar(cx, cy, r, angle) {
  const rad = ((angle - 90) * Math.PI) / 180;
  return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
}

function pieSlice(cx, cy, r, start, end, color) {
  const s = polar(cx, cy, r, end);
  const e = polar(cx, cy, r, start);
  const large = end - start <= 180 ? 0 : 1;
  return `<path d="M ${cx} ${cy} L ${s.x} ${s.y} A ${r} ${r} 0 ${large} 0 ${e.x} ${e.y} Z" fill="${color}" stroke="#ffffff" stroke-width="2"/>`;
}

function drawDistribution(rows, x, y, title) {
  const counts = new Map(BIN_ORDER.map(bin => [bin, 0]));
  rows.forEach(row => counts.set(binFor(row), (counts.get(binFor(row)) || 0) + 1));
  const total = rows.length || 1;
  const cx = x + 280;
  const cy = y + 270;
  const r = 180;
  let angle = 0;
  let svg = `
  <g>
    <text x="${x}" y="${y}" font-family="Inter, Arial, sans-serif" font-size="26" font-weight="800" fill="${TEXT}">${esc(title)}</text>
    <g filter="url(#softShadow)">`;

  for (const bin of BIN_ORDER) {
    const count = counts.get(bin) || 0;
    if (!count) continue;
    if (count === total) {
      svg += `<circle cx="${cx}" cy="${cy}" r="${r}" fill="${BIN_COLORS[bin]}" stroke="#ffffff" stroke-width="2"/>`;
      angle = 360;
      continue;
    }
    const next = angle + (count / total) * 360;
    svg += pieSlice(cx, cy, r, angle, next, BIN_COLORS[bin]);
    angle = next;
  }

  const cleanPct = ((counts.get('Keine Löschung') || 0) / total) * 100;
  svg += `</g>
    <text x="${cx}" y="${cy + 8}" text-anchor="middle" font-family="Inter, Arial, sans-serif" font-size="28" font-weight="800" fill="#ffffff">${fmtNumber(cleanPct, 0)}%</text>`;

  let ly = y + 130;
  for (const bin of BIN_ORDER) {
    const count = counts.get(bin) || 0;
    if (!count && bin !== 'Keine Löschung') continue;
    const pct = (count / total) * 100;
    svg += `
      <rect x="${x + 740}" y="${ly - 15}" width="24" height="24" fill="${BIN_COLORS[bin]}"/>
      <text x="${x + 780}" y="${ly + 4}" font-family="Inter, Arial, sans-serif" font-size="18" fill="${TEXT}">${esc(bin)} — ${count} (${fmtNumber(pct, 1)}%)</text>`;
    ly += 40;
  }

  svg += '</g>';
  return svg;
}

function makeChart(rows, scope, args) {
  const cleanRows = validRows(rows);
  const removedRows = cleanRows.filter(row => row.hasDefamationNotice && row.deletionRatioPct !== null);
  const cleanRanking = cleanRows
    .filter(row => !row.hasDefamationNotice && row.reviewCount >= args.minCleanReviews)
    .sort((a, b) => b.rating - a.rating || b.reviewCount - a.reviewCount)
    .slice(0, args.top);

  const highestRatio = [...removedRows]
    .sort((a, b) => b.deletionRatioPct - a.deletionRatioPct)
    .slice(0, args.top);
  const worstAdjusted = [...removedRows]
    .sort((a, b) => a.realRatingAdjusted - b.realRatingAdjusted)
    .slice(0, args.top);

  const title = scope === 'overall' ? 'Nürnberg — Gesamtstadt' : `Postleitzahl ${scope} — Nürnberg`;
  const subtitle = scope === 'overall'
    ? 'Alle erfassten PLZ — gelöschte Rezensionen wegen „Diffamierung“'
    : 'Google-Maps-Orte: gelöschte Rezensionen wegen „Diffamierung“';
  const stats = `${cleanRows.length} Orte erfasst · ${removedRows.length} mit sichtbarem Banner (${fmtNumber((removedRows.length / Math.max(cleanRows.length, 1)) * 100, 1)}%) · ${removedRows.filter(row => row.removedMax === null).length} in der „Über“-Stufe`;

  const topCount = args.top;
  let svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${WIDTH}" height="${HEIGHT}" viewBox="0 0 ${WIDTH} ${HEIGHT}">`;
  svg += chartFrame(title, subtitle, stats);
  svg += drawBarChart({
    x: 80,
    y: 230,
    width: 470,
    height: 1280,
    title: '1. Höchste Lösch-Quote',
    subtitle: 'Anteil gelöschter Rezensionen (Mitte des Bereichs / sichtbare + gelöschte)',
    rows: highestRatio,
    color: RED,
    value: row => row.deletionRatioPct,
    maxValue: Math.max(10, ...highestRatio.map(row => row.deletionRatioPct)),
    label: row => `${fmtPct(row.deletionRatioPct)}  (${removedRange(row)} von ${fmtNumber(row.reviewCount)})`,
    axisLabel: `Lösch-Quote (%) · Top ${topCount}`
  });
  svg += drawBarChart({
    x: 665,
    y: 230,
    width: 470,
    height: 1280,
    title: '2. Schlechtestes „echtes“ Rating',
    subtitle: 'angenommen: alle gelöschten Rezensionen waren 1-Stern',
    rows: worstAdjusted,
    color: ORANGE,
    value: row => 5 - row.realRatingAdjusted,
    maxValue: Math.max(1, ...worstAdjusted.map(row => 5 - row.realRatingAdjusted)),
    label: row => `${fmtRating(row.realRatingAdjusted, 2)}  (vorher ${fmtRating(row.rating, 1)})`,
    axisLabel: 'Punkte unter 5★'
  });
  svg += drawBarChart({
    x: 1250,
    y: 230,
    width: 470,
    height: 1280,
    title: '3. Beste „saubere“ Orte',
    subtitle: `kein sichtbarer Diffamierungs-Banner — ab ${args.minCleanReviews} Rezensionen`,
    rows: cleanRanking,
    color: GREEN,
    value: row => row.rating,
    maxValue: 5,
    label: row => `${fmtRating(row.rating, 1)}  (${fmtNumber(row.reviewCount)})`,
    axisLabel: 'Rating (★)'
  });
  svg += drawDistribution(cleanRows, 120, 1660, `Verteilung der Lösch-Stufen über alle ${cleanRows.length} Orte${scope === 'overall' ? ' in Nürnberg' : ` in PLZ ${scope}`}`);
  svg += `<text x="40" y="2460" font-family="Inter, Arial, sans-serif" font-size="13" fill="#8a8f94">Quelle: Google Maps, öffentlich sichtbare Banner; Analyse-Snapshot ${new Date().toLocaleDateString('de-DE')}</text>`;
  svg += `<text x="1760" y="2460" text-anchor="end" font-family="Inter, Arial, sans-serif" font-size="13" fill="#8a8f94">generated by scripts/generate-charts.mjs</text>`;
  svg += '</svg>\n';
  return svg;
}

function writeMostRemovedList(rows, outDir) {
  const ranked = validRows(rows)
    .filter(row => row.hasDefamationNotice && row.removedMin !== null && row.removedMin !== undefined)
    .sort((a, b) => removedSortValue(b) - removedSortValue(a)
      || (b.removedMin ?? 0) - (a.removedMin ?? 0)
      || (b.reviewCount ?? 0) - (a.reviewCount ?? 0)
      || String(a.name).localeCompare(String(b.name)));

  const csvColumns = [
    'rank', 'name', 'postcode', 'address', 'rating', 'reviewCount', 'removedRange',
    'removedMin', 'removedMax', 'removedEstimate', 'deletionRatioPct',
    'realRatingAdjusted', 'removedText', 'url'
  ];
  const csvLines = [csvColumns.join(',')];
  ranked.forEach((row, index) => {
    const values = {
      rank: index + 1,
      name: row.name,
      postcode: row.postcode,
      address: row.address,
      rating: row.rating,
      reviewCount: row.reviewCount,
      removedRange: removedRange(row),
      removedMin: row.removedMin,
      removedMax: row.removedMax,
      removedEstimate: removedSortValue(row),
      deletionRatioPct: row.deletionRatioPct,
      realRatingAdjusted: row.realRatingAdjusted,
      removedText: row.removedText,
      url: row.url
    };
    csvLines.push(csvColumns.map(column => csvEscape(values[column])).join(','));
  });

  const mdLines = [
    '# Nürnberg — Orte sortiert nach geschätzter Anzahl entfernter Bewertungen',
    '',
    `Quelle: Google Maps, öffentlich sichtbare Diffamierungs-Banner. Snapshot: ${new Date().toLocaleDateString('de-DE')}.`,
    '',
    'Sortierung: geschätzter Mittelpunkt der Google-Bereiche absteigend. „Über 250“ wird als 300 geschätzt.',
    '',
    `Einträge mit Banner: ${ranked.length}`,
    '',
    '| Rang | Name | PLZ | Rating | Rezensionen | Gelöscht | Schätzwert | Löschquote | Worst-Case-Rating |',
    '|---:|---|---:|---:|---:|---:|---:|---:|---:|'
  ];
  ranked.forEach((row, index) => {
    mdLines.push(`| ${index + 1} | ${String(row.name).replaceAll('|', '\\|')} | ${row.postcode ?? ''} | ${fmtNumber(row.rating, 1)} | ${fmtNumber(row.reviewCount)} | ${removedRange(row)} | ${fmtNumber(removedSortValue(row), 1)} | ${fmtPct(row.deletionRatioPct)} | ${fmtNumber(row.realRatingAdjusted, 2)} |`);
  });

  const postcodeOptions = [...new Set(ranked.map(row => row.postcode).filter(Boolean))]
    .sort((a, b) => String(a).localeCompare(String(b)))
    .map(postcode => `<option value="${esc(postcode)}">${esc(postcode)}</option>`)
    .join('\n');
  const rangeOptions = [...new Map(ranked.map(row => [removedRange(row), removedSortValue(row)])).entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([range]) => `<option value="${esc(range)}">${esc(range)}</option>`)
    .join('\n');

  const htmlRows = ranked.map((row, index) => {
    const range = removedRange(row);
    const search = [row.name, row.postcode, row.address, range, row.removedText].filter(Boolean).join(' ');
    return `
        <tr data-search="${esc(search.toLowerCase())}" data-postcode="${esc(row.postcode ?? '')}" data-range="${esc(range)}" data-rank="${index + 1}" data-name="${esc(String(row.name).toLowerCase())}" data-rating="${row.rating ?? ''}" data-reviews="${row.reviewCount ?? ''}" data-removed="${removedSortValue(row)}" data-ratio="${row.deletionRatioPct ?? ''}" data-real-rating="${row.realRatingAdjusted ?? ''}" data-address="${esc(String(row.address ?? '').toLowerCase())}">
          <td class="rank">${index + 1}</td>
          <td class="name"><a href="${esc(row.url)}" target="_blank" rel="noopener noreferrer">${esc(row.name)}</a></td>
          <td>${esc(row.postcode ?? '')}</td>
          <td class="num">${fmtNumber(row.rating, 1)}</td>
          <td class="num">${fmtNumber(row.reviewCount)}</td>
          <td class="num">${esc(range)}</td>
          <td class="num">${fmtNumber(removedSortValue(row), 1)}</td>
          <td class="num">${fmtPct(row.deletionRatioPct)}</td>
          <td class="num">${fmtNumber(row.realRatingAdjusted, 2)}</td>
        </tr>`;
  }).join('\n');

  const html = `<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Nürnberg — meist entfernte Google-Maps-Bewertungen</title>
  <style>
    :root { color-scheme: light; --bg: #eef6fa; --card: #ffffff; --text: #202124; --muted: #687078; --line: #dce5eb; --red: #c9332c; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: linear-gradient(180deg, #d9e8f2, #f7fbfd 45%, #e1e7ec); color: var(--text); }
    main { width: min(1500px, calc(100vw - 40px)); margin: 40px auto 80px; }
    header { margin-bottom: 24px; }
    h1 { margin: 0 0 10px; font-size: clamp(32px, 5vw, 56px); letter-spacing: -0.04em; }
    .lead { margin: 0; color: var(--muted); font-size: 18px; line-height: 1.5; }
    .stats { display: flex; flex-wrap: wrap; gap: 12px; margin: 24px 0; }
    .stat { background: rgba(255,255,255,0.82); border: 1px solid var(--line); border-radius: 18px; padding: 14px 18px; box-shadow: 0 10px 30px rgba(60,80,95,0.10); }
    .stat strong { display: block; font-size: 26px; line-height: 1; }
    .stat span { color: var(--muted); font-size: 13px; }
    .note { margin: 0 0 24px; color: var(--muted); font-size: 14px; }
    .controls { display: grid; grid-template-columns: minmax(260px, 1fr) 150px 180px 150px auto; gap: 12px; align-items: end; margin: 0 0 18px; padding: 16px; background: rgba(255,255,255,0.72); border: 1px solid var(--line); border-radius: 22px; box-shadow: 0 10px 30px rgba(60,80,95,0.10); }
    .control label { display: block; margin: 0 0 6px; color: var(--muted); font-size: 12px; font-weight: 700; letter-spacing: .03em; text-transform: uppercase; }
    .control input, .control select { width: 100%; height: 42px; border: 1px solid var(--line); border-radius: 12px; padding: 0 12px; background: #fff; color: var(--text); font: inherit; outline: none; }
    .control input:focus, .control select:focus { border-color: var(--red); box-shadow: 0 0 0 3px rgba(201,51,44,0.12); }
    .reset { height: 42px; border: 0; border-radius: 12px; padding: 0 16px; background: var(--text); color: #fff; font-weight: 800; cursor: pointer; }
    .result-count { margin: -6px 0 14px; color: var(--muted); font-size: 14px; }
    .table-wrap { overflow: auto; background: var(--card); border: 1px solid var(--line); border-radius: 22px; box-shadow: 0 14px 42px rgba(60,80,95,0.16); }
    table { width: 100%; border-collapse: collapse; table-layout: fixed; min-width: 1180px; }
    col.rank-col { width: 72px; }
    col.name-col { width: 370px; }
    col.postcode-col { width: 86px; }
    col.rating-col { width: 96px; }
    col.reviews-col { width: 132px; }
    col.range-col { width: 120px; }
    col.estimate-col { width: 126px; }
    col.ratio-col { width: 126px; }
    col.real-rating-col { width: 190px; }
    thead th { position: sticky; top: 0; z-index: 1; background: #f8fbfd; border-bottom: 1px solid var(--line); color: #3b4248; font-size: 13px; text-align: left; padding: 0; white-space: nowrap; }
    thead button { display: flex; align-items: center; gap: 5px; width: 100%; min-height: 44px; padding: 13px 14px; border: 0; background: transparent; color: inherit; font: inherit; font-weight: 800; text-align: left; cursor: pointer; }
    thead th.rank button, thead th.num button { justify-content: flex-end; text-align: right; }
    thead button:hover { color: var(--red); background: #fff4f3; }
    thead button .arrow { display: inline-block; width: 1.1em; color: #9aa2a9; text-align: left; }
    thead button.active .arrow { color: var(--red); }
    tbody td { border-bottom: 1px solid #edf2f5; padding: 12px 14px; vertical-align: top; font-size: 14px; }
    tbody tr:nth-child(even) { background: #fbfdfe; }
    tbody tr:hover { background: #fff6f5; }
    a { color: var(--red); font-weight: 700; text-decoration: none; }
    a:hover { text-decoration: underline; }
    .rank, .num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .name { min-width: 310px; max-width: 430px; }
    .address { color: var(--muted); min-width: 260px; }
    footer { margin-top: 18px; color: var(--muted); font-size: 13px; }
    @media (max-width: 900px) { .controls { grid-template-columns: 1fr 1fr; } .control.search { grid-column: 1 / -1; } }
    @media (max-width: 560px) { main { width: min(100vw - 20px, 1500px); margin-top: 24px; } .controls { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
  <main>
    <header>
      <h1>Nürnberg — meist entfernte Google-Maps-Bewertungen</h1>
      <p class="lead">Orte mit sichtbarem Google-Maps-Hinweis auf entfernte Bewertungen wegen Beschwerden wegen Diffamierung. Namen sind direkt zur jeweiligen Google-Maps-Seite verlinkt.</p>
      <div class="stats">
        <div class="stat"><strong>${fmtNumber(ranked.length)}</strong><span>Einträge mit Banner</span></div>
        <div class="stat"><strong>${fmtNumber(validRows(rows).length)}</strong><span>erfasste Orte</span></div>
        <div class="stat"><strong>${fmtNumber((ranked.length / Math.max(validRows(rows).length, 1)) * 100, 1)}%</strong><span>mit sichtbarem Banner</span></div>
      </div>
      <p class="note">Sortierung: geschätzter Mittelpunkt der Google-Bereiche absteigend. „Über 250“ wird als 300 geschätzt. Snapshot: ${new Date().toLocaleDateString('de-DE')}.</p>
    </header>
    <section class="controls" aria-label="Filter">
      <div class="control search">
        <label for="searchInput">Suche</label>
        <input id="searchInput" type="search" placeholder="Name, PLZ, Löschbereich …" autocomplete="off">
      </div>
      <div class="control">
        <label for="postcodeFilter">PLZ</label>
        <select id="postcodeFilter"><option value="">Alle PLZ</option>${postcodeOptions}</select>
      </div>
      <div class="control">
        <label for="rangeFilter">Gelöscht</label>
        <select id="rangeFilter"><option value="">Alle Bereiche</option>${rangeOptions}</select>
      </div>
      <div class="control">
        <label for="minReviews">Min. Rezensionen</label>
        <input id="minReviews" type="number" min="0" step="1" placeholder="0">
      </div>
      <button class="reset" id="resetFilters" type="button">Reset</button>
    </section>
    <p class="result-count" id="resultCount"></p>
    <section class="table-wrap" aria-label="Ranking nach geschätzter Anzahl entfernter Bewertungen">
      <table id="rankingTable">
        <colgroup>
          <col class="rank-col">
          <col class="name-col">
          <col class="postcode-col">
          <col class="rating-col">
          <col class="reviews-col">
          <col class="range-col">
          <col class="estimate-col">
          <col class="ratio-col">
          <col class="real-rating-col">
        </colgroup>
        <thead>
          <tr>
            <th class="rank"><button type="button" data-sort="rank">Rang <span class="arrow"></span></button></th>
            <th><button type="button" data-sort="name">Name / Google Maps <span class="arrow"></span></button></th>
            <th><button type="button" data-sort="postcode">PLZ <span class="arrow"></span></button></th>
            <th class="num"><button type="button" data-sort="rating">Rating <span class="arrow"></span></button></th>
            <th class="num"><button type="button" data-sort="reviews">Rezensionen <span class="arrow"></span></button></th>
            <th class="num"><button type="button" data-sort="removed">Gelöscht <span class="arrow"></span></button></th>
            <th class="num"><button type="button" data-sort="removed">Schätzwert <span class="arrow"></span></button></th>
            <th class="num"><button type="button" data-sort="ratio">Löschquote <span class="arrow"></span></button></th>
            <th class="num"><button type="button" data-sort="realRating">Worst-Case-Rating <span class="arrow"></span></button></th>
          </tr>
        </thead>
        <tbody>${htmlRows}
        </tbody>
      </table>
    </section>
    <footer>Quelle: Google Maps, öffentlich sichtbare Banner. Generated by scripts/generate-charts.mjs.</footer>
  </main>
  <script>
    (() => {
      const table = document.getElementById('rankingTable');
      const tbody = table.tBodies[0];
      const rows = Array.from(tbody.rows);
      const searchInput = document.getElementById('searchInput');
      const postcodeFilter = document.getElementById('postcodeFilter');
      const rangeFilter = document.getElementById('rangeFilter');
      const minReviews = document.getElementById('minReviews');
      const resetFilters = document.getElementById('resetFilters');
      const resultCount = document.getElementById('resultCount');
      const buttons = Array.from(table.querySelectorAll('thead button[data-sort]'));
      let sortKey = 'rank';
      let sortDir = 'asc';

      function rowValue(row, key) {
        if (key === 'realRating') return Number(row.dataset.realRating || -Infinity);
        if (key === 'name' || key === 'address') return row.dataset[key] || '';
        return Number(row.dataset[key] || -Infinity);
      }

      function compareRows(a, b) {
        const av = rowValue(a, sortKey);
        const bv = rowValue(b, sortKey);
        const result = typeof av === 'string' ? av.localeCompare(bv, 'de') : av - bv;
        return sortDir === 'asc' ? result : -result;
      }

      function matches(row) {
        const query = searchInput.value.trim().toLowerCase();
        const postcode = postcodeFilter.value;
        const range = rangeFilter.value;
        const min = Number(minReviews.value || 0);
        return (!query || row.dataset.search.includes(query))
          && (!postcode || row.dataset.postcode === postcode)
          && (!range || row.dataset.range === range)
          && Number(row.dataset.reviews || 0) >= min;
      }

      function render() {
        const sorted = [...rows].sort(compareRows);
        let visible = 0;
        const fragment = document.createDocumentFragment();
        for (const row of sorted) {
          const show = matches(row);
          row.hidden = !show;
          if (show) visible += 1;
          fragment.appendChild(row);
        }
        tbody.appendChild(fragment);
        resultCount.textContent = visible + ' von ' + rows.length + ' Einträgen sichtbar';
        buttons.forEach(button => {
          const active = button.dataset.sort === sortKey;
          button.classList.toggle('active', active);
          button.querySelector('.arrow').textContent = active ? (sortDir === 'asc' ? '▲' : '▼') : '';
        });
      }

      buttons.forEach(button => button.addEventListener('click', () => {
        const nextKey = button.dataset.sort;
        if (nextKey === sortKey) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
        else {
          sortKey = nextKey;
          sortDir = ['name', 'postcode', 'address', 'rank'].includes(sortKey) ? 'asc' : 'desc';
        }
        render();
      }));
      [searchInput, postcodeFilter, rangeFilter, minReviews].forEach(input => input.addEventListener('input', render));
      resetFilters.addEventListener('click', () => {
        searchInput.value = '';
        postcodeFilter.value = '';
        rangeFilter.value = '';
        minReviews.value = '';
        sortKey = 'rank';
        sortDir = 'asc';
        render();
      });
      render();
    })();
  </script>
</body>
</html>
`;

  const csvFile = path.join(outDir, 'nuernberg_most_removed.csv');
  const mdFile = path.join(outDir, 'nuernberg_most_removed.md');
  const htmlFile = path.join(outDir, 'nuernberg_most_removed.html');
  fs.writeFileSync(csvFile, `${csvLines.join('\n')}\n`);
  fs.writeFileSync(mdFile, `${mdLines.join('\n')}\n`);
  fs.writeFileSync(htmlFile, html);
  console.log(`wrote ${csvFile}`);
  console.log(`wrote ${mdFile}`);
  console.log(`wrote ${htmlFile}`);
}

async function exportPng(svgFile, pngFile) {
  try {
    execFileSync('magick', [svgFile, pngFile], { stdio: 'ignore' });
    return;
  } catch {
    // Fall back to Playwright when ImageMagick is unavailable.
  }

  const { chromium } = await import('playwright');
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: WIDTH, height: HEIGHT }, deviceScaleFactor: 1 });
    await page.goto(pathToFileURL(path.resolve(svgFile)).toString());
    await page.screenshot({ path: pngFile, fullPage: true, timeout: 120000 });
  } finally {
    await browser.close();
  }
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (!fs.existsSync(args.input)) throw new Error(`Input not found: ${args.input}`);
  fs.mkdirSync(args.outDir, { recursive: true });

  const rows = JSON.parse(fs.readFileSync(args.input, 'utf8'));
  writeMostRemovedList(rows, args.outDir);
  const charts = [['overall', rows], ...groupByPostcode(rows).filter(([postcode]) => postcode !== 'unknown')];

  for (const [scope, scopeRows] of charts) {
    const svg = makeChart(scopeRows, scope, args);
    const base = scope === 'overall' ? 'nuernberg_overall_summary' : `nuernberg_${scope}_summary`;
    const svgFile = path.join(args.outDir, `${base}.svg`);
    fs.writeFileSync(svgFile, svg);
    console.log(`wrote ${svgFile}`);

    if (args.png) {
      const pngFile = path.join(args.outDir, `${base}.png`);
      await exportPng(svgFile, pngFile);
      console.log(`wrote ${pngFile}`);
    }
  }
}

main().catch(error => {
  console.error(error);
  process.exit(1);
});
