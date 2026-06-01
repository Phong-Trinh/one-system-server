/* ─── HQ / Equipment Types ───────────────────────────────────────────────────
   View catalog of Equipment Types. DRAFT types can be activated.
   Depends on: helpers.js, mock-data.js, toast.js, router.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQEquipmentTypes() {
  let eqTypes = [];
  try {
    const res = await api.getEquipmentTypes();
    eqTypes = res || [];
  } catch(e) {
    return `<div class="error">Failed to load equipment types: ${e.message}</div>`;
  }

  return `
    ${pageHeader(
      'Equipment Types',
      'Catalog of CapEx asset types for PR submissions'
    )}
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Type ID</th><th>Name</th><th>Capacity Unit</th><th>Status</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${eqTypes.length === 0 ? '<tr><td colspan="5" class="text-center dim py-4">No equipment types found.</td></tr>' : ''}
            ${eqTypes.map(eq => `
              <tr>
                <td><code>${eq.id}</code></td>
                <td>${eq.name}</td>
                <td><span class="badge badge-dim">${eq.capacity_unit}</span></td>
                <td>${eq.status === 'ACTIVE' ? '<span class="badge badge-green">ACTIVE</span>' : '<span class="badge badge-amber">DRAFT</span>'}</td>
                <td>
                  ${eq.status === 'DRAFT'
                    ? `<button class="btn btn-primary btn-sm" onclick="activateEquipmentType('${eq.id}')">Activate</button>`
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
function activateEquipmentType(id) {
  alert('Activating equipment type is not fully implemented in API yet.');
}
