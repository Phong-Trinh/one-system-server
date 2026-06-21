central kitchen != F, central kitchen chỉ phù hợp cho những modal cần 1 nơi để sơ chế trước để chuyển đến các điểm bán
còn F có cả nghiệp vụ thu mua, sản xuất cho đơn hàng ở bên ngoài --- done


chọn sản phẩm cost tốt nhất giữa các suplier khác nhau -> ...
cùng 1 item được báo giá ở 2 supplier khác nhau thì mình track nó ở đâu và ntn? ---done


thiếu description cho PR --done

cần thêm một bước xác nhận equipment type cho HQ khi approve một PR, HQ có thể modify các thông tin của equipment type ở bước này -- done

cho thêm một case ví dụ ở kế PR submition -- done

mark shipped -> on way  -- done

form confirm good receipt thiếu thông tin  --- done

3 way matching -> phai co ca 3 form này để HQ đối chiếu cùng 1 lúc -> xác nhận tất cả thông tin đều đúng -> tạo lệnh chuyển tiền cho supplier



#######################

choose default supplier 
tạm thời bỏ qua các trường data không sử dụng: consumption window,..
Lead Time đơn vị tính trong ERP không phải là days -> default sẽ là 24h


group các SOP step lại thành từng chặng có khả năng làm nhiều bước nhưng dễ dàng làm liên tục

khi đã có SOP của món, phải assign nhân viên đứng bếp vào từng món/equipment cụ thể -> và break các step mỗi nhân viên sẽ làm một công việc cụ thể
-> nhân viên phải chịu trách nhiệm cho sản phẩm họ làm


ví dụ như trong trường hợp có cùng lúc 5 đơn, 2 nhân viên bếp đang cùng làm việc. Đơn hàng đang được xử lý chung và machine cũng sử dụng chung.
Các bước có thể có thời gian ngắt (vd: lấy hành tây ra khỏi nồi chiên)
 Thì việc break task cho nhân viên như thế nào để không bị chồng chéo lẫn nhau

 
các SOP step của 1 ly nước ở các công đoạn thì gần như liên tục -> ko có thời gian đợi
của SB thì có idle time -> thì cần việc chèn task giữa các thời gian này để làm thêm 
-> nhưng việc chèn task sẽ ko được phép làm gián đoạn tới các step khác
-> phải đảm bảo được user vẫn phải complete được các task trước đó

-> hệ thống phải hạn chế nhận thao tác nhiều từ con người -> hệ thống phải guide user làm theo các step được định sẵn


