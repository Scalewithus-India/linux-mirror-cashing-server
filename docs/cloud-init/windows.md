# Windows — package mirror notes

Mirror: `https://mirror.scalewithus.com`

This host is an **on-demand Linux (and related) package object cache**. It is **not** a Windows Update / WSUS proxy.

Windows Update uses Microsoft’s proprietary update protocol. Pointing Windows at `mirror.scalewithus.com` will not accelerate or replace WU, and there is no `/windows/` upstream on this mirror.

## What to use instead

| Need | Recommendation |
|------|----------------|
| OS / security updates | Windows Update or your own WSUS / Microsoft Update infrastructure |
| Linux guests on the same platform | Use the per-OS guides (Ubuntu, Debian, Alma, Rocky, CentOS Stream, Alpine, …) |
| Optional Unix tooling on Windows | Not mirrored here today; install MSYS2/Chocolatey/WinGet from their official sources |

## Verify the mirror (from any host)

```bash
curl -fsS https://mirror.scalewithus.com/healthz
```

That only checks the mirror service is up — it does not configure Windows package updates.

## Related guides

- [Live switch (Linux distros)](switch.md)
- [Ubuntu](ubuntu.md) · [Debian](debian.md) · [Alpine](alpine.md) · [CentOS (Stream)](centos-stream.md)
