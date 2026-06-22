# KDS Strategy Comparison — QSR vs Complex Dining
**Phiên bản:** 1.0  
**Ngày:** 2026-06-23  
**Loại tài liệu:** Phân tích chiến lược & So sánh thử nghiệm  
**Liên quan:** [KDS Redesign.md](./KDS%20Redesign.md)

---

## Mục lục

1. [Bối cảnh và Luận điểm cốt lõi](#1-bối-cảnh-và-luận-điểm-cốt-lõi)
2. [Định nghĩa hai mô hình so sánh](#2-định-nghĩa-hai-mô-hình-so-sánh)
3. [So sánh theo chiều kỹ thuật](#3-so-sánh-theo-chiều-kỹ-thuật)
4. [Thử nghiệm tư duy: Kịch bản thực tế](#4-thử-nghiệm-tư-duy-kịch-bản-thực-tế)
5. [Phân tích giá trị kinh tế](#5-phân-tích-giá-trị-kinh-tế)
6. [Rủi ro và đánh đổi](#6-rủi-ro-và-đánh-đổi)
7. [Quyết định thiết kế đề xuất](#7-quyết-định-thiết-kế-đề-xuất)

---

## 1. Bối cảnh và Luận điểm cốt lõi

### Vấn đề phát sinh

Document KDS Redesign hiện tại (`Section 5`) đề xuất mô hình **Hybrid Station-Based** — được mô tả như cách McDonald's, Jollibee, Gong Cha vận hành. Đây là một lựa chọn đúng đắn và được lập luận chặt chẽ **trong phạm vi QSR (Quick Service Restaurant)**.

Tuy nhiên, nếu OneSystem muốn phục vụ một segment rộng hơn — **pizza truyền thống, 4P's Pizza, premium burger, nhà hàng casual dining** — thì cần đặt câu hỏi: **Mô hình này có scale được không, hay cần thay đổi về bản chất?**

### Luận điểm trung tâm

> Mô hình hiện tại (Model V1) và mô hình mở rộng (Model V2) **không phải là hai lựa chọn loại trừ nhau**. Model V2 là một **superset** của Model V1. Nhưng sự đơn giản của V1 là một *tài sản*, không phải giới hạn — và đánh đổi đó cần được hiểu rõ trước khi quyết định.

---

## 2. Định nghĩa hai mô hình so sánh

### Model V1 — Hybrid Station-Based (Hiện tại)

*Như đã thiết kế trong KDS Redesign.md, Section 5–8.*

**Các giả định nền tảng:**
- Các station hoạt động **độc lập và song song** (Fryer không cần đợi Grill)
- Idle time là **binary**: hoặc `can_fill_idle = true` hoặc `false`
- Bin-packing xảy ra **tự do** trong cùng equipment_type
- Accountability được giải quyết qua **order reference** trên mỗi StaffTask
- Assembly step là **điểm duy nhất** gom đơn lại

**Kiến trúc data cốt lõi:**
```
ProductionOrder
  └── SOPStep (equipment_type, duration, is_idle_step, can_fill_idle)
        └── StaffTask (staff_id, machine_id, reference_order_id, status)
```

---

### Model V2 — Dependency-Aware Hybrid (Mở rộng đề xuất)

**Bổ sung so với V1:**

1. **`attention_level`** thay thế `can_fill_idle: bool`
2. **`depends_on_steps[]`** trong SOPStep — dependency graph
3. **Item-level tracking** bên trong 1 order
4. **`same_item_required: bool`** — ngăn scheduler batch sai

**Kiến trúc data bổ sung:**
```
ProductionOrder
  └── OrderItem (item-level, có timeline riêng)
        └── SOPStep (+ depends_on_steps[], attention_level, same_item_required)
              └── StaffTask (không đổi cơ bản)
```

---

## 3. So sánh theo chiều kỹ thuật

### 3.1 — Mô hình hóa Idle Time

#### V1: Boolean `can_fill_idle`

```
SOPStep {
  is_idle_step:   true
  can_fill_idle:  true   ← hoặc false
}
```

**Điều này giả định:** Khi máy chạy tự động, nhân viên hoặc **hoàn toàn rảnh** hoặc **hoàn toàn bận**.

**Thực tế tại bếp phức tạp:**

| Tình huống | Mô tả vận hành | V1 model được không? |
|---|---|---|
| Lò pizza chạy 10 phút | Nhân viên có thể sang Prep station 5 mét | ✅ `can_fill_idle = true` |
| Patty đang grilling | Cần lật đúng 4 phút, quan sát màu | ❌ Không thể giao task di chuyển xa |
| Risotto đang simmer | Check khuấy mỗi 2 phút | ❌ V1 không model được chu kỳ check |
| Bột bánh đang ủ 20 phút | Hoàn toàn rảnh, không cần quan sát | ✅ `can_fill_idle = true` |

**Kết luận:** V1 xử lý được 2/4 tình huống phổ biến trong bếp phức tạp.

#### V2: `attention_level` enum

```
enum AttentionLevel {
  FULL_IDLE         // Máy tự chạy hoàn toàn, không cần quan sát
  NEARBY_IDLE       // Ở gần (<3m), có thể làm tay nhưng không di chuyển xa
  PERIODIC_CHECK    // Check định kỳ (interval_seconds)
  ACTIVE_WAIT       // Đứng chờ, không rời máy
}

SOPStep {
  attention_level:      AttentionLevel
  check_interval_sec:   int?    // Chỉ dùng khi PERIODIC_CHECK
  max_distance_meters:  float?  // Chỉ dùng khi NEARBY_IDLE
}
```

**Lợi ích:** Scheduler V2 có thể tính toán chính xác hơn:
- `FULL_IDLE` → Tìm fill-in task ở bất kỳ station nào
- `NEARBY_IDLE` → Chỉ fill-in task không cần di chuyển xa (ví dụ: prep cùng khu vực)
- `PERIODIC_CHECK` → Fill-in task ngắn hơn `check_interval_sec`, có flag `is_interruptible = true`
- `ACTIVE_WAIT` → Không fill-in, ghi nhận idle time để cải thiện SOP về sau

---

### 3.2 — Mô hình hóa Step Dependencies

#### V1: Implicit Sequential (dựa vào `seq_no`)

```
SOPStep { seq_no: 1, equipment_type: "PREP" }
SOPStep { seq_no: 2, equipment_type: "OVEN" }   ← giả định: 2 luôn sau 1
SOPStep { seq_no: 3, equipment_type: "ASSEMBLY" }
```

**Giả định ngầm của V1:** Steps chạy tuần tự theo `seq_no`. Station thực hiện chúng song song với nhau (nhiều đơn khác nhau).

**Vấn đề với món phức tạp:**

```
[PIZZA 4 LOẠI TOPPING — SOP phức tạp]

Step 1 (PREP):    Nhào bột                 → 5 phút thao tác
Step 2 (REST):    Ủ bột lạnh               → 20 phút FULL_IDLE
Step 3 (PREP):    Cán bột + trải sauce     → 4 phút thao tác
Step 4 (TOPPING): Đặt topping A + B        → 3 phút thao tác
Step 5 (TOPPING): Đặt topping C (cần ướp trước 10 phút) → phụ thuộc vào prep riêng
Step 6 (OVEN):    Nướng                    → 11 phút PERIODIC_CHECK
Step 7 (ASSEMBLY): Cắt + plating           → 2 phút

Step 5 phụ thuộc vào một sub-process riêng (ướp topping C) chạy song song
Step 6 chỉ bắt đầu khi cả Step 4 VÀ Step 5 đều done
```

Với V1, scheduler không có cách nào biết rằng Step 6 phụ thuộc cả Step 4 **và** Step 5. `seq_no` không đủ diễn đạt dependency nhiều-nguồn.

#### V2: Explicit Dependency Graph

```
SOPStep {
  seq_no:           6
  equipment_type:   "OVEN"
  depends_on_steps: ["step_4", "step_5"]  ← explicit multi-dependency
  same_item_required: true                ← không batch với pizza khác
}
```

**So sánh scheduling behavior:**

| Kịch bản | V1 behavior | V2 behavior |
|---|---|---|
| Step 6 khi Step 4 xong nhưng Step 5 chưa | Scheduler có thể nhầm: coi Step 6 là READY (seq_no 6 > 5) | Scheduler đúng: Step 6 vẫn BLOCKED |
| Nhiều đơn pizza cùng lúc, Step 6 có thể batch không? | Không có thông tin → có thể batch sai | `same_item_required: true` → không batch |
| Sub-process độc lập (ướp topping C) | Không model được | `depends_on_steps` từ sub-process ID |

---

### 3.3 — Mô hình hóa Item-Level Tracking

#### V1: Order-Level Tracking

Toàn bộ đơn hàng được xử lý như một đơn vị. Assembly step trigger khi **tất cả station** báo xong.

```
ProductionOrder PO#01
├── FRYER tasks  → done ✅
├── GRILL tasks  → done ✅
└── BEVERAGE tasks → done ✅
          ↓
   Assembly READY
```

**Giới hạn:** Nếu PO#01 có 3 pizza khác nhau:
- Pizza Margherita: Lò 1, 10 phút
- Pizza Truffle: Lò 2, 14 phút (khác SOP, khác thời gian)
- Bruschetta: Không qua lò, xong sau 3 phút

Trong V1, Assembly chỉ trigger một lần khi **tất cả** xong. Nhưng thực tế, Runner cần biết: *"Bruschetta xong rồi, để lên plate trước, 2 pizza còn 11 phút nữa."* — không phải đợi tất cả xong mới làm.

#### V2: Item-Level Tracking với Partial Assembly

```
ProductionOrder PO#01
├── OrderItem: Margherita → [STEP_PREP → STEP_OVEN → STEP_CUT]   → 10 phút
├── OrderItem: Truffle    → [STEP_PREP → STEP_OVEN_LONG → STEP_CUT] → 14 phút
└── OrderItem: Bruschetta → [STEP_TOAST → STEP_TOPPING]           → 3 phút ✅ DONE

AssemblyTask {
  trigger_condition: ALL_ITEMS_DONE  // hoặc
  trigger_condition: PARTIAL_READY   // Runner được thông báo từng item xong
  current_ready_items: ["Bruschetta"]
  pending_items: ["Margherita (7 phút)", "Truffle (11 phút)"]
}
```

**Lợi ích trong vận hành thực tế:**
- Runner biết *chính xác* khi nào quay lại plate — không đứng chờ
- Không để Bruschetta nguội trong 11 phút chờ pizza
- Manager thấy được tiến độ từng item trong đơn, không chỉ % tổng đơn

---

### 3.4 — Bin-Packing và Batching Logic

#### V1: Bin-Pack tự do trong cùng equipment_type

```
Fryer Station:
  [PO#01 — Khoai 5kg][PO#03 — Gà 3kg] ← batch cùng nồi nếu còn slot
```

V1 giả định: mọi items cùng `equipment_type` đều có thể batch với nhau.

**Thực tế tại bếp pizza:**

| Item | Equipment | Có batch được không? |
|---|---|---|
| Pizza Margherita + Pizza Pepperoni | Oven | ✅ Nếu cùng nhiệt độ |
| Pizza Napolitana (450°C, 90 giây) + Pizza NY style (260°C, 12 phút) | Oven | ❌ Nhiệt độ khác nhau hoàn toàn |
| Khoai tây + Cánh gà | Fryer | ❌ Thường không (nhiễm mùi, thời gian khác nhau) |
| Khoai tây lần 1 (blanch) + Khoai tây lần 2 (fry) | Fryer | ❌ Hai bước SOP của cùng 1 item, không thể overlap |

#### V2: Batch Constraints

```
SOPStep {
  batch_compatible_with: string[]?   // Chỉ batch với equipment_profile nào
  equipment_profile: {               // Chi tiết hơn equipment_type
    temperature_celsius: 260,
    mode: "CONVECTION"
  }
  same_item_required: bool           // true = không batch với đơn khác
}
```

V2 cho phép scheduler tính toán: hai steps chỉ batch khi `equipment_profile` tương thích.

---

## 4. Thử nghiệm tư duy: Kịch bản thực tế

### Kịch bản A — Gong Cha (QSR Beverage)

**Đơn hàng:** 3 ly trà sữa khác nhau, 1 nhân viên beverage station.

| Bước | V1 xử lý | V2 xử lý | Kết quả |
|---|---|---|---|
| Đun nước → FULL_IDLE 5 phút | `can_fill_idle = true` → chèn task pha đồ cold | `FULL_IDLE` → chèn task pha đồ cold | Như nhau ✅ |
| Shaking cup | `can_fill_idle = false` | `ACTIVE_WAIT` → không fill-in | Như nhau ✅ |
| Assembly 3 ly | Order-level trigger | Order-level trigger | Như nhau ✅ |

**Kết luận Kịch bản A:** V1 đủ. V2 thêm complexity không cần thiết. **V1 win.**

---

### Kịch bản B — Premium Burger (Casual QSR)

**Đơn hàng:** 1 Wagyu Burger. SOP: Sear patty → Rest 3 phút → Grill toast bun → Assembly.

| Bước | V1 behavior | V2 behavior | Vấn đề V1 |
|---|---|---|---|
| Sear patty 4 phút (ACTIVE_WAIT) | `can_fill_idle = false` → OK | `ACTIVE_WAIT` → không fill-in | Giống nhau ✅ |
| Rest patty 3 phút (FULL_IDLE) | `can_fill_idle = true` → chèn Grill task | `FULL_IDLE` → chèn Grill task | Giống nhau ✅ |
| Step 3 (Grill) **phụ thuộc** vào Step 1 XONG (cùng patty) | V1 không biết → có thể schedule Grill step của đơn khác làm chậm | `same_item_required: true` → đảm bảo Grill sẵn sàng đúng lúc | **V1 có thể gây conflict nhỏ** ⚠️ |
| Wagyu cần nhiệt độ core 54°C (medium rare) | Không model được | `quality_target` field → thông báo nhân viên | V1 thiếu context vận hành ⚠️ |

**Kết luận Kịch bản B:** V1 hoạt động được nhưng có edge case về timing chính xác. V2 xử lý tốt hơn về dependency và quality gate. **V2 slightly better.**

---

### Kịch bản C — 4P's Pizza (Complex Casual Dining)

**Đơn hàng:** 2 pizza (Margherita + Seafood), 1 Bruschetta. 2 nhân viên: 1 Prep, 1 Oven/Assembly.

**Timeline thực tế cần xảy ra:**
```
T+00: NV Prep bắt đầu nhào bột (Margherita)
T+05: Bột ủ 20 phút → NV Prep chuyển sang làm Bruschetta
T+06: Bruschetta xong → Runner giao Bruschetta (partial assembly)
T+25: Bột ủ xong → NV Prep cán bột Margherita
T+30: Margherita vào lò (11 phút, PERIODIC_CHECK mỗi 4 phút)
T+30: Song song → NV Prep bắt đầu Seafood pizza
T+35: Seafood vào lò (13 phút, cùng lò nếu nhiệt độ match)
T+41: Margherita ra lò, cắt + plate
T+48: Seafood ra lò, cắt + plate
T+48: Assembly: gom toàn đơn (Bruschetta đã ra từ T+06)
```

**V1 có thể xử lý không?**

| Yêu cầu vận hành | V1 có thể? | Lý do |
|---|---|---|
| NV Prep biết ủ bột xong thì làm Bruschetta | ⚠️ Được nếu SOP có fill-in step, nhưng phụ thuộc `seq_no` order | `can_fill_idle = true` → fill-in task Bruschetta. Nhưng scheduler có nhận ra đây là cùng equipment_type không? |
| Runner nhận Bruschetta trước khi pizza xong | ❌ V1 chỉ có 1 assembly trigger khi tất cả station done | Không có partial assembly |
| Margherita + Seafood batch vào cùng lò | ⚠️ Có nếu `equipment_type` giống nhau, nhưng không check temperature profile | Có thể batch sai |
| Step "Seafood vào lò" chờ Step "Seafood prep" xong | ⚠️ Chỉ đúng nếu seq_no được set tuyến tính, không có dependency song song | Nếu scheduler quyết định re-order vì máy available thì sai |
| Alert nhân viên quay lại lò sau mỗi 4 phút | ✅ `requires_attention_at` xấp xỉ model được nếu set đúng | Nhưng không có `check_interval_sec` → chỉ alert 1 lần |

**Kết luận Kịch bản C:** V1 **không xử lý được đầy đủ** và có thể gây ra lỗi runtime trong scheduling. V2 là bắt buộc cho segment này. **V2 wins decisively.**

---

### Kịch bản D — Fine Dining (Bếp cao cấp)

**Đặc điểm:** 1 chef chịu trách nhiệm toàn bộ đơn. Rất ít batching. Chất lượng > throughput.

| Tiêu chí | V1 | V2 |
|---|---|---|
| Model phù hợp | Kém — V1 cố gắng station-based nhưng không có nhiều nhân viên station | Kém hơn V1 — dependency graph phức tạp nhưng vẫn sai về bản chất |
| Model đúng thực sự | **Order-Based thuần** — 1 chef, 1 đơn | Như V1 |
| Khuyến nghị | Thêm profile `FINE_DINING` → tắt station-based hoàn toàn | Thêm profile `FINE_DINING` như V2 |

**Kết luận Kịch bản D:** Cả V1 và V2 không phải model phù hợp cho fine dining. Cần **profile system** cho phép chọn chế độ vận hành theo loại nhà hàng.

---

## 5. Phân tích giá trị kinh tế

### Chi phí Implementation

| Hạng mục | Model V1 | Model V2 |
|---|---|---|
| Data model mới | StaffTask, StaffShift, mở rộng SOPStep | Thêm: OrderItem, dependency fields, attention_level |
| Scheduler complexity | Medium — FIFO/EDF + bin-pack | High — dependency graph topological sort + constraint satisfaction |
| SOP configuration effort | Thấp — Manager điền `duration`, `is_idle_step`, `can_fill_idle` | Cao hơn — phải map dependency, chọn attention_level, set equipment_profile |
| Testing complexity | Thấp — vài kịch bản QSR | Cao — combinatorial từ dependency + attention level |
| Time to ship Phase 1 | Ước tính baseline | +30–50% so với V1 |

### Giá trị mang lại theo segment

| Segment | Chiếm % thị trường F&B VN | V1 đủ không? | V2 cần không? |
|---|---|---|---|
| QSR (Gong Cha, KFC clone) | ~60% của chain F&B | ✅ Hoàn toàn đủ | ❌ Over-engineered |
| Casual QSR (Burger King style) | ~20% | ✅ Phần lớn đủ | ⚠️ Một số tính năng V2 hữu ích |
| Casual Dining (4P's, Pizza) | ~15% | ❌ Không đủ | ✅ Cần thiết |
| Fine Dining | ~5% | ❌ | ❌ Cần profile riêng |

### ROI theo lộ trình

```
Phase MVP (V1 only):
  → Phục vụ được ~80% thị trường chain F&B
  → Ship nhanh hơn ~40%
  → Ít bug hơn do ít complexity

Phase Expansion (V1 + V2 extensions):
  → Mở thêm 15% market (Casual Dining)
  → Tăng Average Revenue Per Restaurant (Casual Dining trả nhiều hơn)
  → Differentiation vs. đối thủ QSR-only
```

---

## 6. Rủi ro và đánh đổi

### Rủi ro của V1 (quá đơn giản)

| Rủi ro | Mức độ | Biện pháp giảm thiểu |
|---|---|---|
| Scheduler batch sai items không tương thích | **Cao** trong complex dining | Document rõ ràng giới hạn segment mục tiêu |
| Nhân viên nhận fill-in task không thể hoàn thành kịp | **Trung bình** | Conservative buffer trong `requires_attention_at` |
| Mất khách hàng Casual Dining vì thiếu tính năng | **Cao nếu** target segment này | Roadmap V2 rõ ràng, không oversell V1 |
| Dependency sai → món ra không đúng thứ tự | **Cao** với SOP phức tạp | Restrict V1 cho SOP linear đơn giản |

### Rủi ro của V2 (quá phức tạp)

| Rủi ro | Mức độ | Biện pháp giảm thiểu |
|---|---|---|
| Scheduler engine phức tạp → khó debug | **Cao** | Tách dependency resolver thành module riêng, unit test kỹ |
| Onboarding nhà hàng khó — phải config dependency graph | **Cao** | UI wizard để build SOP, template library sẵn |
| Over-engineering cho QSR client — họ không cần | **Trung bình** | Profile system: QSR profile ẩn tất cả V2 fields |
| Ship chậm → mất thị trường QSR cho đối thủ | **Cao** | Ship V1 trước, V2 là incremental |

---

## 7. Quyết định thiết kế đề xuất

### ~~Kiến trúc "Progressive Enhancement"~~ → Refactor sang V2 là MVP Baseline

> [!IMPORTANT]
> **Quyết định đã thay đổi:** V2 không phải là opt-in extension của V1. V2 là **mô hình đúng** cho vision sản phẩm. Toàn bộ MVP sẽ được thiết kế và implement trực tiếp theo V2.
>
> Lý do từ chối hướng progressive enhancement:
> - V1 giải quyết bài toán QSR — **không phải bài toán mà OneSystem đang nhắm tới**
> - Thiết kế V1 trước sẽ tạo ra **technical debt ngay từ đầu**: scheduler không có dependency graph sẽ phải bị viết lại hoàn toàn, không phải extend
> - Dependency graph là **thứ thay đổi bản chất kiến trúc scheduler** — không thể bolt-on sau

### Data Model V2 — Baseline (không phải extension)

`can_fill_idle: bool` bị **xóa hoàn toàn**, thay bằng `attention_level`. Không có backward compat với V1 config.

```
SOPStep {
  // Core fields (không đổi)
  equipment_type_id:      string
  duration:               int
  description:            string

  // Idle modeling — V2 replaces boolean
  is_idle_step:           bool             // true = máy tự chạy sau setup
  attention_level:        AttentionLevel   // BẮT BUỘC nếu is_idle_step = true
  check_interval_sec:     int?             // Chỉ dùng khi PERIODIC_CHECK
  max_distance_meters:    float?           // Chỉ dùng khi NEARBY_IDLE
  requires_attention_at:  int?             // Buffer giây trước khi hết idle

  // Dependency graph — V2 core
  depends_on_steps:       string[]         // BẮT BUỘC khai báo, [] = chỉ phụ thuộc seq
  same_item_required:     bool             // true = không bin-pack với đơn khác

  // Equipment constraint — V2 core
  equipment_profile: {
    temperature_celsius:  float?
    mode:                 string?          // "CONVECTION" | "DECK" | "WOOD_FIRED"
  }?
}

// KHÔNG CÒN:
// can_fill_idle: bool   ← ĐÃ XÓA
```

### Scheduler Engine — V2 từ ngày đầu

Scheduler không còn là simple queue + bin-pack. Đây là **constraint-based scheduler**:

```
Scheduler.schedule(productionOrder) {
  1. Parse SOP → build dependency DAG (directed acyclic graph)
  2. Topological sort → xác định thứ tự có thể schedule
  3. Với mỗi step (theo topo order):
     a. Check depends_on_steps → tất cả đã DONE chưa?
     b. Tìm machine: equipment_type + equipment_profile compatible
     c. Nếu same_item_required = true → không merge với batch khác
     d. Tìm staff: station phù hợp, đang available
     e. Tính scheduled_start dựa trên dependency + machine timeline
  4. Với mỗi idle step (is_idle_step = true):
     a. Xác định attention_level → quyết định fill-in constraint
     b. Tìm fill-in task thỏa mãn attention constraint
     c. Tạo fill-in StaffTask với parent_task_id
  5. Output: ordered StaffTask list per staff
}
```

### Item-Level Tracking — Bắt buộc trong MVP

```
ProductionOrder
  └── OrderItem[]          ← MỚI: mỗi món là 1 entity riêng
        ├── item_id
        ├── product_id
        ├── sop_id
        └── SOPStep[]      ← dependency graph riêng cho item này
              └── StaffTask[]

AssemblyTask {
  po_id:              string
  trigger_condition:  "ALL_ITEMS" | "PARTIAL_NOTIFY"
  ready_items:        OrderItem[]   // Cập nhật realtime
  pending_items:      OrderItem[]   // Còn bao nhiêu, còn bao lâu
}
```

### Lộ trình thực thi (đã điều chỉnh)

```
Phase 1 — V2 MVP:
  ├── OrderItem model (item-level tracking)
  ├── SOPStep V2 (attention_level, depends_on_steps, equipment_profile)
  ├── StaffTask + StaffShift (không đổi)
  ├── Scheduler V2: dependency DAG + attention-aware fill-in
  ├── Staff KDS screen (1 task, 1 nút DONE)
  ├── Partial Assembly trigger + Runner view
  └── Manager KDS (readonly dashboard)
  Target: Ship trong 10–12 tuần (thay vì 6–8 của V1)

Phase 2 — Optimization & Tooling:
  ├── SOP wizard UI (visual dependency builder)
  ├── Template library (Pizza SOP, Burger SOP, Beverage SOP)
  ├── OEE analytics + bottleneck detection
  └── Historical timing calibration (estimate vs actual)

Phase 3 — Scale & Intelligence:
  ├── Multi-node scheduling (nhiều bếp/chi nhánh)
  ├── Predictive scheduling dựa trên historical data
  └── Auto-suggest SOP từ menu item
```

---

## Tóm tắt quyết định (cập nhật)

| Câu hỏi | Kết luận |
|---|---|
| V1 hay V2 cho MVP? | **V2** — đây là mô hình đúng cho vision sản phẩm |
| V1 có giá trị không? | Có — như **benchmark** và **test case** đơn giản nhất của V2 scheduler |
| Rủi ro lớn nhất của V2 MVP? | Scheduler phức tạp hơn → cần unit test kỹ dependency resolver |
| Điểm khác biệt tối quan trọng? | **Dependency DAG** trong SOPStep + **item-level tracking** trong OrderItem |
| QSR client có dùng được không? | **Có** — V2 với `depends_on_steps: []` và `same_item_required: false` = V1 behavior |

> [!IMPORTANT]
> **Quyết định cốt lõi:** V2 là MVP baseline. Không có giai đoạn V1. Mọi thiết kế từ data model đến scheduler đều theo V2 ngay từ Phase 1.
>
> QSR client vẫn phục vụ được vì V2 có **safe defaults** biểu hiện giống V1: dependency graph rỗng = sequential; `attention_level: FULL_IDLE` = `can_fill_idle: true` cũ.

> [!NOTE]
> Tài liệu này là phần mở rộng phân tích cho [KDS Redesign.md](./KDS%20Redesign.md).  
> Bước tiếp theo: Cập nhật KDS Redesign.md (Section 5, 6, 8) để phản ánh V2 là baseline thiết kế.
