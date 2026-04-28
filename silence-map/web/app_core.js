(function attachSilenceCore(root, factory) {
  const core = factory();
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = core;
  }
  root.SilenceCore = core;
})(typeof globalThis !== 'undefined' ? globalThis : this, function createSilenceCore() {
  function chooseViewportRadius(rawRadius, maxRadius) {
    const safeRaw = Math.max(0, Math.ceil(Number(rawRadius) || 0));
    const boundsOnly = safeRaw > maxRadius;
    return {
      rawRadius: safeRaw,
      boundsOnly,
      radius: boundsOnly ? 0 : Math.max(1, safeRaw)
    };
  }

  function cloneReport(report) {
    return {
      ...report,
      location: { ...report.location }
    };
  }

  function pointInsideBounds(report, bounds) {
    const point = [report.location.latitude, report.location.longitude];
    if (bounds && typeof bounds.contains === 'function') {
      return bounds.contains(point);
    }
    return report.location.latitude <= bounds.north
      && report.location.latitude >= bounds.south
      && report.location.longitude <= bounds.east
      && report.location.longitude >= bounds.west;
  }

  function rankLocalReports(reports, bounds, distanceForReport) {
    return reports
      .filter(report => pointInsideBounds(report, bounds))
      .map(report => ({
        report,
        distance: distanceForReport(report)
      }))
      .sort((a, b) => (
        b.report.quietness_level - a.report.quietness_level
        || b.report.confirmations - a.report.confirmations
        || a.distance - b.distance
      ))
      .map(({ report }) => ({
        place_name: report.place_name,
        location: report.location,
        average_quietness: report.quietness_level,
        report_count: Math.max(1, report.confirmations),
        confirmation_count: report.confirmations
      }));
  }

  function optimisticConfirmation(report) {
    const next = cloneReport(report);
    next.confirmations += 1;
    return next;
  }

  function rollbackConfirmation(previous) {
    return cloneReport(previous);
  }

  function commitConfirmation(saved) {
    return cloneReport(saved);
  }

  return {
    chooseViewportRadius,
    cloneReport,
    pointInsideBounds,
    rankLocalReports,
    optimisticConfirmation,
    rollbackConfirmation,
    commitConfirmation
  };
});
