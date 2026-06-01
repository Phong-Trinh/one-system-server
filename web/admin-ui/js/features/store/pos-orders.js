/* ─── Store / POS Orders ─────────────────────────────────────────────────────
   Point-of-sale order list, create, start, complete.
   Depends on: helpers.js, mock-data.js, toast.js, router.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

function renderStoPOS() {
  return `
    ${pageHeader(
      'POS Orders',
      'Point of sale and platform order management',
      `<button class="btn btn-primary btn-sm" onclick="openModal('newOrder')">＋ New Order</button>`
    )}
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Order ID</th><th>Items</th><th>Source</th><th>Time</th><th>Status</th><th>Actions</th></tr>
          </thead>
          <tbody>
            ${POS_ORDERS.map(o => `
              <tr>
                <td><code>${o.id}</code></td>
                <td style="max-width:260px">${o.items}</td>
                <td><span class="badge badge-dim">${o.source}</span></td>
                <td>${o.time}</td>
                <td>${statusBadge(o.status)}</td>
                <td>
                  ${o.status === 'PENDING'
                    ? `<button class="btn btn-primary btn-sm" onclick="startOrder('${o.id}')">▶ Start</button>`
                    : o.status === 'IN_PROGRESS'
                    ? `<button class="btn btn-success btn-sm" onclick="completeOrder('${o.id}')">✓ Complete</button>`
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

/* ── Actions ─────────────────────────────────────────────── */
function startOrder(id) {
  const o = POS_ORDERS.find(x => x.id === id);
  if (!o) return;
  o.status = 'IN_PROGRESS';
  toast(`Order ${id} started.`, 'info');
  renderPage(state.page);
}

function completeOrder(id) {
  const o = POS_ORDERS.find(x => x.id === id);
  if (!o) return;
  o.status = 'COMPLETED';
  toast(`Order ${id} completed and handed off!`, 'success');
  renderPage(state.page);
}
