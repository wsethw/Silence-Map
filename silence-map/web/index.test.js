const assert = require('node:assert/strict');
const fs = require('node:fs');

const read = file => fs.readFileSync(new URL(file, `file://${__dirname}/`), 'utf8');
const html = read('./index.html');
const css = read('./styles.css');
const app = read('./app.js');
const core = require('./app_core.js');

const inlineScripts = [...html.matchAll(/<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/gi)];
assert.equal(inlineScripts.length, 0, 'application JavaScript should live in app.js');
new Function(read('./app_core.js'));
new Function(app);

assert.match(app, /basemaps\.cartocdn\.com\/dark_all/, 'uses CartoDB Dark Matter tiles');
assert.match(html, /id="reportCenter"/, 'keyboard-accessible report-at-map-center button exists');
assert.match(html, /id="toast"[\s\S]*aria-live="polite"/, 'status feedback is announced safely');

assert.match(app, /const API_BASE_URL = IS_FILE_PROTOCOL \? 'http:\/\/localhost:8080' : ''/, 'REST URL uses local fallback only for file://');
assert.match(app, /const WS_URL = IS_FILE_PROTOCOL[\s\S]*localhost:8080[\s\S]*location\.host/, 'WebSocket URL supports file fallback and deployed host');
assert.match(app, /credentials: 'include'/, 'fetch sends signed anonymous session cookie');

assert.match(app, /spatial_mode: viewport\.boundsOnly \? 'bounds' : 'radius_bounds'/, 'requests make bounds-only mode explicit');
assert.deepEqual(core.chooseViewportRadius(64000, 50000), {
  rawRadius: 64000,
  boundsOnly: true,
  radius: 0
}, 'large viewports switch to bounds-only mode instead of capped radius');
assert.deepEqual(core.chooseViewportRadius(4200, 50000), {
  rawRadius: 4200,
  boundsOnly: false,
  radius: 4200
}, 'small viewports keep center radius filtering');
assert.match(app, /rankLocalReports\(viewport\.center, viewport\.bounds\)/, 'fallback ranking receives viewport bounds');
const ranked = core.rankLocalReports([
  { id: 'outside', location: { latitude: 11, longitude: 11 }, quietness_level: 5, confirmations: 99, place_name: 'Outside' },
  { id: 'quiet-far', location: { latitude: 0.8, longitude: 0.8 }, quietness_level: 5, confirmations: 2, place_name: 'Quiet Far' },
  { id: 'quiet-confirmed', location: { latitude: 0.2, longitude: 0.2 }, quietness_level: 5, confirmations: 8, place_name: 'Quiet Confirmed' },
  { id: 'less-quiet', location: { latitude: 0.1, longitude: 0.1 }, quietness_level: 4, confirmations: 30, place_name: 'Less Quiet' }
], { north: 1, south: -1, east: 1, west: -1 }, report => Math.abs(report.location.latitude) + Math.abs(report.location.longitude));
assert.deepEqual(ranked.map(place => place.place_name), ['Quiet Confirmed', 'Quiet Far', 'Less Quiet'], 'fallback filters viewport and ranks quietness, confirmations, then distance');

assert.match(app, /scheduleViewportReload/, 'map move reload is debounced');
assert.match(app, /map\.on\('moveend zoomend'[\s\S]*loadRecentReports/, 'pan and zoom reload existing reports');
assert.match(app, /updateSummaryStats[\s\S]*bounds\.contains/, 'visible stats are computed from current bounds');

assert.match(app, /function trapModalFocus/, 'modal focus trap exists');
assert.match(app, /elements\.appShell\.inert = inert/, 'background is inert while modal is open');
assert.match(app, /previouslyFocusedElement\.focus\(\)/, 'modal restores focus when closed');
assert.match(app, /elements\.placeName\.focus\(\)/, 'modal moves focus to the first input');

const previous = { id: 'r1', location: { latitude: 1, longitude: 2 }, quietness_level: 4, confirmations: 2, place_name: 'Safe text' };
const optimistic = core.optimisticConfirmation(previous);
assert.equal(optimistic.confirmations, 3, 'successful optimistic confirmation increments count');
assert.equal(previous.confirmations, 2, 'optimistic confirmation does not mutate previous state');
const rollback = core.rollbackConfirmation(previous);
assert.equal(rollback.confirmations, 2, 'rejected confirmation rolls back to previous count');
const committed = core.commitConfirmation({ ...previous, confirmations: 7 });
assert.equal(committed.confirmations, 7, 'successful confirmation can commit the server result');
assert.match(app, /const previous = core\.cloneReport\(report\)/, 'confirmation stores previous state');
assert.match(app, /rollbackConfirmation\(previous\)/, 'confirmation rollback restores previous state');
assert.match(app, /payload\.type === 'error'/, 'WebSocket server errors are handled');

assert.match(app, /createTooltipContent[\s\S]*textContent/, 'Leaflet tooltip content uses safe DOM text');
assert.doesNotMatch(app, /innerHTML\s*=/, 'dynamic innerHTML assignments are not allowed');
assert.doesNotMatch(html + css + app, /Mapa|Silêncio|Descubra|tranquilidade|Praça|Avenida|Parque|Jardim|Biblioteca/, 'UI text should stay in English');

assert.match(css, /100dvh/, 'CSS uses dynamic viewport units for mobile browser bars');
assert.match(css, /safe-area-inset-top/, 'CSS accounts for top safe-area inset');
assert.match(css, /safe-area-inset-bottom/, 'CSS accounts for bottom safe-area inset');
assert.match(css, /overscroll-behavior: contain/, 'mobile panel scroll is contained');

console.log('frontend static checks OK');
