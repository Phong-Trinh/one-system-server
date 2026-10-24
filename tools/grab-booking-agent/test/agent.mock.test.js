// End-to-end against the local mock page with a scripted stand-in for Claude,
// so the loop, the browser tools and the guards run without an API key.

import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { Browser } from "../src/browser.js";
import { BookingLedger } from "../src/guards.js";
import { runBookingAgent } from "../src/agent.js";
import { ScriptedHuman } from "../src/io.js";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const MOCK = pathToFileURL(path.join(ROOT, "mock/grab-express-mock.html")).href;
const JOB = JSON.parse(fs.readFileSync(path.join(ROOT, "jobs/example.json"), "utf8"));

// Fake client: each step is a function (lastToolResultText) => tool calls.
function scriptedClient(steps) {
  let i = 0;
  return {
    beta: {
      messages: {
        create: async ({ messages }) => {
          const last = messages.at(-1);
          const text = Array.isArray(last.content)
            ? last.content.map((c) => (typeof c.content === "string" ? c.content : "")).join("\n")
            : last.content;
          const step = steps[i++];
          if (!step) throw new Error("script exhausted");
          const calls = step(text);
          return {
            stop_reason: "tool_use",
            content: calls.map(([name, input], k) => ({ type: "tool_use", id: `t${i}_${k}`, name, input })),
          };
        },
      },
    },
  };
}

const ref = (snapshot, pattern) => {
  const line = snapshot.split("\n").find((l) => pattern.test(l));
  assert.ok(line, `no element matching ${pattern} in snapshot`);
  return line.match(/^\[(e\d+)\]/)[1];
};

async function setup(t) {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "gba-"));
  const browser = new Browser({ profileDir: path.join(tmp, "profile"), headless: true, runDir: path.join(tmp, "runs") });
  await browser.open(MOCK);
  t.after(() => browser.close());
  return { browser, ledger: new BookingLedger(path.join(tmp, "state")), reports: [] };
}

// Fill the form; `suggestion` picks which autocomplete entry to choose.
const fillSteps = (suggestion) => [
  () => [["snapshot", {}]],
  (s) => [["type", { ref: ref(s, /Nhập địa chỉ giao hàng/), text: "12 Lê Lợi" }]],
  () => [["snapshot", {}]],
  (s) => [["click", { ref: ref(s, suggestion) }]],
  () => [["snapshot", {}]],
  (s) => [
    ["type", { ref: ref(s, /Tên người nhận/), text: JOB.recipient.name }],
    ["type", { ref: ref(s, /Số điện thoại người nhận/), text: JOB.recipient.phone }],
    ["type", { ref: ref(s, /Ghi chú cho tài xế/), text: JOB.driver_note }],
    ["click", { ref: ref(s, /radio.*Siêu Tốc/) }],
  ],
  () => [["snapshot", {}]],
];

test("books the right address in auto mode and records the order id", async (t) => {
  const { browser, ledger, reports } = await setup(t);
  let bookRef;
  const client = scriptedClient([
    ...fillSteps(/option "12 Lê Lợi, Phường Bến Nghé/),
    (s) => {
      bookRef = ref(s, /button.*"Đặt đơn"/);
      return [["click", { ref: bookRef }]]; // must be refused by the guard
    },
    (r) => {
      assert.match(r, /use confirm_booking/);
      return [["confirm_booking", { ref: bookRef, quoted_price: 19636, dropoff_shown: "12 Lê Lợi, Phường Bến Nghé, Quận 1, Hồ Chí Minh" }]];
    },
    () => [["snapshot", {}]],
    (s) => {
      const id = s.match(/GE-VN-\d+/)[0];
      return [["report_result", { outcome: "BOOKED", grab_order_id: id, fee: 19636 }]];
    },
  ]);

  const result = await runBookingAgent({
    job: JOB,
    browser,
    ledger,
    human: new ScriptedHuman(),
    reporter: { report: async (r) => reports.push(r) },
    mode: "auto",
    client,
    log: process.env.DEBUG ? console.log : () => {},
  });

  assert.equal(result.outcome, "BOOKED");
  assert.match(result.grab_order_id, /^GE-VN-\d+$/);
  assert.equal(await browser.page.evaluate(() => window.__mockBookings), 1);
  assert.equal(reports.length, 1);
  assert.ok(ledger.isBooked(JOB.shipment_id));

  // Re-running the same shipment must not book again.
  const again = await runBookingAgent({
    job: JOB, browser, ledger, human: new ScriptedHuman(), reporter: { report: async () => {} },
    mode: "auto", client: scriptedClient([]), log: process.env.DEBUG ? console.log : () => {},
  });
  assert.equal(again.outcome, "ALREADY_BOOKED");
});

test("wrong suggestion is caught and staff can refuse", async (t) => {
  const { browser, ledger } = await setup(t);
  const human = new ScriptedHuman(["n"]);
  let bookRef;
  const client = scriptedClient([
    ...fillSteps(/option "12 Lê Lợi, Phường 4, Gò Vấp/),
    (s) => {
      bookRef = ref(s, /button.*"Đặt đơn"/);
      return [["confirm_booking", { ref: bookRef, quoted_price: 60874, dropoff_shown: "12 Lê Lợi, Phường 4, Gò Vấp, Hồ Chí Minh" }]];
    },
    (r) => {
      assert.match(r, /Staff declined/);
      return [["report_result", { outcome: "FAILED", notes: "address mismatch" }]];
    },
  ]);

  const result = await runBookingAgent({
    job: JOB, browser, ledger, human, reporter: { report: async () => {} },
    mode: "auto", client, log: process.env.DEBUG ? console.log : () => {},
  });

  assert.equal(result.outcome, "FAILED");
  assert.equal(human.questions.length, 1);
  assert.match(human.questions[0], /exceeds max_fee/);
  assert.match(human.questions[0], /does not contain/);
  assert.equal(await browser.page.evaluate(() => window.__mockBookings ?? 0), 0);
});

test("prefill mode always asks staff before placing the order", async (t) => {
  const { browser, ledger } = await setup(t);
  const human = new ScriptedHuman(["y"]);
  let bookRef;
  const client = scriptedClient([
    ...fillSteps(/option "12 Lê Lợi, Phường Bến Nghé/),
    (s) => {
      bookRef = ref(s, /button.*"Đặt đơn"/);
      return [["confirm_booking", { ref: bookRef, quoted_price: 19636, dropoff_shown: "12 Lê Lợi, Phường Bến Nghé, Quận 1, Hồ Chí Minh" }]];
    },
    () => [["wait", { seconds: 4 }]],
    () => [["snapshot", {}]],
    (s) => {
      assert.match(s, /59X1-123\.45/);
      return [["report_result", { outcome: "BOOKED", grab_order_id: s.match(/GE-VN-\d+/)[0], plate: "59X1-123.45" }]];
    },
  ]);

  const result = await runBookingAgent({
    job: JOB, browser, ledger, human, reporter: { report: async () => {} },
    mode: "prefill", client, log: process.env.DEBUG ? console.log : () => {},
  });

  assert.equal(result.outcome, "BOOKED");
  assert.equal(human.questions.length, 1);
  assert.match(human.questions[0], /prefill/);
});

test("quote mode stops at the price and can never place an order", async (t) => {
  const { browser, ledger } = await setup(t);
  let offered;
  let bookRef;
  const steps = [
    ...fillSteps(/option "12 Lê Lợi, Phường Bến Nghé/),
    (s) => {
      bookRef = ref(s, /button.*"Đặt đơn"/);
      return [["click", { ref: bookRef }], ["confirm_booking", { ref: bookRef, quoted_price: 19636, dropoff_shown: "12 Lê Lợi" }]];
    },
    (r) => {
      assert.match(r, /use confirm_booking/);
      assert.match(r, /quote mode: placing an order is not allowed/);
      return [["report_result", { outcome: "QUOTED", fee: 19636, notes: "Siêu Tốc 19.636đ, Nhanh 13.711đ" }]];
    },
  ];
  const client = scriptedClient(steps);
  const create = client.beta.messages.create;
  client.beta.messages.create = async (p) => ((offered ??= p.tools.map((x) => x.name)), create(p));

  const result = await runBookingAgent({
    job: JOB, browser, ledger, human: new ScriptedHuman(), reporter: { report: async () => {} },
    mode: "quote", client, log: () => {},
  });

  assert.equal(result.outcome, "QUOTED");
  assert.equal(result.fee, 19636);
  assert.ok(!offered.includes("confirm_booking"));
  assert.equal(await browser.page.evaluate(() => window.__mockBookings ?? 0), 0);
  assert.ok(!ledger.isBooked(JOB.shipment_id));
});
