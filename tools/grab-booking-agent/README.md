# Grab Booking Agent (PoC)

Agent AI đặt GrabExpress hộ cửa hàng qua **giao diện web của GrabExpress** (express.grab.com), thao tác trên trình duyệt thật y như nhân viên. Thiết kế: `business_documents/development/shipment_web_order_research.md` §3.7.

```
job (shipment) ──► Claude đọc trang (snapshot) ──► click / type / chọn gợi ý
                                   │
                                   ▼
                    confirm_booking ──► guards.js kiểm tra bằng code:
                       • giá hiển thị trên trang ≤ max_fee
                       • địa chỉ giao trên trang chứa đủ match_tokens
                       • shipment chưa từng được đặt (state/<id>.json)
                       • mode=prefill → luôn hỏi nhân viên
                                   │ ok
                                   ▼
                    bấm "Đặt đơn" ──► đọc mã đơn / tài xế ──► report_result ──► OneSystem
```

**Ranh giới an toàn:** model **không thể** tự bấm nút đặt đơn. Tool `click` từ chối mọi nút có nhãn kiểu "Đặt đơn / Đặt giao / Xác nhận đặt…". Chỉ `confirm_booking` mới bấm được, và chỉ sau khi các kiểm tra ở trên đã qua hoặc nhân viên đã đồng ý. `max_fee` và `match_tokens` không được gửi cho model, nên model không thể "chỉnh" theo chúng.

## Cấu trúc

| File | Vai trò |
|---|---|
| `src/agent.js` | Vòng lặp tool-use với Claude (`claude-opus-5-5`), system prompt, các tool |
| `src/browser.js` | Playwright: snapshot (element refs `e1`, `e2`…), click, type, press, screenshot |
| `src/guards.js` | Kiểm tra giá/địa chỉ, nhận diện nút đặt đơn, ledger chống đặt trùng |
| `src/io.js` | Hỏi nhân viên qua terminal; gửi kết quả về OneSystem (`ONESYSTEM_URL`) |
| `src/cli.js` | Điểm chạy |
| `mock/grab-express-mock.html` | Trang giả lập GrabExpress để test offline (có gợi ý địa chỉ gây nhầm, giá theo bảng giá Grab) |
| `jobs/example.json` | Ví dụ một job |

## Chạy

Yêu cầu: Node 20+, Playwright + Chromium (`npm i -g playwright && npx playwright install chromium` nếu máy chưa có), và biến môi trường `ANTHROPIC_API_KEY`.

```bash
cd tools/grab-booking-agent
npm install

# Test offline (không cần API key, không cần Grab): Claude được thay bằng kịch bản
npm test

# Chạy agent thật trên trang mock
npm run start:mock

# Chạy với GrabExpress thật, tại máy của quán:
npm run login                                   # lần đầu: nhân viên đăng nhập SĐT + OTP, phiên được lưu ở .profile/
node src/cli.js --job jobs/example.json         # mặc định --mode prefill: agent điền, nhân viên duyệt trước khi đặt
node src/cli.js --job jobs/example.json --mode auto   # tự đặt nếu qua hết kiểm tra
```

Ảnh chụp từng bước nằm ở `runs/<shipment_id>-<timestamp>/`, trạng thái chống đặt trùng ở `state/`.

Thêm `--record` để quay video cả phiên (file `.webm` trong cùng thư mục `runs/`). Ở chế độ này, hành động hiện tại hiện thành thanh chú thích dưới đáy trang và ô đang được thao tác có viền xanh. Video dùng để nhân viên/HQ xem lại agent đã làm gì với mỗi đơn. Thanh chú thích được loại khỏi phần đọc trang nên không ảnh hưởng đến các kiểm tra giá/địa chỉ.

**Lần chạy thật đầu tiên (Claude Opus 5.5, trang mock):** 7 lượt gọi model, khoảng 32 giây, khoảng $0.05–0.06/đơn (phần lớn token được đọc từ cache).

### Định dạng job

```jsonc
{
  "shipment_id": "SHP-DEMO-0001",        // khoá chống đặt trùng
  "order_ref": "W-1042",
  "service": "Siêu Tốc",
  "payment": "Tiền mặt - người gửi trả",
  "max_fee": 35000,                       // trần phí (VND), chỉ code thấy
  "dropoff": {
    "address": "12 Lê Lợi, Phường Bến Nghé, Quận 1, TP. Hồ Chí Minh",
    "match_tokens": ["12", "Lê Lợi", "Bến Nghé", "Quận 1"]   // địa chỉ được chọn phải chứa đủ
  },
  "recipient": { "name": "Nguyễn Văn A", "phone": "0901234567" },
  "driver_note": "Đơn đồ ăn W-1042, gọi trước khi tới."
}
```

## Kết quả

`report_result` gửi `POST $ONESYSTEM_URL/api/v1/shipments/:id/agent-events` (endpoint này **chưa có** ở backend, sẽ làm ở bước tiếp theo). Các outcome:

- `BOOKED`: đã đặt, có `grab_order_id`, `fee`, có thể có `driver_name` và `plate`.
- `FAILED`: chưa bấm đặt (không tìm được địa chỉ, nhân viên từ chối…).
- `NEEDS_HUMAN`: **đã bấm đặt** nhưng không đọc được kết quả. Nhân viên phải kiểm tra trên Grab, tuyệt đối không đặt lại.
- `ALREADY_BOOKED`: shipment này đã được đặt trước đó, agent bỏ qua.

## Giới hạn của PoC

- **Chưa chạy thử với express.grab.com thật.** Môi trường phát triển không truy cập được trang của Grab. Agent không phụ thuộc vào markup cụ thể (đọc theo nhãn và placeholder), nhưng cần một buổi chạy thử `--mode prefill` tại quán để chỉnh lại prompt.
- Chưa theo dõi trạng thái sau khi đặt (tài xế đã lấy / đã giao). Bước sau sẽ thêm lệnh `track` mở trang đơn định kỳ.
- Hỏi nhân viên qua terminal. Bước sau sẽ đẩy câu hỏi lên màn hình handoff của OneSystem.
- Ảnh chụp màn hình được giữ trong lịch sử hội thoại. Ở mức 5 đơn/ngày thì không đáng kể, nhưng cần theo dõi chi phí token trong tuần đầu.
