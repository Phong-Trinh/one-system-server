/* ─── HQ / Equipment Types ───────────────────────────────────────────────────
   View catalog of Equipment Types. DRAFT types can be activated.
   Depends on: helpers.js, mock-data.js, toast.js, router.js.
──────────────────────────────────────────────────────────────────────────── */

let _hqEqTypes = [];

async function renderHQEquipmentTypes() {
  try {
    const res = await api.getEquipmentTypes();
    _hqEqTypes = res || [];
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
            ${_hqEqTypes.length === 0 ? '<tr><td colspan="5" class="text-center dim py-4">No equipment types found.</td></tr>' : ''}
            ${_hqEqTypes.map(eq => `
              <tr>
                <td><code>${eq.id}</code></td>
                <td>${eq.name}</td>
                <td><span class="badge badge-dim">${eq.capacity_unit}</span></td>
                <td>${eq.status === 'ACTIVE' ? '<span class="badge badge-green">ACTIVE</span>' : '<span class="badge badge-amber">DRAFT</span>'}</td>
                <td>
                  <div style="display:flex; gap:8px;">
                    <button class="btn btn-ghost btn-sm" onclick="editEquipmentType('${eq.id}')">✎ Edit</button>
                    ${eq.status === 'DRAFT'
                      ? `<button class="btn btn-primary btn-sm" onclick="activateEquipmentType('${eq.id}')">Activate</button>`
                      : ''}
                  </div>
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
function editEquipmentType(id) {
  const eq = _hqEqTypes.find(e => e.id === id);
  if (!eq) return;

  const modalHtml = `
    <div class="modal-header">
      <h3>Edit Equipment Type — <code>${eq.id}</code></h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Name</label>
        <input type="text" id="eq-edit-name" value="${eq.name}">
      </div>
      <div class="field">
        <label>Capacity Unit</label>
        <input type="text" id="eq-edit-unit" value="${eq.capacity_unit}">
      </div>
      <div class="flex gap-12 mt-4">
        <button class="btn btn-primary" onclick="submitEditEquipmentType('${eq.id}')">Save Changes</button>
        <button class="btn btn-ghost" onclick="closeModal()">Cancel</button>
      </div>
    </div>
  `;
  openModal(modalHtml, { maxWidth: '400px' });
}

async function submitEditEquipmentType(id) {
  const newName = document.getElementById('eq-edit-name')?.value?.trim();
  const newUnit = document.getElementById('eq-edit-unit')?.value?.trim();

  if (!newName || !newUnit) {
    toast('Name and Capacity Unit are required.', 'error');
    return;
  }

  try {
    await api.updateEquipmentType(id, { name: newName, capacity_unit: newUnit });
    toast('Equipment type updated successfully.', 'success');
    closeModal();
    renderPage(state.page);
  } catch (err) {
    toast('Failed to update equipment type: ' + err.message, 'error');
  }
}

async function activateEquipmentType(id) {
  if (!confirm('Are you sure you want to activate this equipment type?')) return;
  try {
    await api.updateEquipmentType(id, { status: 'ACTIVE' });
    toast('Equipment type activated successfully.', 'success');
    renderPage(state.page);
  } catch (err) {
    toast('Failed to activate equipment type: ' + err.message, 'error');
  }
}
