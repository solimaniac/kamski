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
