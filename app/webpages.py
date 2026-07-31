"""HTML site pages for mirror.scalewithus.com (home + guides + metrics)."""

from __future__ import annotations

import html
import re
from pathlib import Path
from typing import Any

import markdown
from fastapi.responses import HTMLResponse

DOCS_ROOT = Path(__file__).resolve().parent / "docs"

GUIDES: list[dict[str, str]] = [
    {
        "slug": "switch",
        "title": "Live switch (all distros)",
        "file": "cloud-init/switch.md",
        "blurb": "curl|bash installer for already-running servers",
    },
    {
        "slug": "windows",
        "title": "Windows",
        "file": "cloud-init/windows.md",
        "blurb": "Docs only — not a Windows Update mirror",
    },
    {"slug": "ubuntu", "title": "Ubuntu", "file": "cloud-init/ubuntu.md", "blurb": "cloud-init + live apt (amd64 / ports)"},
    {"slug": "alpine", "title": "Alpine Linux", "file": "cloud-init/alpine.md", "blurb": "cloud-init + live apk repositories"},
    {
        "slug": "centos-stream",
        "title": "CentOS (Stream)",
        "file": "cloud-init/centos-stream.md",
        "blurb": "cloud-init + live Stream 9/10 + EPEL",
    },
    {"slug": "debian", "title": "Debian", "file": "cloud-init/debian.md", "blurb": "cloud-init + live main/security sources"},
    {"slug": "almalinux", "title": "AlmaLinux 9", "file": "cloud-init/almalinux.md", "blurb": "cloud-init + live split repos + EPEL"},
    {"slug": "almalinux-10", "title": "AlmaLinux 10", "file": "cloud-init/almalinux-10.md", "blurb": "cloud-init + live Alma 10 repos + EPEL"},
    {"slug": "rocky", "title": "Rocky Linux 9", "file": "cloud-init/rocky.md", "blurb": "cloud-init + live rocky*.repo patch"},
    {"slug": "arch", "title": "Arch Linux", "file": "cloud-init/arch.md", "blurb": "cloud-init + live pacman mirrorlist"},
    {"slug": "cpanel", "title": "cPanel / WHM", "file": "cloud-init/cpanel.md", "blurb": "FastUpdate HTTPUPDATE / hosts + --cpanel-hosts"},
]

_CSS = """
:root {
  --bg0: #0c1210;
  --bg1: #14201a;
  --ink: #e8f0ea;
  --muted: #8fa396;
  --accent: #3dd68c;
  --accent-dim: #1f6b4a;
  --line: #24332c;
  --code-bg: #0a0f0d;
  --warn: #e6b35c;
  --danger: #e86a6a;
  --font-display: "Fraunces", "Georgia", serif;
  --font-body: "Sora", system-ui, sans-serif;
  --font-mono: "IBM Plex Mono", ui-monospace, monospace;
}
* { box-sizing: border-box; }
html { scroll-behavior: smooth; }
body {
  margin: 0;
  min-height: 100vh;
  color: var(--ink);
  font-family: var(--font-body);
  background:
    radial-gradient(1200px 600px at 10% -10%, #1a3d2e 0%, transparent 55%),
    radial-gradient(900px 500px at 100% 0%, #163028 0%, transparent 50%),
    linear-gradient(165deg, var(--bg0), var(--bg1) 45%, #0e1612);
  line-height: 1.55;
}
a { color: var(--accent); text-decoration-thickness: 1px; text-underline-offset: 3px; }
a:hover { color: #7dffc0; }
.wrap { width: min(920px, calc(100% - 2.5rem)); margin: 0 auto; }
.wrap-wide { width: min(1080px, calc(100% - 2.5rem)); margin: 0 auto; }
.site-header {
  display: flex; align-items: baseline; justify-content: space-between;
  gap: 1rem; padding: 1.4rem 0 0.5rem; border-bottom: 1px solid var(--line);
}
.brand {
  font-family: var(--font-display);
  font-weight: 600; font-size: 1.15rem; letter-spacing: -0.02em;
  color: var(--ink); text-decoration: none;
}
.brand span { color: var(--accent); }
.nav { display: flex; gap: 1.1rem; font-size: 0.92rem; }
.nav a { color: var(--muted); text-decoration: none; }
.nav a:hover, .nav a.active { color: var(--ink); }
.hero { padding: 3.2rem 0 2.2rem; animation: rise 0.7s ease both; }
.hero h1 {
  font-family: var(--font-display);
  font-size: clamp(2.1rem, 5vw, 3.4rem);
  font-weight: 550; letter-spacing: -0.03em; line-height: 1.08;
  margin: 0 0 0.85rem;
}
.hero p { margin: 0; max-width: 36rem; color: var(--muted); font-size: 1.05rem; }
.cta { display: flex; flex-wrap: wrap; gap: 0.75rem; margin-top: 1.6rem; }
.btn {
  display: inline-flex; align-items: center; gap: 0.4rem;
  padding: 0.7rem 1.1rem; border-radius: 0.35rem;
  font-weight: 600; font-size: 0.92rem; text-decoration: none;
  border: 1px solid transparent; transition: transform 0.15s ease, background 0.15s ease;
}
.btn:hover { transform: translateY(-1px); }
.btn-primary { background: var(--accent); color: #062015; }
.btn-primary:hover { background: #7dffc0; color: #062015; }
.btn-ghost { background: transparent; color: var(--ink); border-color: var(--line); }
.btn-ghost:hover { border-color: var(--accent-dim); color: var(--accent); }
.section { padding: 0.5rem 0 3rem; animation: rise 0.85s ease both; }
.section h2 {
  font-family: var(--font-display); font-size: 1.45rem; font-weight: 550;
  margin: 0 0 1rem; letter-spacing: -0.02em;
}
.paths {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
  gap: 0.55rem; margin: 0; padding: 0; list-style: none;
}
.paths li {
  font-family: var(--font-mono); font-size: 0.82rem;
  padding: 0.55rem 0.7rem; border: 1px solid var(--line); border-radius: 0.3rem;
  background: rgba(0,0,0,0.22); color: var(--muted);
}
.guide-grid {
  display: grid; grid-template-columns: repeat(auto-fill, minmax(15rem, 1fr));
  gap: 0.85rem;
}
.guide-card {
  display: block; padding: 1rem 1.05rem; border: 1px solid var(--line);
  border-radius: 0.4rem; background: rgba(8,14,11,0.55);
  text-decoration: none; color: inherit;
  transition: border-color 0.15s ease, transform 0.15s ease;
}
.guide-card:hover { border-color: var(--accent-dim); transform: translateY(-2px); }
.guide-card strong { display: block; color: var(--ink); margin-bottom: 0.25rem; }
.guide-card span { color: var(--muted); font-size: 0.88rem; }
.article { padding: 1.6rem 0 3.5rem; animation: rise 0.6s ease both; }
.article .crumb { color: var(--muted); font-size: 0.88rem; margin-bottom: 1rem; }
.article .md { max-width: 48rem; }
.article .md h1 {
  font-family: var(--font-display); font-size: clamp(1.7rem, 3.5vw, 2.3rem);
  letter-spacing: -0.02em; margin: 0 0 1rem;
}
.article .md h2 { font-family: var(--font-display); font-size: 1.25rem; margin: 1.8rem 0 0.7rem; }
.article .md h3 { font-size: 1.05rem; margin: 1.3rem 0 0.5rem; color: #cfe0d5; }
.article .md p, .article .md li { color: #c5d4cb; }
.article .md blockquote {
  margin: 1rem 0; padding: 0.7rem 1rem; border-left: 3px solid var(--warn);
  background: rgba(230,179,92,0.08); color: #ecd9b0;
}
.article .md .code-block {
  position: relative;
  margin: 1rem 0;
}
.article .md .code-block pre {
  margin: 0;
  padding-top: 2.4rem;
}
.article .md pre {
  overflow-x: auto; padding: 0.95rem 1rem; border-radius: 0.35rem;
  background: var(--code-bg); border: 1px solid var(--line);
  font-family: var(--font-mono); font-size: 0.8rem; line-height: 1.45;
}
.article .md .copy-btn {
  position: absolute; top: 0.45rem; right: 0.45rem; z-index: 2;
  appearance: none; cursor: pointer;
  font-family: var(--font-body); font-size: 0.72rem; font-weight: 600;
  letter-spacing: 0.02em;
  padding: 0.28rem 0.65rem; border-radius: 0.25rem;
  color: var(--ink); background: rgba(20, 40, 30, 0.92);
  border: 1px solid var(--line);
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}
.article .md .copy-btn:hover {
  border-color: var(--accent-dim); color: var(--accent);
}
.article .md .copy-btn.copied {
  color: #062015; background: var(--accent); border-color: var(--accent);
}
.article .md .lang-tag {
  position: absolute; top: 0.55rem; left: 0.7rem; z-index: 1;
  font-family: var(--font-mono); font-size: 0.68rem; text-transform: uppercase;
  letter-spacing: 0.06em; color: var(--muted);
}
.article .md code {
  font-family: var(--font-mono); font-size: 0.86em;
  background: rgba(255,255,255,0.05); padding: 0.1em 0.35em; border-radius: 0.2rem;
}
.article .md pre code { background: none; padding: 0; font-size: inherit; }
.article .md table { width: 100%; border-collapse: collapse; font-size: 0.92rem; margin: 1rem 0; }
.article .md th, .article .md td {
  border: 1px solid var(--line); padding: 0.45rem 0.6rem; text-align: left;
}
.article .md th { background: rgba(255,255,255,0.04); }
/* highlight.js overrides to match mirror theme */
.hljs { background: transparent !important; color: #d6e4db; }
.hljs-comment, .hljs-quote { color: #6f8578; font-style: italic; }
.hljs-keyword, .hljs-selector-tag, .hljs-literal { color: #7dffc0; }
.hljs-string, .hljs-doctag, .hljs-template-variable { color: #e6b35c; }
.hljs-number, .hljs-bullet { color: #8ecbff; }
.hljs-built_in, .hljs-type, .hljs-attr, .hljs-attribute { color: #9ad4b5; }
.hljs-title, .hljs-section { color: #cfe8d8; font-weight: 600; }
.hljs-meta, .hljs-meta .hljs-keyword { color: #a8c4b4; }
.hljs-symbol, .hljs-name { color: #f0a8a8; }
.metrics-hero { padding: 2.4rem 0 1.2rem; animation: rise 0.65s ease both; }
.metrics-hero h1 {
  font-family: var(--font-display); font-size: clamp(2rem, 4vw, 2.8rem);
  margin: 0 0 0.4rem; letter-spacing: -0.03em;
}
.metrics-hero .sub { color: var(--muted); margin: 0; }
.metrics-hero .sub code {
  font-family: var(--font-mono); font-size: 0.85em;
  color: var(--accent);
}
.metrics-layout {
  display: grid; gap: 1rem; padding-bottom: 3rem;
  animation: rise 0.8s ease both;
}
.metrics-top {
  display: grid; grid-template-columns: minmax(10rem, 14rem) 1fr;
  gap: 1rem; align-items: stretch;
}
@media (max-width: 720px) {
  .metrics-top { grid-template-columns: 1fr; }
}
.hit-ring {
  border: 1px solid var(--line); border-radius: 0.5rem;
  background: rgba(8,14,11,0.55);
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 1.15rem 1rem 1rem; gap: 0.65rem;
  min-width: 0;
}
.hit-ring .ring-wrap {
  position: relative;
  width: 8.5rem; height: 8.5rem;
  flex-shrink: 0;
}
.hit-ring svg {
  width: 100%; height: 100%;
  display: block;
  transform: rotate(-90deg);
}
.hit-ring .ring-bg { fill: none; stroke: #1c2b23; stroke-width: 10; }
.hit-ring .ring-fg {
  fill: none; stroke: var(--accent); stroke-width: 10;
  stroke-linecap: round; transition: stroke-dashoffset 0.6s ease;
}
.hit-ring .pct {
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center;
  margin: 0; padding: 0 0.4rem;
  font-family: var(--font-display);
  font-size: clamp(1.15rem, 2.4vw, 1.65rem); font-weight: 550;
  color: var(--ink); line-height: 1; letter-spacing: -0.02em;
  white-space: nowrap; overflow: visible;
  pointer-events: none;
}
.hit-ring .pct-label {
  font-size: 0.78rem; color: var(--muted); text-align: center;
  line-height: 1.2; max-width: 9rem;
}
.s3-usage {
  border: 1px solid var(--line); border-radius: 0.5rem;
  background: rgba(8,14,11,0.55); padding: 1rem 1.05rem;
}
.s3-usage h2 {
  font-family: var(--font-display); font-size: 1.2rem; font-weight: 550;
  margin: 0 0 0.85rem; letter-spacing: -0.02em;
}
.s3-usage .s3-stats {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(8.5rem, 1fr));
  gap: 0.75rem; margin-bottom: 0.85rem;
}
.s3-usage .s3-stat .label {
  color: var(--muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em;
}
.s3-usage .s3-stat .value {
  font-family: var(--font-mono); font-size: 1.2rem; font-weight: 500;
  margin-top: 0.2rem; color: var(--ink);
}
.s3-usage .s3-stat .value.accent { color: var(--accent); }
.s3-usage .usage-meta { color: var(--muted); font-size: 0.8rem; margin: 0.45rem 0 0; }
.stat-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
  gap: 0.7rem;
}
.stat {
  border: 1px solid var(--line); border-radius: 0.4rem;
  background: rgba(8,14,11,0.55); padding: 0.9rem 0.95rem;
}
.stat .label { color: var(--muted); font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
.stat .value {
  font-family: var(--font-mono); font-size: 1.35rem; font-weight: 500;
  margin-top: 0.25rem; color: var(--ink); overflow-wrap: anywhere;
}
.stat .value.accent { color: var(--accent); }
.stat .value.warn { color: var(--warn); }
.stat .value.danger { color: var(--danger); }
.detail-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.7rem;
}
.bar-wrap { margin-top: 0.55rem; }
.bar-track {
  height: 0.45rem; border-radius: 999px; background: #1c2b23; overflow: hidden;
}
.bar-fill {
  height: 100%; border-radius: 999px; background: linear-gradient(90deg, var(--accent-dim), var(--accent));
  width: 0%; transition: width 0.5s ease;
}
.metrics-note { color: var(--muted); font-size: 0.85rem; margin-top: 0.5rem; }
.site-footer {
  border-top: 1px solid var(--line); padding: 1.2rem 0 2rem;
  color: var(--muted); font-size: 0.85rem;
}
@keyframes rise {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: none; }
}
@media (max-width: 640px) {
  .site-header { flex-direction: column; align-items: flex-start; }
}
"""

_COPY_JS = r"""
(function () {
  document.querySelectorAll(".copy-btn").forEach(function (btn) {
    btn.addEventListener("click", async function () {
      var block = btn.closest(".code-block");
      var pre = block && block.querySelector("pre");
      if (!pre) return;
      var text = pre.innerText.replace(/\n$/, "");
      try {
        await navigator.clipboard.writeText(text);
      } catch (e) {
        var ta = document.createElement("textarea");
        ta.value = text;
        document.body.appendChild(ta);
        ta.select();
        document.execCommand("copy");
        document.body.removeChild(ta);
      }
      var prev = btn.textContent;
      btn.textContent = "Copied";
      btn.classList.add("copied");
      setTimeout(function () {
        btn.textContent = prev;
        btn.classList.remove("copied");
      }, 1400);
    });
  });
})();
"""

_HL_JS = r"""
(function () {
  if (window.hljs) {
    document.querySelectorAll("pre code").forEach(function (el) {
      hljs.highlightElement(el);
    });
  }
})();
"""


def _fmt_int(n: int | float) -> str:
    return f"{int(n):,}"


def _fmt_bytes(n: int) -> str:
    n = float(n)
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if n < 1024 or unit == "TB":
            if unit == "B":
                return f"{int(n)} {unit}"
            return f"{n:.1f} {unit}"
        n /= 1024
    return f"{n:.1f} TB"


def _page(
    title: str,
    body: str,
    *,
    active: str = "",
    wide: bool = False,
    extra_head: str = "",
    extra_script: str = "",
) -> HTMLResponse:
    def nav_cls(name: str) -> str:
        return ' class="active"' if active == name else ""

    wrap = "wrap-wide" if wide else "wrap"
    doc = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{html.escape(title)} · ScaleWithUs Mirror</title>
  <link rel="preconnect" href="https://fonts.googleapis.com" />
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
  <link href="https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9..144,500;9..144,600&family=IBM+Plex+Mono:wght@400;500&family=Sora:wght@400;600&display=swap" rel="stylesheet" />
  <style>{_CSS}</style>
  {extra_head}
</head>
<body>
  <div class="{wrap}">
    <header class="site-header">
      <a class="brand" href="/">ScaleWith<span>Us</span> Mirror</a>
      <nav class="nav">
        <a href="/"{nav_cls("home")}>Home</a>
        <a href="/guides"{nav_cls("guides")}>Guides</a>
        <a href="/metrics"{nav_cls("metrics")}>Metrics</a>
        <a href="/healthz">Health</a>
      </nav>
    </header>
    {body}
    <footer class="site-footer">
      On-demand S3 cache · <a href="https://mirror.scalewithus.com">mirror.scalewithus.com</a>
    </footer>
  </div>
  <script>{_COPY_JS}</script>
  {extra_script}
</body>
</html>"""
    return HTMLResponse(doc)


def home_page(paths: list[str]) -> HTMLResponse:
    path_items = "".join(f"<li>{html.escape(p)}</li>" for p in paths)
    guide_cards = "".join(
        f'<a class="guide-card" href="/guides/{html.escape(g["slug"])}">'
        f'<strong>{html.escape(g["title"])}</strong>'
        f'<span>{html.escape(g["blurb"])}</span></a>'
        for g in GUIDES
    )
    body = f"""
    <section class="hero">
      <h1>Linux package mirror,<br/>cached on demand.</h1>
      <p>Replace upstream URLs with this host. First hit fetches from the official archive and stores in S3; later hits are served from cache.</p>
      <div class="cta">
        <a class="btn btn-primary" href="/guides/switch">Switch live server</a>
        <a class="btn btn-ghost" href="/guides">Setup guides</a>
        <a class="btn btn-ghost" href="/metrics">Live metrics</a>
      </div>
    </section>
    <section class="section">
      <h2>Mirror prefixes</h2>
      <ul class="paths">{path_items}</ul>
    </section>
    <section class="section">
      <h2>Guides</h2>
      <div class="guide-grid">{guide_cards}</div>
    </section>
    """
    return _page("Home", body, active="home")


def guides_index_page() -> HTMLResponse:
    cards = "".join(
        f'<a class="guide-card" href="/guides/{html.escape(g["slug"])}">'
        f'<strong>{html.escape(g["title"])}</strong>'
        f'<span>{html.escape(g["blurb"])}</span></a>'
        for g in GUIDES
    )
    body = f"""
    <section class="hero">
      <h1>Setup guides</h1>
      <p>Cloud-init for first boot, or switch an existing server with the universal installer. Each OS guide covers both.</p>
    </section>
    <section class="section">
      <div class="guide-grid">{cards}</div>
    </section>
    """
    return _page("Guides", body, active="guides")


def metrics_page(snap: dict[str, Any], *, bucket: str) -> HTMLResponse:
    hits = int(snap.get("hits_s3") or 0)
    misses = int(snap.get("misses_stored") or 0)
    total = hits + misses
    hit_pct = (100.0 * hits / total) if total else 0.0
    circumference = 2 * 3.1415926535 * 42  # r=42
    offset = circumference * (1 - hit_pct / 100.0)

    s3_used = int(snap.get("s3_used_bytes") or 0)
    s3_objs = int(snap.get("s3_object_count") or 0)
    s3_quota = snap.get("s3_quota_bytes")
    s3_free = snap.get("s3_free_bytes")
    s3_err = snap.get("s3_usage_error")
    if s3_quota is not None and int(s3_quota) > 0:
        usage_pct = min(100.0, 100.0 * s3_used / int(s3_quota))
        free_html = _fmt_bytes(int(s3_free or 0))
        quota_html = _fmt_bytes(int(s3_quota))
        usage_note = f"Quota {quota_html}"
    else:
        usage_pct = 0.0
        free_html = "—"
        quota_html = "—"
        usage_note = "Set S3_QUOTA_BYTES to show free space"
    if s3_err:
        usage_note = f"Usage refresh error: {html.escape(str(s3_err))}"

    body = f"""
    <section class="metrics-hero">
      <h1>Live metrics</h1>
      <p class="sub">Bucket <code id="m-bucket">{html.escape(bucket)}</code> · auto-refresh every 5s · raw JSON at <a href="/api/metrics">/api/metrics</a></p>
    </section>
    <section class="metrics-layout">
      <div class="metrics-top">
        <div class="hit-ring">
          <div class="ring-wrap">
            <svg viewBox="0 0 100 100" aria-hidden="true">
              <circle class="ring-bg" cx="50" cy="50" r="42"></circle>
              <circle class="ring-fg" id="m-ring" cx="50" cy="50" r="42"
                stroke-dasharray="{circumference:.2f}"
                stroke-dashoffset="{offset:.2f}"></circle>
            </svg>
            <div class="pct" id="m-hit-pct">{hit_pct:.0f}%</div>
          </div>
          <div class="pct-label">S3 hit rate</div>
        </div>
        <div class="stat-grid">
          <div class="stat"><div class="label">S3 hits</div><div class="value accent" id="m-hits">{_fmt_int(hits)}</div></div>
          <div class="stat"><div class="label">Misses stored</div><div class="value" id="m-misses">{_fmt_int(misses)}</div></div>
          <div class="stat"><div class="label">Bytes served</div><div class="value" id="m-bytes">{_fmt_bytes(int(snap.get("bytes_served") or 0))}</div></div>
          <div class="stat"><div class="label">In-flight</div><div class="value" id="m-inflight">{_fmt_int(int(snap.get("inflight") or 0))}</div></div>
          <div class="stat"><div class="label">In-flight peak</div><div class="value" id="m-peak">{_fmt_int(int(snap.get("inflight_peak") or 0))}</div></div>
          <div class="stat"><div class="label">Tmp free</div><div class="value" id="m-tmp">{_fmt_bytes(int(snap.get("tmp_free_bytes") or 0))}</div></div>
        </div>
      </div>
      <div class="s3-usage">
        <h2>S3 storage</h2>
        <div class="s3-stats">
          <div class="s3-stat"><div class="label">Used</div><div class="value accent" id="m-s3-used">{_fmt_bytes(s3_used)}</div></div>
          <div class="s3-stat"><div class="label">Available</div><div class="value" id="m-s3-free">{free_html}</div></div>
          <div class="s3-stat"><div class="label">Quota</div><div class="value" id="m-s3-quota">{quota_html}</div></div>
          <div class="s3-stat"><div class="label">Objects</div><div class="value" id="m-s3-objs">{_fmt_int(s3_objs)}</div></div>
        </div>
        <div class="bar-wrap">
          <div class="bar-track"><div class="bar-fill" id="m-s3-bar" style="width:{usage_pct:.1f}%"></div></div>
        </div>
        <p class="usage-meta" id="m-s3-note">{usage_note}</p>
      </div>
      <div>
        <h2 style="font-family:var(--font-display);font-size:1.2rem;margin:0 0 0.75rem;">Cache mix</h2>
        <div class="bar-wrap">
          <div class="bar-track"><div class="bar-fill" id="m-bar" style="width:{hit_pct:.1f}%"></div></div>
        </div>
        <p class="metrics-note" id="m-mix-note">{_fmt_int(hits)} hits · {_fmt_int(misses)} misses stored</p>
      </div>
      <div class="detail-grid">
        <div class="stat"><div class="label">Revalidated 304</div><div class="value" id="m-reval">{_fmt_int(int(snap.get("revalidated_304") or 0))}</div></div>
        <div class="stat"><div class="label">Range / 206</div><div class="value" id="m-range">{_fmt_int(int(snap.get("range_hits") or 0))}</div></div>
        <div class="stat"><div class="label">Negative cache hits</div><div class="value" id="m-neg">{_fmt_int(int(snap.get("negative_hits") or 0))}</div></div>
        <div class="stat"><div class="label">Not found</div><div class="value" id="m-nf">{_fmt_int(int(snap.get("not_found") or 0))}</div></div>
        <div class="stat"><div class="label">Upstream errors</div><div class="value danger" id="m-err">{_fmt_int(int(snap.get("upstream_errors") or 0))}</div></div>
        <div class="stat"><div class="label">Store failed</div><div class="value warn" id="m-storefail">{_fmt_int(int(snap.get("misses_store_failed") or 0))}</div></div>
        <div class="stat"><div class="label">Package conflicts</div><div class="value warn" id="m-conflict">{_fmt_int(int(snap.get("package_conflicts") or 0))}</div></div>
        <div class="stat"><div class="label">Neg. cache entries</div><div class="value" id="m-neg-entries">{_fmt_int(int(snap.get("negative_cache_entries") or 0))}</div></div>
        <div class="stat"><div class="label">Validated meta</div><div class="value" id="m-validated">{_fmt_int(int(snap.get("validated_entries") or 0))}</div></div>
      </div>
    </section>
    """

    script = f"""
<script>
(function () {{
  var CIRC = {circumference:.4f};
  function fmtInt(n) {{ return Number(n || 0).toLocaleString("en-US"); }}
  function fmtBytes(n) {{
    n = Number(n || 0);
    var u = ["B","KB","MB","GB","TB","PB"];
    var i = 0;
    while (n >= 1024 && i < u.length - 1) {{ n /= 1024; i++; }}
    return (i === 0 ? String(Math.round(n)) : n.toFixed(1)) + " " + u[i];
  }}
  function setText(id, v) {{
    var el = document.getElementById(id);
    if (el) el.textContent = v;
  }}
  function apply(d) {{
    var hits = d.hits_s3 || 0;
    var misses = d.misses_stored || 0;
    var total = hits + misses;
    var pct = total ? (100 * hits / total) : 0;
    setText("m-hit-pct", Math.round(pct) + "%");
    setText("m-hits", fmtInt(hits));
    setText("m-misses", fmtInt(misses));
    setText("m-bytes", fmtBytes(d.bytes_served));
    setText("m-inflight", fmtInt(d.inflight));
    setText("m-peak", fmtInt(d.inflight_peak));
    setText("m-tmp", fmtBytes(d.tmp_free_bytes));
    setText("m-reval", fmtInt(d.revalidated_304));
    setText("m-range", fmtInt(d.range_hits));
    setText("m-neg", fmtInt(d.negative_hits));
    setText("m-nf", fmtInt(d.not_found));
    setText("m-err", fmtInt(d.upstream_errors));
    setText("m-storefail", fmtInt(d.misses_store_failed));
    setText("m-conflict", fmtInt(d.package_conflicts));
    setText("m-neg-entries", fmtInt(d.negative_cache_entries));
    setText("m-validated", fmtInt(d.validated_entries));
    setText("m-mix-note", fmtInt(hits) + " hits · " + fmtInt(misses) + " misses stored");
    var bar = document.getElementById("m-bar");
    if (bar) bar.style.width = pct.toFixed(1) + "%";
    var ring = document.getElementById("m-ring");
    if (ring) ring.style.strokeDashoffset = String(CIRC * (1 - pct / 100));

    var used = d.s3_used_bytes || 0;
    var objs = d.s3_object_count || 0;
    var quota = d.s3_quota_bytes;
    var free = d.s3_free_bytes;
    setText("m-s3-used", fmtBytes(used));
    setText("m-s3-objs", fmtInt(objs));
    if (quota != null && Number(quota) > 0) {{
      setText("m-s3-quota", fmtBytes(quota));
      setText("m-s3-free", fmtBytes(free == null ? 0 : free));
      var upct = Math.min(100, 100 * used / Number(quota));
      var s3bar = document.getElementById("m-s3-bar");
      if (s3bar) s3bar.style.width = upct.toFixed(1) + "%";
      setText("m-s3-note", "Quota " + fmtBytes(quota));
    }} else {{
      setText("m-s3-quota", "—");
      setText("m-s3-free", "—");
      var s3bar2 = document.getElementById("m-s3-bar");
      if (s3bar2) s3bar2.style.width = "0%";
      setText("m-s3-note", "Set S3_QUOTA_BYTES to show free space");
    }}
    if (d.s3_usage_error) {{
      setText("m-s3-note", "Usage refresh error: " + d.s3_usage_error);
    }}
  }}
  async function tick() {{
    try {{
      var r = await fetch("/api/metrics", {{ headers: {{ "Accept": "application/json" }} }});
      if (!r.ok) return;
      apply(await r.json());
    }} catch (e) {{}}
  }}
  setInterval(tick, 5000);
}})();
</script>
"""
    return _page("Metrics", body, active="metrics", wide=True, extra_script=script)


def _wrap_code_blocks(rendered: str) -> str:
    def repl(match: re.Match[str]) -> str:
        block = match.group(0)
        lang = ""
        m = re.search(r'<code[^>]*class="([^"]*)"', block)
        if m:
            for part in m.group(1).split():
                if part.startswith("language-"):
                    lang = part.removeprefix("language-")
                    break
        lang_html = (
            f'<span class="lang-tag">{html.escape(lang)}</span>' if lang else ""
        )
        return (
            '<div class="code-block">'
            f"{lang_html}"
            '<button type="button" class="copy-btn" aria-label="Copy to clipboard">Copy</button>'
            f"{block}"
            "</div>"
        )

    return re.sub(r"<pre(?:\s[^>]*)?>[\s\S]*?</pre>", repl, rendered)


def _md_to_html(text: str) -> str:
    text = re.sub(
        r"\]\(([\w.-]+\.md)\)",
        lambda m: f"](/guides/{m.group(1).removesuffix('.md')})",
        text,
    )
    rendered = markdown.markdown(
        text,
        extensions=["fenced_code", "tables", "nl2br"],
    )
    return _wrap_code_blocks(rendered)


def guide_page(slug: str) -> HTMLResponse | None:
    guide = next((g for g in GUIDES if g["slug"] == slug), None)
    if guide is None:
        return None
    path = DOCS_ROOT / guide["file"]
    if not path.is_file():
        return None
    rendered = _md_to_html(path.read_text(encoding="utf-8"))
    body = f"""
    <article class="article">
      <div class="crumb"><a href="/guides">Guides</a> / {html.escape(guide["title"])}</div>
      <div class="md">{rendered}</div>
    </article>
    """
    extra_head = (
        '<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/highlight.min.js"></script>'
        '<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/languages/yaml.min.js"></script>'
        '<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/languages/bash.min.js"></script>'
        '<script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/languages/ini.min.js"></script>'
    )
    extra_script = f"<script>{_HL_JS}</script>"
    return _page(
        guide["title"],
        body,
        active="guides",
        extra_head=extra_head,
        extra_script=extra_script,
    )
