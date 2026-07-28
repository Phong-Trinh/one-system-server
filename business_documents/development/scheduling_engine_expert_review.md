# Đánh Giá Chuyên Gia: Scheduling Engine (SC1 → SC8)

> *Đánh giá độc lập dưới góc nhìn của một Operations Expert F&B — không có vùng cấm, không nịnh nọt.*
> 
> Ngày đánh giá: 2026-07-28

---

## 1. Tổng Quan Hiện Trạng

Sau khi review toàn bộ log test, code base và kết quả stress test, đây là bức tranh toàn cảnh:

| Nhóm | Test Suites | Kết quả |
|:--|:--|:--|
| **Core Logic (SC1–SC3)** | `TestFactory_SC1–3`, `TestSchedulingEngine_T1–T27` | ✅ 100% PASS |
| **Production Stress (SC4)** | `TestFactory_SC4_HappyPath`, `ExtremeStress`, `SC4_FullSimulation/A1` | ✅ PASS |
| **SC4 Full Edge Case** | `SC4_FullSimulation/A2_HygieneSwitchWithinLimit` | ❌ FAIL |
| **KDS Execution (SC8 Basic)** | `TestKDS_Case1/2/3`, `TestKDS_TC41–44` | ✅ 100% PASS |
| **SC8 Stress Test** | `TestStress_Case1/2/3` | ✅ 100% PASS |

**Verdict:** Hệ thống đang ở mức **Production-Ready MVP** cho luồng chính. Còn 1 lỗi Regression đang sống ở phần Hygiene Switch của SC4.

---

## 2. Những Điều Engine Đang Làm Rất Tốt

### ✅ 2.1. Predictive Scheduling — Đúng Bản Chất
Engine tính toán lịch trình **trước** khi thực tế xảy ra. Đây là nền tảng vững chắc nhất. Tất cả staff, máy móc được phân công **có thứ tự logic, có kiểm tra dependency**. Không "bốc mò".

### ✅ 2.2. Fill-In Task (Chèn Việc Vào Idle Window)
Khi lò nướng đang chạy (machine idle), hệ thống tự động tìm việc thủ công khác để chèn cho nhân viên. Đây là điểm **khó nhất** trong lập lịch F&B và đang hoạt động ổn định. Log từ SC3/SC4 cho thấy không có khoảng trống vô lý.

### ✅ 2.3. Machine Fallback (Dự Phòng Máy Móc — SC2)
Khi 1 lò hỏng, Dispatcher **không cần can thiệp thủ công**. Hệ thống tự động chuyển toàn bộ task sang máy còn lại. Đây là tính năng "must-have" cho bếp thực tế và đang hoạt động tốt.

### ✅ 2.4. KDS Execution Loop (SC8 — Vừa Hoàn Thành)
Ba luồng thực thi KDS đều hoạt động đúng:
- `StartTask` / `CompleteTask` (Happy Path)
- `CompleteTask` trễ > 5 phút → Trigger **Global Reschedule** tự động
- `FailTask` (Hỏng máy) → Khóa máy + Trigger **Global Reschedule**

### ✅ 2.5. Chaos Resilience (Phát Hiện Qua Stress Test)
Hệ thống xử lý được **3 sự kiện gián đoạn liên hoàn** mà không bị deadlock, duplicate task, hay mất task. Đây là tiêu chuẩn cực kỳ quan trọng khi đưa ra môi trường thực tế.

Kịch bản đã kiểm chứng:
1. **Domino Effect** — Trễ 1 khâu → Các khâu downstream tự lùi, khâu độc lập không bị ảnh hưởng.
2. **Catastrophic Breakdown** — Máy nướng hỏng giữa ca → Dồn hết sang máy dự phòng, không leak task sang máy hỏng.
3. **The Chaos Monkey** — Trễ → Hỏng máy → Trễ tiếp (3 sự kiện liên hoàn) → Hệ thống tự phục hồi cả 3 lần, không deadlock.

---

## 3. Những Điểm Yếu Hiện Tại (Từ Góc Nhìn Vận Hành)

### ❌ 3.1. SC4/A2 — Hygiene Switch Logic Bị Hỏng (BUG REGRESSION)

**Triệu chứng:** `TestFactory_SC4_FullSimulation/A2_HygieneSwitchWithinLimit` đang FAIL.

**Phân tích vận hành:**
Trong bếp thực tế, khi nhân viên chuyển từ xử lý **thịt sống** (Raw Meat) sang **thực phẩm đã chín** (Cooked Food), cần 2-3 phút **rửa tay + sát khuẩn**. Đây là quy định Vệ Sinh An Toàn Thực Phẩm bắt buộc. Hệ thống đã từng có logic kiểm tra giới hạn thời gian chuyển đổi này, nhưng test A2 đang thất bại — có thể bị phá vỡ bởi thay đổi liên quan đến fix Bug Catastrophic Breakdown.

> **⚠️ CẢNH BÁO:** Đây là vấn đề **an toàn thực phẩm**, không phải chỉ là vấn đề kỹ thuật. Nếu deploy lên production mà Hygiene Switch bị sai, hệ thống có thể ra lệnh cho nhân viên cầm bánh chín mà không rửa tay sau khi làm thịt sống. **Cần fix trước khi đưa ra thị trường.**

### ⚠️ 3.2. Global Reschedule — Chiến Lược Đúng Nhưng Chi Phí Cao

**Phân tích:**  
Mỗi khi có sự kiện drift/fail, Engine "hủy toàn bộ PENDING → QUEUED và chạy lại Dispatcher". Điều này **an toàn nhưng cồng kềnh**. Từ **Chaos Monkey log**, sau mỗi sự kiện Dispatcher phải reset và gán lại 9–11 tasks. Với quy mô bếp lớn hơn (50+ tasks PENDING), mỗi lần Reschedule sẽ là một vòng loop nặng. **Chưa cần xử lý ngay** cho MVP, nhưng cần ghi nhận để tối ưu sau.

### ⚠️ 3.3. Fill-In Assignment — Vẫn Còn Bỏ Lỡ Cơ Hội Cross-Staff

Từ log stress test, rất nhiều dòng như:

```
findFillInCandidate rejecting patty_prep because AssignedTo=f_staff_2 != staffID=f_staff_1
```

Khi Staff 1 đang rảnh chờ lò, hệ thống từ chối cho Staff 1 làm `patty_prep` vì task đó đang "gán" cho Staff 2. Hành vi đúng về kỹ thuật, nhưng nếu Staff 2 chưa bắt đầu, việc Staff 1 tiếp tay là hoàn toàn hợp lý trong thực tế vận hành. → Hệ thống chưa có **Cross-Staff Reassignment**.

### ⚠️ 3.4. Task PENDING Không Có "Bảo Vệ" Khi Reschedule

Khi Reschedule được trigger, tất cả task PENDING (kể cả task nhân viên **thực tế đang chuẩn bị làm nhưng chưa bấm nút**) bị reset về QUEUED. Trong thực tế, nhân viên thường quên bấm `StartTask` ngay. Đây là **UX risk** cần xử lý ở tầng API/KDS UI sau.

---

## 4. Scorecard Tổng Hợp

| Tiêu Chí Vận Hành | Điểm | Nhận Xét |
|:---|:---|:---|
| **Lập lịch đa PO song song** | 9/10 | Hoạt động tốt, xử lý được Deadline Priority |
| **Dự phòng khi máy hỏng** | 9/10 | Tự động failover, đã được kiểm chứng SC2 + SC8 |
| **Phân công nhân viên thông minh** | 7/10 | Ổn nhưng chưa có Reassign Fill-In cross-staff |
| **Chèn việc vào idle time** | 7/10 | Đang hoạt động nhưng bỏ lỡ cơ hội cross-staff |
| **Phục hồi sau sự cố runtime (SC8)** | 9/10 | Drift detection + Global Reschedule rất tốt |
| **An toàn thực phẩm (Hygiene)** | 6/10 | Test đang FAIL — cần review ngay |
| **Hiệu suất ở quy mô lớn** | 6/10 | Chưa kiểm chứng với 50+ tasks |
| **Độ ổn định (No Deadlock/Duplicate)** | 10/10 | Đã vượt qua Chaos Monkey 3 lần liên tiếp |

**Tổng: 63/80 — Rất tốt cho giai đoạn MVP**

---

## 5. Khuyến Nghị Theo Thứ Tự Ưu Tiên

### 🔴 Trước Khi Demo/Deploy
1. **Fix lỗi SC4/A2 Hygiene Switch** — Rủi ro an toàn thực phẩm thực sự, không thể bỏ qua.

### 🟡 Trong Giai Đoạn MVP
2. **Tầng HTTP API / WebSocket cho KDS** — Mảnh ghép cuối để nối Engine với màn hình KDS thực tế. Không có cái này thì toàn bộ engine chỉ là logic test, chưa dùng được.

### 🟢 Sau Khi MVP Chạy Thực Tế Và Thu Thập Feedback
3. **Cross-Staff Fill-In Reassignment** — Cho phép Staff rảnh "mượn" task chưa bắt đầu của Staff khác.
4. **Incremental Reschedule** — Thay Global Reschedule khi số task tăng để cải thiện hiệu suất.
5. **Dynamic Timeboxing** — Task dài 200 phút (làm bò) cần được cắt nhỏ theo idle window để tận dụng tối đa thời gian rảnh.

---

## 6. Kết Luận

> Hệ thống đang ở giai đoạn **"rất giỏi làm bài kiểm tra trong phòng lab"**. Core Engine vững, logic chính xác, khả năng chịu sự cố đã được kiểm chứng qua bão Chaos Monkey.
>
> Thách thức lớn nhất phía trước **không phải thuật toán** mà là **kết nối với thế giới thực**: làm sao nhân viên bếp bấm nút đúng thời điểm, làm sao xử lý khi họ quên bấm, làm sao hiển thị lịch trình đủ đơn giản để người không biết IT vẫn hiểu.
>
> **Tóm lại: Fix Hygiene Switch → Build API Layer → Demo cho khách hàng đầu tiên. Đó là 2 việc duy nhất cần làm trước khi ra thị trường.**
