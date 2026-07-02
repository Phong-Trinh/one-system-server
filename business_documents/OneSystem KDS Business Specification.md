# OneSystem KDS — Business Specification
**Version:** 1.0  
**Date:** 2026-06-27  
**Status:** DRAFT — Pending approval  
**Related Technical Document:** `development/KDS Redesign.md`

---

## Overview

The **Kitchen Display System (KDS)** is the operational core of OneSystem's production module. It is the primary interface between the system and kitchen staff — the moment where planning meets execution.

The current KDS implementation functions as a **machine monitor**: it displays equipment status and batch progress for managers to observe. This leaves kitchen staff with no actionable guidance and creates a critical gap between the system's intended value and its real-world utility.

This document describes the redesigned KDS — a **Staff Support System** built on the philosophy that technology should help people perform at their best, not control or surveil them.

---

## Design Philosophy

> **OneSystem KDS is a Staff Support System — not a Staff Command System.**

This distinction is the foundation of every design decision in the module.

| Old Thinking | New Thinking |
|---|---|
| System monitors machines | System supports people |
| Control staff behavior | Help staff succeed |
| Track who made mistakes | Prevent mistakes before they happen |
| Accountability = punishment | Accountability = tool for improvement |
| Staff report to the system | System serves the staff |

**Why this matters operationally:**

Kitchen staff who feel monitored and pressured make more mistakes. Staff who feel supported and guided perform better, maintain quality, and stay longer. A support-oriented system produces better operational outcomes than a control-oriented one — not just better culture.

The KDS is designed so that **staff don't need to think about what to do next.** The system tells them. They focus entirely on execution.

---

## The Problem: What the Current KDS Does Wrong

### It answers the wrong question

The current KDS answers: *"What is each machine doing right now?"*

The right question is: *"What should each person do right now?"*

These are fundamentally different. A machine status display helps managers observe. It does nothing for the kitchen staff standing in front of it.

### It creates invisible bottlenecks

In a real kitchen shift with multiple concurrent orders and shared equipment, the operational challenge is:

> *Who does what — on which machine — at what time — for how long — and what comes next?*

The current system has no answer for this. Staff rely on verbal communication, memory, and experience to coordinate — all of which break down under peak load.

### It wastes idle time

When a machine runs automatically (e.g., a fryer timer is counting down), staff have free time. The current system does not detect this, does not suggest what to do, and does not track it. This is pure operational waste.

### It cannot prevent errors

The current system records what happened after the fact. It has no mechanism to warn staff before a mistake occurs — before something burns, before the wrong step happens, before a timer runs out unattended.

---

## The Solution: Staff-Centric KDS

### Core Concept

Each kitchen staff member has a **personal screen** (tablet at their station). The screen shows exactly **one task at a time** — the single most important thing they should be doing right now.

There is **one button**: Done.

That's it. No navigation. No choices. No cognitive load.

The system handles all scheduling, sequencing, dependency resolution, and timing behind the scenes. Staff simply execute and confirm.

### What staff see on their screen

**Active task — direct action required:**

> *"Place 5kg of potato into Fryer Station 01"*
>
> Step-by-step instructions. Machine to use. Order reference. Estimated machine runtime.
>
> → Button: **Done ✓**

**Waiting state — machine running, secondary task assigned:**

> *Fryer running — 7:23 remaining*
>
> *"Make bubble tea for Order #02"*
>
> Step-by-step instructions for the secondary task, fitted within the available window.
>
> → Button: **Done ✓**

**Return reminder — progressive, non-stressful:**

> 🟡 *(2 minutes before)* Fryer almost done — finish current task and prepare to return
>
> 🟠 *(45 seconds before)* Head back to Fryer Station
>
> 🔴 *(Now)* Go to Fryer — retrieve potato now

Note the progression: staff receive advance notice, not sudden alerts. The system is a calm, reliable guide — not a source of urgency or pressure.

### What shift leaders see

The **Manager View** is a read-only dashboard — no action buttons. It provides:

- Live status of each staff member: who is doing what, on which order, with a progress timer
- Machine status: which equipment is busy, idle, or under maintenance
- Order progress: which stations have completed their steps, which are still running, estimated completion per order

The Manager screen is for awareness and exception handling — not for controlling staff.

---

## How It Works: The Operational Model

### Station-Based Execution with Full Order Traceability

OneSystem KDS uses a station-based task model — the operational standard proven by McDonald's, Jollibee, Gong Cha, and similar high-throughput operations — enhanced to support more complex menus.

**How tasks flow:**

1. A Production Order enters the system, carrying a list of required ingredients (BOM) and the procedure to follow (SOP)
2. The system breaks the procedure into individual tasks, grouped by which equipment each step requires
3. Tasks are assigned to staff at the relevant station
4. Where possible, tasks from multiple orders are batched together on the same equipment simultaneously — maximizing utilization
5. Every task carries a reference to its source order — so if a quality issue is found, the system can immediately surface who performed which step, at what time, on which machine

**Why station-based instead of order-based?**

If each staff member were responsible for one order from start to finish, they would all compete for the same equipment at the same time. One person waits. Then another. Throughput collapses. Station-based assignment prevents this entirely.

### Idle Time Utilization

When a machine is running automatically (fryer timer, oven, etc.), the system detects the available window and assigns a secondary task from another order — but only if one fits within the time available and the machine's requirements.

- If staff need to stay nearby the machine, only nearby tasks are offered
- If staff can move freely, any compatible short task may be assigned
- If no task fits, staff receive a brief rest — clearly communicated, not left ambiguous

### Shift Continuity

When a shift changes, the incoming staff member sees a structured handover screen before starting work:

> *"Morning shift left: Fryer Station 01 running — 3:20 remaining. Order #17 in progress. Note: Chicken marinating in lower fridge, retrieve at 14:30."*

The incoming staff confirms they have reviewed the handover — then the normal task flow begins. No verbal relay required. No context lost between shifts.

### Inventory Protection on Task Cancellation

The system distinguishes between two types of task failure — and handles inventory differently for each:

| Situation | Outcome | Inventory |
|---|---|---|
| Machine breaks **before** task starts | Task CANCELLED | Reserved ingredients returned to stock automatically |
| Machine breaks **while** task is running | Task FAILED | Ingredients are not returned — they were partially consumed |

This distinction ensures inventory accuracy is maintained without manual intervention.

---

## Business Value

### For Restaurant Owners and Operators

| Value | Description |
|---|---|
| **Fewer errors, lower waste cost** | Staff are guided proactively — mistakes are caught before they happen, not discovered after food is already wasted |
| **Faster service** | Idle time is eliminated through smart task insertion; equipment is utilized to its maximum |
| **Lower training cost** | New staff can operate immediately by following on-screen instructions — no need to memorize complex procedures on day one |
| **Full traceability** | Every step of every order is logged with who, when, and which machine — quality issues can be traced to the exact point of failure |
| **Staff retention** | A system that supports rather than pressures staff produces better morale and lower turnover |

### For Managers and Shift Leaders

| Value | Description |
|---|---|
| **Live operational visibility** | See every staff member's current task and every order's progress on one screen, in real time |
| **Proactive problem detection** | System surfaces equipment conflicts and scheduling issues before they reach the kitchen floor |
| **Fair workload distribution** | Scheduling logic distributes tasks fairly across staff at the same station — no one person is overloaded |
| **Confident shift handovers** | Incoming shifts receive a structured briefing — nothing is missed in the transition |

### For the OneSystem Platform

| Value | Description |
|---|---|
| **Clear product differentiation** | Most F&B systems are machine monitors or order trackers. A genuine staff-support KDS is a meaningful product advantage |
| **Broad segment applicability** | QSR, casual dining, central kitchen, pizza, premium burger — the model is designed to serve all |
| **Operational data foundation** | Every completed task generates timing data that can feed back into procedure improvement over time |

---

## Scope and Boundaries

### What KDS manages

- Step-by-step task guidance for kitchen staff during active production
- Task assignment and sequencing across staff and equipment
- Idle time detection and secondary task insertion
- Progressive return alerts
- Shift handover protocol
- Task outcome logging with structured notes (deviation, quality issue, equipment issue)

### What KDS does not manage

- Menu configuration or pricing — handled by POS / Menu module
- Purchase Orders or supplier relationships — handled by Supply Chain module
- Staff HR records or payroll — handled separately
- Financial reporting — handled by Finance module

---

## Rollout Roadmap

The KDS is delivered in four sequential phases. Each phase produces immediate, usable value independently.

### Phase 1 — Staff Screen + Manual Assignment

*Estimated delivery: 6–8 weeks*

Kitchen staff have a personal screen. They look at it; they know what to do. Shift leaders assign tasks manually through a simple management interface.

- Staff screen: current task, step-by-step instructions, one Done button
- Shift leader interface: assign staff members to SOP steps for each active order
- Auto-progression: when a staff member taps Done, the next task appears automatically

**Immediate value:** Eliminates verbal coordination. Eliminates the "what do I do next?" problem. New staff can onboard faster.

---

### Phase 2 — Automatic Scheduling Engine

*Estimated delivery: 10–12 weeks after Phase 1*

The system assigns tasks automatically. Manual assignment is no longer required for standard operations.

- Smart scheduling based on order priority, equipment availability, and staff availability
- Equipment batching: multiple orders processed simultaneously when equipment and ingredients are compatible
- Conflict prevention: equipment conflicts are resolved before they reach the kitchen floor

**Value delivered:** Shift leaders are freed from moment-to-moment task assignment. The system achieves near-optimal throughput with minimal management overhead.

---

### Phase 3 — Idle Time Insertion

*Estimated delivery: 4–6 weeks after Phase 2*

Idle time during machine-wait steps is automatically filled with secondary tasks from other orders.

- System detects available idle windows from machine runtimes
- Assigns compatible short tasks that fit within the available window
- Progressive return alerts: staff receive reminders at 2 minutes, 45 seconds, and at completion

**Value delivered:** Eliminates unproductive waiting time during machine runs. Staff are always productively guided.

---

### Phase 4 — Manager Dashboard and Analytics

*Estimated delivery: 6–8 weeks after Phase 3*

Shift leaders and operations teams gain real-time visibility and historical performance data.

- Live Manager KDS: all staff, all machines, all orders in a single view
- Equipment effectiveness tracking (OEE) per machine
- Bottleneck detection: which stations consistently slow production
- SOP timing analysis: compare estimated step durations vs. actual outcomes — surface opportunities to improve procedures

**Value delivered:** Operations management shifts from reactive to proactive. Continuous, data-informed operational improvement becomes possible.

---

## Key Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Task assignment model | Station-Based (Hybrid) | Maximum throughput; eliminates equipment conflict |
| Staff screen unit | One task at a time | Zero cognitive load; minimal training required |
| Alert style | 3-stage progressive | Calm guidance rather than sudden urgency |
| Idle time management | Attention-aware fill-in | Respects machine requirements; does not overcommit staff |
| Task failure handling | CANCELLED vs. FAILED distinction | Accurate inventory management depending on whether consumption occurred |
| Shift continuity | Structured handover protocol | No operational context is lost between shifts |
| Performance data | Internal use only | Staff task data is used to improve the system — not displayed to staff in ways that create pressure |

---

*This document describes the business and operational specification for the OneSystem KDS module.*  
*For technical architecture, data models, and implementation details, refer to `development/KDS Redesign.md`.*
