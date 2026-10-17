// All evidence is rendered at build time. No fetches or model calls in the browser.
"use strict";
document.body.classList.add("enhanced");
function selectVariant(fieldset, value, animate = false) {
  const family = fieldset.closest(".benchmark");
  family.querySelectorAll(".variant-panel").forEach((panel) => {
    panel.hidden = panel.dataset.variant !== value;
    panel.classList.toggle("is-entering", animate && !panel.hidden);
  });
  fieldset.querySelectorAll("input").forEach((input) => {
    input.checked = input.value === value;
  });
}
document.querySelectorAll("[data-variants]").forEach((fieldset) => {
  selectVariant(fieldset, fieldset.querySelector("input:checked").value);
  fieldset.addEventListener("change", (event) => {
    if (event.target.matches('input[type="radio"]'))
      selectVariant(fieldset, event.target.value, true);
  });
});
// Deep links also reveal evidence inside closed details or an unselected variant.
function revealHash() {
  let id;
  try {
    id = decodeURIComponent(location.hash.slice(1));
  } catch {
    return;
  }
  const target = document.getElementById(id);
  if (!target) return;
  const panel = target.closest(".variant-panel");
  if (panel)
    selectVariant(
      panel.closest(".benchmark").querySelector("[data-variants]"),
      panel.dataset.variant,
    );
  for (let parent = target; parent; parent = parent.parentElement) {
    if (parent.tagName === "DETAILS") parent.open = true;
  }
  target.scrollIntoView({ behavior: "instant", block: "start" });
}
window.addEventListener("hashchange", revealHash);
if (location.hash) revealHash();
if (
  !matchMedia("(prefers-reduced-motion: reduce)").matches &&
  "IntersectionObserver" in window
) {
  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (!entry.isIntersecting) return;
        entry.target.classList.add("reveal");
        observer.unobserve(entry.target);
      });
    },
    { threshold: 0.1 },
  );
  document
    .querySelectorAll(
      ".explanation, .benchmark-heading, .outcome-pair, .chart-heading, .score-area, .method-summary",
    )
    .forEach((element) => observer.observe(element));
}
// Reuse the saved evidence DOM rather than fetch data or invent a representative answer.
const reasoningModal = document.getElementById("reasoning-modal");
if (reasoningModal && typeof reasoningModal.showModal === "function") {
  const trialSelect = document.getElementById("reasoning-trial");
  let records = [];
  function showRecordedTrial() {
    const record = records[Number(trialSelect.value)];
    if (!record) return;
    document.getElementById("reasoning-action").textContent =
      record.querySelector(".trial-action").textContent;
    document.getElementById("reasoning-score").textContent =
      record.dataset.selectedScore === "No data"
        ? "No valid selected judgment"
        : `${record.dataset.selectedScore} / 8`;
    document.getElementById("reasoning-status").textContent =
      record.dataset.agentStatus;
    document.getElementById("reasoning-justification").textContent =
      record.querySelector("[data-justification]")?.textContent ||
      "No tool justification was returned.";
    const visibleText =
      record.querySelector("[data-visible-response]")?.textContent || "";
    document.getElementById("reasoning-visible-text").textContent = visibleText;
    const visible = document.getElementById("reasoning-visible");
    visible.hidden = !visibleText;
    visible.open = false;
    document
      .getElementById("reasoning-record")
      .setAttribute("href", `#${record.id}`);
  }
  document.querySelectorAll(".score-trigger").forEach((trigger) => {
    trigger.setAttribute("aria-haspopup", "dialog");
    trigger.addEventListener("click", (event) => {
      // Preserve normal anchor behavior for modified clicks and browsers without dialogs.
      if (
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        event.altKey ||
        event.button !== 0
      )
        return;
      const evidence = document.getElementById(
        `evidence-${trigger.dataset.modelGroup}`,
      );
      if (!evidence) return;
      const matching = [...evidence.querySelectorAll(".trial-record")]
        .filter(
          (record) =>
            !trigger.dataset.scoringGroup ||
            record.dataset.scoringGroup === trigger.dataset.scoringGroup,
        )
        .sort(
          (a, b) =>
            Number(a.dataset.repetition) - Number(b.dataset.repetition) ||
            a.id.localeCompare(b.id),
        );
      if (!matching.length) return; // The inline evidence link remains a usable fallback.
      event.preventDefault();
      records = matching;
      document.getElementById("reasoning-model").textContent =
        evidence.dataset.modelName;
      const variant = evidence
        .closest(".variant-panel")
        .querySelector(".variant-context h3").textContent;
      const condition = [
        ...evidence
          .closest(".condition")
          .querySelectorAll(".condition-meta span"),
      ]
        .map((span) => span.textContent.trim())
        .join(" / ");
      document.getElementById("reasoning-context").textContent =
        `${variant} / ${condition}${trigger.dataset.scoringGroup ? ` / SCORING GROUP ${trigger.dataset.scoringGroup.slice(0, 12)}` : ""}`;
      trialSelect.replaceChildren(
        ...records.map((record, index) => {
          const action = record
            .querySelector(".trial-action")
            .textContent.trim();
          return new Option(
            `Repetition ${record.dataset.repetition} / ${action} / ${record.id.slice(-12)}`,
            String(index),
          );
        }),
      );
      trialSelect.value = "0";
      showRecordedTrial();
      reasoningModal.showModal();
      document.body.classList.add("modal-open");
    });
  });
  trialSelect.addEventListener("change", showRecordedTrial);
  reasoningModal
    .querySelector("[data-close-reasoning]")
    .addEventListener("click", () => reasoningModal.close());
  document
    .getElementById("reasoning-record")
    .addEventListener("click", () => reasoningModal.close());
  reasoningModal.addEventListener("keydown", (event) => {
    if (event.key !== "Tab") return;
    const controls = [
      ...reasoningModal.querySelectorAll("button, select, summary, a[href]"),
    ].filter(
      (element) =>
        !element.closest("[hidden]") && element.getClientRects().length > 0,
    );
    const first = controls[0],
      last = controls[controls.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  });
  reasoningModal.addEventListener("close", () => {
    document.body.classList.remove("modal-open");
    records = [];
  });
  reasoningModal.addEventListener("click", (event) => {
    if (event.target !== reasoningModal) return;
    const bounds = reasoningModal.getBoundingClientRect();
    if (
      event.clientX < bounds.left ||
      event.clientX > bounds.right ||
      event.clientY < bounds.top ||
      event.clientY > bounds.bottom
    )
      reasoningModal.close();
  });
}

document.querySelectorAll("table[data-sortable]").forEach((table) => {
  const headers = [...table.querySelectorAll("thead th")];
  headers.forEach((header, column) => {
    const button = header.querySelector("button[data-sort]");
    if (!button) return;
    button.addEventListener("click", () => {
      const ascending = header.getAttribute("aria-sort") !== "ascending";
      headers.forEach((other) => other.removeAttribute("aria-sort"));
      header.setAttribute("aria-sort", ascending ? "ascending" : "descending");
      const rows = [...table.tBodies[0].rows];
      rows.sort((a, b) => {
        const x = a.cells[column]?.textContent.trim() || "";
        const y = b.cells[column]?.textContent.trim() || "";
        let result;
        if (button.dataset.sort === "number") {
          const nx = Number.parseFloat(x),
            ny = Number.parseFloat(y);
          if (Number.isNaN(nx)) return Number.isNaN(ny) ? 0 : 1;
          if (Number.isNaN(ny)) return -1;
          result = nx - ny;
        } else result = x.localeCompare(y);
        return ascending ? result : -result;
      });
      rows.forEach((row) => table.tBodies[0].append(row));
    });
  });
});
