/* ─── HQ / Discrepancy Tickets ───────────────────────────────────────────────
   Review transit discrepancies from GRs.
   Depends on: helpers.js, mock-data.js, toast.js, router.js.
──────────────────────────────────────────────────────────────────────────── */

function renderHQDiscrepancy() {
  return `
    ${pageHeader(
      'Discrepancy Tickets',
      'Resolve logistics and receiving mismatches'
    )}
    
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Ticket ID</th><th>GR Ref</th><th>Item</th><th>Missing</th><th>Damaged</th><th>Reason</th><th>Evidence</th><th>Status</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${DISC_TICKETS.map(dt => `
              <tr>
                <td><code>${dt.id}</code></td>
                <td><code>${dt.gr}</code></td>
                <td>${dt.item}</td>
                <td><span style="color:var(--red); font-weight:600">${dt.missing}</span></td>
                <td><span style="color:var(--amber); font-weight:600">${dt.damaged}</span></td>
                <td class="small" style="max-width:200px">${dt.reason || '<span class="faint">—</span>'}</td>
                <td>${dt.evidence_url ? `<a href="${dt.evidence_url}" target="_blank" class="btn btn-sm btn-outline">View Photo</a>` : '<span class="faint">—</span>'}</td>
                <td>${statusBadge(dt.status)}</td>
                <td>
                  ${dt.status === 'OPEN' ? `
                    <button class="btn btn-primary btn-sm" onclick="resolveTicket('${dt.id}')">Resolve</button>
                  ` : '<span class="faint">—</span>'}
                </td>
              </tr>
            `).join('')}
            ${DISC_TICKETS.length === 0 ? `<tr><td colspan="9" class="faint" style="text-align:center">No open discrepancies.</td></tr>` : ''}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

function resolveTicket(id) {
  const dt = DISC_TICKETS.find(t => t.id === id);
  if (!dt) return;
  dt.status = 'RESOLVED';
  toast(`Ticket ${id} resolved. Supplier contacted for credit.`, 'success');
  renderPage(state.page);
}
