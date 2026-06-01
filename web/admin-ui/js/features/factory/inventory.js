/* ─── Factory / Inventory ────────────────────────────────────────────────────
   NodeStock view for the Factory node with ROP indicators.
   Depends on: helpers.js, mock-data.js.
──────────────────────────────────────────────────────────────────────────── */

function renderFacInventory() {
  return `
    ${pageHeader('Factory Inventory', 'NodeStock — live stock at Central Kitchen node')}
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Item</th><th>SKU</th><th>Qty on Hand</th><th>ROP</th><th>Unit</th><th>Stock Level</th></tr>
          </thead>
          <tbody>
            ${NODE_STOCK.FACTORY.map(s => {
              const clr = ropColor(s.qty, s.rop);
              const pct = ropPct(s.qty, s.rop);
              const sku = ITEMS.find(i => i.id === s.item_id)?.sku || '—';
              return `
                <tr>
                  <td style="font-weight:500">${s.name}</td>
                  <td><code>${sku}</code></td>
                  <td style="font-weight:700; color:${s.qty <= s.rop ? 'var(--amber)' : 'var(--text)'}">
                    ${s.qty}
                  </td>
                  <td class="dim">${s.rop}</td>
                  <td class="dim">${s.unit}</td>
                  <td style="min-width:140px">
                    <div class="rop-bar-wrap">
                      <div class="rop-bar-label">
                        <span>${s.qty <= s.rop ? '⚠ Below ROP' : ''}</span>
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
  `;
}
