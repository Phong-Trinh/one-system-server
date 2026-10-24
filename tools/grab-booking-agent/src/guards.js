// Deterministic safety checks for the booking agent.
// The LLM decides *what* to click; these functions decide whether the
// irreversible step (placing the Grab order) is allowed to happen.

import fs from "node:fs";
import path from "node:path";

// Lowercase, strip Vietnamese diacritics, collapse whitespace.
export function normalizeVi(text) {
  return String(text ?? "")
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/đ/g, "d")
    .replace(/Đ/g, "D")
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .trim();
}

// Button labels that place an order. Matched on normalized text.
const BOOKING_LABELS = [
  "dat don",
  "dat giao",
  "dat ngay",
  "dat xe",
  "xac nhan dat",
  "book",
  "place order",
  "confirm booking",
];

export function isBookingControl(label) {
  const n = ` ${normalizeVi(label)} `;
  return BOOKING_LABELS.some((l) => n.includes(` ${l} `));
}

// Every token the shipment expects (house number, street, district…)
// must appear in the address text the page is showing.
export function addressMatches(shownText, matchTokens) {
  if (!Array.isArray(matchTokens) || matchTokens.length === 0) return false;
  const shown = ` ${normalizeVi(shownText)} `;
  return matchTokens.every((t) => shown.includes(` ${normalizeVi(t)} `));
}

// Extract VND amounts such as "26.509đ", "26,509 ₫", "26509 VND".
export function extractPrices(text) {
  const out = new Set();
  const re = /(\d{1,3}(?:[.,]\d{3})+|\d{4,7})\s*(?:đ|₫|vnd|vnđ)/gi;
  for (const m of String(text ?? "").matchAll(re)) {
    out.add(Number(m[1].replace(/[.,]/g, "")));
  }
  return [...out];
}

// Checks run before the booking click. Returns a list of human-readable
// problems; an empty list means the booking may proceed.
export function checkBooking({ job, quotedPrice, dropoffShown, pageText }) {
  const problems = [];
  if (!Number.isFinite(quotedPrice) || quotedPrice <= 0) {
    problems.push("quoted_price is missing or invalid");
  } else {
    if (!extractPrices(pageText).includes(quotedPrice)) {
      problems.push(`quoted_price ${quotedPrice} is not visible on the page`);
    }
    if (quotedPrice > job.max_fee) {
      problems.push(`quoted_price ${quotedPrice} exceeds max_fee ${job.max_fee}`);
    }
  }
  if (!addressMatches(dropoffShown, job.dropoff.match_tokens)) {
    problems.push(
      `dropoff "${dropoffShown}" does not contain all of [${job.dropoff.match_tokens.join(", ")}]`,
    );
  }
  if (!addressMatches(pageText, job.dropoff.match_tokens)) {
    problems.push("the expected dropoff address is not visible on the page");
  }
  return problems;
}

// One state file per shipment so a crash or re-run never books twice.
export class BookingLedger {
  constructor(dir) {
    this.dir = dir;
    fs.mkdirSync(dir, { recursive: true });
  }

  file(shipmentId) {
    return path.join(this.dir, `${shipmentId.replace(/[^\w-]/g, "_")}.json`);
  }

  get(shipmentId) {
    try {
      return JSON.parse(fs.readFileSync(this.file(shipmentId), "utf8"));
    } catch {
      return null;
    }
  }

  isBooked(shipmentId) {
    const s = this.get(shipmentId);
    return s?.status === "BOOKING_CLICKED" || s?.status === "BOOKED";
  }

  put(shipmentId, patch) {
    const next = { ...this.get(shipmentId), ...patch, updated_at: new Date().toISOString() };
    fs.writeFileSync(this.file(shipmentId), JSON.stringify(next, null, 2));
    return next;
  }
}
