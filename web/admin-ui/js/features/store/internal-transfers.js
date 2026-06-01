/* ─── Store / Internal Transfers ─────────────────────────────────────────────
   Incoming ITOs from Factory — auto-confirmed for same-site transfers.
   Depends on: helpers.js, mock-data.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

function renderStoITO() {
  const stoITOs = ITORDERS.filter(i => i.to === 'STORE');

  return `
    ${pageHeader(
      'Internal Transfers',
      'Incoming replenishment from Factory',
      `<button class="btn btn-primary btn-sm" onclick="openModal('newITO')">＋ Manual ITO Request</button>`
    )}

    <!-- Same-site notice -->
    <div class="card"
         style="background:hsl(155,55%,7%); border-color:hsl(155,55%,18%); margin-bottom:16px">
      <div class="flex ai-c gap-8">
        <span style="font-size:18px">⚡</span>
        <div style="font-weight:600; color:var(--sto)">Same-Site: Auto-Receive Enabled</div>
      </div>
      <div class="small dim mt-4">
        When Factory issues a "Move to Store", both GI and GR are auto-confirmed.
        No action needed on the Store side — stock appears instantly.
      </div>
    </div>

    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>ITO ID</th><th>Item</th><th>Qty</th><th>Trigger</th><th>Same-Site</th><th>Status</th></tr>
          </thead>
          <tbody>
            ${stoITOs.map(t => `
              <tr>
                <td><code>${t.id}</code></td>
                <td>${t.item}</td>
                <td>${t.qty} pcs</td>
                <td><span class="badge badge-dim">${t.trigger}</span></td>
                <td>${t.same_site
                  ? '<span class="badge badge-green">⚡ Same-Site</span>'
                  : '<span class="badge badge-dim">Cross-Site</span>'}</td>
                <td>${statusBadge(t.status)}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    </div>
  `;
}
