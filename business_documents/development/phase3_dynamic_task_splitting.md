# Phase 3: Dynamic Task Splitting (Phân mảnh động)

Chắc chắn rồi! Dựa vào phân tích vừa rồi về "Nút thắt cổ chai" (Bottleneck) khi làm các task thủ công lớn, dưới đây là kế hoạch chi tiết để hiện thực hóa Hướng C (Dynamic Timeboxing) cho hệ thống Dispatcher.

## Mục tiêu (Goal)
Tự động chẻ nhỏ (Split) các công việc tốn nhiều thời gian (như Nặn Bò, Cân Bò) thành các phần nhỏ hơn (Sub-tasks/Chunks) ngay tại thời điểm lập lịch. Các phần thừa (Remainder) sẽ được trả lại vào Global Queue để bất kỳ nhân viên nào đang rảnh tay cũng có thể nhảy vào làm cùng, từ đó tối đa hóa sức mạnh của Multi-staff Collaboration.

## User Review Required

> [!IMPORTANT]
> **Quyết định về "Max Chunk Time" cho Normal Tasks**
> 
> Trong chức năng `FILL-IN` (Làm xen kẽ lúc rảnh), task sẽ được cắt vừa khít với thời gian rảnh của nhân viên (ví dụ: rảnh 30 phút thì cắt 30 phút).
> 
> Tuy nhiên, với chức năng `NORMAL` (Ví dụ đầu giờ sáng mọi người đều rảnh 10 tiếng), nếu không giới hạn thì một nhân viên sẽ "ôm" luôn cả task 300 viên bò (tốn 2.5 tiếng). Để buộc hệ thống chia đều việc cho 2 nhân viên rảnh rỗi, tôi đề xuất thiết lập một mức **`MaxChunkTime = 60 phút`** (Mức tối đa cho một chunk công việc liên tục). 
> 
> Tức là, nếu task nặn bò tốn 150 phút, Dispatcher sẽ tự động cắt 60 phút giao cho `Nhân viên A`, 90 phút còn lại đẩy ra Queue. Tiếp đó, `Nhân viên B` rảnh sẽ nhặt tiếp 60 phút từ Queue, và 30 phút cuối cùng sẽ được giao cho ai xong việc trước. Bạn có đồng ý với thiết lập `MaxChunkTime = 60 phút` này không?

> [!NOTE]
> **Giải pháp Theo dõi Tiến độ (KDS Tracking)**
> - **Nhóm Task (Grouping):** Các phần bị cắt (Remainder) sẽ dùng chung `RootTaskID`. UI/KDS sẽ tổng hợp chúng lại để hiển thị gộp (Ví dụ: "Đã nặn bò 180/300 viên - Đạt 60%").
> - **Planning vs Runtime:** Mức 60 phút chỉ là "Dự kiến" để chia việc. Thực tế nếu nhân viên làm nhanh (45 phút) và bấm "Hoàn thành" trên KDS sớm, hệ thống Runtime sẽ lập tức bơm task tiếp theo cho họ mà không lãng phí 15 phút thừa. Ngược lại nếu làm chậm, các task sau sẽ tự động trễ theo.

## Proposed Changes

### `internal/usecase/dispatcher.go`

#### [MODIFY] `dispatcher.go` (Luồng `assignSingleTask` cho NORMAL tasks)
- Khai báo hằng số `MaxChunkTime = 60 * time.Minute` ở đầu file.
- Trong hàm `assignSingleTask`, khi chuẩn bị giao 1 task `TaskKindNormal`:
  - Kiểm tra xem task có cờ `IsSplittable == true` hay không.
  - Tính toán tổng thời gian làm task (Estimated Duration). Nếu Duration > `MaxChunkTime`, tính toán `MaxChunkQty` (Số lượng tối đa có thể làm trong 60 phút).
  - Cập nhật `TargetQty` của task hiện tại thành `MaxChunkQty`.
  - Sinh ra một Task mới (Remainder) với số lượng còn lại, gán `RootTaskID = currentTask.RootTaskID` (hoặc `currentTask.ID` nếu nó là gốc).
  - Lưu Remainder task vào Repository (`d.taskRepo.Create`) và trả nó về `pendingTasks` để vòng lặp Dispatcher tiếp tục gán cho nhân viên khác.

#### [MODIFY] `dispatcher.go` (Luồng `assignFillInTasks` cho FILL-IN tasks)
- Xoá logic hardcode cũ: `strings.Contains(candidate.SOPStepID, "weigh")`.
- Thay thế bằng logic cấu hình chuẩn: `step.IsSplittable == true`.
- Cập nhật biến `MinUsefulTime`: Đọc từ `step.MinUsefulTime`, nếu `nil` thì dùng `defaultMinUsefulTime = 15 * time.Minute` (để tránh việc nhân viên chỉ vào nặn 1 viên bò rồi đi rửa tay).
- Cập nhật cơ chế sinh ID cho Remainder task: Sử dụng UUID chuẩn (`uuid.New().String()`) thay vì nối chuỗi `rem-rem-rem` như bản nháp cũ. Đảm bảo set `RootTaskID` đầy đủ.

### `internal/usecase/scheduling_engine_factory_sc3full_test.go` & `stress_test.go`

#### [MODIFY] Cập nhật Mocks (Mocking Configurations)
- Trong các Test (`TestFactory_SC3_FullLoad_1Staff`), cập nhật cấu hình của các SOPStep (như `f3_patty_weigh` và `f3_patty_mold`) để bổ sung các cờ:
  - `IsSplittable: true`
  - `MinUsefulTime: ptr(15 * 60)`

## Verification Plan

### Automated Tests
- Chạy lại bài test `TestFactory_SC3_FullLoad_1Staff` (Với cấu hình 2 Nhân viên, 2 Lò Nướng).
- **Kỳ vọng:** Công đoạn "Tạo hình Patty bằng khuôn" (trước đây tốn từ 12:05 đến 14:35 cho 1 người) sẽ bị cắt thành nhiều Chunk 60 phút. Cả 2 nhân viên sẽ cùng xúm vào nặn bò song song. 
- Tổng thời gian hoàn thành (Gantt Chart) sẽ giảm xuống mức tối ưu cực hạn (Kỳ vọng ~10:30 sáng).

---
Vui lòng review `MaxChunkTime` (60 phút) và bấm Approve nếu bạn đồng ý với kế hoạch này để tôi bắt đầu code!
