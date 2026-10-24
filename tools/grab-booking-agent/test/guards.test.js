import test from "node:test";
import assert from "node:assert/strict";
import { addressMatches, checkBooking, extractPrices, isBookingControl, normalizeVi } from "../src/guards.js";

const job = {
  max_fee: 35000,
  dropoff: { match_tokens: ["12", "Lê Lợi", "Bến Nghé", "Quận 1"] },
};

test("normalizeVi strips diacritics and đ", () => {
  assert.equal(normalizeVi("Đặt đơn  Quận 1"), "dat don quan 1");
});

test("isBookingControl recognises order buttons only", () => {
  assert.ok(isBookingControl("Đặt đơn"));
  assert.ok(isBookingControl("Xác nhận đặt giao"));
  assert.ok(!isBookingControl("Đăng nhập"));
  assert.ok(!isBookingControl("Siêu Tốc"));
});

test("addressMatches needs every token as a whole word", () => {
  assert.ok(addressMatches("12 Lê Lợi, Phường Bến Nghé, Quận 1, HCM", job.dropoff.match_tokens));
  assert.ok(!addressMatches("120 Lê Lợi, Phường Bến Thành, Quận 1", job.dropoff.match_tokens));
  assert.ok(!addressMatches("12 Lê Lợi, Phường 4, Gò Vấp", job.dropoff.match_tokens));
  assert.ok(!addressMatches("anything", []));
});

test("extractPrices reads VND formats", () => {
  assert.deepEqual(extractPrices("Siêu Tốc — 19.636đ, Nhanh 13,711 ₫, 26509 VND"), [19636, 13711, 26509]);
});

test("checkBooking passes a correct booking", () => {
  const pageText = "Điểm giao: 12 Lê Lợi, Phường Bến Nghé, Quận 1\nSiêu Tốc — 19.636đ";
  assert.deepEqual(
    checkBooking({ job, quotedPrice: 19636, dropoffShown: "12 Lê Lợi, Phường Bến Nghé, Quận 1", pageText }),
    [],
  );
});

test("checkBooking rejects price over cap, invisible price and wrong address", () => {
  const pageText = "12 Lê Lợi, Phường 4, Gò Vấp\nSiêu Tốc — 60.874đ";
  const problems = checkBooking({ job, quotedPrice: 19636, dropoffShown: "12 Lê Lợi, Phường 4, Gò Vấp", pageText });
  assert.ok(problems.some((p) => p.includes("not visible")));
  assert.ok(problems.some((p) => p.includes("does not contain")));
  const over = checkBooking({
    job,
    quotedPrice: 60874,
    dropoffShown: "12 Lê Lợi, Phường Bến Nghé, Quận 1",
    pageText: "12 Lê Lợi, Phường Bến Nghé, Quận 1 — 60.874đ",
  });
  assert.ok(over.some((p) => p.includes("exceeds max_fee")));
});
