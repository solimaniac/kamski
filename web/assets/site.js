// Progressive enhancement only: all prompts, results and links are server-rendered.
"use strict";
document.querySelectorAll("[data-navigation]").forEach(select => {
  select.addEventListener("change", () => {
    if (select.value) window.location.assign(select.value);
  });
});
document.querySelectorAll("table[data-sortable]").forEach(table => {
  const headers = [...table.querySelectorAll("thead th")];
  headers.forEach((header, column) => {
    const button = header.querySelector("button[data-sort]");
    if (!button) return;
    button.addEventListener("click", () => {
      const ascending = header.getAttribute("aria-sort") !== "ascending";
      headers.forEach(other => other.removeAttribute("aria-sort"));
      header.setAttribute("aria-sort", ascending ? "ascending" : "descending");
      const rows = [...table.tBodies[0].rows];
      rows.sort((a, b) => {
        const x = a.cells[column]?.textContent.trim() || "";
        const y = b.cells[column]?.textContent.trim() || "";
        let result;
        if (button.dataset.sort === "number") {
          const nx = Number.parseFloat(x), ny = Number.parseFloat(y);
          if (Number.isNaN(nx)) return Number.isNaN(ny) ? 0 : 1;
          if (Number.isNaN(ny)) return -1;
          result = nx - ny;
        } else result = x.localeCompare(y);
        return ascending ? result : -result;
      });
      rows.forEach(row => table.tBodies[0].append(row));
    });
  });
});
