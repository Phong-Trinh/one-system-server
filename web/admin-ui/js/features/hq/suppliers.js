/* ─── HQ / Suppliers ─────────────────────────────────────────────────────────
   View catalog of external Suppliers.
   Depends on: helpers.js, mock-data.js, toast.js, router.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQSuppliers() {
  let suppliers = [];
  try {
    const res = await api.getSuppliers(state.currentUser.orgId);
    suppliers = res || [];
  } catch(e) {
    return `<div class="error">Failed to load suppliers: ${e.message}</div>`;
  }

  return `
    ${pageHeader(
      'Suppliers',
      'External vendors for procurement',
      `<button class="btn btn-primary btn-sm" onclick="alert('Add Supplier UI not implemented')">＋ Add Supplier</button>`
    )}
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Supplier ID</th><th>Name</th><th>Contact</th>
            </tr>
          </thead>
          <tbody>
            ${suppliers.map(s => `
              <tr>
                <td><code>${s.id}</code></td>
                <td style="font-weight: 500">${s.name}</td>
                <td class="dim">${s.contact_info || s.contact}</td>
              </tr>
            `).join('')}
            ${suppliers.length === 0 ? `<tr><td colspan="3" class="text-center dim py-4">No suppliers found.</td></tr>` : ''}
          </tbody>
        </table>
      </div>
    </div>
  `;
}
