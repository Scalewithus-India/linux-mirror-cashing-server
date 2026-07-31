"""HTML site pages for mirror.scalewithus.com (home + guides)."""

from __future__ import annotations

import html
import re
from pathlib import Path

import markdown
from fastapi.responses import HTMLResponse

DOCS_ROOT = Path(__file__).resolve().parent / "docs"

GUIDES: list[dict[str, str]] = [
    {"slug": "ubuntu", "title": "Ubuntu", "file": "cloud-init/ubuntu.md", "blurb": "amd64 and arm64 / ports apt mirrors"},
    {"slug": "debian", "title": "Debian", "file": "cloud-init/debian.md", "blurb": "main + security sources"},
    {"slug": "almalinux", "title": "AlmaLinux 9", "file": "cloud-init/almalinux.md", "blurb": "split repo write_files + EPEL"},
    {"slug": "almalinux-10", "title": "AlmaLinux 10", "file": "cloud-init/almalinux-10.md", "blurb": "Alma 10 split repos + EPEL"},
    {"slug": "rocky", "title": "Rocky Linux 9", "file": "cloud-init/rocky.md", "blurb": "patch rocky*.repo baseurls"},
    {"slug": "centos-stream", "title": "CentOS Stream", "file": "cloud-init/centos-stream.md", "blurb": "Stream 9/10 + EPEL"},
    {"slug": "arch", "title": "Arch Linux", "file": "cloud-init/arch.md", "blurb": "pacman mirrorlist"},
    {"slug": "cpanel", "title": "cPanel / WHM", "file": "cloud-init/cpanel.md", "blurb": "FastUpdate HTTPUPDATE / hosts override"},
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
  position: absolute; top: 0.45rem; right: 0.45rem; z-index: 1;
  appearance: none; cursor: pointer;
  font-family: var(--font-body); font-size: 0.72rem; font-weight: 600;
  letter-spacing: 0.02em;
  padding: 0.28rem 0.65rem; border-radius: 0.25rem;
  color: var(--ink); background: rgba(20, 40, 30, 0.9);
  border: 1px solid var(--line);
  transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
}
.article .md .copy-btn:hover {
  border-color: var(--accent-dim); color: var(--accent);
}
.article .md .copy-btn.copied {
  color: #062015; background: var(--accent); border-color: var(--accent);
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


def _page(title: str, body: str, *, active: str = "") -> HTMLResponse:
    def nav_cls(name: str) -> str:
        return ' class="active"' if active == name else ""

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
</head>
<body>
  <div class="wrap">
    <header class="site-header">
      <a class="brand" href="/">ScaleWith<span>Us</span> Mirror</a>
      <nav class="nav">
        <a href="/"{nav_cls("home")}>Home</a>
        <a href="/guides"{nav_cls("guides")}>Guides</a>
        <a href="/metrics">Metrics</a>
        <a href="/healthz">Health</a>
      </nav>
    </header>
    {body}
    <footer class="site-footer">
      On-demand S3 cache · <a href="https://mirror.scalewithus.com">mirror.scalewithus.com</a>
    </footer>
  </div>
  <script>
  (function () {{
    document.querySelectorAll(".copy-btn").forEach(function (btn) {{
      btn.addEventListener("click", async function () {{
        var block = btn.closest(".code-block");
        var pre = block && block.querySelector("pre");
        if (!pre) return;
        var text = pre.innerText.replace(/\\n$/, "");
        try {{
          await navigator.clipboard.writeText(text);
        }} catch (e) {{
          var ta = document.createElement("textarea");
          ta.value = text;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand("copy");
          document.body.removeChild(ta);
        }}
        var prev = btn.textContent;
        btn.textContent = "Copied";
        btn.classList.add("copied");
        setTimeout(function () {{
          btn.textContent = prev;
          btn.classList.remove("copied");
        }}, 1400);
      }});
    }});
  }})();
  </script>
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
        <a class="btn btn-primary" href="/guides">Setup guides</a>
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
      <p>Cloud-init and client configs so apt, dnf, pacman, or cPanel pull through this mirror.</p>
    </section>
    <section class="section">
      <div class="guide-grid">{cards}</div>
    </section>
    """
    return _page("Guides", body, active="guides")


def _wrap_code_blocks(rendered: str) -> str:
    def repl(match: re.Match[str]) -> str:
        return (
            '<div class="code-block">'
            '<button type="button" class="copy-btn" aria-label="Copy to clipboard">Copy</button>'
            f"{match.group(0)}"
            "</div>"
        )

    return re.sub(r"<pre(?:\s[^>]*)?>[\s\S]*?</pre>", repl, rendered)


def _md_to_html(text: str) -> str:
    # Soften absolute repo links that point at sibling md files
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
    return _page(guide["title"], body, active="guides")
