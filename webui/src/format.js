const SOURCE_SHORT = {
  avd: "阿里云",
  chaitin: "长亭",
  oscs: "OSCS",
  ti: "奇安信",
  threatbook: "微步",
  seebug: "Seebug",
  venustech: "启明星辰",
  kev: "CISA KEV",
  struts2: "Struts2",
};

export function sourceShort(item) {
  return SOURCE_SHORT[item && item.source] || (item && item.source_name) || "-";
}

export function formatInterval(raw) {
  if (!raw) return "-";
  const text = String(raw);
  const m = text.match(/^(\d+)([smh])/);
  if (!m) return text;
  const map = { s: "秒", m: "分钟", h: "小时" };
  return `${m[1]} ${map[m[2]]}`;
}

export function formatDateTime(value) {
  if (!value) return "-";
  return String(value).replace(/:\d{2}$/, "");
}

export function isToday(value) {
  if (!value) return false;
  const raw = String(value).slice(0, 10);
  const now = new Date();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  return raw === `${now.getFullYear()}-${m}-${d}`;
}

export function sevClass(sev) {
  if (sev === "严重") return "sev-critical";
  if (sev === "高危") return "sev-high";
  if (sev === "中危") return "sev-medium";
  if (sev === "低危") return "sev-low";
  return "";
}

export function pageNumbers(current, total) {
  const pages = [];
  const windowSize = 5;
  let start = Math.max(1, current - 2);
  let end = Math.min(total, start + windowSize - 1);
  start = Math.max(1, end - windowSize + 1);
  if (start > 1) pages.push(1);
  if (start > 2) pages.push("...");
  for (let i = start; i <= end; i += 1) pages.push(i);
  if (end < total - 1) pages.push("...");
  if (end < total) pages.push(total);
  return pages;
}
