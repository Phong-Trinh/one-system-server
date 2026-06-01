/* ─── Store / Inventory ──────────────────────────────────────────────────────
   NodeStock view for the Store with ROP bars and auto-replenishment notice.
   Depends on: helpers.js, mock-data.js.
──────────────────────────────────────────────────────────────────────────── */

function renderStoInventory() {
  return `
    ${pageHeader('Store Inventory', 'NodeStock — live stock at Store node')}

    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Item</th><th>Qty on Hand</th><th>ROP</th><th>Unit</th><th>Stock Level</th></tr>
          </thead>
          <tbody>
            ${NODE_STOCK.STORE.map(s => {
              const clr    = ropColor(s.qty, s.rop);
              const pct    = ropPct(s.qty, s.rop);
              const label  = s.qty <= s.rop
                ? '⚠ Below ROP'
                : s.qty <= s.rop * 1.2
                ? '↓ Near ROP'
                : '✓ OK';
              const labelColor = clr === 'red'
                ? 'var(--red)'
                : clr === 'amber'
                ? 'var(--amber)'
                : 'var(--green)';

              return `
                <tr>
                  <td style="font-weight:500">${s.name}</td>
                  <td style="font-weight:700; color:${s.qty <= s.rop ? 'var(--amber)' : 'var(--text)'}">
                    ${s.qty}
                  </td>
                  <td class="dim">${s.rop}</td>
                  <td class="dim">${s.unit}</td>
                  <td style="min-width:160px">
                    <div class="rop-bar-wrap">
                      <div class="rop-bar-label">
                        <span style="color:${labelColor}">${label}</span>
                        <span>${pct}%</span>
                      </div>
                      <div class="progress-bar">
                        <div class="progress-fill ${clr}" style="width:${pct}%"></div>
                      </div>
                    </div>
                  </td>
                </tr>
              `;
            }).join('')}
          </tbody>
        </table>
      </div>
    </div>

    <div class="card mt-16"
         style="margin-top:16px; background:hsl(155,55%,7%); border-color:hsl(155,55%,18%)">
      <div class="flex ai-c gap-8">
        <span style="font-size:18px">⚡</span>
        <div>
          <div style="font-weight:600; color:var(--sto)">Auto-Replenishment Active</div>
          <div class="small dim mt-4">
            When stock hits ROP, the system auto-creates an InternalTransferOrder to the Factory
            (same-site). Stock updates instantly once Factory confirms the 1-click transfer.
          </div>
        </div>
      </div>
    </div>
  `;
}
