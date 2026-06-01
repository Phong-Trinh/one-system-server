/* ─── App Init ───────────────────────────────────────────────────────────────
   Entry point: attach global event listeners once DOM is ready.
   All feature modules, core, and shared files are loaded before this script.
──────────────────────────────────────────────────────────────────────────── */

// Allow pressing Enter on the password field to submit login
document.getElementById('login-password').addEventListener('keydown', function (e) {
  if (e.key === 'Enter') doLogin();
});
