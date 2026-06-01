/* ─── HQ / Financials ────────────────────────────────────────────────────────
   Production cost records and financial overview.
   Depends on: helpers.js (fmt, pageHeader), mock-data.js.
──────────────────────────────────────────────────────────────────────────── */

function renderHQFinancials() {
  return `
    ${pageHeader('Financials', 'Production cost records, asset depreciation')}

    <h3 style="margin-bottom:12px">Completed Production Cost Records</h3>
    <div class="card p-0" style="margin-bottom:16px">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>PO Ref</th><th>Item</th><th>Material Cost</th><th>Labor Cost</th>
              <th>Overhead</th><th>Total</th><th>Cost / Unit</th><th>Output</th>
            </tr>
          </thead>
          <tbody>
            ${COST_RECORDS.map(c => `
              <tr>
                <td><code>${c.po}</code></td>
                <td>${c.item}</td>
                <td>${fmt(c.material)}</td>
                <td>${fmt(c.labor)}</td>
                <td>${fmt(c.overhead)}</td>
                <td style="font-weight:700">${fmt(c.total)}</td>
                <td style="color:var(--accent); font-weight:700">${fmt(c.per_unit)}</td>
                <td>${c.output} pcs</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    </div>

    <div class="card" style="text-align:center; color:var(--text-faint); padding:32px">
      <div style="font-size:32px; margin-bottom:8px">📈</div>
      <div>Asset depreciation schedules and full financial reports coming in v2.</div>
    </div>
  `;
}
