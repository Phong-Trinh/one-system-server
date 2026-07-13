# Ghi chú: Vấn đề phân bổ Trạm (Station Assignment) cho KDS

File này lưu lại bối cảnh và những vướng mắc trong việc thiết kế luồng `StartShift` (Block C1) để chuẩn bị cho việc thảo luận và tìm giải pháp tiếp theo.

## 1. Bài toán và Các Ràng buộc (Constraints)
- **Mục tiêu:** Cần thiết kế tính năng `StartShift` cho nhân viên (Block C1).
- **Ràng buộc 1 (Tự động hóa):** Hệ thống phải tự động ra quyết định ưu tiên gán trạm (Station) cố định cho nhân viên. Không bắt quản lý (Manager) phải tự nghĩ và gán tay.
- **Ràng buộc 2 (Cross-trained):** Giả định mọi nhân viên đều biết sử dụng mọi loại máy móc (đa năng).
- **Ràng buộc 3 (Khả năng chịu tải - Edge case):** Hệ thống phải hoạt động hoàn hảo, không bị kẹt (bottleneck) order kể cả khi **chỉ có 1 nhân viên duy nhất** trực trong bếp.

## 2. Điểm nghẽn cốt lõi (Core Problem)
Luồng Dispatcher (C3) hiện tại đang có logic lọc cứng trong hàm `pickStaff`:
- Nếu nhân viên được gán `StationID = FRYER`, Dispatcher **tuyệt đối không bao giờ** giao task của trạm `GRILL` cho nhân viên đó.
- Dẫn đến việc: Nếu bếp chỉ có 1 nhân viên, và hệ thống "tối ưu" bằng cách gán cố định họ vào `FRYER`, thì toàn bộ các order cần `GRILL` sẽ bị kẹt mãi mãi ở trạng thái `QUEUED`.

## 3. Giải pháp đã đề xuất (Nhưng có thể không phù hợp)
**Dynamic Rebalancing (Tái cân bằng động ở C1):**
- *Cách làm:* Bất cứ khi nào số người trong bếp = 1, C1 sẽ ép đổi `StationID = nil` (Runner linh hoạt) để người đó nhận được mọi task. Nếu có nhiều người, tự động tính toán gán người vào các trạm nóng.
- *Vì sao chưa ổn:* Việc liên tục ép đổi `StationID` trên bản ghi `StaffShift` có thể làm hỏng trải nghiệm người dùng trên KDS (màn hình đang là trạm Chiên bỗng dưng nảy sang toàn bộ bếp), và có thể tạo ra cảm giác hệ thống bị "giật cục", thiếu tính ổn định cho nhân viên đứng trạm.

## 4. Hướng suy nghĩ cho Giải pháp mới (Food for thought)
Thay vì cố gắng "hack" ở tầng C1 (gán đổi StationID liên tục), có thể chúng ta cần giải quyết gốc rễ ở tầng C3 (Dispatcher):
- **Soft Constraint thay vì Hard Constraint:** Liệu `StationID` của nhân viên có nên chỉ là "Sự ưu tiên" thay vì "Sự cấm đoán"? 
  - (Ví dụ: Ưu tiên phát task chiên cho người đứng trạm chiên. Nhưng nếu trạm nướng không có ai và người trạm chiên đang rảnh quá 2 phút, Dispatcher tự động dội task nướng sang cho họ).
- Thiết kế này sẽ giữ cho `StationID` cố định và ổn định trong `StaffShift`, C1 không cần tính toán đổi trạm phức tạp, mà bản thân C3 tự biết "tràn việc" (spillover) khi có nút thắt cổ chai.
