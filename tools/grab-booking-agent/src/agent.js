// Claude-driven booking loop: the model reads page snapshots and picks
// actions; guards.js decides whether the final "place order" click happens.

import Anthropic from "@anthropic-ai/sdk";
import { checkBooking } from "./guards.js";

const MODEL = "claude-opus-5-5";
const MAX_STEPS = 60;

const SYSTEM = `You are a delivery-booking assistant for a Vietnamese F&B store.
You operate the GrabExpress web app (Vietnamese UI) in a real browser exactly
like a staff member would, to book one motorbike delivery for the job you are given.

How to work:
- Call snapshot to see the page. Elements are addressed by refs like "e12";
  refs change after every snapshot, so take a new snapshot after each action
  that changes the page. Use screenshot only when the text view is ambiguous.
- Pickup is the store's saved address. For the dropoff, type the address, wait
  for suggestions, and pick the suggestion that matches the job's street,
  ward and district. If no suggestion clearly matches, call ask_human.
- Fill recipient name, phone and driver note from the job. Choose the service
  named in the job. Choose the payment option named in the job.
- Never invent data. Never change the pickup address. Never apply promo codes.
- To place the order, call confirm_booking with the ref of the order button,
  the exact price shown for the selected service, and the dropoff address
  exactly as the page shows it. The click tool refuses that button by design.
- If the page asks you to log in or enter an OTP, call ask_human and wait.
- After the order is placed, take a snapshot, read the Grab order ID (and the
  driver and plate if already shown), then call report_result.
- If something blocks you and a human cannot fix it, call report_result with
  outcome FAILED and explain why. Keep replies between tool calls short.`;

const tool = (name, description, properties = {}, required = []) => ({
  name,
  description,
  input_schema: { type: "object", properties, required, additionalProperties: false },
});

const TOOLS = [
  tool("snapshot", "Text view of the current page: interactive elements with refs, then visible text."),
  tool("screenshot", "PNG screenshot of the viewport, for when the text view is ambiguous."),
  tool("click", "Click an element by ref. Refuses order-placing buttons.", { ref: { type: "string" } }, ["ref"]),
  tool(
    "type",
    "Clear a field and type text into it with real key events (triggers autocomplete).",
    { ref: { type: "string" }, text: { type: "string" } },
    ["ref", "text"],
  ),
  tool("press", "Press a key such as ArrowDown, Escape, Tab or Enter.", { key: { type: "string" } }, ["key"]),
  tool("scroll", "Scroll the page.", { direction: { type: "string", enum: ["up", "down"] } }, ["direction"]),
  tool("wait", "Wait for the page, e.g. while searching for a driver.", { seconds: { type: "integer", minimum: 1, maximum: 30 } }, ["seconds"]),
  tool(
    "confirm_booking",
    "Place the order. Runs safety checks (price cap, address match, no duplicate booking) and asks a human when required.",
    {
      ref: { type: "string", description: "Ref of the order-placing button" },
      quoted_price: { type: "integer", description: "Price in VND shown for the selected service" },
      dropoff_shown: { type: "string", description: "Dropoff address exactly as shown on the page" },
    },
    ["ref", "quoted_price", "dropoff_shown"],
  ),
  tool("ask_human", "Ask the store staff a question and wait for the answer.", { question: { type: "string" } }, ["question"]),
  tool(
    "report_result",
    "Finish the job and report the outcome to OneSystem.",
    {
      outcome: { type: "string", enum: ["BOOKED", "FAILED"] },
      grab_order_id: { type: "string" },
      fee: { type: "integer" },
      driver_name: { type: "string" },
      plate: { type: "string" },
      notes: { type: "string" },
    },
    ["outcome"],
  ),
];

export async function runBookingAgent({
  job,
  browser,
  ledger,
  human,
  reporter,
  mode,
  client = new Anthropic(),
  log = console.log,
}) {
  if (ledger.isBooked(job.shipment_id)) {
    const prev = ledger.get(job.shipment_id);
    log(`shipment ${job.shipment_id} already booked (${prev.grab_order_id ?? "order id unknown"}), skipping`);
    return { ...prev, outcome: "ALREADY_BOOKED" };
  }

  const messages = [
    {
      role: "user",
      content: `Book this delivery. Booking mode: ${mode}.\n\nJob:\n${JSON.stringify(publicJob(job), null, 2)}`,
    },
  ];

  const handlers = {
    snapshot: async () => browser.snapshot(),
    screenshot: async () => {
      const { file, base64 } = await browser.screenshot("agent");
      log(`  screenshot ${file}`);
      return [{ type: "image", source: { type: "base64", media_type: "image/png", data: base64 } }];
    },
    click: async ({ ref }) => (await browser.click(ref), "clicked"),
    type: async ({ ref, text }) => (await browser.type(ref, text), "typed"),
    press: async ({ key }) => (await browser.press(key), "pressed"),
    scroll: async ({ direction }) => (await browser.scroll(direction), "scrolled"),
    wait: async ({ seconds }) => (await browser.settle(seconds * 1000), "waited"),
    ask_human: async ({ question }) => `Staff answered: ${await human.ask(question)}`,
    confirm_booking: async (input) => confirmBooking({ input, job, browser, ledger, human, mode, log }),
  };

  for (let step = 1; step <= MAX_STEPS; step++) {
    const response = await client.beta.messages.create({
      model: MODEL,
      max_tokens: 16000,
      system: SYSTEM,
      tools: TOOLS,
      messages,
      output_config: { effort: "medium" },
      cache_control: { type: "ephemeral" },
      // On a safety decline the API re-serves the turn on a fallback model.
      betas: ["server-side-fallback-2026-07-01"],
      fallbacks: "default",
    });

    for (const b of response.content) if (b.type === "text" && b.text.trim()) log(`agent: ${b.text.trim()}`);

    if (response.stop_reason === "refusal") {
      return finish({ outcome: "FAILED", notes: `model refused: ${response.stop_details?.explanation ?? ""}` });
    }
    if (response.stop_reason === "max_tokens") {
      return finish({ outcome: "FAILED", notes: "model output hit max_tokens" });
    }

    messages.push({ role: "assistant", content: response.content });
    const calls = response.content.filter((b) => b.type === "tool_use");
    if (calls.length === 0) {
      messages.push({ role: "user", content: "Continue the job. Finish by calling report_result." });
      continue;
    }

    const results = [];
    for (const call of calls) {
      log(`→ ${call.name} ${JSON.stringify(call.input)}`);
      if (call.name === "report_result") {
        return finish(call.input);
      }
      try {
        const handler = handlers[call.name];
        if (!handler) throw new Error(`unknown tool ${call.name}`);
        const content = await handler(call.input ?? {});
        results.push({ type: "tool_result", tool_use_id: call.id, content });
      } catch (err) {
        log(`  ✗ ${err.message}`);
        results.push({ type: "tool_result", tool_use_id: call.id, is_error: true, content: err.message });
      }
    }
    messages.push({ role: "user", content: results });
  }

  return finish({ outcome: "FAILED", notes: `gave up after ${MAX_STEPS} steps` });

  async function finish(result) {
    // A click on the order button means Grab may have an order even if the
    // agent could not read its ID: escalate instead of reporting FAILED.
    const clicked = ledger.get(job.shipment_id)?.status === "BOOKING_CLICKED";
    const outcome = result.outcome === "FAILED" && clicked ? "NEEDS_HUMAN" : result.outcome;
    const final = { ...result, outcome, shipment_id: job.shipment_id };
    if (outcome === "BOOKED") ledger.put(job.shipment_id, { status: "BOOKED", ...final });
    await browser.screenshot("final").catch(() => {});
    await reporter.report(final);
    log(`result: ${JSON.stringify(final)}`);
    return final;
  }
}

async function confirmBooking({ input, job, browser, ledger, human, mode, log }) {
  if (ledger.isBooked(job.shipment_id)) {
    throw new Error("this shipment was already booked; do not book again, read the order ID instead");
  }
  const pageText = await browser.pageText();
  const problems = checkBooking({
    job,
    quotedPrice: input.quoted_price,
    dropoffShown: input.dropoff_shown,
    pageText,
  });

  let approved = problems.length === 0 && mode === "auto";
  if (!approved) {
    const why = problems.length ? `Checks failed:\n- ${problems.join("\n- ")}` : "Mode is prefill: staff approval required.";
    const answer = await human.ask(
      `${why}\nPlace order to "${input.dropoff_shown}" for ${input.quoted_price}đ? (y/n)`,
    );
    approved = /^(y|yes|c|co|có|ok)$/i.test(answer.trim());
    if (!approved) return "Staff declined. Do not place the order; call report_result with outcome FAILED.";
  }

  ledger.put(job.shipment_id, { status: "BOOKING_CLICKED", fee: input.quoted_price });
  await browser.clickBookingControl(input.ref);
  log(`  ✓ order button clicked (${input.quoted_price}đ)`);
  return "Order button clicked. Take a snapshot, read the Grab order ID, then call report_result.";
}

// Only the fields the model needs; max_fee and match tokens stay in code.
function publicJob(job) {
  return {
    shipment_id: job.shipment_id,
    order_ref: job.order_ref,
    service: job.service,
    payment: job.payment,
    dropoff_address: job.dropoff.address,
    recipient_name: job.recipient.name,
    recipient_phone: job.recipient.phone,
    driver_note: job.driver_note,
  };
}
