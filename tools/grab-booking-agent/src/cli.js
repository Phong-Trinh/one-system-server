#!/usr/bin/env node
// Usage:
//   node src/cli.js --job jobs/example.json [--mode prefill|auto] [--url URL] [--headless]
//   node src/cli.js --login     # open the browser once so staff can log in with OTP

import fs from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";
import { fileURLToPath, pathToFileURL } from "node:url";
import { Browser } from "./browser.js";
import { BookingLedger } from "./guards.js";
import { runBookingAgent } from "./agent.js";
import { OneSystemReporter, TerminalHuman } from "./io.js";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const GRAB_URL = "https://express.grab.com/vi";

const { values: args } = parseArgs({
  options: {
    job: { type: "string" },
    mode: { type: "string", default: "prefill" },
    url: { type: "string" },
    mock: { type: "boolean", default: false },
    login: { type: "boolean", default: false },
    headless: { type: "boolean", default: false },
    record: { type: "boolean", default: false },
  },
});

const url = args.url ?? (args.mock ? pathToFileURL(path.join(ROOT, "mock/grab-express-mock.html")).href : GRAB_URL);
const profileDir = path.join(ROOT, ".profile");

if (args.login) {
  const browser = new Browser({ profileDir, runDir: path.join(ROOT, "runs", "login") });
  await browser.open(url);
  await new TerminalHuman().ask("Đăng nhập GrabExpress (SĐT + OTP) trong cửa sổ trình duyệt, xong bấm Enter.");
  await browser.close();
  process.exit(0);
}

if (!args.job) {
  console.error("missing --job <file>");
  process.exit(2);
}
if (!["prefill", "auto"].includes(args.mode)) {
  console.error("--mode must be prefill or auto");
  process.exit(2);
}

const job = JSON.parse(fs.readFileSync(args.job, "utf8"));
for (const k of ["shipment_id", "max_fee"]) if (!job[k]) throw new Error(`job.${k} is required`);
if (!job.dropoff?.match_tokens?.length) throw new Error("job.dropoff.match_tokens is required");

const runDir = path.join(ROOT, "runs", `${job.shipment_id}-${Date.now()}`);
const browser = new Browser({ profileDir, headless: args.headless, runDir, record: args.record });
try {
  await browser.open(url);
  const result = await runBookingAgent({
    job,
    browser,
    ledger: new BookingLedger(path.join(ROOT, "state")),
    human: new TerminalHuman(),
    reporter: new OneSystemReporter(),
    mode: args.mode,
  });
  console.log(`\nScreenshots${args.record ? " + video" : ""}: ${runDir}`);
  process.exitCode = result.outcome === "BOOKED" || result.outcome === "ALREADY_BOOKED" ? 0 : 1;
} finally {
  await browser.close();
}
