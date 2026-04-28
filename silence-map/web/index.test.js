const assert = require('node:assert/strict');
const fs = require('node:fs');

const html = fs.readFileSync(new URL('./index.html', `file://${__dirname}/`), 'utf8');
const inlineScripts = [...html.matchAll(/<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/gi)].map(match => match[1]);

assert.equal(inlineScripts.length, 1, 'expected exactly one inline application script');
new Function(inlineScripts[0]);

assert.match(html, /basemaps\.cartocdn\.com\/dark_all/, 'uses CartoDB Dark Matter tiles');
assert.match(html, /const WS_URL = IS_FILE_PROTOCOL[\s\S]*localhost:8080[\s\S]*location\.host/, 'WebSocket URL supports file fallback and deployed host');
assert.match(html, /north: String\(viewport\.bounds\.getNorth\(\)\)/, 'search sends north bound');
assert.match(html, /south: String\(viewport\.bounds\.getSouth\(\)\)/, 'search sends south bound');
assert.match(html, /east: String\(viewport\.bounds\.getEast\(\)\)/, 'search sends east bound');
assert.match(html, /west: String\(viewport\.bounds\.getWest\(\)\)/, 'search sends west bound');
assert.match(html, /function trapModalFocus/, 'modal focus trap exists');
assert.match(html, /scheduleViewportReload/, 'map move reload is debounced');
assert.doesNotMatch(html, /innerHTML\s*=/, 'dynamic innerHTML assignments are not allowed');
assert.doesNotMatch(html, /Mapa|Silêncio|Descubra|tranquilidade|Praça|Avenida|Parque|Jardim|Biblioteca/, 'UI text should stay in English');

console.log('frontend static checks OK');
