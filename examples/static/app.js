// Desi HTTP Server — External JS
document.addEventListener('DOMContentLoaded', () => {
  // Live clock
  const clock = document.getElementById('clock');
  if (clock) {
    const tick = () => {
      const now = new Date();
      clock.textContent = '⏰ ' + now.toLocaleTimeString();
    };
    tick();
    setInterval(tick, 1000);
  }

  // Animate cards on load
  document.querySelectorAll('.card').forEach((card, i) => {
    card.style.opacity = '0';
    card.style.transform = 'translateY(20px)';
    setTimeout(() => {
      card.style.transition = 'opacity 0.5s, transform 0.5s';
      card.style.opacity = '1';
      card.style.transform = 'translateY(0)';
    }, 200 + i * 150);
  });

  // Fetch API health on load
  fetch('/api/health')
    .then(r => r.json())
    .then(data => {
      const el = document.getElementById('health-status');
      if (el) el.textContent = '● ' + data.status;
    })
    .catch(() => {
      const el = document.getElementById('health-status');
      if (el) el.textContent = '○ offline';
    });
});
