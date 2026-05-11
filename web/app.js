const API_BASE = 'http://localhost:8080/api/v1';

let globalItems = [];
let currentQuickAddSelect = null;
let currentQuickStationSelect = null;
let editingItemId = null; // ID của item đang được chỉnh sửa (form cơ bản)
let globalStationTypes = [];
let editingBOMID = null;
let editingSOPID = null;
let editingWizardItemID = null;
let globalNodes = [];
let activeNodeId = localStorage.getItem('activeNodeId') || null;


// Load Initial Data
async function loadInitialData() {
    await Promise.all([
        loadItems(),
        loadStationTypes(),
        loadMachines(),
        loadNodes()
    ]);
    loadProductionOrders(); // Initial load of active orders
}

let globalMachines = [];

async function loadMachines() {
    try {
        const url = activeNodeId ? `${API_BASE}/machines?node_id=${activeNodeId}` : `${API_BASE}/machines`;
        const res = await fetch(url);
        if (res.ok) {
            globalMachines = await res.json();
            renderMachinesTable();
        }
    } catch (err) {
        console.error("Failed to load machines", err);
    }
}

function renderMachinesTable() {
    const tbody = document.getElementById('machinesTableBody');
    if (!tbody) return;
    tbody.innerHTML = "";

    globalMachines.forEach(m => {
        const st = globalStationTypes.find(s => s.id === m.station_type_id);
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td style="font-weight: 600;">${m.id}</td>
            <td><span class="type-badge" style="background: #4b5563;">${st ? st.name : m.station_type_id}</span></td>
            <td>${m.max_slots}</td>
            <td><span class="status-pill status-${m.status.toLowerCase()}">${m.status}</span></td>
        `;
        tbody.appendChild(tr);
    });
}

async function loadStationTypes() {
    try {
        const res = await fetch(`${API_BASE}/station-types`);
        if (res.ok) {
            globalStationTypes = await res.json();
            updateWizStationSelects();
        }
    } catch (err) {
        console.error("Failed to load station types", err);
    }
}

async function loadNodes() {
    try {
        const res = await fetch(`${API_BASE}/nodes`);
        if (res.ok) {
            globalNodes = await res.json();

            // Cập nhật Selector Chi nhánh toàn cục
            const globalSelect = document.getElementById('globalNodeSelect');
            if (globalSelect) {
                globalSelect.innerHTML = '<option value="">-- Chọn Chi nhánh --</option>';
                globalNodes.forEach(node => {
                    const opt = document.createElement('option');
                    opt.value = node.id;
                    opt.textContent = `${node.name} (${node.type})`;
                    globalSelect.appendChild(opt);
                });

                if (activeNodeId) {
                    globalSelect.value = activeNodeId;
                } else if (globalNodes.length > 0) {
                    // Mặc định chọn node đầu tiên nếu chưa có session
                    activeNodeId = globalNodes[0].id;
                    globalSelect.value = activeNodeId;
                    localStorage.setItem('activeNodeId', activeNodeId);
                }
            }

            updatePOSelects();
            renderNodesList();
        }
    } catch (err) {
        console.error("Failed to load nodes", err);
    }
}

function switchNodeContext(nodeId) {
    if (!nodeId) return;
    activeNodeId = nodeId;
    localStorage.setItem('activeNodeId', nodeId);

    // Tìm OrgID tương ứng với Node này
    const node = globalNodes.find(n => n.id === nodeId);
    if (node) {
        activeOrgId = node.org_id;
        localStorage.setItem('activeOrgId', activeOrgId);
    }

    // Refresh relevant data for this node/org
    loadMachines();
    loadProductionOrders();
    loadItems(); // Lọc lại danh mục sản phẩm theo Org

    // Update local selects to match
    const poNodeSelect = document.getElementById('poNodeSelect');
    if (poNodeSelect) poNodeSelect.value = nodeId;

    const machineNodeSelect = document.getElementById('machineNodeId');
    if (machineNodeSelect) machineNodeSelect.value = nodeId;
}

function updatePOSelects() {
    const nodeSelect = document.getElementById('poNodeSelect');
    if (!nodeSelect) return;

    nodeSelect.innerHTML = '<option value="" disabled selected>Chọn bếp thực hiện...</option>';
    globalNodes.forEach(node => {
        const opt = document.createElement('option');
        opt.value = node.id;
        opt.textContent = `${node.name} (${node.type})`;
        nodeSelect.appendChild(opt);
    });

    const itemSelect = document.getElementById('poItemSelect');
    if (!itemSelect) return;
    itemSelect.innerHTML = '<option value="" disabled selected>Chọn món cần sản xuất...</option>';

    globalItems.filter(i => i.type !== 'RAW_MATERIAL').forEach(item => {
        const opt = document.createElement('option');
        opt.value = item.id;
        opt.textContent = `${item.name} (${item.type})`;
        itemSelect.appendChild(opt);
    });
}

// Load Items
async function loadItems() {
    try {
        const url = activeOrgId ? `${API_BASE}/items?org_id=${activeOrgId}` : `${API_BASE}/items`;
        const res = await fetch(url);
        if (res.ok) {
            globalItems = await res.json();
            updateAllItemSelects();
            updatePOSelects(); // Thêm dòng này để cập nhật dropdown tab Sản xuất


            // Cập nhật các select box trong Wizard
            const typeSelect = document.getElementById('wizItemType');
            const isProduct = typeSelect ? typeSelect.value === 'PRODUCT' : false;
            updateWizIngredientSelects(isProduct);

            renderItemsTable(); // Đổ dữ liệu vào bảng
        }
    } catch (err) {
        console.error("Failed to load items", err);
    }
}

function renderItemsTable(filter = "") {
    const tbody = document.getElementById('itemsTableBody');
    tbody.innerHTML = "";

    const filtered = globalItems.filter(item =>
        item.name.toLowerCase().includes(filter.toLowerCase()) ||
        (item.sku && item.sku.toLowerCase().includes(filter.toLowerCase()))
    );

    filtered.forEach(item => {
        const tr = document.createElement('tr');
        tr.innerHTML = `
        <td>${item.name}</td>
        <td>${item.sku || '-'}</td>
        <td><span class="badge">${item.type}</span></td>
        <td>${item.base_unit}</td>
        <td>
            <button class="btn-edit" onclick="editItem('${item.id}')" title="Sửa thông tin cơ bản">✏️</button>
            <button class="btn-primary" onclick="editBOMSOP('${item.id}')" style="font-size: 0.75rem; padding: 0.3rem 0.6rem;">⚙️ BOM/SOP</button>
            ${(item.type === 'PRODUCT' || item.type === 'SEMI_PRODUCT') ?
                `<button class="btn-accent" onclick="prepareProductionOrder('${item.id}')" style="font-size: 0.75rem; padding: 0.3rem 0.6rem;">🔥 Sản xuất</button>` : ''}
        </td>
    `;
        tbody.appendChild(tr);
    });
}

function prepareProductionOrder(itemId) {
    const item = globalItems.find(i => i.id === itemId);
    if (!item) return;

    // Chuyển sang tab Sản xuất
    document.querySelector('[data-target="production-section"]').click();

    // Tự động chọn item
    const itemSelect = document.getElementById('poItemSelect');
    itemSelect.value = itemId;
    document.getElementById('poUnitLabel').textContent = item.base_unit;

    // Kiểm tra BOM
    checkBOMForItem(itemId);
}

async function checkBOMForItem(itemId) {
    const info = document.getElementById('poBomInfo');
    info.style.display = 'none';
    try {
        const res = await fetch(`${API_BASE}/production/boms/by-item/${itemId}`);
        if (res.ok) {
            const data = await res.json();
            info.style.display = 'block';
            info.setAttribute('data-bom-id', data.bom.id);
        } else {
            alert("Món này chưa có công thức (BOM). Vui lòng tạo BOM trước!");
        }
    } catch (err) {
        console.error("Check BOM error", err);
    }
}

// Lắng nghe thay đổi trên select món ăn ở tab PO
document.getElementById('poItemSelect')?.addEventListener('change', (e) => {
    const item = globalItems.find(i => i.id === e.target.value);
    if (item) {
        document.getElementById('poUnitLabel').textContent = item.base_unit;
        checkBOMForItem(item.id);
    }
});

function editItem(id) {
    const item = globalItems.find(i => i.id === id);
    if (!item) return;

    editingItemId = id;
    document.getElementById('itemName').value = item.name;
    document.getElementById('itemSku').value = item.sku || "";
    document.getElementById('itemType').value = item.type;
    document.getElementById('itemBaseUnit').value = item.base_unit;

    const submitBtn = document.getElementById('itemSubmitBtn');
    submitBtn.textContent = "Cập nhật thay đổi";
    submitBtn.classList.replace('btn-primary', 'btn-accent');

    // Cuộn lên đầu trang
    window.scrollTo({ top: 0, behavior: 'smooth' });
}

// Xử lý tìm kiếm
document.getElementById('itemSearchInput').addEventListener('input', (e) => {
    renderItemsTable(e.target.value);
});

function updateAllItemSelects() {
    // Cập nhật select cho Thành phẩm (chỉ chọn Product / Semi-product)
    const outputSelect = document.getElementById('outputItemId');
    const currentOutputVal = outputSelect.value;
    outputSelect.innerHTML = '<option value="" disabled selected>Chọn thành phẩm...</option>';

    // Cập nhật select cho thành phần (nguyên liệu)
    const ingredientSelects = document.querySelectorAll('.item-id-input');

    globalItems.forEach(item => {
        // Cho select Thành phẩm
        if (item.type === 'PRODUCT' || item.type === 'SEMI_PRODUCT') {
            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            outputSelect.appendChild(opt);
        }
    });
    outputSelect.value = currentOutputVal || "";

    ingredientSelects.forEach(select => {
        const currentVal = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn nguyên liệu...</option>';
        globalItems.forEach(item => {
            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            select.appendChild(opt);
        });
        select.value = currentVal || "";
    });
}

// Navigation Logic
document.querySelectorAll('.nav-item').forEach(button => {
    button.addEventListener('click', () => {
        // Remove active class from all
        document.querySelectorAll('.nav-item').forEach(btn => btn.classList.remove('active'));
        document.querySelectorAll('.section-card').forEach(sec => sec.classList.remove('active'));

        // Add active class to clicked
        button.classList.add('active');
        const targetId = button.getAttribute('data-target');
        document.getElementById(targetId).classList.add('active');

        // Special logic per tab
        if (targetId === 'wizard-section') {
            resetWizard();
        }
        if (targetId === 'org-section') {
            renderNodesList();
        }
    });
});

function resetWizard() {
    editingWizardItemID = null;
    editingBOMID = null;
    editingSOPID = null;
    document.getElementById('wizItemName').value = "";
    document.getElementById('wizItemSku').value = "";
    document.getElementById('wizItemBaseUnit').value = "gram";
    document.getElementById('wizIngredientsList').innerHTML = "";
    document.getElementById('wizStepsList').innerHTML = "";
    addWizIngredientRow();
    addWizStepRow();
    document.getElementById('wizard-btn-finish').textContent = "✨ Hoàn tất & Lưu dữ liệu";
    goToStep(1);
}

async function editBOMSOP(itemId) {
    const item = globalItems.find(i => i.id === itemId);
    if (!item) return;

    resetWizard();
    editingWizardItemID = itemId;

    // Switch to Wizard tab
    document.querySelectorAll('.nav-item').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.section-card').forEach(sec => sec.classList.remove('active'));
    document.querySelector('[data-target="wizard-section"]').classList.add('active');
    document.getElementById('wizard-section').classList.add('active');

    // Step 1: Info
    document.getElementById('wizItemName').value = item.name;
    document.getElementById('wizItemSku').value = item.sku || "";
    document.getElementById('wizItemType').value = item.type;
    document.getElementById('wizItemBaseUnit').value = item.base_unit;

    // Load BOM
    try {
        const bomRes = await fetch(`${API_BASE}/production/boms/by-item/${itemId}`);
        if (bomRes.ok) {
            const bomData = await bomRes.json();
            editingBOMID = bomData.bom.id;

            // Clear default row
            document.getElementById('wizIngredientsList').innerHTML = "";
            bomData.lines.forEach(line => {
                const template = document.getElementById('ingredientTemplate');
                const clone = template.content.cloneNode(true);
                const row = clone.querySelector('.list-row');
                row.setAttribute('data-line-id', line.id);

                row.querySelector('.wiz-item-id-input').value = line.item_id;
                row.querySelector('.wiz-qty-input').value = line.qty;

                row.querySelector('.btn-remove').addEventListener('click', () => { row.remove(); updateSOPPreview(); });
                row.querySelector('.wiz-item-id-input').addEventListener('change', updateSOPPreview);
                row.querySelector('.wiz-qty-input').addEventListener('input', updateSOPPreview);

                document.getElementById('wizIngredientsList').appendChild(clone);
            });
            updateWizIngredientSelects(item.type === 'PRODUCT');

            // Load SOP
            const sopRes = await fetch(`${API_BASE}/production/sops/by-bom/${editingBOMID}`);
            if (sopRes.ok) {
                const sopData = await sopRes.json();
                editingSOPID = sopData.sop.id;

                document.getElementById('wizStepsList').innerHTML = "";
                sopData.steps.forEach(step => {
                    const template = document.getElementById('stepTemplate');
                    const clone = template.content.cloneNode(true);
                    const row = clone.querySelector('.list-row');
                    row.setAttribute('data-step-id', step.id);

                    row.querySelector('.wiz-desc-input').value = step.description;
                    row.querySelector('.wiz-station-input').value = step.station_type_id || "";
                    row.querySelector('.wiz-duration-input').value = step.duration;

                    // Add dependencies badges
                    const depContainer = row.querySelector('.depends-container');
                    step.depends_on.forEach(depId => {
                        const badge = document.createElement('span');
                        badge.className = 'dep-badge';
                        badge.setAttribute('data-dep-id', depId);
                        badge.innerHTML = `Bước ? <button type="button">×</button>`;
                        badge.querySelector('button').addEventListener('click', () => badge.remove());
                        depContainer.appendChild(badge);
                    });

                    // Add ingredient badges
                    const ingContainer = row.querySelector('.step-ingredients-container');
                    step.ingredient_bom_line_ids.forEach(lineId => {
                        const badge = document.createElement('span');
                        badge.className = 'dep-badge';
                        badge.style.background = '#065f46';
                        badge.style.border = '1px solid #10b981';
                        badge.setAttribute('data-line-id', lineId);
                        badge.innerHTML = `NL ? <button type="button">×</button>`;
                        badge.querySelector('button').addEventListener('click', () => badge.remove());
                        ingContainer.appendChild(badge);
                    });

                    document.getElementById('wizStepsList').appendChild(clone);
                });
                updateWizStationSelects();
                updateSOPPreview(); // This will refresh the badge texts with proper numbers/names
            }
        }
    } catch (err) {
        console.error("Error loading BOM/SOP", err);
    }

    document.getElementById('wizard-btn-finish').textContent = "💾 Cập nhật Dữ liệu";
    goToStep(1);
}

// WIZARD LOGIC
function nextWizardStep(step) {
    // Validate Step 1 before moving to Step 2
    if (step === 2) {
        const name = document.getElementById('wizItemName').value.trim();
        if (!name) return alert("Vui lòng nhập tên món!");

        // Kiểm tra trùng tên
        const isDuplicate = globalItems.some(i => i.name.toLowerCase() === name.toLowerCase());
        if (isDuplicate) {
            const confirmDup = confirm(`Cảnh báo: Nguyên liệu/Món ăn mang tên "${name}" đã tồn tại trong hệ thống! Bạn có chắc chắn muốn tạo thêm một bản ghi trùng tên không?`);
            if (!confirmDup) return;
        }

        // Cập nhật cảnh báo và danh sách thả xuống ở Step 2
        const type = document.getElementById('wizItemType').value;
        const warning = document.getElementById('bomWarning');
        warning.style.display = type === 'PRODUCT' ? 'block' : 'none';

        // Populate current ingredients with filtered options
        updateWizIngredientSelects(type === 'PRODUCT');
    }

    // Step 3: Populate Station Types
    if (step === 3) {
        updateWizStationSelects();
    }

    // Step 4: Show Summary
    if (step === 4) {
        const name = document.getElementById('wizItemName').value;
        const type = document.getElementById('wizItemType').value;

        const bomRows = document.getElementById('wizIngredientsList').querySelectorAll('.list-row').length;
        const sopRows = document.getElementById('wizStepsList').querySelectorAll('.list-row').length;

        document.getElementById('wizardSummary').innerHTML = `
        <p><strong>Tên món:</strong> ${name}</p>
        <p><strong>Loại:</strong> ${type}</p>
        <p><strong>Thành phần (BOM):</strong> ${bomRows} nguyên liệu</p>
        <p><strong>Công đoạn (SOP):</strong> ${sopRows} bước</p>
    `;
    }

    // Đổi view
    document.querySelectorAll('.wizard-step').forEach(s => s.style.display = 'none');
    document.getElementById(`wizard-step-${step}`).style.display = 'block';

    document.querySelectorAll('.step-indicator').forEach((ind, index) => {
        if (index < step) {
            ind.classList.add('active');
        } else {
            ind.classList.remove('active');
        }
    });
}

function prevWizardStep(step) {
    document.querySelectorAll('.wizard-step').forEach(s => s.style.display = 'none');
    document.getElementById(`wizard-step-${step}`).style.display = 'block';

    document.querySelectorAll('.step-indicator').forEach((ind, index) => {
        if (index < step) {
            ind.classList.add('active');
        } else {
            ind.classList.remove('active');
        }
    });
}

function addWizIngredientRow() {
    const template = document.getElementById('ingredientTemplate');
    const clone = template.content.cloneNode(true);
    const row = clone.querySelector('.list-row');

    // Generate Temp ID for BOM Line
    const lineId = 'L_' + Math.random().toString(36).substr(2, 9);
    row.setAttribute('data-line-id', lineId);

    row.querySelector('.btn-remove').addEventListener('click', () => {
        row.remove();
        updateSOPPreview();
    });

    const select = clone.querySelector('.wiz-item-id-input');
    const type = document.getElementById('wizItemType').value;

    // Nút tạo nhanh
    clone.querySelector('.btn-quick-add').addEventListener('click', () => {
        currentQuickAddSelect = select;
        document.getElementById('quickCreateModal').classList.add('active');
    });

    select.addEventListener('change', updateSOPPreview);
    row.querySelector('.wiz-qty-input').addEventListener('input', updateSOPPreview);

    document.getElementById('wizIngredientsList').appendChild(clone);
    updateWizIngredientSelects(type === 'PRODUCT');
}

function updateWizIngredientSelects(onlyRawAndSemi) {
    const selects = document.querySelectorAll('.wiz-item-id-input');
    const safeItems = Array.isArray(globalItems) ? globalItems : [];

    selects.forEach(select => {
        const currentVal = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn nguyên liệu...</option>';
        safeItems.forEach(item => {
            if (onlyRawAndSemi && item.type === 'PRODUCT') return; // Bỏ qua Product nếu món đang tạo là Product

            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            select.appendChild(opt);
        });
        select.value = currentVal || "";
    });
}

// Initialize Mermaid
mermaid.initialize({ startOnLoad: false, theme: 'dark' });

function addWizStepRow() {
    const template = document.getElementById('stepTemplate');
    const clone = template.content.cloneNode(true);
    const row = clone.querySelector('.list-row');

    // Generate Temp ID
    const stepId = 'S_' + Math.random().toString(36).substr(2, 9);
    row.setAttribute('data-step-id', stepId);

    // Remove button
    clone.querySelector('.btn-remove').addEventListener('click', () => {
        row.remove();
        updateSOPPreview();
    });

    // Add Dependency logic
    const selectDep = clone.querySelector('.wiz-depends-select');
    const containerDep = clone.querySelector('.depends-container');
    const selectIng = clone.querySelector('.wiz-step-ingredients-select');
    const containerIng = clone.querySelector('.step-ingredients-container');
    const selectStation = clone.querySelector('.wiz-station-input');

    // Nút tạo nhanh Station
    clone.querySelector('.wiz-station-quick-add').addEventListener('click', () => {
        currentQuickStationSelect = selectStation;
        document.getElementById('quickStationModal').classList.add('active');
    });

    selectDep.addEventListener('change', (e) => {
        const depId = e.target.value;
        const depText = e.target.options[e.target.selectedIndex].text;
        if (depId) {
            const badge = document.createElement('div');
            badge.className = 'dep-badge';
            badge.setAttribute('data-dep-id', depId);
            badge.innerHTML = `${depText} <button type="button">×</button>`;

            badge.querySelector('button').addEventListener('click', () => {
                badge.remove();
                updateSOPPreview();
            });

            containerDep.appendChild(badge);
            e.target.value = ""; // Reset
            updateSOPPreview();
        }
    });

    selectIng.addEventListener('change', (e) => {
        const lineId = e.target.value;
        const ingText = e.target.options[e.target.selectedIndex].text;
        if (lineId) {
            const badge = document.createElement('div');
            badge.className = 'dep-badge'; // Reuse styling
            badge.style.background = 'rgba(16, 185, 129, 0.2)'; // Greenish for ingredients
            badge.style.border = '1px solid #10b981';
            badge.setAttribute('data-line-id', lineId);
            badge.innerHTML = `${ingText} <button type="button">×</button>`;

            badge.querySelector('button').addEventListener('click', () => {
                badge.remove();
            });

            containerIng.appendChild(badge);
            e.target.value = ""; // Reset
        }
    });

    // Listen to changes to update graph
    clone.querySelector('.wiz-desc-input').addEventListener('input', updateSOPPreview);
    clone.querySelector('.wiz-station-input').addEventListener('change', updateSOPPreview);

    document.getElementById('wizStepsList').appendChild(clone);
    updateWizStationSelects();
    updateSOPPreview();
}

function updateSOPPreview() {
    const stepRows = document.querySelectorAll('#wizStepsList .sop-step-row');
    const ingRows = document.querySelectorAll('#wizIngredientsList .ingredient-row');
    const stepsData = [];

    // Get ingredients from Step 2
    const currentIngredients = Array.from(ingRows).map(row => {
        const itemId = row.querySelector('.wiz-item-id-input').value;
        const lineId = row.getAttribute('data-line-id');
        const item = globalItems.find(i => i.id === itemId);
        return { lineId, name: item ? item.name : 'Unknown' };
    }).filter(i => i.lineId && i.name !== 'Unknown');

    // 1. Gather current state
    stepRows.forEach((row, index) => {
        const num = index + 1;
        row.querySelector('.step-number').textContent = num;

        const id = row.getAttribute('data-step-id');
        const desc = row.querySelector('.wiz-desc-input').value.trim() || `Bước ${num}`;
        const stationSelect = row.querySelector('.wiz-station-input');
        const station = stationSelect.options[stationSelect.selectedIndex]?.text || '';
        const deps = Array.from(row.querySelectorAll('.dep-badge[data-dep-id]')).map(b => b.getAttribute('data-dep-id'));
        const usedIngs = Array.from(row.querySelectorAll('.dep-badge[data-line-id]')).map(b => b.getAttribute('data-line-id'));

        stepsData.push({ id, num, desc, station, deps, usedIngs, row });
    });

    // 2. Refresh Dropdowns (Dependencies & Ingredients)
    stepsData.forEach(step => {
        // Refresh Dependencies
        const selectDep = step.row.querySelector('.wiz-depends-select');
        selectDep.innerHTML = '<option value="">+ Thêm bước cần chờ...</option>';
        stepsData.forEach(other => {
            if (other.id !== step.id && !step.deps.includes(other.id)) {
                const opt = document.createElement('option');
                opt.value = other.id;
                opt.textContent = `B${other.num}. ${other.desc}`;
                selectDep.appendChild(opt);
            }
        });

        // Refresh Ingredients
        const selectIng = step.row.querySelector('.wiz-step-ingredients-select');
        selectIng.innerHTML = '<option value="">+ Thêm nguyên liệu từ BOM...</option>';
        currentIngredients.forEach(ing => {
            if (!step.usedIngs.includes(ing.lineId)) {
                const opt = document.createElement('option');
                opt.value = ing.lineId;
                opt.textContent = ing.name;
                selectIng.appendChild(opt);
            }
        });
    });

    // 3. Render Mermaid
    const preview = document.getElementById('mermaidPreview');
    if (stepsData.length === 0) {
        preview.innerHTML = '<div class="empty-state">Thêm bước để xem sơ đồ...</div>';
        return;
    }

    let graphDef = 'graph TD;\n';

    stepsData.forEach(step => {
        const safeDesc = step.desc.replace(/["\\]/g, '');
        const safeStation = step.station && step.station !== 'Chọn trạm (Station)' ? `\\n[${step.station.replace(/["\\]/g, '')}]` : '';
        const nodeLabel = `"${step.num}. ${safeDesc}${safeStation}"`;

        graphDef += `  ${step.id}[${nodeLabel}]\n`;

        // Remove invalid badges and add valid edges
        step.deps.forEach(depId => {
            const depStep = stepsData.find(s => s.id === depId);
            const badge = step.row.querySelector(`.dep-badge[data-dep-id="${depId}"]`);
            if (depStep) {
                graphDef += `  ${depId} --> ${step.id}\n`;
                if (badge) badge.childNodes[0].textContent = `B${depStep.num}. ${depStep.desc} `;
            } else {
                if (badge) badge.remove();
            }
        });

        step.usedIngs.forEach(lineId => {
            const ing = currentIngredients.find(i => i.lineId === lineId);
            const badge = step.row.querySelector(`.dep-badge[data-line-id="${lineId}"]`);
            if (ing && badge) {
                badge.childNodes[0].textContent = `${ing.name} `;
            } else if (badge) {
                badge.remove();
            }
        });
    });

    try {
        mermaid.render('sopGraphRendered', graphDef).then(({ svg }) => {
            preview.innerHTML = svg;
        }).catch(err => console.warn(err));
    } catch (err) {
        console.warn("Mermaid render error", err);
    }
}

function updateWizStationSelects() {
    const selects = document.querySelectorAll('.wiz-station-input');
    const machineSelect = document.getElementById('machineStationType');
    const safeStationTypes = Array.isArray(globalStationTypes) ? globalStationTypes : [];

    selects.forEach(select => {
        const currentVal = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn trạm...</option>';
        safeStationTypes.forEach(st => {
            const opt = document.createElement('option');
            opt.value = st.id;
            opt.textContent = st.name;
            select.appendChild(opt);
        });
        select.value = currentVal || "";
    });

    if (machineSelect) {
        const currentVal = machineSelect.value;
        machineSelect.innerHTML = '<option value="" disabled selected>Chọn loại trạm...</option>';
        safeStationTypes.forEach(st => {
            const opt = document.createElement('option');
            opt.value = st.id;
            opt.textContent = st.name;
            machineSelect.appendChild(opt);
        });
        machineSelect.value = currentVal || "";
    }
}

document.getElementById('wizAddIngredientBtn').addEventListener('click', addWizIngredientRow);
document.getElementById('wizAddStepBtn').addEventListener('click', addWizStepRow);

async function submitWizard() {
    const btn = document.getElementById('wizard-btn-finish');
    if (!btn) return;
    btn.disabled = true;
    btn.textContent = "Đang xử lý...";

    try {
        // 1. Item
        let itemId = editingWizardItemID;
        const name = document.getElementById('wizItemName').value;
        const sku = document.getElementById('wizItemSku').value;
        const type = document.getElementById('wizItemType').value;
        const baseUnit = document.getElementById('wizItemBaseUnit').value;

        if (!itemId) {
            const itemRes = await fetch(`${API_BASE}/items`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    org_id: activeOrgId, // <-- Thêm OrgID
                    name, sku, type, base_unit: baseUnit
                })
            });
            const itemData = await itemRes.json();
            if (!itemRes.ok) throw new Error("Lỗi tạo Item: " + (itemData.error || itemData));
            itemId = itemData.id;
        }

        // 2. BOM
        const bomLines = [];
        document.getElementById('wizIngredientsList').querySelectorAll('.ingredient-row').forEach(row => {
            const ingItemId = row.querySelector('.wiz-item-id-input').value;
            const qty = parseFloat(row.querySelector('.wiz-qty-input').value);
            if (ingItemId && qty) {
                bomLines.push({ item_id: ingItemId, qty: qty });
            }
        });

        let bomId = editingBOMID;
        if (bomLines.length > 0) {
            const url = bomId ? `${API_BASE}/production/boms/${bomId}` : `${API_BASE}/production/boms`;
            const method = bomId ? 'PUT' : 'POST';
            const body = bomId ? { lines: bomLines } : { output_item_id: itemId, lines: bomLines };

            const bomRes = await fetch(url, {
                method: method,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });

            if (bomRes.ok) {
                if (!bomId) {
                    const bomData = await bomRes.json();
                    bomId = bomData.id;
                }
            } else {
                const bomData = await bomRes.json();
                throw new Error("Lỗi lưu BOM: " + (bomData.error || JSON.stringify(bomData)));
            }
        }

        // 3. SOP
        if (bomId) {
            const steps = [];
            document.getElementById('wizStepsList').querySelectorAll('.sop-step-row').forEach(row => {
                const id = row.getAttribute('data-step-id');
                const stationId = row.querySelector('.wiz-station-input').value;
                const duration = parseInt(row.querySelector('.wiz-duration-input').value);
                const desc = row.querySelector('.wiz-desc-input').value;
                const dependsOn = Array.from(row.querySelectorAll('.dep-badge[data-dep-id]')).map(b => b.getAttribute('data-dep-id'));
                const ingredients = Array.from(row.querySelectorAll('.dep-badge[data-line-id]')).map(b => b.getAttribute('data-line-id'));

                if (duration) {
                    steps.push({
                        id: id,
                        depends_on: dependsOn,
                        station_type_id: stationId || "",
                        ingredient_bom_line_ids: ingredients,
                        duration: duration,
                        description: desc
                    });
                }
            });

            if (steps.length > 0) {
                const sopId = editingSOPID;
                const url = sopId ? `${API_BASE}/production/sops/${sopId}` : `${API_BASE}/production/sops`;
                const method = sopId ? 'PUT' : 'POST';
                const body = sopId ? { steps } : { bom_id: bomId, steps };

                const sopRes = await fetch(url, {
                    method: method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                if (!sopRes.ok) {
                    const sopData = await sopRes.json();
                    throw new Error("Lỗi lưu SOP: " + (sopData.error || JSON.stringify(sopData)));
                }
            }
        }

        alert("✨ Đã lưu quy trình sản xuất thành công!");
        resetWizard();
        loadItems();
    } catch (err) {
        alert("Lỗi: " + err.message);
    } finally {
        btn.disabled = false;
        btn.textContent = "✨ Hoàn tất & Lưu dữ liệu";
    }
}

// ---------------------------------------------------------
// EVENT LISTENERS
// ---------------------------------------------------------

// Item Form (Simple Add)
document.getElementById('itemForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('itemName').value;
    const sku = document.getElementById('itemSku').value;
    const type = document.getElementById('itemType').value;
    const baseUnit = document.getElementById('itemBaseUnit').value;

    const url = editingItemId ? `${API_BASE}/items/${editingItemId}` : `${API_BASE}/items`;
    const method = editingItemId ? 'PUT' : 'POST';

    try {
        const res = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                org_id: activeOrgId,
                name, sku, type, base_unit: baseUnit
            })
        });
        const data = await res.json();
        if (res.ok) {
            alert(editingItemId ? "✅ Đã cập nhật thành công!" : "✅ Đã tạo mới thành công!");
            document.getElementById('itemForm').reset();
            editingItemId = null;
            const submitBtn = document.getElementById('itemSubmitBtn');
            submitBtn.textContent = "Tạo mới";
            submitBtn.classList.replace('btn-accent', 'btn-primary');
            loadItems();
        } else {
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

// Quick Add Item Modal
// Quick Add Item Modal
document.getElementById('quickCreateForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('quickItemName').value;
    const type = document.getElementById('quickItemType').value;
    const unit = "cái"; // Default unit for quick add

    try {
        const res = await fetch(`${API_BASE}/items`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ org_id: activeOrgId, name, type, base_unit: unit })
        });
        const data = await res.json();
        if (res.ok) {
            await loadItems();
            if (currentQuickAddSelect) {
                currentQuickAddSelect.value = data.id;
                updateSOPPreview();
            }
            document.getElementById('quickCreateModal').classList.remove('active');
            document.getElementById('quickCreateForm').reset();
        } else {
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

document.getElementById('cancelQuickCreate')?.addEventListener('click', () => {
    document.getElementById('quickCreateModal').classList.remove('active');
});

// Quick Add Station Modal
document.getElementById('quickStationForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('quickStationId').value;
    const name = document.getElementById('quickStationName').value;
    const unit = document.getElementById('quickStationUnit').value;

    try {
        const res = await fetch(`${API_BASE}/station-types`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, name, capacity_unit: unit })
        });
        const data = await res.json();
        if (res.ok) {
            await loadStationTypes();
            if (currentQuickStationSelect) {
                currentQuickStationSelect.value = data.id;
                updateSOPPreview();
            }
            document.getElementById('quickStationModal').classList.remove('active');
            document.getElementById('quickStationForm').reset();
        } else {
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

document.getElementById('cancelQuickStation')?.addEventListener('click', () => {
    document.getElementById('quickStationModal').classList.remove('active');
});

// Machine Registration
document.getElementById('machineForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('machineId').value;
    const stationTypeId = document.getElementById('machineStationType').value;
    const nodeId = document.getElementById('machineNodeId').value;
    const maxSlots = parseInt(document.getElementById('machineCapacity').value);

    try {
        const res = await fetch(`${API_BASE}/machines`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id, station_type_id: stationTypeId, node_id: nodeId, max_slots: maxSlots })
        });
        if (res.ok) {
            alert("✅ Đã đăng ký máy thành công!");
            document.getElementById('machineForm').reset();
            loadMachines();
        } else {
            const data = await res.json();
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

// Production Order
document.getElementById('poForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const itemId = document.getElementById('poItemSelect').value;
    const nodeId = document.getElementById('poNodeSelect').value;
    const qty = parseFloat(document.getElementById('poTargetQty').value);

    const bomId = document.getElementById('poBomInfo').getAttribute('data-bom-id');

    if (!itemId || !nodeId || !qty || !bomId) {
        return alert("Vui lòng nhập đầy đủ thông tin Lệnh sản xuất!");
    }

    try {
        const res = await fetch(`${API_BASE}/production/orders`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                bom_id: bomId,
                node_id: nodeId,
                target_qty: qty
            })
        });
        if (res.ok) {
            alert("✨ Đã gửi lệnh sản xuất thành công!");
            document.getElementById('poForm').reset();
            loadProductionOrders();
        } else {
            const data = await res.json();
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

// Helper: Show Result (Simple Alert wrapper or UI update)
function showResult(elementId, data, isError = false) {
    const el = document.getElementById(elementId);
    if (!el) {
        if (isError) console.error(data);
        return;
    }
    el.innerHTML = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    el.className = 'result-box ' + (isError ? 'error' : 'success');
    el.style.display = 'block';
}

async function loadProductionOrders() {
    try {
        const url = activeNodeId ? `${API_BASE}/production/orders?node_id=${activeNodeId}` : `${API_BASE}/production/orders`;
        const res = await fetch(url);
        if (res.ok) {
            const orders = await res.json();
            renderOrdersList(orders);
        }
    } catch (err) {
        console.error("Failed to load orders", err);
    }
}

function renderOrdersList(orders) {
    const list = document.getElementById('activePOList');

    if (!list) return;
    list.innerHTML = "";

    if (orders.length === 0) {
        list.innerHTML = '<div class="empty-state">Chưa có lệnh sản xuất nào.</div>';
        return;
    }

    orders.sort((a, b) => new Date(b.created_at) - new Date(a.created_at)).forEach(order => {
        const item = globalItems.find(i => i.id === order.item_id);
        const node = globalNodes.find(n => n.id === order.node_id);
        const card = document.createElement('div');
        card.className = 'order-card';
        card.innerHTML = `
            <div class="order-header">
                <span class="order-id">#${order.id.slice(-6)}</span>
                <span class="status-pill status-${order.status.toLowerCase()}">${order.status}</span>
            </div>
            <div class="order-body">
                <h3>${item ? item.name : order.item_id}</h3>
                <p>Số lượng: <strong>${order.planned_qty} ${item ? item.base_unit : ''}</strong></p>
                <p>Bếp: <strong>${node ? node.name : order.node_id}</strong></p>
            </div>
            <div class="order-footer">
                <small>Bắt đầu: ${new Date(order.created_at).toLocaleString('vi-VN')}</small>
            </div>
        `;
        list.appendChild(card);
    });
}

async function setupSimulatedOrg() {
    const name = document.getElementById('orgNameInput').value.trim();
    if (!name) return alert("Vui lòng nhập tên chuỗi!");

    const statusEl = document.getElementById('orgSetupStatus');
    statusEl.innerHTML = "⏳ Đang khởi tạo chuỗi...";

    try {
        // 1. Create Org
        const orgRes = await fetch(`${API_BASE}/orgs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: name })
        });
        const orgData = await orgRes.json();
        if (!orgRes.ok) throw new Error("Lỗi tạo Org: " + (orgData.error || JSON.stringify(orgData)));
        const orgId = orgData.id;

        statusEl.innerHTML += `<br>✅ Đã tạo Org: ${orgId}`;

        // 2. Create Nodes
        const nodesToCreate = [
            { name: "Trụ sở chính (HQ)", type: "HQ", address: "Quận 1, TP.HCM" },
            { name: "Bếp trung tâm (Factory)", type: "FACTORY", address: "Khu Công Nghiệp Bình Chiểu" },
            { name: "Cửa hàng #1 (Store 1)", type: "STORE", address: "Quận 7, TP.HCM" },
            { name: "Cửa hàng #2 (Store 2)", type: "STORE", address: "Quận Gò Vấp, TP.HCM" }
        ];

        for (const n of nodesToCreate) {
            const nodeRes = await fetch(`${API_BASE}/nodes`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    org_id: orgId,
                    name: n.name,
                    type: n.type,
                    address: n.address
                })
            });
            const nodeData = await nodeRes.json();
            if (nodeRes.ok) {
                statusEl.innerHTML += `<br>✅ Đã tạo ${n.type}: ${nodeData.id}`;
            } else {
                statusEl.innerHTML += `<br>❌ Lỗi tạo ${n.type}: ${JSON.stringify(nodeData)}`;
            }
        }

        statusEl.innerHTML += `<br>✨ **Hoàn tất thiết lập hệ thống!**`;
        await loadNodes(); // Refresh nodes list globally
        renderNodesList(); // Refresh local list
    } catch (err) {
        statusEl.innerHTML += `<br>❌ Lỗi: ${err.message}`;
    }
}

function renderNodesList() {
    const container = document.getElementById('nodesList');
    if (!container) return;

    if (globalNodes.length === 0) {
        container.innerHTML = '<div class="empty-state">Chưa có chi nhánh nào được tạo.</div>';
        return;
    }

    container.innerHTML = "";
    globalNodes.forEach(node => {
        const div = document.createElement('div');
        div.className = "list-row";
        div.style = "padding: 0.75rem; margin-bottom: 0.5rem; border: 1px solid rgba(255,255,255,0.1); border-radius: 8px;";
        div.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center;">
                <div>
                    <div style="font-weight: 600; color: #fff;">${node.name}</div>
                    <div style="font-size: 0.75rem; color: #9ca3af;">📍 ${node.address}</div>
                </div>
                <span class="type-badge" style="background: ${node.type === 'HQ' ? '#3b82f6' : (node.type === 'FACTORY' ? '#f59e0b' : '#10b981')}">
                    ${node.type}
                </span>
            </div>
        `;
        container.appendChild(div);
    });
}

// Window Exports
window.nextWizardStep = nextWizardStep;
window.prevWizardStep = prevWizardStep;
window.submitWizard = submitWizard;
window.addWizIngredientRow = addWizIngredientRow;
window.addWizStepRow = addWizStepRow;
window.editItem = editItem;
window.editBOMSOP = editBOMSOP;
window.prepareProductionOrder = prepareProductionOrder;
window.setupSimulatedOrg = setupSimulatedOrg;
window.switchNodeContext = switchNodeContext;

// Final Initializations
loadInitialData();
