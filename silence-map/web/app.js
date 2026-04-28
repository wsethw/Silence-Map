const SAO_PAULO = [-23.5505, -46.6333];
    const MAX_RADIUS_METERS = 50000;
    const core = globalThis.SilenceCore;
    const IS_FILE_PROTOCOL = location.protocol === 'file:';
    const API_BASE_URL = IS_FILE_PROTOCOL ? 'http://localhost:8080' : '';
    const WS_URL = IS_FILE_PROTOCOL
      ? 'ws://localhost:8080/ws'
      : `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`;
    const QUIETNESS_COLORS = {
      1: '#ff4444',
      2: '#ff8800',
      3: '#ffbb33',
      4: '#66bb6a',
      5: '#43a047'
    };
    const QUIETNESS_LABELS = {
      1: 'noisy',
      2: 'active',
      3: 'balanced',
      4: 'quiet',
      5: 'extremely quiet'
    };
    const DEMO_REPORTS = [
      {
        id: 'demo-republic',
        user_id: 'demo',
        location: { latitude: -23.5436, longitude: -46.6426 },
        quietness_level: 2,
        place_name: 'Republic Square',
        confirmations: 6,
        created_at: new Date().toISOString()
      },
      {
        id: 'demo-luz',
        user_id: 'demo',
        location: { latitude: -23.5341, longitude: -46.6351 },
        quietness_level: 4,
        place_name: 'Luz Park',
        confirmations: 11,
        created_at: new Date().toISOString()
      },
      {
        id: 'demo-ibirapuera',
        user_id: 'demo',
        location: { latitude: -23.5874, longitude: -46.6576 },
        quietness_level: 5,
        place_name: 'Ibirapuera Park',
        confirmations: 18,
        created_at: new Date().toISOString()
      },
      {
        id: 'demo-paulista',
        user_id: 'demo',
        location: { latitude: -23.5614, longitude: -46.6559 },
        quietness_level: 3,
        place_name: 'Paulista Avenue',
        confirmations: 9,
        created_at: new Date().toISOString()
      },
      {
        id: 'demo-aclimacao',
        user_id: 'demo',
        location: { latitude: -23.5728, longitude: -46.6294 },
        quietness_level: 5,
        place_name: 'Aclimacao Park',
        confirmations: 14,
        created_at: new Date().toISOString()
      }
    ];

    const markers = new Map();
    const reportsByID = new Map();
    const userId = getOrCreateUserId();

    const elements = {
      appShell: document.getElementById('appShell'),
      controlPanel: document.getElementById('controlPanel'),
      mobileToggle: document.getElementById('mobileToggle'),
      toggleGlyph: document.getElementById('toggleGlyph'),
      dayOfWeek: document.getElementById('dayOfWeek'),
      hour: document.getElementById('hour'),
      searchQuiet: document.getElementById('searchQuiet'),
      reportCenter: document.getElementById('reportCenter'),
      resultsList: document.getElementById('resultsList'),
      reportCount: document.getElementById('reportCount'),
      avgQuietness: document.getElementById('avgQuietness'),
      statusDot: document.getElementById('statusDot'),
      statusText: document.getElementById('statusText'),
      livePill: document.getElementById('livePill'),
      reportModal: document.getElementById('reportModal'),
      reportForm: document.getElementById('reportForm'),
      placeName: document.getElementById('placeName'),
      quietnessSlider: document.getElementById('quietnessSlider'),
      quietnessReadout: document.getElementById('quietnessReadout'),
      submitReport: document.getElementById('submitReport'),
      cancelReport: document.getElementById('cancelReport'),
      closeModal: document.getElementById('closeModal'),
      modalLocation: document.getElementById('modalLocation'),
      toast: document.getElementById('toast')
    };

    let selectedLatLng = null;
    let socket = null;
    let reconnectTimer = null;
    let reconnectAttempt = 0;
    let viewportReloadTimer = null;
    let lastRecentLoadID = 0;
    let previouslyFocusedElement = null;

    const map = L.map('map', {
      zoomControl: false,
      minZoom: 3,
      maxZoom: 19,
      preferCanvas: true
    }).setView(SAO_PAULO, 13);

    L.control.zoom({ position: 'topright' }).addTo(map);

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
      minZoom: 3,
      maxZoom: 19,
      crossOrigin: true,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/">CARTO</a>'
    }).addTo(map);

    initializeUI();
    renderDemoReports();
    connectWebSocket();
    loadRecentReports();

    map.whenReady(() => {
      map.invalidateSize();
      subscribeVisibleBounds();
    });

    map.on('click', event => {
      selectedLatLng = event.latlng;
      openReportModal(event.latlng);
    });

    map.on('moveend zoomend', () => {
      subscribeVisibleBounds();
      scheduleViewportReload();
      updateSummaryStats();
    });

    elements.searchQuiet.addEventListener('click', queryQuietPlaces);
    elements.reportCenter.addEventListener('click', () => {
      selectedLatLng = map.getCenter();
      openReportModal(selectedLatLng, elements.reportCenter);
    });
    elements.reportForm.addEventListener('submit', submitReport);
    elements.cancelReport.addEventListener('click', closeReportModal);
    elements.closeModal.addEventListener('click', closeReportModal);
    elements.reportModal.addEventListener('click', event => {
      if (event.target === elements.reportModal) closeReportModal();
    });
    elements.reportModal.addEventListener('keydown', trapModalFocus);
    elements.quietnessSlider.addEventListener('input', updateQuietnessReadout);
    elements.mobileToggle.addEventListener('click', toggleMobilePanel);
    document.addEventListener('keydown', event => {
      if (event.key === 'Escape' && elements.reportModal.classList.contains('open')) {
        closeReportModal();
      }
    });

    function initializeUI() {
      const now = new Date();
      let isoDay = now.getDay();
      if (isoDay === 0) isoDay = 7;

      for (let hour = 0; hour < 24; hour += 1) {
        const option = document.createElement('option');
        option.value = String(hour);
        option.textContent = `${String(hour).padStart(2, '0')}:00`;
        elements.hour.appendChild(option);
      }

      elements.dayOfWeek.value = String(isoDay);
      elements.hour.value = String(now.getHours());
      updateQuietnessReadout();
    }

    function renderDemoReports() {
      DEMO_REPORTS.forEach(report => upsertReport(report, { demo: true }));
      updateSummaryStats();
    }

    function upsertReport(report, options = {}) {
      if (!report || !report.id || !report.location) return;

      const normalized = normalizeReport(report, options.source);
      reportsByID.set(normalized.id, normalized);

      const color = quietnessColor(normalized.quietness_level);
      const latLng = [normalized.location.latitude, normalized.location.longitude];
      const fillOpacity = Math.min(0.9, 0.68 + normalized.confirmations * 0.012);
      const markerOptions = {
        radius: 12,
        color: 'rgba(255, 255, 255, 0.72)',
        weight: 2,
        opacity: 0.9,
        fillColor: color,
        fillOpacity: options.demo ? Math.max(0.72, fillOpacity) : fillOpacity
      };

      const existing = markers.get(normalized.id);
      if (existing) {
        existing.setLatLng(latLng);
        existing.setStyle(markerOptions);
        existing.setTooltipContent(createTooltipContent(normalized));
        existing.setPopupContent(createPopupContent(normalized));
      } else {
        const marker = L.circleMarker(latLng, markerOptions)
          .bindTooltip(createTooltipContent(normalized), {
            className: 'silence-tooltip',
            direction: 'top',
            offset: [0, -10]
          })
          .bindPopup(createPopupContent(normalized))
          .addTo(map);

        marker.on('popupopen', event => {
          const popupEl = event.popup.getElement();
          const button = popupEl && popupEl.querySelector('button[data-confirm-report]');
          if (button && button.dataset.confirmReport === normalized.id) {
            button.addEventListener('click', () => confirmReport(normalized.id), { once: true });
          }
        });

        markers.set(normalized.id, marker);
      }

      updateSummaryStats();
    }

    function normalizeReport(report, source) {
      return {
        id: report.id,
        user_id: report.user_id || 'unknown',
        location: {
          latitude: Number(report.location.latitude),
          longitude: Number(report.location.longitude)
        },
        quietness_level: Math.max(1, Math.min(5, Math.round(Number(report.quietness_level || report.quietness || 3)))),
        place_name: report.place_name || 'Unnamed place',
        confirmations: Number(report.confirmations || report.confirmation_count || 0),
        created_at: report.created_at || new Date().toISOString(),
        source: source || report.source || 'api'
      };
    }

    function createTooltipContent(report) {
      const tooltip = document.createElement('span');
      tooltip.textContent = `${report.place_name} — Silence ${report.quietness_level}`;
      return tooltip;
    }

    function createPopupContent(report) {
      const color = quietnessColor(report.quietness_level);
      const root = document.createElement('div');
      root.className = 'popup-actions';

      const title = document.createElement('span');
      title.className = 'popup-title';
      title.textContent = report.place_name;

      const meta = document.createElement('span');
      meta.className = 'popup-meta';
      meta.append('Silence ');
      const score = document.createElement('strong');
      score.style.color = color;
      score.textContent = `${report.quietness_level}/5`;
      meta.append(score, ` · ${report.confirmations} confirmations`);

      const button = document.createElement('button');
      button.className = 'primary-action';
      button.type = 'button';
      button.dataset.confirmReport = report.id;
      button.textContent = 'Confirm quietness';

      root.append(title, meta, button);
      return root;
    }

    function updateSummaryStats() {
      const bounds = map.getBounds();
      const reports = Array.from(reportsByID.values()).filter(report => {
        return bounds.contains([report.location.latitude, report.location.longitude]);
      });
      const total = reports.length;
      const average = total === 0
        ? 0
        : reports.reduce((sum, report) => sum + report.quietness_level, 0) / total;

      elements.reportCount.textContent = String(total);
      elements.avgQuietness.textContent = average.toFixed(1);
    }

    async function queryQuietPlaces() {
      setButtonLoading(elements.searchQuiet, true);
      renderEmptyState(elements.resultsList, 'Searching quiet places in the visible region...');

      const viewport = getViewportQuery();
      const params = new URLSearchParams({
        lat: String(viewport.center.lat),
        lng: String(viewport.center.lng),
        radius: String(viewport.radius),
        spatial_mode: viewport.boundsOnly ? 'bounds' : 'radius_bounds',
        north: String(viewport.bounds.getNorth()),
        south: String(viewport.bounds.getSouth()),
        east: String(viewport.bounds.getEast()),
        west: String(viewport.bounds.getWest()),
        day_of_week: elements.dayOfWeek.value,
        hour: elements.hour.value,
        limit: '5'
      });

      try {
        const places = await fetchJSON(`${API_BASE_URL}/api/places/quiet?${params.toString()}`);
        renderQuietResults(places, viewport.center);
      } catch (error) {
        const fallback = rankLocalReports(viewport.center, viewport.bounds).slice(0, 5);
        if (fallback.length > 0) {
          renderQuietResults(fallback, viewport.center, 'Backend unavailable. Showing local demo data.');
        } else {
          renderEmptyState(elements.resultsList, 'No quiet places found.');
        }
      } finally {
        setButtonLoading(elements.searchQuiet, false);
      }
    }

    function renderQuietResults(places, center, note = '') {
      if (!places || places.length === 0) {
        renderEmptyState(elements.resultsList, 'No quiet places found.');
        return;
      }

      clearElement(elements.resultsList);
      if (note) {
        const noteEl = document.createElement('div');
        noteEl.className = 'empty-state';
        noteEl.textContent = note;
        elements.resultsList.appendChild(noteEl);
      }

      places.slice(0, 5).forEach(place => {
        const normalized = normalizeQuietPlace(place);
        const level = Math.max(1, Math.min(5, Math.round(normalized.average_quietness)));
        const distance = map.distance(center, [normalized.location.latitude, normalized.location.longitude]);
        const color = quietnessColor(level);
        const item = document.createElement('button');
        item.type = 'button';
        item.className = 'result-item';

        const swatch = document.createElement('span');
        swatch.className = 'result-swatch';
        swatch.style.background = color;
        swatch.style.color = color;

        const copy = document.createElement('span');
        copy.className = 'result-copy';
        const title = document.createElement('span');
        title.className = 'result-title';
        title.textContent = normalized.place_name;
        const meta = document.createElement('span');
        meta.className = 'result-meta';
        meta.textContent = `${formatDistance(distance)} · ${normalized.report_count} reports`;
        copy.append(title, meta);

        const score = document.createElement('span');
        score.className = 'result-score';
        const scoreValue = document.createElement('strong');
        scoreValue.style.color = color;
        scoreValue.textContent = normalized.average_quietness.toFixed(1);
        const scoreSuffix = document.createElement('span');
        scoreSuffix.textContent = '/ 5';
        score.append(scoreValue, scoreSuffix);

        item.append(swatch, copy, score);
        item.addEventListener('click', () => {
          map.setView([normalized.location.latitude, normalized.location.longitude], Math.max(map.getZoom(), 15), { animate: true });
        });
        elements.resultsList.appendChild(item);
      });
    }

    function renderEmptyState(container, message) {
      clearElement(container);
      const empty = document.createElement('div');
      empty.className = 'empty-state';
      empty.textContent = message;
      container.appendChild(empty);
    }

    function clearElement(element) {
      while (element.firstChild) {
        element.removeChild(element.firstChild);
      }
    }

    function normalizeQuietPlace(place) {
      return {
        place_name: place.place_name || place.placeName || 'Unnamed place',
        location: {
          latitude: Number(place.location.latitude),
          longitude: Number(place.location.longitude)
        },
        average_quietness: Number(place.average_quietness || place.quietness_level || 3),
        report_count: Number(place.report_count || place.confirmation_count || 1)
      };
    }

    function rankLocalReports(center, bounds) {
      return core.rankLocalReports(
        Array.from(reportsByID.values()),
        bounds,
        report => map.distance(center, [report.location.latitude, report.location.longitude])
      );
    }

    function openReportModal(latLng, triggerElement) {
      previouslyFocusedElement = triggerElement || (document.activeElement instanceof HTMLElement ? document.activeElement : null);
      elements.modalLocation.textContent = `${latLng.lat.toFixed(5)}, ${latLng.lng.toFixed(5)}`;
      setAppInert(true);
      elements.reportModal.classList.add('open');
      elements.reportModal.setAttribute('aria-hidden', 'false');
      requestAnimationFrame(() => elements.placeName.focus());
    }

    function closeReportModal() {
      elements.reportModal.classList.remove('open');
      elements.reportModal.setAttribute('aria-hidden', 'true');
      setAppInert(false);
      elements.reportForm.reset();
      elements.quietnessSlider.value = '4';
      selectedLatLng = null;
      updateQuietnessReadout();
      if (previouslyFocusedElement && document.contains(previouslyFocusedElement)) {
        previouslyFocusedElement.focus();
      }
      previouslyFocusedElement = null;
    }

    function setAppInert(inert) {
      if ('inert' in elements.appShell) {
        elements.appShell.inert = inert;
      }
      if (inert) {
        elements.appShell.setAttribute('aria-hidden', 'true');
      } else {
        elements.appShell.removeAttribute('aria-hidden');
      }
    }

    function trapModalFocus(event) {
      if (event.key === 'Escape') {
        closeReportModal();
        return;
      }
      if (event.key !== 'Tab') {
        return;
      }

      const focusable = getFocusableElements(elements.reportModal);
      if (focusable.length === 0) {
        event.preventDefault();
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    function getFocusableElements(root) {
      return Array.from(root.querySelectorAll([
        'a[href]',
        'button:not([disabled])',
        'input:not([disabled])',
        'select:not([disabled])',
        'textarea:not([disabled])',
        '[tabindex]:not([tabindex="-1"])'
      ].join(','))).filter(element => {
        return element instanceof HTMLElement && element.offsetParent !== null;
      });
    }

    async function submitReport(event) {
      event.preventDefault();
      if (!selectedLatLng) return;

      setButtonLoading(elements.submitReport, true);

      const quietness = Number(elements.quietnessSlider.value);
      const placeName = elements.placeName.value.trim() || 'New quiet spot';
      const optimisticReport = {
        id: `local-${Date.now()}`,
        user_id: userId,
        location: { latitude: selectedLatLng.lat, longitude: selectedLatLng.lng },
        quietness_level: quietness,
        place_name: placeName,
        confirmations: 0,
        created_at: new Date().toISOString()
      };

      upsertReport(optimisticReport, { source: 'local' });
      closeReportModal();

      try {
        const saved = await fetchJSON(`${API_BASE_URL}/api/reports`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            latitude: optimisticReport.location.latitude,
            longitude: optimisticReport.location.longitude,
            quietness,
            place_name: placeName
          })
        });
        removeReport(optimisticReport.id);
        upsertReport(saved, { source: 'api' });
        showToast('Report saved and shared with nearby viewers.');
      } catch (error) {
        showToast('Backend unavailable. The report is visible locally until sync returns.');
      } finally {
        setButtonLoading(elements.submitReport, false);
      }
    }

    function removeReport(reportID) {
      const marker = markers.get(reportID);
      if (marker) {
        map.removeLayer(marker);
        markers.delete(reportID);
      }
      reportsByID.delete(reportID);
      updateSummaryStats();
    }

    async function loadRecentReports() {
      const loadID = ++lastRecentLoadID;
      const viewport = getViewportQuery();
      const params = new URLSearchParams({
        lat: String(viewport.center.lat),
        lng: String(viewport.center.lng),
        radius: String(viewport.radius),
        spatial_mode: viewport.boundsOnly ? 'bounds' : 'radius_bounds',
        north: String(viewport.bounds.getNorth()),
        south: String(viewport.bounds.getSouth()),
        east: String(viewport.bounds.getEast()),
        west: String(viewport.bounds.getWest())
      });

      try {
        const reports = await fetchJSON(`${API_BASE_URL}/api/reports/recent?${params.toString()}`);
        if (loadID !== lastRecentLoadID) return;

        const seenAPIReports = new Set();
        reports.forEach(report => {
          seenAPIReports.add(report.id);
          upsertReport(report, { source: 'api' });
        });

        for (const report of reportsByID.values()) {
          if (report.source === 'api' && !seenAPIReports.has(report.id)) {
            removeReport(report.id);
          }
        }
        updateSummaryStats();
      } catch (error) {
        // The demo points keep the map valuable even when the backend is not running.
        updateSummaryStats();
      }
    }

    function scheduleViewportReload() {
      clearTimeout(viewportReloadTimer);
      viewportReloadTimer = setTimeout(loadRecentReports, 300);
    }

    function connectWebSocket() {
      clearTimeout(reconnectTimer);
      setConnectionStatus('reconnecting', 'Reconnecting...', 'retrying');

      socket = new WebSocket(WS_URL);

      socket.addEventListener('open', () => {
        reconnectAttempt = 0;
        setConnectionStatus('online', 'Live', 'connected');
        subscribeVisibleBounds();
      });

      socket.addEventListener('message', event => {
        try {
          const payload = JSON.parse(event.data);
          if (payload.type === 'error') {
            showToast(payload.message || 'Realtime subscription error.');
            return;
          }
          if (payload.type === 'new_report' && payload.report) {
            upsertReport(payload.report, { source: 'api' });
          }
          if (payload.type === 'confirmation' && payload.report) {
            upsertReport(payload.report, { source: 'api' });
          }
        } catch (error) {
          // Ignore malformed realtime events without affecting the map.
        }
      });

      socket.addEventListener('close', () => {
        scheduleReconnect();
      });

      socket.addEventListener('error', () => {
        setConnectionStatus('offline', 'Offline', 'offline');
        socket.close();
      });
    }

    function scheduleReconnect() {
      reconnectAttempt += 1;
      const delay = Math.min(8000, 1000 * Math.pow(2, reconnectAttempt - 1));
      setConnectionStatus('reconnecting', 'Reconnecting...', `${Math.ceil(delay / 1000)}s`);
      loadRecentReports();
      reconnectTimer = setTimeout(connectWebSocket, delay);
    }

    function subscribeVisibleBounds() {
      if (!socket || socket.readyState !== WebSocket.OPEN) return;

      const bounds = map.getBounds();
      socket.send(JSON.stringify({
        action: 'subscribe',
        bounds: {
          north: bounds.getNorth(),
          south: bounds.getSouth(),
          east: bounds.getEast(),
          west: bounds.getWest()
        }
      }));
    }

    function setConnectionStatus(state, text, pill) {
      elements.statusDot.className = `status-dot ${state}`;
      elements.statusText.textContent = text;
      elements.livePill.textContent = pill;
    }

    async function confirmReport(reportID) {
      const report = reportsByID.get(reportID);
      if (!report) return;

      const previous = core.cloneReport(report);
      const optimistic = core.optimisticConfirmation(report);
      upsertReport(optimistic, { source: report.source });

      if (reportID.startsWith('demo-') || reportID.startsWith('local-')) {
        showToast('Confirmation added locally for the demo point.');
        return;
      }

      try {
        const saved = await fetchJSON(`${API_BASE_URL}/api/reports/${encodeURIComponent(reportID)}/confirm`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' }
        });
        upsertReport(saved, { source: 'api' });
        showToast('Quietness confirmed. Thank you for keeping the map accurate.');
      } catch (error) {
        upsertReport(core.rollbackConfirmation(previous), { source: previous.source });
        showToast(error.message || 'Could not confirm this report. The previous count was restored.');
      }
    }

    function getViewportQuery() {
      const center = map.getCenter();
      const bounds = map.getBounds();
      const corners = [bounds.getNorthEast(), bounds.getNorthWest(), bounds.getSouthEast(), bounds.getSouthWest()];
      const rawRadius = Math.ceil(Math.max(...corners.map(corner => map.distance(center, corner))));
      const { radius, boundsOnly } = core.chooseViewportRadius(rawRadius, MAX_RADIUS_METERS);
      return { center, bounds, radius, rawRadius, boundsOnly };
    }

    function updateQuietnessReadout() {
      const value = Number(elements.quietnessSlider.value);
      const color = quietnessColor(value);
      elements.quietnessReadout.textContent = `${value} · ${QUIETNESS_LABELS[value]}`;
      elements.quietnessSlider.style.setProperty('--slider-color', color);
      elements.quietnessReadout.style.setProperty('--slider-color', color);
    }

    function toggleMobilePanel() {
      const collapsed = elements.controlPanel.classList.toggle('collapsed');
      elements.mobileToggle.setAttribute('aria-expanded', String(!collapsed));
      elements.toggleGlyph.textContent = collapsed ? '⌃' : '⌄';
      setTimeout(() => map.invalidateSize(), 220);
    }

    function quietnessColor(level) {
      const safeLevel = Math.max(1, Math.min(5, Math.round(Number(level) || 1)));
      return QUIETNESS_COLORS[safeLevel];
    }

    function formatDistance(meters) {
      if (meters < 1000) return `${Math.round(meters)} m`;
      return `${(meters / 1000).toFixed(1)} km`;
    }

    function setButtonLoading(button, loading) {
      button.disabled = loading;
      button.classList.toggle('is-loading', loading);
    }

    async function fetchJSON(url, options) {
      const response = await fetch(url, {
        credentials: 'include',
        ...options
      });
      const contentType = response.headers.get('content-type') || '';
      const payload = contentType.includes('application/json') ? await response.json() : null;
      if (!response.ok) {
        throw new Error(payload?.error || `HTTP ${response.status}`);
      }
      return payload;
    }

    function showToast(message) {
      elements.toast.textContent = message;
      elements.toast.hidden = false;
      clearTimeout(showToast.timer);
      showToast.timer = setTimeout(() => {
        elements.toast.hidden = true;
      }, 4200);
    }

    function getOrCreateUserId() {
      const key = 'silence-map-user-id';
      const existing = localStorage.getItem(key);
      if (existing) return existing;

      const generated = globalThis.crypto?.randomUUID
        ? `web-${globalThis.crypto.randomUUID().slice(0, 8)}`
        : `web-${Date.now().toString(36)}`;
      localStorage.setItem(key, generated);
      return generated;
    }
