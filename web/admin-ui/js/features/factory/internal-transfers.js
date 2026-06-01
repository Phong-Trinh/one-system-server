/* ─── Factory / Internal Transfers ──────────────────────────────────────────
   ITO list and 1-click same-site "Move to Store" action.
   Depends on: helpers.js, mock-data.js, toast.js, router.js.
──────────────────────────────────────────────────────────────────────────── */

function renderFacITO() {
  const facITOs = ITORDERS.filter(i => i.from === 'FACTORY');

  return `
    ${pageHeader('Internal Transfers', 'Fulfill stock transfers to the Store — same-site 1-click path')}

    <!-- Same-site notice -->
    <div class="card"
         style="background:hsl(38,60%,8%); border-color:hsl(38,60%,20%);
                margin-bottom:16px; display:flex; gap:12px; align-items:flex-start">
      <span style="font-size:20px">⚡</span>
      <div>
        <div style="font-weight:700; color:var(--fac)">Same-Site Transfer Active</div>
        <div class="small" style="color:var(--text-dim); margin-top:4px">
          Factory and Store share <code>site-main</code>.
          The "Move to Store" action auto-generates a GI + GR simultaneously —
          no IN_TRANSIT phase, instant stock update at both nodes.
        </div>
      </div>
    </div>

    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>ITO ID</th><th>Item</th><th>Qty</th><th>Trigger</th><th>Status</th><th>Actions</th></tr>
          </thead>
          <tbody>
            ${facITOs.map(t => `
              <tr>
                <td><code>${t.id}</code></td>
                <td>${t.item}</td>
                <td>${t.qty} pcs</td>
                <td><span class="badge badge-dim">${t.trigger}</span></td>
                <td>${statusBadge(t.status)}</td>
                <td>
                  ${t.status === 'AUTO_APPROVED'
                    ? `<button class="btn btn-primary btn-sm" onclick="moveToStore('${t.id}')">⚡ Move to Store</button>`
                    : '<span class="faint small">—</span>'}
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

/* ── Action ──────────────────────────────────────────────── */
/**
 * Same-site 1-click transfer: auto GI + GR, instant stock update.
 * @param {string} id - ITO id.
 */
function moveToStore(id) {
  const t = ITORDERS.find(i => i.id === id);
  if (!t) return;

  t.status = 'COMPLETED';

  // Decrement Factory stock
  const facStock = NODE_STOCK.FACTORY.find(s => s.name === t.item);
  if (facStock) facStock.qty = Math.max(0, facStock.qty - t.qty);

  // Increment Store stock
  const stoStock = NODE_STOCK.STORE.find(s => s.name === t.item);
  if (stoStock) stoStock.qty += t.qty;

  toast(
    `⚡ Same-site transfer complete! GI + GR auto-confirmed. ` +
    `${t.qty}× ${t.item} moved to Store instantly.`,
    'success'
  );
  renderPage(state.page);
}
