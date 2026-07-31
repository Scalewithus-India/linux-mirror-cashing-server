"""HTML site pages for mirror.scalewithus.com (Jinja templates + Tailwind)."""

from __future__ import annotations

import html
import logging
import re
import time
from pathlib import Path
from typing import Any

import httpx
import markdown
from fastapi import Request
from fastapi.responses import HTMLResponse
from fastapi.templating import Jinja2Templates

APP_DIR = Path(__file__).resolve().parent
DOCS_ROOT = APP_DIR / "docs"
TEMPLATES_DIR = APP_DIR / "templates"

GITHUB_REPO = "Scalewithus-India/linux-mirror-cashing-server"
GITHUB_URL = f"https://github.com/{GITHUB_REPO}"
GITHUB_API_URL = f"https://api.github.com/repos/{GITHUB_REPO}"
GITHUB_STARS_TTL = 600  # seconds

log = logging.getLogger("mirror.web")
_stars_cache: dict[str, Any] = {"count": None, "fetched_at": 0.0}

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
    {"slug": "almalinux-8", "title": "AlmaLinux 8", "file": "cloud-init/almalinux-8.md", "blurb": "cloud-init + live Alma 8 + EPEL"},
    {"slug": "almalinux", "title": "AlmaLinux 9", "file": "cloud-init/almalinux.md", "blurb": "cloud-init + live split repos + EPEL"},
    {"slug": "almalinux-10", "title": "AlmaLinux 10", "file": "cloud-init/almalinux-10.md", "blurb": "cloud-init + live Alma 10 repos + EPEL"},
    {"slug": "rocky-8", "title": "Rocky Linux 8", "file": "cloud-init/rocky-8.md", "blurb": "cloud-init + live rocky*.repo patch"},
    {"slug": "rocky", "title": "Rocky Linux 9", "file": "cloud-init/rocky.md", "blurb": "cloud-init + live rocky*.repo patch"},
    {"slug": "rocky-10", "title": "Rocky Linux 10", "file": "cloud-init/rocky-10.md", "blurb": "cloud-init + live rocky*.repo patch"},
    {"slug": "arch", "title": "Arch Linux", "file": "cloud-init/arch.md", "blurb": "cloud-init + live pacman mirrorlist"},
    {"slug": "cpanel", "title": "cPanel / WHM", "file": "cloud-init/cpanel.md", "blurb": "FastUpdate HTTPUPDATE / hosts + --cpanel-hosts"},
]

templates = Jinja2Templates(directory=str(TEMPLATES_DIR))


def _fmt_int(n: int | float) -> str:
    return f"{int(n):,}"


def _fmt_bytes(n: int | float) -> str:
    val = float(n)
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if val < 1024 or unit == "TB":
            if unit == "B":
                return f"{int(val)} {unit}"
            return f"{val:.1f} {unit}"
        val /= 1024
    return f"{val:.1f} TB"


templates.env.filters["fmt_int"] = _fmt_int
templates.env.filters["fmt_bytes"] = _fmt_bytes


def github_stars() -> int | None:
    """Return cached stargazers_count from GitHub (None if unavailable)."""
    now = time.monotonic()
    if (
        _stars_cache["count"] is not None
        and (now - float(_stars_cache["fetched_at"])) < GITHUB_STARS_TTL
    ):
        return int(_stars_cache["count"])
    # Serve stale value while refreshing fails
    stale = _stars_cache["count"]
    try:
        with httpx.Client(timeout=4.0) as client:
            resp = client.get(
                GITHUB_API_URL,
                headers={
                    "Accept": "application/vnd.github+json",
                    "User-Agent": "scalewithus-linux-mirror",
                },
            )
            resp.raise_for_status()
            count = int(resp.json().get("stargazers_count") or 0)
        _stars_cache["count"] = count
        _stars_cache["fetched_at"] = now
        return count
    except Exception as exc:  # noqa: BLE001
        log.debug("GitHub stars fetch failed: %s", exc)
        if stale is not None:
            return int(stale)
        return None


def _base_ctx(**extra: Any) -> dict[str, Any]:
    return {
        "github_url": GITHUB_URL,
        "github_repo": GITHUB_REPO,
        "github_stars": github_stars(),
        **extra,
    }


def home_page(request: Request, paths: list[str]) -> HTMLResponse:
    return templates.TemplateResponse(
        request,
        "home.html",
        _base_ctx(
            title="Home",
            active="home",
            wide=False,
            paths=paths,
            guides=GUIDES,
        ),
    )


def guides_index_page(request: Request) -> HTMLResponse:
    return templates.TemplateResponse(
        request,
        "guides_index.html",
        _base_ctx(
            title="Guides",
            active="guides",
            wide=False,
            guides=GUIDES,
        ),
    )


def metrics_page(request: Request, snap: dict[str, Any], *, bucket: str) -> HTMLResponse:
    hits = int(snap.get("hits_s3") or 0)
    misses = int(snap.get("misses_stored") or 0)
    total = hits + misses
    hit_pct = (100.0 * hits / total) if total else 0.0
    circumference = 2 * 3.1415926535 * 42
    offset = circumference * (1 - hit_pct / 100.0)

    s3_used = int(snap.get("s3_used_bytes") or 0)
    s3_objs = int(snap.get("s3_object_count") or 0)
    s3_quota = snap.get("s3_quota_bytes")
    s3_free = snap.get("s3_free_bytes")
    s3_err = snap.get("s3_usage_error")
    if s3_quota is not None and int(s3_quota) > 0:
        usage_pct = min(100.0, 100.0 * s3_used / int(s3_quota))
        s3_free_label = _fmt_bytes(int(s3_free or 0))
        s3_quota_label = _fmt_bytes(int(s3_quota))
        usage_note = f"Quota {s3_quota_label}"
    else:
        usage_pct = 0.0
        s3_free_label = "—"
        s3_quota_label = "—"
        usage_note = "Set S3_QUOTA_BYTES to show free space"
    if s3_err:
        usage_note = f"Usage refresh error: {html.escape(str(s3_err))}"

    details = [
        {"id": "m-reval", "label": "Revalidated 304", "value": _fmt_int(int(snap.get("revalidated_304") or 0)), "tone": ""},
        {"id": "m-range", "label": "Range / 206", "value": _fmt_int(int(snap.get("range_hits") or 0)), "tone": ""},
        {"id": "m-neg", "label": "Negative cache hits", "value": _fmt_int(int(snap.get("negative_hits") or 0)), "tone": ""},
        {"id": "m-nf", "label": "Not found", "value": _fmt_int(int(snap.get("not_found") or 0)), "tone": ""},
        {"id": "m-err", "label": "Upstream errors", "value": _fmt_int(int(snap.get("upstream_errors") or 0)), "tone": "danger"},
        {"id": "m-storefail", "label": "Store failed", "value": _fmt_int(int(snap.get("misses_store_failed") or 0)), "tone": "warn"},
        {"id": "m-conflict", "label": "Package conflicts", "value": _fmt_int(int(snap.get("package_conflicts") or 0)), "tone": "warn"},
        {"id": "m-neg-entries", "label": "Neg. cache entries", "value": _fmt_int(int(snap.get("negative_cache_entries") or 0)), "tone": ""},
        {"id": "m-validated", "label": "Validated meta", "value": _fmt_int(int(snap.get("validated_entries") or 0)), "tone": ""},
    ]

    return templates.TemplateResponse(
        request,
        "metrics.html",
        _base_ctx(
            title="Metrics",
            active="metrics",
            wide=True,
            bucket=bucket,
            hits=hits,
            misses=misses,
            hit_pct=hit_pct,
            circumference=f"{circumference:.2f}",
            offset=f"{offset:.2f}",
            bytes_served=int(snap.get("bytes_served") or 0),
            inflight=int(snap.get("inflight") or 0),
            inflight_peak=int(snap.get("inflight_peak") or 0),
            tmp_free=int(snap.get("tmp_free_bytes") or 0),
            s3_used=s3_used,
            s3_objs=s3_objs,
            s3_free_label=s3_free_label,
            s3_quota_label=s3_quota_label,
            usage_pct=f"{usage_pct:.1f}",
            usage_note=usage_note,
            details=details,
        ),
    )


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
        r"\]\(([\w.-]+\.md)(#[\w.-]+)?\)",
        lambda m: f"](/guides/{m.group(1).removesuffix('.md')}{m.group(2) or ''})",
        text,
    )
    rendered = markdown.markdown(
        text,
        extensions=["fenced_code", "tables", "nl2br"],
    )
    return _wrap_code_blocks(rendered)


def guide_page(request: Request, slug: str) -> HTMLResponse | None:
    guide = next((g for g in GUIDES if g["slug"] == slug), None)
    if guide is None:
        return None
    path = DOCS_ROOT / guide["file"]
    if not path.is_file():
        return None
    return templates.TemplateResponse(
        request,
        "guide.html",
        _base_ctx(
            title=guide["title"],
            active="guides",
            wide=False,
            guide=guide,
            body_html=_md_to_html(path.read_text(encoding="utf-8")),
        ),
    )
