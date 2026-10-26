(() => {
  const config = JSON.parse(document.getElementById('analyticsConfig').textContent);
  const key = 'dashboardAnalyticsConsent';
  const consent = () => { try { return sessionStorage.getItem(key) === 'yes'; } catch (_) { return false; } };
  if (consent()) {
    const script = document.createElement('script');
    script.src = config.src;
    script.defer = true;
    if (config.domain) script.dataset.domain = config.domain;
    else {
      window.plausible = window.plausible || function() { (window.plausible.q = window.plausible.q || []).push(arguments); };
      window.plausible.init = window.plausible.init || function(options) { window.plausible.o = options || {}; };
      window.plausible.init();
    }
    document.head.appendChild(script);
  }
  document.addEventListener('DOMContentLoaded', () => {
    const button = document.getElementById('analyticsConsent');
    const status = document.getElementById('analyticsConsentStatus');
    button.textContent = consent() ? 'Analytics deaktivieren' : 'Analytics erlauben';
    button.addEventListener('click', () => {
      const wasEnabled = consent();
      try {
        if (wasEnabled) sessionStorage.removeItem(key);
        else sessionStorage.setItem(key, 'yes');
        location.reload();
      } catch (_) { status.textContent = 'Einstellung konnte nicht gespeichert werden. Analytics bleibt unverändert.'; }
    });
  });
})();
