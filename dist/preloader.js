// A native modal keeps the page inert while its booking controls initialize.
(async () => {
  const dialog = document.getElementById('PagePreloader');
  if (!dialog.showModal) return;
  dialog.showModal();
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  let timer;
  function closeSplash() {
    clearTimeout(timer);
    if (dialog.classList.contains('leaving')) return;
    dialog.classList.add('leaving');
    setTimeout(() => {
      dialog.close();
      dialog.remove();
      reduced.removeEventListener('change', closeSplash);
    }, reduced.matches ? 0 : 500);
  }
  dialog.addEventListener('cancel', event => {
    event.preventDefault();
    closeSplash();
  });
  reduced.addEventListener('change', closeSplash);
  const cfg = await SiteAPI.getSettings().catch(() => initialData().settings);
  if (reduced.matches || !cfg.animations || !cfg.preloaderDuration) {
    timer = setTimeout(closeSplash, 120);
    return;
  }
  const art = dialog.querySelector('.splash-art');
  const image = new Image();
  image.alt = '';
  timer = setTimeout(closeSplash, 8000);
  image.onload = () => {
    if (dialog.classList.contains('leaving')) return;
    art.querySelector('.splash-static').hidden = true;
    art.append(image);
    clearTimeout(timer);
    timer = setTimeout(closeSplash, cfg.preloaderDuration);
  };
  image.onerror = () => {
    clearTimeout(timer);
    timer = setTimeout(closeSplash, 600);
  };
  image.src = 'assets/brand/Breath-of-Unity.svg';
})();
