/* ─── HQ / B2B Sales Orders ──────────────────────────────────────────────────
   Wholesale fulfillment — HQ sells, Factory executes.
   Depends on: helpers.js, mock-data.js, toast.js, router.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

function renderHQB2B() {
  return `
    ${pageHeader(
      'B2B Sales Orders',
      'Wholesale fulfillment — HQ sells, Factory executes',
      `<button class="btn btn-primary btn-sm" onclick="openModal('newB2B')">＋ New B2B Order</button>`
    )}
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>ID</th><th>Customer</th><th>Item</th><th>Qty</th>
              <th>Value</th><th>Factory</th><th>Status</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${B2B_ORDERS.map(b => `
              <tr>
                <td><code>${b.id}</code></td>
                <td>${b.customer}</td>
                <td>${b.item}</td>
                <td>${b.qty} pcs</td>
                <td>${fmt(b.price)}</td>
                <td>${b.factory
                  ? `<span class="badge badge-fac">${b.factory}</span>`
                  : '<span class="faint">Unassigned</span>'}</td>
                <td>${statusBadge(b.status)}</td>
                <td>
                  ${b.status === 'PENDING'
                    ? `<button class="btn btn-primary btn-sm" onclick="assignB2B('${b.id}')">Assign Factory</button>`
                    : b.status === 'GOODS_ISSUED'
                    ? `<button class="btn btn-success btn-sm" onclick="completeB2B('${b.id}')">Mark Delivered</button>`
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
function assignB2B(id) {
  const b = B2B_ORDERS.find(o => o.id === id);
  if (!b) return;
  b.factory = 'FACTORY';
  b.status  = 'ASSIGNED';
  toast(`B2B Order ${id} assigned to Factory.`, 'success');
  renderPage(state.page);
}

function completeB2B(id) {
  const b = B2B_ORDERS.find(o => o.id === id);
  if (!b) return;
  b.status = 'COMPLETED';
  toast(`B2B Order ${id} marked as delivered. Revenue recognized.`, 'success');
  renderPage(state.page);
}
