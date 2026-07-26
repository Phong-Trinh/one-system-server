# Phân Tích & Phản Biện Kế Hoạch Tối Ưu Task & Capacity

Dưới góc độ của một chuyên gia vận hành (Operations/Manufacturing Expert), đánh giá của bạn BA Junior là có cơ sở (nhìn ra được triệu chứng), nhưng **hướng giải quyết lại quá ngây thơ và thiếu tính thực tiễn của một gian bếp chuyên nghiệp**.

## 1. Phản biện về "Chia nhỏ công việc làm Bò" & Preemption
**Vấn đề BA đưa ra:** Task "Làm bò 200 phút" là khối nguyên khối (Monolithic), dẫn đến lãng phí thời gian rảnh. -> **Chính xác.**

**Sai lầm trong tư duy của BA (Ngây thơ về vận hành):**
BA nghĩ rằng nhân viên có thể "đang nặn bò, lò kêu, rửa tay đi lấy bánh rồi quay lại nặn tiếp" một cách trơn tru. Trong thực tế ngành F&B:
1. **Context Switching Cost (Chi phí chuyển đổi):** Đang xử lý thịt sống (Raw Meat), muốn chuyển sang cầm bánh chín (Cooked Food) đòi hỏi phải tháo găng tay, rửa tay bằng xà phòng, sát khuẩn, và có thể phải cất dọn bớt đồ sống để tránh nhiễm khuẩn chéo (Cross-contamination). Quá trình này mất ít nhất 2-3 phút.
2. **Min Useful Time:** Nếu lò nướng chỉ còn 5 phút nữa là chín, việc điều động nhân viên ra "nặn bò" trong 5 phút là thảm họa. Họ vừa rửa tay xong chuẩn bị nặn được 1 viên thì lò lại kêu.

### Phân tích Hướng A (Pre-splitting - Cắt nhỏ từ đầu)
*   **Ưu điểm:** Dễ code, dễ map vào thuật toán.
*   **Nhược điểm:** Cứng nhắc. Nếu chia nhỏ thành các task 15 phút, nhưng nhân viên lại có khoảng rảnh liên tục 60 phút, hệ thống sẽ bắt nhân viên bấm "Hoàn thành" 4 lần liên tục trên màn hình KDS. Rất rác UI và phiền phức. Không giải quyết được Context Switching.

### Phân tích Hướng B (Task Preemption - Cắt ngang runtime)
*   **Ưu điểm:** Linh hoạt.
*   **Nhược điểm:** Cực kỳ khó code. Rủi ro về Data Consistency rất cao. Và quan trọng nhất: Thuật toán lập lịch (Scheduling Engine) là để **DỰ ĐOÁN VÀ SẮP XẾP TRƯỚC**. Nếu để hệ thống chạy tới đâu cắt ngang tới đó (Reactive), chúng ta đang phá vỡ bản chất của "Predictive Schedule". Lò nướng kêu không phải là một "sự kiện bất ngờ", Dispatcher ĐÃ BIẾT TRƯỚC lò sẽ kêu lúc nào!

### 🔥 Đề xuất của Chuyên Gia: Hướng C - Dynamic Timeboxing (Phân mảnh động tại thời điểm lập lịch)
*   **Cách hoạt động:** Giữ nguyên task 200 phút ở Database. Khi `Dispatcher` tìm thấy một khoảng thời gian rảnh (Idle Window) hợp lý (ví dụ 45 phút), nó sẽ:
    1. Kiểm tra: `IdleWindow (45m)` > `MinUsefulTime (15m)`? (Bỏ qua nếu thời gian rảnh quá ngắn).
    2. Cắt ra 45 phút của task bò để gán (Assign) vào window này.
    3. Phần còn lại (155 phút) được hệ thống tự động sinh ra thành một task Remainder (Phần thừa) đưa lại vào Hàng Đợi (Queue) để lấp vào các window tiếp theo.
*   **Tại sao tốt nhất?** Code nằm hoàn toàn gọn gàng trong thuật toán `assignFillInTasks`. Không phá vỡ UI, không làm rác DB từ ban đầu, và tôn trọng tuyệt đối thời gian chuyển đổi (Context Switching) của đời thực!
*   **🌟 Lợi ích cốt lõi mở rộng (Multi-staff Collaboration):** Nhờ cơ chế đẩy task thừa (Remainder) ngược lại vào Global Queue ở bước 3, Hướng C **tự động giải quyết bài toán điều phối nhiều người cùng làm một việc**. Ví dụ: Staff A đang làm task Bò, Staff B đột nhiên rảnh. `Dispatcher` sẽ tự động nhặt phần Remainder từ Queue và tiếp tục dùng cơ chế Cắt (Split) để giao việc cho Staff B. Tiến độ được đẩy nhanh mà không cần tạo logic "Join/Group Task" phức tạp, tránh hoàn toàn xung đột dữ liệu (Data Race), và UI hiển thị cho từng cá nhân vô cùng tách bạch.

---

## 2. Phản biện về Lỗi SlotCapacity Lò Nướng (Xếp chồng Task)
**Vấn đề BA đưa ra:** Lò sức chứa 48, nhưng bị nhét đè mẻ liên tục vì `MachineRepo` chưa trừ dung lượng. -> **BA đúng về hiện tượng, nhưng sai về bản chất code hiện tại.**

**Phân tích code thực tế:** 
Thuật toán `Dispatcher` hiện tại (trong `assignSingleTask` và `pickMachine`) **HOÀN TOÀN KHÔNG QUAN TÂM** đến `MaxCapacity` hay `SlotConsumption`. Hệ thống đang dùng một biến `machineFreeAt[machineID] = time.Time` để khoá toàn bộ máy.
Lý do nhiều mẻ Nướng bị nhét đè lên nhau (theo log SC3) là do hàm `assignFillInTasks` đang bốc task đưa cho Staff nhưng lại "bỏ quên" logic gán và kiểm tra trạng thái độc quyền của Machine!

### 🔥 Hướng xử lý bắt buộc cho Slot Capacity:
Chúng ta không thể dùng biến `machineFreeAt` đơn giản nữa. Lò nướng có thể chạy song song 2 mẻ (Batch 1: 24 cái, Batch 2: 24 cái) miễn là tổng <= 48.
Chúng ta cần một cấu trúc **Machine Timeline Capacity Tracker**:
*   Mỗi máy sẽ có một Time-Series array. Ví dụ: `[07:00 - 08:00]: Used 24/48 slots`.
*   Khi Dispatcher muốn gán mẻ tiếp theo (24 slots, từ 07:15 - 08:15), nó sẽ check dọc theo timeline này xem có khoảng nào vượt quá 48 không.

---

## 3. Report Độ Phức Tạp (Complexity Report) Cho Bước Tiếp Theo
=-k,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,z¥
| Hạng mục | Mức độ | Khối lượng công việc dự kiến |
| :--- | :--- | :--- |
| **1. Mô hình hoá SlotCapacity & Timeline Tracker** | Cao (High) | Cần đập bỏ biến `machineFreeAt`. Viết lại module `MachineCapacityTracker` để track dung lượng khả dụng theo từng block thời gian (Time intervals) cho phép chạy song song các batch nhỏ trên cùng máy lớn. |
| **2. Dynamic Splitting cho Fill-in Tasks** | Trung Bình (Medium) | Cập nhật `assignFillInTasks`. Thêm thuộc tính `IsInterruptible`. Bổ sung tham số cấu hình `MinUsefulTime` (ví dụ 10 mins). Viết logic tách task động: Tạo Sub-task cho phần fit vào window, sinh Remainder task cho phần còn lại. |
| **3. Sửa lỗi gán Machine trong Fill-in** | Thấp (Low) | Fix bug hiện tại của Dispatcher đang gán Fill-in task mà bỏ qua bước check Machine Availability. |
| **4. Testing (SC3 Run)** | Thấp (Low) | Chạy lại mô phỏng, xác minh log. |

---

## 4. Vấn đề Đang Phân Tích: Lỗ Hổng Ưu Tiên Đơn Hàng (PO Prioritization)

**Hiện trạng hệ thống:**
Hiện tại, khi có nhiều Production Orders (PO) được tạo ra cùng một thời điểm (ví dụ: BUN, Patty Bò, Sốt cùng submit lúc 8:00 AM), thuật toán `Dispatcher` trong file `dispatcher.go` **chỉ sử dụng duy nhất biến `EarliestStart`** (Thời điểm sớm nhất có thể bắt đầu) để sắp xếp và chọn task. 

**Vấn đề phát sinh (Lãng phí Idle Time):**
Vì các bước đầu tiên của các PO tạo cùng lúc sẽ có giá trị `EarliestStart` bằng nhau, hàm Sort của hệ thống sẽ xếp chúng một cách ngẫu nhiên (không có Tie-breaker). Điều này dẫn đến rủi ro nghiêm trọng về mặt vận hành:
*   Nếu hệ thống vô tình bốc PO **Patty Bò** (Một task tốn rất nhiều thời gian Manual, ví dụ 300 phút, hoàn toàn không có Idle time) ra giao cho nhân viên trước.
*   Nhân viên sẽ bị khóa cứng vào task Bò này. Trong khi đó, PO **BUN** (một PO chỉ tốn ít thời gian Manual ban đầu để trộn bột, phần lớn là Idle time chờ máy ủ/nướng) lại phải nằm chờ.
*   **Hậu quả:** Hệ thống máy móc (Lò nướng, Máy ủ) bị bỏ không lãng phí trong khi nhân viên thì làm không hết việc. Việc không nhận diện được đặc thù của từng PO (Cái nào xài máy nhiều, cái nào manual nhiều) để chèn ép tiến độ đang là một điểm yếu của luồng Scheduling hiện tại.

*(Ghi chú: Vấn đề đã được record lại để theo dõi. Chưa đưa ra phương án xử lý cho đến khi có đủ thông tin).*

**Kết luận:** Để chốt lại cuộc họp ngày mai, hãy mạnh dạn gạt bỏ cả Hướng A và Hướng B của BA. Hướng đi đúng đắn nhất là **Hướng C (Dynamic Timeboxing)** kết hợp với việc **Viết lại Capacity Tracker dạng Timeline** cho máy móc. 
