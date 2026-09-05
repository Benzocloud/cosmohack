/* ═══ TerraLens hero · scroll-scrub движок ═══
   Видео — подложка без UI; все интерфейсные слои включаются
   полосами прозрачности, привязанными к секундам мастер-ролика
   (тайм-карта разметки: sheet_1s / sheet_scan_zoom / sheet_final_zoom). */

(() => {
  const clamp01 = (v) => Math.min(1, Math.max(0, v));
  const lerp = (a, b, t) => a + (b - a) * t;

  // Полоса прозрачности: 0 до a, линейный рост a→b, удержание, спад c→d.
  const band = (p, a, b, c, d) => {
    if (p <= a || p >= d) return 0;
    if (p < b) return (p - a) / (b - a);
    if (p > c) return (d - p) / (d - c);
    return 1;
  };

  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;

  const hero = document.querySelector('.hero');
  const footer = document.querySelector('.site-footer');
  const siteHeader = document.querySelector('[data-site-header]');
  const video = document.querySelector('.hero__video');
  const startStill = document.querySelector('.still--start');
  const endStill = document.querySelector('.still--end');
  const layers = {
    hint: document.querySelector('[data-layer="hint"]'),
    intro: document.querySelector('[data-layer="intro"]'),
    orbit: document.querySelector('[data-layer="orbit"]'),
    scan: document.querySelector('[data-layer="scan"]'),
    polygon: document.querySelector('[data-layer="polygon"]'),
    field: document.querySelector('[data-layer="field"]'),
  };
  const outline = document.querySelector('.field-polygon__outline');
  const dots = document.querySelectorAll('.field-polygon__dots circle');

  const areaOptions = document.querySelectorAll('[data-area-id]');
  const areaIds = new Set([...areaOptions].map((option) => option.dataset.areaId));
  const panelLink = document.querySelector('.stepper__run');
  const regionTrigger = document.querySelector('[data-region-trigger]');
  const regionValue = document.querySelector('[data-region-value]');
  const regionMenu = document.querySelector('[data-region-menu]');
  const regionOptions = document.querySelectorAll('[data-region]');

  // Шапка нужна во время hero, но не должна висеть над отдельным CTA-футером.
  if (footer && siteHeader && 'IntersectionObserver' in window) {
    const footerObserver = new IntersectionObserver(([entry]) => {
      siteHeader.classList.toggle('is-hidden', entry.isIntersecting);
    }, { threshold: 0.05 });
    footerObserver.observe(footer);
  }
  const selectExampleArea = (id) => {
    if (!id || !areaIds.has(id)) return;
    areaOptions.forEach((option) => {
      const active = option.dataset.areaId === id;
      option.classList.toggle('is-selected', active);
      option.setAttribute('aria-pressed', String(active));
      const check = option.querySelector('.plots__check');
      check?.classList.toggle('is-on', active);
      if (check) check.textContent = active ? '✓' : '';
    });
    if (panelLink) panelLink.href = `/panel.html?area=${encodeURIComponent(id)}`;
  };
  areaOptions.forEach((option) => {
    option.addEventListener('click', () => selectExampleArea(option.dataset.areaId));
    option.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        selectExampleArea(option.dataset.areaId);
      }
    });
  });
  selectExampleArea(document.querySelector('.plots .is-selected')?.dataset.areaId || '');
  regionTrigger?.addEventListener('click', () => {
    const open = regionMenu.hidden;
    regionMenu.hidden = !open;
    regionTrigger.setAttribute('aria-expanded', String(open));
  });
  regionOptions.forEach((option) => option.addEventListener('click', () => {
    regionValue.textContent = option.dataset.region;
    regionMenu.hidden = true;
    regionTrigger.setAttribute('aria-expanded', 'false');
  }));
  if (reduced) {
    // Статичный вариант: стилл начала + орбитальная копия, без скраба.
    video.remove();
    endStill.remove();
    layers.hint.remove();
    layers.scan.remove();
    layers.polygon.remove();
    layers.field.remove();
    layers.orbit.remove();
    layers.intro.style.opacity = '1';
    layers.intro.classList.add('is-on');
    return;
  }

  // ── Тайм-карта (секунды мастер-ролика) ──
  const T = (sec) => sec / 30;
  const BANDS = {
    hint:    [-0.001, T(0.01), T(1.2),  T(3.2)],
    intro:   [T(-0.5), 0,       T(3.8),  T(5.4)],
    orbit:   [T(5.5),  T(6.8),  T(13.2), T(14.4)],
    scan:    [T(14.8), T(16.2), T(19.0), T(20.2)],
    field:   [T(25.4), T(27.0), 1.01,    1.02], // хвост «сырой»: слой держится до конца скролла
    outline: [T(26.3), T(28.3)],
    dots:    [T(27.6), T(28.8)],
  };

  let target = 0;
  let current = -1;          // −1 → форсируем первую отрисовку
  let duration = 0;
  let lastWritten = -1;

  const measure = () => {
    const total = hero.offsetHeight - innerHeight;
    target = total > 0 ? clamp01(-hero.getBoundingClientRect().top / total) : 0;
  };

  addEventListener('scroll', measure, { passive: true });
  addEventListener('resize', measure);

  video.addEventListener('loadedmetadata', () => { duration = video.duration; });

  const apply = () => {
    // Сглаживание дёрганья колеса/тачпада; на малых дельтах — снап.
    if (current < 0) current = target;
    current = Math.abs(target - current) < 0.0004 ? target : lerp(current, target, 0.14);
    const p = current;

    if (duration > 0 && !video.seeking) {
      const t = p * (duration - 0.06);
      if (Math.abs(t - lastWritten) > 0.012) {
        video.currentTime = t;
        lastWritten = t;
      }
    }

    const hintO = band(p, ...BANDS.hint);
    const introO = band(p, ...BANDS.intro);
    const orbitO = band(p, ...BANDS.orbit);
    const scanO = band(p, ...BANDS.scan);
    const fieldO = band(p, ...BANDS.field);

    // Гибрид качества: в покое (старт/финал) поверх видео — стиллы 1920×1080,
    // в движении видео мягко проступает/уходит под них.
    startStill.style.opacity = String(clamp01(1 - (p - 0.004) / 0.014));
    endStill.style.opacity = String(band(p, 0.975, 0.99, 1.01, 1.02));

    layers.hint.style.opacity = hintO;
    layers.hint.classList.toggle('is-on', hintO > 0.5);

    layers.intro.style.opacity = introO;
    layers.intro.style.transform = `translateY(${lerp(12, -12, clamp01((p - BANDS.intro[0]) / (BANDS.intro[3] - BANDS.intro[0])))}px)`;
    layers.intro.classList.toggle('is-on', introO > 0.5);

    layers.orbit.style.opacity = orbitO;
    layers.orbit.style.transform = `translateY(${lerp(18, -18, clamp01((p - BANDS.orbit[0]) / (BANDS.orbit[3] - BANDS.orbit[0])))}px)`;
    layers.orbit.classList.toggle('is-on', orbitO > 0.5);

    layers.scan.style.opacity = scanO;
    layers.scan.classList.toggle('is-on', scanO > 0.5);

    layers.field.style.opacity = fieldO;
    layers.field.style.transform = `translateY(${lerp(26, 0, clamp01((p - BANDS.field[0]) / (BANDS.field[1] - BANDS.field[0])))}px)`;
    layers.field.classList.toggle('is-on', fieldO > 0.5);

    const draw = clamp01((p - BANDS.outline[0]) / (BANDS.outline[1] - BANDS.outline[0]));
    outline.style.strokeDashoffset = 100 - 100 * draw;
    layers.polygon.style.opacity = draw > 0 ? 1 : 0;

    const dotsO = band(p, BANDS.dots[0], BANDS.dots[1], BANDS.dots[1], 1.02);
    dots.forEach((d) => { d.style.opacity = dotsO; });
  };

  const tick = () => {
    if (!document.hidden) apply();
    requestAnimationFrame(tick);
  };

  const start = () => {
    // loadedmetadata мог выстрелить до инициализации — берём длительность напрямую.
    if (!duration && Number.isFinite(video.duration)) duration = video.duration;
    // На мобильных браузерах одного seek через currentTime недостаточно для
    // запуска декодера. Тихо прогреваем видео и сразу ставим на паузу: дальше
    // кадр по-прежнему выбирается прокруткой.
    video.muted = true;
    const playback = video.play();
    if (playback && typeof playback.then === 'function') {
      playback.then(() => video.pause()).catch(() => {});
    }
    measure();
    requestAnimationFrame(tick);
  };

  // Если autoplay заблокирован политикой мобильного браузера, первое касание
  // даёт необходимый user gesture для того же короткого прогрева.
  const primeOnTouch = () => {
    video.muted = true;
    const playback = video.play();
    if (playback && typeof playback.then === 'function') {
      playback.then(() => video.pause()).catch(() => {});
    }
  };
  addEventListener('touchstart', primeOnTouch, { once: true, passive: true });

  if (video.readyState >= 1) start();
  else video.addEventListener('loadedmetadata', start, { once: true });
  // Если видео тянется медленно — движок всё равно стартует по постеру.
  setTimeout(() => { if (current < 0) { duration = 30; start(); } }, 2500);
})();
