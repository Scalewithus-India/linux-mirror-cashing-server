"""On-demand Linux package mirror: S3 cache with upstream fetch on miss."""

from __future__ import annotations

import asyncio
import logging
import os
import re
import shutil
import tempfile
import time
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from datetime import datetime, timezone
from email.utils import format_datetime, parsedate_to_datetime
from typing import Any, AsyncIterator
from urllib.parse import quote, unquote

import aioboto3
import httpx
from botocore.config import Config
from botocore.exceptions import ClientError
from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse, StreamingResponse

logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "INFO").upper(),
    format="%(asctime)s %(levelname)s %(message)s",
)
log = logging.getLogger("mirror")

AWS_ACCESS_KEY_ID = os.environ["AWS_ACCESS_KEY_ID"]
AWS_SECRET_ACCESS_KEY = os.environ["AWS_SECRET_ACCESS_KEY"]
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "https://s3.scalewithus.com").rstrip("/")
S3_BUCKET = os.getenv("S3_BUCKET", "linux-mirrors")
S3_REGION = os.getenv("S3_REGION", "us-east-1")
S3_ADDRESSING = os.getenv("S3_ADDRESSING", "path")  # path | virtual
UPSTREAM_TIMEOUT = float(os.getenv("UPSTREAM_TIMEOUT", "120"))
METADATA_CACHE_SECONDS = int(os.getenv("METADATA_CACHE_SECONDS", "21600"))  # 6h
PACKAGE_CACHE_SECONDS = int(os.getenv("PACKAGE_CACHE_SECONDS", "15552000"))  # 6 months
NEGATIVE_CACHE_SECONDS = int(os.getenv("NEGATIVE_CACHE_SECONDS", "60"))
# Max bytes to spool for a single object (default 4 GiB)
MAX_SPOOL_BYTES = int(os.getenv("MAX_SPOOL_BYTES", str(4 * 1024**3)))
# Require this much free space on tmp before starting a spool
MIN_TMP_FREE_BYTES = int(os.getenv("MIN_TMP_FREE_BYTES", str(512 * 1024**2)))

# Longest-prefix match: public path -> upstream base URL (must end with /)
# Longer prefixes listed first; also sorted at import for safety.
_UPSTREAMS_RAW: list[tuple[str, str]] = [
    ("/debian-security/", "https://security.debian.org/debian-security/"),
    ("/ubuntu-security/", "https://security.ubuntu.com/ubuntu/"),
    ("/ubuntu-ports/", "https://ports.ubuntu.com/ubuntu-ports/"),
    ("/ubuntu/", "https://archive.ubuntu.com/ubuntu/"),
    ("/debian/", "https://deb.debian.org/debian/"),
    ("/almalinux/", "https://repo.almalinux.org/almalinux/"),
    ("/rocky/", "https://dl.rockylinux.org/pub/rocky/"),
    ("/centos-stream/", "https://mirror.stream.centos.org/"),
    ("/archlinux/", "https://geo.mirror.pkgbuild.com/"),
    ("/epel/", "https://download.fedoraproject.org/pub/epel/"),
    # cPanel FastUpdate / httpupdate tree (also via httpupdate.scalewithus.com → /cpanel/)
    ("/cpanel/", "https://httpupdate.cpanel.net/"),
]
UPSTREAMS: list[tuple[str, str]] = sorted(_UPSTREAMS_RAW, key=lambda x: len(x[0]), reverse=True)

METADATA_NAME_RE = re.compile(
    r"(InRelease|Release(\.gpg)?|Packages(\.(gz|xz|bz2|zst))?|"
    r"Sources(\.(gz|xz|bz2|zst))?|Contents-.*|repomd\.xml(\.asc)?|"
    r"lastupdate|lastsync|mirrorlist|md5sums|sha256sums|sha512sums|"
    r"TIERS\.json(\.asc)?|.*\.digest\.list(\.bz2)?|"
    r"index\.html|README)$",
    re.IGNORECASE,
)

RANGE_RE = re.compile(r"bytes=(\d*)-(\d*)\Z")

http_client: httpx.AsyncClient | None = None
s3_session: aioboto3.Session | None = None

# Per-key singleflight: waiters share one upstream fetch + S3 store
_inflight: dict[str, asyncio.Future[None]] = {}
# Metadata keys recently revalidated against upstream (304)
_validated_at: dict[str, float] = {}
# Negative cache: key -> monotonic expiry
_negative_until: dict[str, float] = {}


@dataclass
class Metrics:
    hits_s3: int = 0
    misses_stored: int = 0
    misses_store_failed: int = 0
    upstream_errors: int = 0
    not_found: int = 0
    negative_hits: int = 0
    revalidated_304: int = 0
    range_hits: int = 0
    package_conflicts: int = 0
    bytes_served: int = 0
    inflight_peak: int = 0
    lock: asyncio.Lock = field(default_factory=asyncio.Lock)

    async def incr(self, name: str, n: int = 1) -> None:
        async with self.lock:
            setattr(self, name, getattr(self, name) + n)
            cur = len(_inflight)
            if cur > self.inflight_peak:
                self.inflight_peak = cur

    def snapshot(self) -> dict[str, Any]:
        return {
            "hits_s3": self.hits_s3,
            "misses_stored": self.misses_stored,
            "misses_store_failed": self.misses_store_failed,
            "upstream_errors": self.upstream_errors,
            "not_found": self.not_found,
            "negative_hits": self.negative_hits,
            "revalidated_304": self.revalidated_304,
            "range_hits": self.range_hits,
            "package_conflicts": self.package_conflicts,
            "bytes_served": self.bytes_served,
            "inflight": len(_inflight),
            "inflight_peak": self.inflight_peak,
            "negative_cache_entries": len(_negative_until),
            "validated_entries": len(_validated_at),
        }


metrics = Metrics()


def resolve_upstream(path: str) -> tuple[str, str, str] | None:
    """Return (prefix, relative_key, upstream_url) or None."""
    if not path.startswith("/"):
        path = "/" + path
    while "//" in path:
        path = path.replace("//", "/")
    for prefix, base in UPSTREAMS:
        if path == prefix.rstrip("/") or path.startswith(prefix):
            rel = path[len(prefix) :] if path.startswith(prefix) else ""
            rel = unquote(rel)
            upstream = base + quote(rel, safe="/:@!$&'()*+,;=-._~")
            key = path.lstrip("/")
            if path.endswith("/") or path == prefix.rstrip("/"):
                if not key.endswith("/"):
                    key = key + "/"
            return prefix, key, upstream
    return None


def is_metadata_key(key: str) -> bool:
    name = key.rstrip("/").rsplit("/", 1)[-1]
    return bool(
        METADATA_NAME_RE.search(name)
        or name.endswith(".xml")
        or name.endswith(".db")
        or name.endswith(".db.sig")
        or name.endswith(".files")
        or name.endswith(".files.sig")
    )


def cache_control_for(key: str) -> str:
    if is_metadata_key(key):
        return f"public, max-age={METADATA_CACHE_SECONDS}"
    return f"public, max-age={PACKAGE_CACHE_SECONDS}, immutable"


def s3_object_is_fresh(head: dict, key: str) -> bool:
    """Packages are kept indefinitely; metadata expires after METADATA_CACHE_SECONDS."""
    if not is_metadata_key(key):
        return True
    now = time.monotonic()
    validated = _validated_at.get(key)
    if validated is not None and (now - validated) <= METADATA_CACHE_SECONDS:
        return True
    modified = head.get("LastModified")
    if modified is None:
        return False
    if modified.tzinfo is None:
        modified = modified.replace(tzinfo=timezone.utc)
    age = (datetime.now(timezone.utc) - modified).total_seconds()
    return age <= METADATA_CACHE_SECONDS


def s3_client_kwargs() -> dict:
    addressing = "path" if S3_ADDRESSING == "path" else "virtual"
    return {
        "service_name": "s3",
        "endpoint_url": S3_ENDPOINT,
        "aws_access_key_id": AWS_ACCESS_KEY_ID,
        "aws_secret_access_key": AWS_SECRET_ACCESS_KEY,
        "region_name": S3_REGION,
        "config": Config(s3={"addressing_style": addressing}, signature_version="s3v4"),
    }


def parse_bytes_range(range_header: str | None, size: int) -> tuple[int, int] | None:
    """Return inclusive (start, end) or None for full-body. Raises ValueError if unsatisfiable."""
    if not range_header:
        return None
    header = range_header.strip()
    if "," in header:
        # Multi-range not supported — ignore and serve full body
        return None
    m = RANGE_RE.match(header)
    if not m:
        raise ValueError("invalid range")
    start_s, end_s = m.group(1), m.group(2)
    if start_s == "" and end_s == "":
        raise ValueError("invalid range")
    if start_s == "":
        # suffix: bytes=-N
        length = int(end_s)
        if length <= 0:
            raise ValueError("invalid range")
        if size == 0:
            raise ValueError("unsatisfiable")
        start = max(0, size - length)
        end = size - 1
    else:
        start = int(start_s)
        end = int(end_s) if end_s else size - 1
        if size == 0:
            raise ValueError("unsatisfiable")
        if start >= size:
            raise ValueError("unsatisfiable")
        end = min(end, size - 1)
        if start > end:
            raise ValueError("invalid range")
    return start, end


def tmp_free_bytes() -> int:
    return shutil.disk_usage(tempfile.gettempdir()).free


def ensure_spool_capacity(expected: int | None = None) -> None:
    free = tmp_free_bytes()
    need = MIN_TMP_FREE_BYTES
    if expected is not None:
        need = max(need, expected + MIN_TMP_FREE_BYTES)
    if free < need:
        raise OSError(
            f"insufficient tmp space: free={free} need>={need} tmp={tempfile.gettempdir()}"
        )


def negative_cached(key: str) -> bool:
    exp = _negative_until.get(key)
    if exp is None:
        return False
    if time.monotonic() < exp:
        return True
    _negative_until.pop(key, None)
    return False


def remember_negative(key: str) -> None:
    _negative_until[key] = time.monotonic() + NEGATIVE_CACHE_SECONDS
    # Opportunistic prune
    if len(_negative_until) > 10_000:
        now = time.monotonic()
        dead = [k for k, e in _negative_until.items() if e <= now]
        for k in dead:
            _negative_until.pop(k, None)


def clear_negative(key: str) -> None:
    _negative_until.pop(key, None)


@asynccontextmanager
async def lifespan(_app: FastAPI):
    global http_client, s3_session
    http_client = httpx.AsyncClient(
        timeout=httpx.Timeout(UPSTREAM_TIMEOUT, connect=30.0),
        follow_redirects=True,
        headers={"User-Agent": "scalewithus-linux-mirror/1.0"},
    )
    s3_session = aioboto3.Session()
    try:
        async with s3_session.client(**s3_client_kwargs()) as s3:
            await s3.head_bucket(Bucket=S3_BUCKET)
        log.info("S3 OK endpoint=%s bucket=%s", S3_ENDPOINT, S3_BUCKET)
    except Exception as exc:  # noqa: BLE001
        log.warning("S3 head_bucket failed (will still try per-object): %s", exc)
    yield
    await http_client.aclose()
    http_client = None
    s3_session = None


app = FastAPI(title="ScaleWithUs Linux Mirror", lifespan=lifespan)


@app.get("/healthz")
async def healthz():
    return {
        "status": "ok",
        "bucket": S3_BUCKET,
        "tmp_free_bytes": tmp_free_bytes(),
        "inflight": len(_inflight),
    }


@app.get("/metrics")
async def metrics_endpoint():
    snap = metrics.snapshot()
    snap["tmp_free_bytes"] = tmp_free_bytes()
    return JSONResponse(snap)


@app.get("/")
async def root():
    return {
        "mirror": "mirror.scalewithus.com",
        "paths": [p for p, _ in UPSTREAMS],
        "cache": "s3-on-demand",
        "metrics": "/metrics",
    }


async def stream_s3(
    key: str,
    head: dict,
    range_header: str | None = None,
) -> Response:
    assert s3_session is not None
    size = int(head.get("ContentLength") or 0)
    ctype = head.get("ContentType") or "application/octet-stream"

    try:
        byte_range = parse_bytes_range(range_header, size)
    except ValueError as exc:
        if "unsatisfiable" in str(exc) or size == 0 and range_header:
            return Response(
                status_code=416,
                headers={
                    "Content-Range": f"bytes */{size}",
                    "X-Cache": "HIT-S3",
                },
            )
        byte_range = None

    if byte_range is None:
        async def body() -> AsyncIterator[bytes]:
            async with s3_session.client(**s3_client_kwargs()) as s3:
                obj = await s3.get_object(Bucket=S3_BUCKET, Key=key)
                stream = obj["Body"]
                while True:
                    chunk = await stream.read(1024 * 1024)
                    if not chunk:
                        break
                    yield chunk

        headers = {
            "Cache-Control": cache_control_for(key),
            "X-Cache": "HIT-S3",
            "Accept-Ranges": "bytes",
        }
        if size:
            headers["Content-Length"] = str(size)
        await metrics.incr("hits_s3")
        await metrics.incr("bytes_served", size)
        return StreamingResponse(body(), media_type=ctype, headers=headers)

    start, end = byte_range
    length = end - start + 1
    range_spec = f"bytes={start}-{end}"

    async def ranged_body() -> AsyncIterator[bytes]:
        async with s3_session.client(**s3_client_kwargs()) as s3:
            obj = await s3.get_object(Bucket=S3_BUCKET, Key=key, Range=range_spec)
            stream = obj["Body"]
            while True:
                chunk = await stream.read(1024 * 1024)
                if not chunk:
                    break
                yield chunk

    headers = {
        "Cache-Control": cache_control_for(key),
        "X-Cache": "HIT-S3",
        "Accept-Ranges": "bytes",
        "Content-Range": f"bytes {start}-{end}/{size}",
        "Content-Length": str(length),
    }
    await metrics.incr("hits_s3")
    await metrics.incr("range_hits")
    await metrics.incr("bytes_served", length)
    return StreamingResponse(ranged_body(), status_code=206, media_type=ctype, headers=headers)


async def head_response_s3(key: str, head: dict, range_header: str | None = None) -> Response:
    size = int(head.get("ContentLength") or 0)
    ctype = head.get("ContentType") or "application/octet-stream"
    base = {
        "Content-Type": ctype,
        "Cache-Control": cache_control_for(key),
        "X-Cache": "HIT-S3",
        "Accept-Ranges": "bytes",
    }
    try:
        byte_range = parse_bytes_range(range_header, size)
    except ValueError as exc:
        if "unsatisfiable" in str(exc):
            return Response(
                status_code=416,
                headers={**base, "Content-Range": f"bytes */{size}"},
            )
        byte_range = None
    if byte_range is None:
        base["Content-Length"] = str(size)
        await metrics.incr("hits_s3")
        return Response(status_code=200, headers=base)
    start, end = byte_range
    length = end - start + 1
    base["Content-Range"] = f"bytes {start}-{end}/{size}"
    base["Content-Length"] = str(length)
    await metrics.incr("hits_s3")
    await metrics.incr("range_hits")
    return Response(status_code=206, headers=base)


async def revalidate_metadata(key: str, upstream_url: str, head: dict) -> bool:
    """
    Return True if S3 object is still valid (upstream 304 or fresh enough to keep).
    Return False if caller should re-fetch from upstream.
    """
    assert http_client is not None
    headers: dict[str, str] = {}
    modified = head.get("LastModified")
    if isinstance(modified, datetime):
        # format_datetime(..., usegmt=True) requires an aware UTC datetime
        if modified.tzinfo is None:
            modified = modified.replace(tzinfo=timezone.utc)
        else:
            modified = modified.astimezone(timezone.utc)
        headers["If-Modified-Since"] = format_datetime(modified, usegmt=True)

    try:
        r = await http_client.head(upstream_url, headers=headers)
    except httpx.HTTPError as exc:
        log.warning("revalidate HEAD failed for %s: %s — keeping S3 copy", key, exc)
        _validated_at[key] = time.monotonic()
        return True

    if r.status_code == 304:
        _validated_at[key] = time.monotonic()
        await metrics.incr("revalidated_304")
        log.info("REVALIDATED 304 %s", key)
        return True
    if r.status_code == 404:
        remember_negative(key)
        return False
    if r.status_code >= 400:
        # Keep serving cached copy on transient upstream errors
        log.warning("revalidate status %s for %s — keeping S3 copy", r.status_code, key)
        _validated_at[key] = time.monotonic()
        return True

    # 200: compare size / last-modified when available
    remote_len = r.headers.get("content-length")
    local_len = head.get("ContentLength")
    if remote_len is not None and local_len is not None and int(remote_len) == int(local_len):
        remote_lm = r.headers.get("last-modified")
        if remote_lm and isinstance(modified, datetime):
            try:
                remote_dt = parsedate_to_datetime(remote_lm)
                if remote_dt.tzinfo is None:
                    remote_dt = remote_dt.replace(tzinfo=timezone.utc)
                if modified.tzinfo is None:
                    modified = modified.replace(tzinfo=timezone.utc)
                if remote_dt <= modified:
                    _validated_at[key] = time.monotonic()
                    await metrics.incr("revalidated_304")
                    return True
            except (TypeError, ValueError, IndexError):
                pass

    return False


async def _upload_spool(
    key: str,
    tmp_path: str,
    ctype: str,
    size: int,
    upstream_etag: str | None,
    existing_head: dict | None,
) -> str:
    """Upload spool to S3. Returns X-Cache token."""
    assert s3_session is not None

    if existing_head is not None and not is_metadata_key(key):
        existing_size = existing_head.get("ContentLength")
        if existing_size is not None and int(existing_size) != size:
            await metrics.incr("package_conflicts")
            log.warning(
                "PACKAGE CONFLICT s3://%s/%s existing=%s new=%s — refusing overwrite",
                S3_BUCKET,
                key,
                existing_size,
                size,
            )
            return "MISS-STORE-SKIPPED-CONFLICT"

    # Do not send S3 user Metadata — this endpoint returns SignatureDoesNotMatch for it.
    _ = upstream_etag  # reserved for future revalidation; IMS uses LastModified
    extra: dict[str, Any] = {"ContentType": ctype or "application/octet-stream"}

    try:
        async with s3_session.client(**s3_client_kwargs()) as s3:
            with open(tmp_path, "rb") as fh:
                await s3.upload_fileobj(fh, S3_BUCKET, key, ExtraArgs=extra)
        log.info("CACHED s3://%s/%s (%d bytes)", S3_BUCKET, key, size)
        await metrics.incr("misses_stored")
        clear_negative(key)
        if is_metadata_key(key):
            _validated_at[key] = time.monotonic()
        return "MISS-STORED"
    except Exception as exc:  # noqa: BLE001
        log.exception("S3 upload failed for %s: %s", key, exc)
        await metrics.incr("misses_store_failed")
        return "MISS-STORE-FAILED"


async def _spool_upstream(
    key: str,
    upstream_url: str,
    *,
    existing_head: dict | None = None,
    range_header: str | None = None,
) -> Response:
    """Fetch upstream into tmp, upload to S3, stream to client (optionally ranged)."""
    assert http_client is not None and s3_session is not None

    # Always fetch the full object on miss so S3 stays complete; Range is applied from the spool.
    ensure_spool_capacity()

    async with http_client.stream("GET", upstream_url) as resp:
        if resp.status_code == 404:
            remember_negative(key)
            await metrics.incr("not_found")
            return Response(status_code=404, content=b"Not Found", headers={"X-Cache": "MISS"})
        if resp.status_code >= 400:
            data = await resp.aread()
            await metrics.incr("upstream_errors")
            return Response(
                content=data,
                status_code=resp.status_code,
                media_type=resp.headers.get("content-type"),
                headers={"X-Cache": "UPSTREAM-ERROR"},
            )

        ctype = resp.headers.get("content-type") or "application/octet-stream"
        upstream_etag = resp.headers.get("etag")
        declared = resp.headers.get("content-length")
        expected = int(declared) if declared and declared.isdigit() else None
        if expected is not None and expected > MAX_SPOOL_BYTES:
            await metrics.incr("upstream_errors")
            return Response(
                status_code=502,
                content=b"Object exceeds MAX_SPOOL_BYTES\n",
                media_type="text/plain",
                headers={"X-Cache": "SPOOL-TOO-LARGE"},
            )
        if expected is not None:
            ensure_spool_capacity(expected)

        tmp = tempfile.NamedTemporaryFile(delete=False, prefix="mirror-", suffix=".bin")
        tmp_path = tmp.name
        size = 0
        try:
            async for chunk in resp.aiter_bytes(1024 * 1024):
                size += len(chunk)
                if size > MAX_SPOOL_BYTES:
                    tmp.close()
                    try:
                        os.unlink(tmp_path)
                    except OSError:
                        pass
                    await metrics.incr("upstream_errors")
                    return Response(
                        status_code=502,
                        content=b"Object exceeds MAX_SPOOL_BYTES\n",
                        media_type="text/plain",
                        headers={"X-Cache": "SPOOL-TOO-LARGE"},
                    )
                if size == len(chunk):
                    # First chunk: re-check free space roughly
                    if tmp_free_bytes() < MIN_TMP_FREE_BYTES:
                        tmp.close()
                        try:
                            os.unlink(tmp_path)
                        except OSError:
                            pass
                        return Response(
                            status_code=503,
                            content=b"Insufficient temporary storage\n",
                            media_type="text/plain",
                            headers={"X-Cache": "SPOOL-NO-SPACE"},
                        )
                tmp.write(chunk)
            tmp.flush()
            tmp.close()

            xcache = await _upload_spool(
                key, tmp_path, ctype, size, upstream_etag, existing_head
            )

            # Serve from spool (full or ranged), delete file afterward
            try:
                client_range = parse_bytes_range(range_header, size)
            except ValueError as exc:
                try:
                    os.unlink(tmp_path)
                except OSError:
                    pass
                if "unsatisfiable" in str(exc):
                    return Response(
                        status_code=416,
                        headers={
                            "Content-Range": f"bytes */{size}",
                            "X-Cache": xcache,
                        },
                    )
                client_range = None

            if client_range is None:
                async def iterfile() -> AsyncIterator[bytes]:
                    loop = asyncio.get_running_loop()
                    f = await loop.run_in_executor(None, open, tmp_path, "rb")
                    try:
                        while True:
                            chunk = await loop.run_in_executor(None, f.read, 1024 * 1024)
                            if not chunk:
                                break
                            yield chunk
                    finally:
                        f.close()
                        try:
                            os.unlink(tmp_path)
                        except OSError:
                            pass

                headers = {
                    "Cache-Control": cache_control_for(key),
                    "X-Cache": xcache,
                    "Content-Length": str(size),
                    "Accept-Ranges": "bytes",
                }
                await metrics.incr("bytes_served", size)
                return StreamingResponse(iterfile(), media_type=ctype, headers=headers)

            start, end = client_range
            length = end - start + 1

            async def iter_range() -> AsyncIterator[bytes]:
                loop = asyncio.get_running_loop()
                f = await loop.run_in_executor(None, open, tmp_path, "rb")
                try:
                    await loop.run_in_executor(None, f.seek, start)
                    remaining = length
                    while remaining > 0:
                        chunk = await loop.run_in_executor(
                            None, f.read, min(1024 * 1024, remaining)
                        )
                        if not chunk:
                            break
                        remaining -= len(chunk)
                        yield chunk
                finally:
                    f.close()
                    try:
                        os.unlink(tmp_path)
                    except OSError:
                        pass

            headers = {
                "Cache-Control": cache_control_for(key),
                "X-Cache": xcache,
                "Accept-Ranges": "bytes",
                "Content-Range": f"bytes {start}-{end}/{size}",
                "Content-Length": str(length),
            }
            await metrics.incr("range_hits")
            await metrics.incr("bytes_served", length)
            return StreamingResponse(
                iter_range(), status_code=206, media_type=ctype, headers=headers
            )
        except Exception:
            try:
                os.unlink(tmp_path)
            except OSError:
                pass
            raise


async def fetch_upstream_and_cache(
    key: str,
    upstream_url: str,
    *,
    existing_head: dict | None = None,
    range_header: str | None = None,
) -> Response:
    """Singleflight-aware fetch: one upstream GET per key; waiters re-read from S3."""
    assert s3_session is not None

    existing = _inflight.get(key)
    if existing is not None:
        try:
            await existing
        except Exception:  # noqa: BLE001
            pass
        # After leader finishes, serve from S3 when present
        try:
            async with s3_session.client(**s3_client_kwargs()) as s3:
                head = await s3.head_object(Bucket=S3_BUCKET, Key=key)
            if range_header:
                return await stream_s3(key, head, range_header)
            return await stream_s3(key, head, None)
        except ClientError:
            if negative_cached(key):
                await metrics.incr("negative_hits")
                return Response(
                    status_code=404, content=b"Not Found", headers={"X-Cache": "NEGATIVE"}
                )
            return Response(
                status_code=502,
                content=b"Coalesced fetch failed\n",
                media_type="text/plain",
                headers={"X-Cache": "COALESCE-FAIL"},
            )

    loop = asyncio.get_running_loop()
    fut: asyncio.Future[None] = loop.create_future()
    _inflight[key] = fut
    async with metrics.lock:
        if len(_inflight) > metrics.inflight_peak:
            metrics.inflight_peak = len(_inflight)

    try:
        resp = await _spool_upstream(
            key,
            upstream_url,
            existing_head=existing_head,
            range_header=range_header,
        )
        if not fut.done():
            fut.set_result(None)
        return resp
    except Exception as exc:
        if not fut.done():
            fut.set_exception(exc)
        raise
    finally:
        if _inflight.get(key) is fut:
            _inflight.pop(key, None)


@app.api_route("/{full_path:path}", methods=["GET", "HEAD"])
async def mirror(full_path: str, request: Request):
    path = "/" + full_path
    resolved = resolve_upstream(path)
    if resolved is None:
        resolved = resolve_upstream(path + "/")
        if resolved is None:
            return Response(
                status_code=404,
                content=b"Unknown mirror prefix. See GET / for paths.\n",
                media_type="text/plain",
            )

    _prefix, key, upstream_url = resolved
    range_header = request.headers.get("range")

    if key.endswith("/"):
        assert http_client is not None
        r = await http_client.get(upstream_url)
        return Response(
            content=r.content,
            status_code=r.status_code,
            media_type=r.headers.get("content-type", "text/html"),
            headers={
                "X-Cache": "BYPASS-DIR",
                "Cache-Control": f"public, max-age={METADATA_CACHE_SECONDS}",
            },
        )

    if negative_cached(key):
        await metrics.incr("negative_hits")
        return Response(
            status_code=404,
            content=b"Not Found",
            headers={"X-Cache": "NEGATIVE", "Cache-Control": f"public, max-age={NEGATIVE_CACHE_SECONDS}"},
        )

    assert s3_session is not None
    head: dict | None = None
    try:
        async with s3_session.client(**s3_client_kwargs()) as s3:
            head = await s3.head_object(Bucket=S3_BUCKET, Key=key)
        if s3_object_is_fresh(head, key):
            if request.method == "HEAD":
                return await head_response_s3(key, head, range_header)
            return await stream_s3(key, head, range_header)

        # Stale metadata: conditional revalidation
        if is_metadata_key(key):
            keep = await revalidate_metadata(key, upstream_url, head)
            if keep:
                if request.method == "HEAD":
                    return await head_response_s3(key, head, range_header)
                return await stream_s3(key, head, range_header)
            log.info("S3 stale metadata, re-fetching %s", key)
    except ClientError as exc:
        code = exc.response.get("Error", {}).get("Code", "")
        if code not in ("404", "NoSuchKey", "NotFound", "403"):
            log.debug("S3 head %s: %s", key, code)
        head = None

    if request.method == "HEAD":
        # Coalesce with GET path when possible: if inflight, wait; else probe upstream
        if key in _inflight:
            try:
                await _inflight[key]
            except Exception:  # noqa: BLE001
                pass
            try:
                async with s3_session.client(**s3_client_kwargs()) as s3:
                    head = await s3.head_object(Bucket=S3_BUCKET, Key=key)
                return await head_response_s3(key, head, range_header)
            except ClientError:
                pass
        assert http_client is not None
        r = await http_client.head(upstream_url)
        if r.status_code == 404:
            remember_negative(key)
            await metrics.incr("not_found")
        headers = {
            "Content-Type": r.headers.get("content-type", "application/octet-stream"),
            "X-Cache": "MISS",
            "Cache-Control": cache_control_for(key),
            "Accept-Ranges": "bytes",
        }
        if cl := r.headers.get("content-length"):
            headers["Content-Length"] = cl
        return Response(status_code=r.status_code, headers=headers)

    return await fetch_upstream_and_cache(
        key,
        upstream_url,
        existing_head=head,
        range_header=range_header,
    )
