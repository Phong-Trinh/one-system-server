# OneSystem KDS — Thiết Kế Lại Toàn Diện
**Phiên bản:** 1.0  
**Ngày:** 2026-06-22  
**Tác giả:** F&B Operations & System Architecture Review  
**Trạng thái:** DRAFT — Chờ phê duyệt để bắt đầu implementation

---

## Mục lục

1. [Tóm tắt điều hành](#1-tóm-tắt-điều-hành)
2. [Chẩn đoán: KDS hiện tại đang làm gì?](#2-chẩn-đoán-kds-hiện-tại-đang-làm-gì)
3. [Khoảng cách cốt lõi](#3-khoảng-cách-cốt-lõi)
4. [Phân tích chiến lược: Giao task theo Máy hay theo Món?](#4-phân-tích-chiến-lược-giao-task-theo-máy-hay-theo-món)
5. [Mô hình đề xuất: Hybrid Station-Based](#5-mô-hình-đề-xuất-hybrid-station-based)
6. [Kiến trúc dữ liệu mới](#6-kiến-trúc-dữ-liệu-mới)
7. [Flow vận hành hoàn chỉnh](#7-flow-vận-hành-hoàn-chỉnh)
8. [Thách thức kỹ thuật cần giải quyết](#8-thách-thức-kỹ-thuật-cần-giải-quyết)
9. [Lộ trình thực hiện](#9-lộ-trình-thực-hiện)

---

## 1. Tóm tắt điều hành

KDS hiện tại của OneSystem về bản chất là một **machine monitor** — nó hiển thị trạng thái máy móc cho người quản lý, không phải hướng dẫn thao tác cho nhân viên bếp. Đây là khoảng cách nghiêm trọng giữa sản phẩm hiện có và vision ban đầu: *"hướng dẫn nhân viên bếp chi tiết từng bước, để họ như robot — chỉ nghe lệnh hệ thống và làm theo."*

Tài liệu này phân tích nguyên nhân, đánh giá hai mô hình giao task khác nhau, và đề xuất kiến trúc mới dựa trên mô hình **Hybrid Station-Based** — cách McDonald's, Jollibee, và Gong Cha thực sự vận hành bếp của họ.

> [!IMPORTANT]
> **Kết luận cốt lõi:** KDS cần được thiết kế lại từ *"machine-centric monitoring"* sang *"staff-centric command system"*. Đơn vị hiển thị không còn là Batch (theo Máy) mà là StaffTask (theo Người).

---

## 2. Chẩn đoán: KDS hiện tại đang làm gì?

Nhìn vào `kds.js` và `production-orders.js`, KDS hiện tại hiển thị:

- Máy nào đang IDLE / BUSY / UNDER_MAINTENANCE
- Batch nào đang được assign vào máy nào
- Thanh tiến trình timer đếm ngược (elapsed vs duration)
- Hai nút hành động: **▶ Start Batch** và **✔ Complete**

### Vấn đề căn bản

> [!CAUTION]
> **Đây là "Manager View", không phải "Staff View."**
> Một nhân viên bếp nhìn vào màn hình này sẽ không biết phải làm gì tiếp theo. Hệ thống đang nói với họ **cái gì đang xảy ra**, thay vì **họ phải làm gì ngay bây giờ**.

Nói thẳng hơn: KDS hiện tại đang giải quyết sai vấn đề. Nó tracking máy, thay vì guiding người.

---

## 3. Khoảng cách cốt lõi

| Chiều | KDS Hiện Tại | KDS Đúng Nghĩa |
|---|---|---|
| **Đơn vị hiển thị** | Batch (theo Máy) | StaffTask (theo Người) |
| **Góc nhìn** | Machine-centric | Staff-centric |
| **Người dùng chính** | Manager quan sát | Nhân viên bếp thao tác |
| **Câu hỏi trả lời** | "Máy nào đang bận?" | "Tôi phải làm gì ngay bây giờ?" |
| **Hành động** | Start / Complete batch | Chỉ 1 nút: **DONE ✓** |
| **Task scheduling** | Không có | Phân công theo từng người, từng step |
| **Idle time** | Bị bỏ qua | Chèn task phụ có kiểm soát |
| **Accountability** | Không ai chịu trách nhiệm | Mỗi step gắn với 1 người cụ thể |

### Vấn đề thực tế trong bếp

Trong một ca bếp có N đơn và M nhân viên, câu hỏi vận hành thực sự là:

```
Ai làm gì  →  trên cái gì  →  lúc nào  →  trong bao lâu  →  xong thì làm gì tiếp?
```

**Ví dụ cụ thể (từ note vận hành):**
- 5 đơn đồng thời, 2 nhân viên bếp, dùng chung máy
- Nhân viên A đang chiên hành → set timer → có 3 phút idle → hệ thống phải **ngay lập tức** giao task phụ
- Nhân viên B đang pha nước → liên tục, không có idle → **không được interrupt**
- Khi timer hành kết thúc → hệ thống phải gọi đúng nhân viên A quay lại, không phải B

Đây là bài toán **Real-time Task Scheduling** — không phải machine monitoring.

---

## 4. Phân tích chiến lược: Giao task theo Máy hay theo Món?

Đây là câu hỏi thiết kế cốt lõi nhất của KDS. Không có câu trả lời đúng tuyệt đối — nó phụ thuộc vào mô hình vận hành cụ thể.

---

### 4.1 — Model A: Station-Based (Giao theo Máy)

> *"Bạn đứng fryer. Bạn làm TẤT CẢ mọi thứ đi qua fryer."*

Mỗi nhân viên được assign cứng vào 1 station. Họ nhận task từ nhiều đơn hàng, nhưng chỉ là phần task thuộc station của họ.

```
Nhân viên A — Fryer Station
├── [Đơn #01] Chiên khoai 5kg  → 8 phút
├── [Đơn #03] Chiên gà 3kg    → 6 phút  ← batch cùng nồi nếu còn slot
└── [Đơn #05] Chiên hành 1kg  → 3 phút

Nhân viên B — Beverage Station
├── [Đơn #01] Pha trà sữa
├── [Đơn #02] Pha nước ép
└── [Đơn #04] Pha cà phê
```

**Ưu điểm ✅**

| # | Điểm mạnh | Lý do |
|---|---|---|
| 1 | **Throughput cao nhất** | Bin-packing tối ưu: cùng lúc nấu nhiều món trong 1 nồi |
| 2 | **Nhân viên thành thạo nhanh** | Chỉ học 1 station → muscle memory hình thành nhanh |
| 3 | **Utilization máy tối đa** | Không có máy chờ người, không có người chờ máy |
| 4 | **Dễ đo hiệu suất máy** | OEE (Overall Equipment Effectiveness) đơn giản |
| 5 | **Scale tốt theo đơn** | Thêm đơn = tăng tải đồng đều trên từng station |
| 6 | **"Dummy user" dễ train** | Nhân viên chỉ cần biết 1 skill set duy nhất |

**Nhược điểm ❌**

| # | Điểm yếu | Tác động |
|---|---|---|
| 1 | **Accountability bị pha loãng** | Khi món lỗi, ai chịu? Nhân viên fryer hay grill? |
| 2 | **Phụ thuộc handoff giữa station** | Khoai chiên xong phải "trao" cho station khác → dễ mất sync |
| 3 | **Visibility đơn hàng kém** | Không ai thấy tổng thể đơn #01 đang ở bước nào |
| 4 | **Khó detect cross-station bottleneck** | Nếu Beverage chậm, Fryer cũng đình trệ nhưng khó nhận ra |

---

### 4.2 — Model B: Order-Based (Giao theo Món)

> *"Bạn làm đơn #01 từ đầu đến cuối."*

Mỗi nhân viên được assign 1 hoặc vài đơn hàng cụ thể. Họ chịu trách nhiệm toàn bộ các bước SOP của đơn đó, di chuyển giữa các máy.

```
Nhân viên A — Đơn #01 & #03
├── Bước 1: Đến Fryer → Chiên khoai  (8 phút)
├── Bước 2: (idle) → Sang Grill → Nướng bánh mì
├── Bước 3: Quay lại Fryer → Lấy khoai ra
└── Bước 4: Đến Beverage → Pha trà sữa

Nhân viên B — Đơn #02 & #04 & #05
├── Bước 1: Đến Fryer → Chiên gà  ←  CONFLICT! A đang chiếm Fryer!
...
```

**Ưu điểm ✅**

| # | Điểm mạnh | Lý do |
|---|---|---|
| 1 | **Accountability rõ ràng** | Món lỗi = truy ngay người được assign |
| 2 | **SOP enforce tự nhiên** | Task theo đúng thứ tự SOP, không cần "dịch" |
| 3 | **End-to-end visibility cho nhân viên** | Họ biết đơn của mình đang ở bước nào |
| 4 | **Phù hợp fine dining** | Nhân viên hiểu toàn bộ món → chất lượng nhất quán |

**Nhược điểm ❌**

| # | Điểm yếu | Tác động |
|---|---|---|
| 1 | **Machine conflict cực kỳ nghiêm trọng** | 2 người cùng cần fryer → 1 người chờ → throughput sụp đổ |
| 2 | **Throughput thấp hơn đáng kể** | Không batch được → nấu riêng lẻ từng đơn |
| 3 | **Utilization máy thấp** | Máy idle khi nhân viên đang ở station khác |
| 4 | **Nhân viên phải đa năng** | Training lâu, tỉ lệ lỗi cao hơn |
| 5 | **Không scale được** | Thêm đơn = machine conflict tăng theo cấp số nhân |

---

### 4.3 — So sánh trực tiếp: Kịch bản 5 đơn, 2 nhân viên

**Model A — Station-Based:**
```
Timeline (phút):  0    2    4    6    8   10   12
                  |----|----|----|----|----|----|

Fryer (A):        [====== Khoai#01+#03 ======][=Gà#05=]
Grill (A):                  [===== Bánh#01+#02+#04 =====]
Beverage (B):     [Trà#01][Nước ép#02][CF#04][Trà#03]

→ Tất cả 5 đơn hoàn thành trong ~12 phút ✅
```

**Model B — Order-Based:**
```
Timeline (phút):  0    2    4    6    8   10   12   14   16   18   20
                  |----|----|----|----|----|----|----|----|----|----|

A làm #01:        [Fryer: Khoai 8'              ][Grill 4'][Bev 2']
B làm #02:        [   WAIT Fryer...             ][Fryer 6'][Grill 3'][Bev 2']
A làm #03:                                                 [WAIT.][Fryer...]
...

→ 5 đơn cần ~20+ phút ❌ (chậm hơn 67%)
```

> [!CAUTION]
> Với mô hình Factory/Central Kitchen, Order-Based **gần như không khả thi** vì mọi đơn đều share thiết bị. Machine conflict là điểm nghẽn không thể tránh.

---

## 5. Mô hình đề xuất: Dependency-Aware Hybrid

> *"Station-Based execution + Dependency-Graph scheduling + Order-Based accountability"*

Mô hình này kế thừa nền tảng Station-Based (như McDonald's, Jollibee vận hành) nhưng **mở rộng lên một tầng trừu tượng cao hơn** để phục vụ các nhà hàng có SOP phức tạp: pizza truyền thống, premium burger, casual dining. Đây là V2 — baseline thiết kế của OneSystem KDS, không phải progressive extension.

### Nguyên tắc cốt lõi

| Layer | Model | Lý do |
|---|---|---|
| **Execution** (nhân viên làm gì) | Station-Based | Throughput, bin-packing, scale |
| **Tracking** (ai chịu trách nhiệm) | Order-Based | Accountability, truy vết lỗi |
| **Quality gate cuối** | Assembly Step | 1 người gom toàn bộ đơn, kiểm tra trước khi xuất |

### Cơ chế hoạt động

```
[Đơn hàng vào]
      │
      ▼
[Hệ thống phân rã SOP → tasks theo equipment_type]
      │
      ├──→ FRYER tasks   →  Fryer Station Queue
      ├──→ GRILL tasks   →  Grill Station Queue
      └──→ BEVERAGE tasks →  Beverage Station Queue
                │
                ▼
      [Nhân viên xử lý queue của station mình]
      [Bin-pack tối đa, FIFO hoặc EDF priority]
      [MỖI task gắn reference_order_id để tracking]
                │
                ▼
      [Tất cả station báo xong cho đơn #01]
                │
                ▼
      [ASSEMBLY STEP: 1 người gom đơn, QC, đánh dấu DONE]
```

### Giải quyết Accountability trong Station-Based

Thay vì assign người vào đơn, **gắn order reference vào mỗi StaffTask và log đầy đủ**:

```
StaffTask {
  staff_id:            "NV_NGUYEN_A"   ← người thực hiện
  sop_step_id:         "STEP_FRYER_03" ← biết làm gì, dùng máy gì, mất bao lâu
  reference_order_id:  "PO#01"         ← tracking accountability
  machine_id:          "M_FRYER_01"    ← máy cụ thể
}
```

Khi món lỗi → query: *"Ai đã làm Fryer step của đơn #01?"* → ra ngay tên người, thời gian, máy cụ thể.

### Kiến trúc tổng thể

```
                    ┌──────────────────────────────┐
                    │      PRODUCTION ORDER        │
                    │    (đơn hàng, có BOM + SOP)  │
                    └──────────┬───────────────────┘
                               │ phân rã theo equipment_type
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
     ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
     │ FRYER QUEUE │  │ GRILL QUEUE │  │  BEV QUEUE  │
     │  Station A  │  │  Station A  │  │  Station B  │
     └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
            │                │                │
            ▼                ▼                ▼
     [StaffTask          [StaffTask       [StaffTask
      ref: PO#01]         ref: PO#01]      ref: PO#01]

            └────────────────┼────────────────┘
                             ▼
                    ┌─────────────────────┐
                    │  ASSEMBLY / RUNNER  │
                    │  Gom đơn #01 lại   │
                    │  QC + đánh dấu DONE │
                    └─────────────────────┘
```

---

## 6. Kiến trúc dữ liệu mới

Mô hình hiện tại cần bổ sung 2 model mới và mở rộng 1 model hiện có.

---

### 6.1 — [MỚI] StaffTask
Atomic work unit giao cho 1 nhân viên cụ thể tại 1 thời điểm cụ thể.

```
StaffTask {
  id                  string       Unique identifier
  po_id               string       FK → ProductionOrder
  sop_step_id         string       FK → SOPStep (biết làm gì, cần gì, mất bao lâu)
  assigned_to         string       FK → Staff
  machine_id          string       FK → Machine (cụ thể, không phải chỉ type)
  
  status              TaskStatus   PENDING | ACTIVE | WAITING | DONE | FAILED
  priority            int          Thứ tự trong queue của station (thấp = ưu tiên hơn)
  
  scheduled_start     time         Kế hoạch bắt đầu (do scheduler tính)
  scheduled_end       time         Kế hoạch kết thúc
  started_at          time         Thực tế bắt đầu (do nhân viên bấm DONE bước trước)
  completed_at        time         Thực tế hoàn thành

  is_interruptible    bool         false = không được chèn task khác vào giữa bước này
  parent_task_id      string?      FK → StaffTask (nếu là fill-in task trong idle time)
  
  note                string?      Ghi chú từ nhân viên khi done/fail
}

TaskStatus:
  PENDING   → Đã scheduled, chưa đến lượt
  ACTIVE    → Nhân viên đang thực hiện (bấm "Bắt đầu")
  WAITING   → Bước idle (máy đang tự chạy, timer đếm ngược)
  DONE      → Hoàn thành, nhân viên đã confirm
  FAILED    → Báo lỗi (hỏng nguyên liệu, máy hỏng, v.v.)
```

---

### 6.2 — [MỚI] StaffShift
Ca làm việc — để scheduler biết ai đang available vào thời điểm nào.

```
StaffShift {
  id            string       Unique identifier
  staff_id      string       FK → Staff
  node_id       string       FK → Node
  shift_start   time         Giờ bắt đầu ca
  shift_end     time         Giờ kết thúc ca (dự kiến)
  actual_end    time?        Giờ kết thúc thực tế (nếu kết thúc sớm)
  status        ShiftStatus  SCHEDULED | ACTIVE | ENDED
  station_id    string?      FK → EquipmentType (station được assign trong ca này)
}
```

---

### 6.3 — [SỬA] SOPStep — V2 Spec

> [!NOTE]
> `can_fill_idle: bool` đã bị **xóa hoàn toàn**. Thay bằng `attention_level` enum để model chính xác hơn mức độ chú ý cần thiết trong idle time. `step_type` cũng đã được hợp nhất vào `attention_level` + `depends_on_steps` để tránh dư thừa.

```
SOPStep {
  // --- Core fields (không đổi) ---
  sop_id              string
  seq_no              int
  equipment_type_id   string
  duration            int          Tổng thời gian của bước (giây)
  description         string       Hướng dẫn chi tiết cho nhân viên

  // --- Idle modeling (V2 — thay thế can_fill_idle) ---
  is_idle_step        bool         true = máy tự chạy sau khi setup, nhân viên có thể rời
  active_time         int?         Thời gian thao tác trực tiếp (giây), nếu is_idle_step
  idle_time           int?         = duration - active_time (tính tự động)
  attention_level     AttentionLevel  BẮT BUỘC khi is_idle_step = true
  check_interval_sec  int?         Chỉ dùng khi attention_level = PERIODIC_CHECK
  max_distance_meters float?       Chỉ dùng khi attention_level = NEARBY_IDLE
  requires_attention_at int?       Buffer giây trước khi hết idle, nhân viên phải quay lại

  // --- Dependency graph (V2 core) ---
  depends_on_steps    string[]     IDs của các SOPStep phải DONE trước bước này
                                   [] = chỉ phụ thuộc vào seq_no (default behavior)
  same_item_required  bool         true = không bin-pack với item/đơn khác
                                   false = scheduler tự do batch (default)

  // --- Equipment constraint (V2 core) ---
  equipment_profile   object?      Chi tiết hơn equipment_type_id
    temperature_celsius float?     Nhiệt độ yêu cầu (để match machine profile)
    mode              string?      "CONVECTION" | "DECK" | "WOOD_FIRED" | "INDUCTION"
}

AttentionLevel:
  FULL_IDLE       → Máy tự chạy hoàn toàn. Nhân viên tự do đến station khác.
  NEARBY_IDLE     → Cần ở gần máy (<= max_distance_meters), làm việc tay được.
  PERIODIC_CHECK  → Check định kỳ (mỗi check_interval_sec giây), giữa đó tự do.
  ACTIVE_WAIT     → Không rời máy — đứng chờ để phản ứng ngay.
```

**Mapping từ V1 sang V2 (cho QSR client hiện có):**

| V1 (cũ) | V2 equivalent | Ghi chú |
|---|---|---|
| `can_fill_idle: true` | `attention_level: FULL_IDLE` | 1-1 |
| `can_fill_idle: false` (nhân viên không rời) | `attention_level: ACTIVE_WAIT` | 1-1 |
| `can_fill_idle: false` (nhân viên gần đó) | `attention_level: NEARBY_IDLE` | V2 diễn đạt đúng hơn |
| *(không có trong V1)* | `attention_level: PERIODIC_CHECK` | V2 mới |

---

### 6.4 — [MỚI] OrderItem — Item-Level Tracking

Trong V2, mỗi món trong đơn là một entity độc lập với SOP và timeline riêng. Đây là nền tảng cho **partial assembly** và **multi-timeline scheduling**.

```
OrderItem {
  id              string       Unique identifier
  po_id           string       FK → ProductionOrder
  product_id      string       FK → Product (menu item)
  sop_id          string       FK → SOP (quy trình làm món này)
  quantity        float        Số lượng

  status          ItemStatus   PENDING | IN_PROGRESS | READY | ASSEMBLED
  ready_at        time?        Khi nào item này hoàn thành (thực tế)
  estimated_ready time?        Scheduler dự đoán
}

ItemStatus:
  PENDING      → Chưa bắt đầu
  IN_PROGRESS  → Đang trong quá trình làm (ít nhất 1 SOPStep đang ACTIVE)
  READY        → Tất cả SOPStep của item này đã DONE, chờ Assembly
  ASSEMBLED    → Đã được Runner gom vào đơn hoàn chỉnh
```

**AssemblyTask V2 — Partial Notification:**

```
AssemblyTask {
  po_id               string
  trigger_condition   "ALL_ITEMS_READY" | "PARTIAL_NOTIFY"
                      ALL_ITEMS_READY → Assembly chỉ trigger khi tất cả items READY
                      PARTIAL_NOTIFY  → Runner được notify mỗi khi 1 item READY
  ready_items         OrderItem[]     Cập nhật realtime khi item READY
  pending_items       OrderItem[]     Còn lại, kèm estimated_ready time
}
```

---

### 6.5 — Sơ đồ quan hệ mới (Layer bổ sung)

```
Layer 3.5 — Staff Scheduling + Item Tracking (V2)
┌─────────────────────────────────────────────┐
│  StaffShift                                  │
│  (Ca làm việc, station được assign)          │
├─────────────────────────────────────────────┤
│  OrderItem                                   │
│  (Mỗi món trong đơn, có SOP + timeline riêng)│
│  → FK: ProductionOrder, SOP                  │
├─────────────────────────────────────────────┤
│  StaffTask                                   │
│  (Atomic task: ai, làm gì, khi nào, máy nào)│
│  → FK: ProductionOrder, OrderItem, SOPStep,  │
│        Staff, Machine, StaffTask (parent)    │
└─────────────────────────────────────────────┘
```

---

## 7. Flow vận hành hoàn chỉnh

### 7.1 — Từ Production Order đến StaffTask (V2 Scheduler)

```
[ProductionOrder → IN_PROGRESS]
        │
        ▼
[Scheduling Engine V2 kích hoạt]
  │
  ├── PHASE 1: Build Dependency DAG
  │   ├── Với mỗi OrderItem trong PO:
  │   │   └── Parse SOPStep[] → build directed acyclic graph
  │   │       (edge: step A → step B nếu B.depends_on_steps chứa A.id)
  │   └── Topological sort → danh sách steps theo thứ tự an toàn
  │
  ├── PHASE 2: Schedule từng Step (theo topo order)
  │   ├── Check: tất cả depends_on_steps đã DONE chưa?
  │   │   → Nếu chưa: step này BLOCKED, bỏ qua đến khi dependency clear
  │   ├── Tìm Machine: equipment_type_id + equipment_profile compatible
  │   │   → Nếu same_item_required = true: không merge batch với OrderItem khác
  │   ├── Tìm Staff: StaffShift.station = equipment_type, đang available
  │   ├── Tính scheduled_start: max(dependency_done_at, machine_free_at, staff_free_at)
  │   └── Tạo StaffTask với status = PENDING
  │
  ├── PHASE 3: Idle Time Fill-In (attention-aware)
  │   ├── Với mỗi step có is_idle_step = true:
  │   │   ├── Tạo 2 tasks: "SET_UP" + "RETRIEVE" (cả 2 là MANUAL)
  │   │   └── Dựa vào attention_level:
  │   │       FULL_IDLE      → Tìm fill-in task ở bất kỳ station nào
  │   │       NEARBY_IDLE    → Chỉ fill-in task không cần di chuyển xa (max_distance)
  │   │       PERIODIC_CHECK → Fill-in task ngắn hơn check_interval_sec
  │   │                        Task phải có is_interruptible = true
  │   │       ACTIVE_WAIT    → Không fill-in. Log idle time cho analytics
  │   └── Constraint: fill_task.estimated_duration ≤ idle_time - requires_attention_at - buffer
  │
  ├── PHASE 4: Assembly Scheduling
  │   ├── Tạo AssemblyTask với trigger_condition từ PO config
  │   ├── PARTIAL_NOTIFY: subscribe vào mỗi OrderItem.status → READY event
  │   └── ALL_ITEMS_READY: trigger khi tất cả OrderItem.status = READY
  │
  └── Output: Ordered StaffTask list per staff + AssemblyTask
```

### 7.2 — Màn hình Staff KDS (nhân viên bếp)

Mỗi nhân viên chỉ nhìn thấy **1 task tại 1 thời điểm**. Không có lựa chọn. Chỉ có 1 nút.

**State 1: Task thao tác trực tiếp**
```
┌──────────────────────────────────────────┐
│  🔴  NGAY BÂY GIỜ                        │
│  ─────────────────────────────────────── │
│  Đặt 5 kg khoai vào M_FRYER_01          │
│                                          │
│  📋 Hướng dẫn:                           │
│  1. Rải đều khoai trong giỏ chiên        │
│  2. Set nhiệt độ 180°C                   │
│  3. Nhấn Start trên máy                  │
│                                          │
│  🏷 Đơn: PO#01 · Station: Fryer          │
│  ⏱ Máy sẽ chạy: 8 phút                  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │          ✓  ĐÃ ĐẶT VÀO            │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

**State 2: Bước idle — được chèn task phụ**
```
┌──────────────────────────────────────────┐
│  🟡  TRONG KHI CHỜ                       │
│  ⏳ Fryer đang chạy  •  7:23 còn lại     │
│  ████████████████░░░░░░░  78%             │
│  ─────────────────────────────────────── │
│  Pha trà sữa cho đơn #02                 │
│                                          │
│  📋 Hướng dẫn:                           │
│  1. Lấy ly 500ml                         │
│  2. Đổ 300ml trà đen đã ủ               │
│  3. Thêm 50ml syrup đường               │
│  4. Thêm 150ml sữa tươi                  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │              ✓  XONG               │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

**State 3: Alert — nhân viên phải quay lại**
```
┌──────────────────────────────────────────┐
│  🔴  QUAY LẠI FRYER NGAY!               │
│  ⚠️  Còn 30 giây                         │
│  ─────────────────────────────────────── │
│  Lấy khoai ra khỏi M_FRYER_01           │
│                                          │
│  📋 Hướng dẫn:                           │
│  1. Nhấc giỏ chiên ra                   │
│  2. Để ráo 30 giây                       │
│  3. Đổ vào khay đựng có nhãn PO#01      │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │          ✓  ĐÃ LẤY RA              │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

### 7.3 — Màn hình Manager KDS (ca trưởng)

Màn hình này **không có nút thao tác** — chỉ để theo dõi tổng quan.

```
┌───────────────────────────────────────────────────────────────┐
│  MANAGER VIEW — Ca sáng  •  Live                              │
├───────────────────────────────────────────────────────────────┤
│  NGUYỄN VĂN A — Fryer Station                                 │
│  🟢 ACTIVE: Đặt khoai vào M_FRYER_01  (PO#01)  ▓▓▓▓░  3:20  │
├───────────────────────────────────────────────────────────────┤
│  TRẦN THỊ B — Beverage Station                                │
│  🟢 ACTIVE: Pha trà sữa  (PO#02)               ▓▓░░░  5:01  │
├───────────────────────────────────────────────────────────────┤
│  Máy móc                                                      │
│  FRYER_01  ████████████░░░  BUSY  •  8:00 còn 5:40           │
│  GRILL_01  ░░░░░░░░░░░░░░░  IDLE                             │
│  BEV_01    ████████████████  BUSY                             │
├───────────────────────────────────────────────────────────────┤
│  Đơn hàng đang xử lý                                          │
│  PO#01  ██████░░░░  Fryer✅  Grill⏳  Assembly⏸   60%         │
│  PO#02  ████████░░  Bev✅   Assembly⏸             80%         │
└───────────────────────────────────────────────────────────────┘
```

### 7.4 — Phân tách 2 loại màn hình

| | Staff KDS | Manager KDS |
|---|---|---|
| **Người dùng** | Nhân viên bếp | Ca trưởng / Manager |
| **Đơn vị** | 1 task tại 1 thời điểm | Tổng quan tất cả staff + machine |
| **Hành động** | Chỉ: DONE ✓ | Không có nút thao tác |
| **Visibility** | Chỉ task của mình | Toàn bộ ca |
| **Thiết bị** | Tablet tại station | Màn hình lớn hoặc PC quản lý |

---

## 8. Thách thức kỹ thuật cần giải quyết

### 8.1 — Fill-in task không được gián đoạn task chính

**Rule:**
```
fill_task.estimated_duration + safety_buffer ≤ parent_idle_task.requires_attention_at
```

- Nếu không có task phụ nào đủ ngắn → nhân viên được "standby" (nghỉ) trong idle time
- Không bao giờ để nhân viên tự quyết định làm gì

### 8.2 — Machine conflict khi nhiều PO cùng cần 1 máy

- Scheduler phải tính toán **trước khi** assign, không để conflict xảy ra real-time
- Priority mặc định: **FIFO** (theo thời gian tạo PO)
- Priority nâng cao: **EDF** (Earliest Deadline First) khi có deadline khác nhau
- Nếu tất cả máy cùng loại đều bận: task vào queue PENDING, hiển thị "Chờ máy" cho Manager

### 8.3 — Nhân viên vắng mặt đột xuất giữa ca

```
StaffShift.status → ENDED sớm
      │
      ▼
Tất cả StaffTask của nhân viên đó (status = PENDING) → UNASSIGNED
      │
      ▼
Scheduler re-assign cho nhân viên khác đang trong ca tại cùng station
      │
      ▼ Nếu không có nhân viên thay thế:
Notify Manager → Manual reassign
```

### 8.4 — Machine breakdown giữa chừng

```
Machine.status → UNDER_MAINTENANCE
      │
      ▼
StaffTask đang ACTIVE/WAITING trên máy đó → FAILED
ProductionBatch liên quan → FAILED
      │
      ▼
Scheduler tìm Machine backup cùng equipment_type còn IDLE
      │
      ▼ Nếu có backup:
Tạo StaffTask mới → re-route
      │
      ▼ Nếu không có backup:
PO trở về PENDING, Notify Manager
```

### 8.5 — SOP của 1 đơn có nhiều steps cần cùng 1 machine type

Ví dụ: Bước 1 → Fryer (blanching), Bước 4 → Fryer (fry final). Hai bước này không thể overlap trên cùng 1 máy nếu máy đang bận.

- Scheduler phải model dependency chain: Step 4 chỉ được schedule khi machine đã free sau Step 1 + các batch khác đang dùng máy đó.

---

## 9. Lộ trình thực hiện

### Phase 1 — Manual Scheduling + Staff KDS MVP *(Nên làm trước)*

**Mục tiêu:** Nhân viên bếp có màn hình riêng, nhìn vào biết phải làm gì.

**Việc cần làm:**
- [ ] Thêm `StaffTask` model vào backend
- [ ] Thêm `StaffShift` model vào backend
- [ ] Mở rộng `SOPStep` với các fields mới
- [ ] Màn hình ca trưởng: assign thủ công Staff vào từng SOPStep của PO
- [ ] **Staff KDS Screen**: hiển thị task hiện tại của nhân viên, nút DONE duy nhất
- [ ] Khi nhân viên bấm DONE → task chuyển sang task kế tiếp của họ tự động

**Kết quả:** Nhân viên không cần biết gì ngoài việc nhìn màn hình và bấm DONE.

---

### Phase 2 — Auto Scheduling Engine

**Mục tiêu:** Hệ thống tự phân công, không cần ca trưởng assign thủ công.

**Việc cần làm:**
- [ ] Viết Scheduling Engine: nhận PO, trả về danh sách StaffTask
- [ ] Tích hợp với Machine availability (IDLE/BUSY timeline)
- [ ] Tích hợp với StaffShift (ai đang available, đang ở station nào)
- [ ] Bin-packing logic cho batch tối ưu
- [ ] Priority queue: FIFO hoặc EDF có thể config

---

### Phase 3 — Idle Time Insertion

**Mục tiêu:** Tự động lấp đầy thời gian chờ máy bằng task phụ.

**Việc cần làm:**
- [ ] Identify idle slots từ `is_idle_step = true` steps
- [ ] Tìm và chèn fill-in tasks phù hợp vào idle slots
- [ ] Timer alert: push notification/visual alert khi `requires_attention_at` giây còn lại
- [ ] Đảm bảo fill-in task không overrun qua deadline quay lại

---

### Phase 4 — Manager Dashboard & Analytics

**Mục tiêu:** Ca trưởng có công cụ theo dõi và hệ thống có dữ liệu để cải thiện.

**Việc cần làm:**
- [ ] Real-time Manager KDS: tổng quan staff + machine + order progress
- [ ] OEE per machine: Availability × Performance × Quality
- [ ] Staff performance: tasks done on time / total tasks per shift
- [ ] Bottleneck detection: station nào thường xuyên là điểm nghẽn
- [ ] Historical SOP timing: so sánh `SOPStep.duration` (estimate) vs actual → gợi ý điều chỉnh

---

## Tóm tắt quyết định kiến trúc (V2 Baseline)

| Quyết định | Lựa chọn | Lý do |
|---|---|---|
| Task assignment model | **Dependency-Aware Hybrid** | Station-based throughput + dependency graph cho SOP phức tạp |
| Scheduling core | **Constraint-based scheduler (DAG)** | Topological sort → đảm bảo đúng thứ tự cho mọi loại SOP |
| Đơn vị hiển thị KDS | **StaffTask (per person)** | Nhân viên nhìn thấy việc của mình, không phải của máy |
| Idle time modeling | **AttentionLevel enum (4 mức)** | Granular hơn boolean — phân biệt được FULL_IDLE vs PERIODIC_CHECK |
| Item tracking | **OrderItem (item-level)** | Hỗ trợ partial assembly, multi-timeline trong 1 đơn |
| Assembly trigger | **Configurable: ALL_ITEMS hoặc PARTIAL_NOTIFY** | Linh hoạt cho cả QSR (all) lẫn casual dining (partial) |
| Batch constraint | **equipment_profile + same_item_required** | Không batch những gì không tương thích (nhiệt độ, item riêng) |
| Accountability | **Order reference + item reference trên mọi task** | Truy vết lỗi đến cấp item, không chỉ cấp đơn |

> [!IMPORTANT]
> **Sự thay đổi tư duy cốt lõi:**
> - **From:** *"Batch nào đang ở máy nào"* → System monitors machines
> - **To:** *"Nhân viên A phải làm gì trong 30 giây tới"* → System commands humans
>
> Đây là sự khác biệt giữa một ERP simulation và một Real Operational Tool.

---

*Tài liệu này là nền tảng để thiết kế lại KDS module của OneSystem.*  
*Bước tiếp theo: Phê duyệt model → Bắt đầu Phase 1 implementation.*
