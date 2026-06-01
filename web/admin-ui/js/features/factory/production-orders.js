/* ─── Factory / Production Orders ────────────────────────────────────────────
   Production order list, create, start, complete.
   Depends on: helpers.js, mock-data.js, toast.js, router.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

function renderFacOrders() {
  return `
    ${pageHeader(
      'Production Orders',
      'Create and manage production runs',
      `<button class="btn btn-primary btn-sm" onclick="openModal('newPO')">＋ New Production Order</button>`
    )}
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>ID</th><th>Item</th><th>Qty</th><th>Status</th><th>Scheduled</th><th>Actions</th></tr>
          </thead>
          <tbody>
            ${PRODUCTION_ORDERS.map(p => `
              <tr>
                <td><code>${p.id}</code></td>
                <td>${p.item}</td>
                <td>${p.qty} pcs</td>
                <td>${statusBadge(p.status)}</td>
                <td><span class="small">${p.start} → ${p.end}</span></td>
                <td>
                  ${p.status === 'PENDING'
                    ? `<button class="btn btn-primary btn-sm" onclick="startPO('${p.id}')">▶ Start</button>`
                    : p.status === 'IN_PROGRESS'
                    ? `<button class="btn btn-success btn-sm" onclick="completePO('${p.id}')">✓ Complete</button>`
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
function startPO(id) {
  const p = PRODUCTION_ORDERS.find(o => o.id === id);
  if (!p) return;
  p.status = 'IN_PROGRESS';
  toast(`Production Order ${id} started!`, 'success');
  renderPage(state.page);
}

function completePO(id) {
  const p = PRODUCTION_ORDERS.find(o => o.id === id);
  if (!p) return;
  p.status = 'COMPLETED';

  // Auto-create cost record
  COST_RECORDS.push({
    po:       id,
    item:     p.item,
    material: 2800000,
    labor:    350000,
    overhead: 250000,
    total:    3400000,
    per_unit: Math.round(3400000 / p.qty),
    output:   p.qty,
  });

  toast(`Production Order ${id} completed! Cost record created.`, 'success');
  renderPage(state.page);
}
