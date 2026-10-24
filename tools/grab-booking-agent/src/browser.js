// Browser actions exposed to the agent. Elements are addressed by short refs
// ("e12") assigned during snapshot(), so the model never writes CSS selectors.

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { isBookingControl } from "./guards.js";

const require = createRequire(import.meta.url);
// Use the Playwright install that ships with the machine (see README).
const { chromium } = require("playwright");

const INTERACTIVE = [
  "a[href]",
  "button",
  "input:not([type=hidden])",
  "textarea",
  "select",
  "[role=button]",
  "[role=option]",
  "[role=tab]",
  "[role=radio]",
  "[role=checkbox]",
  "[role=link]",
  "[role=menuitem]",
  "[contenteditable=true]",
].join(",");

const MAX_TEXT = 4000;

export class Browser {
  constructor({ profileDir, headless = false, runDir }) {
    this.profileDir = profileDir;
    this.headless = headless;
    this.runDir = runDir;
    this.shot = 0;
  }

  async open(url) {
    this.context = await chromium.launchPersistentContext(this.profileDir, {
      headless: this.headless,
      locale: "vi-VN",
      viewport: { width: 1280, height: 860 },
    });
    this.page = this.context.pages()[0] ?? (await this.context.newPage());
    await this.page.goto(url, { waitUntil: "domcontentloaded" });
  }

  async close() {
    await this.context?.close();
  }

  // Text view of the page: interactive elements with refs + visible text.
  async snapshot() {
    await this.page.waitForLoadState("domcontentloaded").catch(() => {});
    const { elements, text } = await this.page.evaluate(
      ({ selector, maxText }) => {
        const visible = (el) => {
          const r = el.getBoundingClientRect();
          const s = getComputedStyle(el);
          return r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none";
        };
        document.querySelectorAll("[data-agent-ref]").forEach((el) => el.removeAttribute("data-agent-ref"));
        const elements = [];
        let n = 0;
        for (const el of document.querySelectorAll(selector)) {
          if (!visible(el)) continue;
          const ref = `e${++n}`;
          el.setAttribute("data-agent-ref", ref);
          const role = el.getAttribute("role") || el.tagName.toLowerCase();
          const type = el.getAttribute("type");
          const label =
            el.getAttribute("aria-label") ||
            (el.labels && el.labels[0]?.innerText) ||
            el.innerText ||
            el.getAttribute("title") ||
            "";
          const parts = [`[${ref}] ${role}${type ? `(${type})` : ""} "${label.trim().replace(/\s+/g, " ").slice(0, 120)}"`];
          const placeholder = el.getAttribute("placeholder");
          if (placeholder) parts.push(`placeholder="${placeholder.slice(0, 80)}"`);
          if ("value" in el && el.value && type !== "password") parts.push(`value="${String(el.value).slice(0, 120)}"`);
          if (el.checked) parts.push("checked");
          if (el.disabled) parts.push("disabled");
          if (el.getAttribute("aria-selected") === "true") parts.push("selected");
          elements.push(parts.join(" "));
        }
        return { elements, text: document.body.innerText.replace(/\n{3,}/g, "\n\n").slice(0, maxText) };
      },
      { selector: INTERACTIVE, maxText: MAX_TEXT },
    );
    return [
      `URL: ${this.page.url()}`,
      `TITLE: ${await this.page.title()}`,
      "",
      "INTERACTIVE ELEMENTS:",
      ...elements,
      "",
      "VISIBLE TEXT:",
      text,
    ].join("\n");
  }

  // Visible text plus the values of visible form fields: a chosen address
  // usually lives in an <input> value, which innerText does not include.
  async pageText() {
    return this.page.evaluate(() => {
      const values = [...document.querySelectorAll("input:not([type=hidden]):not([type=password]),textarea,select")]
        .filter((el) => el.getBoundingClientRect().width > 0 && el.value)
        .map((el) => (el.tagName === "SELECT" ? el.selectedOptions[0]?.text : el.value));
      return `${document.body.innerText}\n${values.join("\n")}`;
    });
  }

  locator(ref) {
    if (!/^e\d+$/.test(ref)) throw new Error(`invalid ref "${ref}"`);
    return this.page.locator(`[data-agent-ref="${ref}"]`);
  }

  async labelOf(ref) {
    return this.locator(ref).evaluate((el) =>
      [el.getAttribute("aria-label"), el.innerText, el.value, el.getAttribute("title")].filter(Boolean).join(" "),
    );
  }

  // Regular click. Refuses the order-placing button: that goes through
  // clickBookingControl() after the guards in agent.js have passed.
  async click(ref) {
    if (isBookingControl(await this.labelOf(ref))) {
      throw new Error("this control places the order; use confirm_booking instead of click");
    }
    await this.locator(ref).click({ timeout: 5000 });
    await this.settle();
  }

  async clickBookingControl(ref) {
    if (!isBookingControl(await this.labelOf(ref))) {
      throw new Error("ref is not an order-placing button");
    }
    await this.locator(ref).click({ timeout: 5000 });
    await this.settle(2000);
  }

  async type(ref, text, { clear = true } = {}) {
    const loc = this.locator(ref);
    if (clear) await loc.fill("", { timeout: 5000 });
    // pressSequentially fires key events, so autocomplete widgets react.
    await loc.pressSequentially(text, { delay: 40, timeout: 15000 });
    await this.settle();
  }

  async press(key) {
    if (key === "Enter") {
      const inForm = await this.page.evaluate(() => !!document.activeElement?.closest("form"));
      if (inForm) throw new Error("Enter inside a form may submit the order; click the intended element instead");
    }
    await this.page.keyboard.press(key);
    await this.settle();
  }

  async scroll(direction) {
    await this.page.mouse.wheel(0, direction === "up" ? -600 : 600);
    await this.settle(300);
  }

  async screenshot(name = "step") {
    const file = path.join(this.runDir, `${String(++this.shot).padStart(3, "0")}-${name}.png`);
    fs.mkdirSync(this.runDir, { recursive: true });
    const buf = await this.page.screenshot({ path: file });
    return { file, base64: buf.toString("base64") };
  }

  async settle(ms = 600) {
    await this.page.waitForLoadState("domcontentloaded").catch(() => {});
    await this.page.waitForTimeout(ms);
  }
}
