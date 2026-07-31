(function () {
  var CIRC = Number(window.__METRICS_CIRC__ || 263.8938);
  function fmtInt(n) {
    return Number(n || 0).toLocaleString("en-US");
  }
  function fmtBytes(n) {
    n = Number(n || 0);
    var u = ["B", "KB", "MB", "GB", "TB", "PB"];
    var i = 0;
    while (n >= 1024 && i < u.length - 1) {
      n /= 1024;
      i++;
    }
    return (i === 0 ? String(Math.round(n)) : n.toFixed(1)) + " " + u[i];
  }
  function setText(id, v) {
    var el = document.getElementById(id);
    if (el) el.textContent = v;
  }
  function apply(d) {
    var hits = d.hits_s3 || 0;
    var misses = d.misses_stored || 0;
    var total = hits + misses;
    var pct = total ? (100 * hits) / total : 0;
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
    if (quota != null && Number(quota) > 0) {
      setText("m-s3-quota", fmtBytes(quota));
      setText("m-s3-free", fmtBytes(free == null ? 0 : free));
      var upct = Math.min(100, (100 * used) / Number(quota));
      var s3bar = document.getElementById("m-s3-bar");
      if (s3bar) s3bar.style.width = upct.toFixed(1) + "%";
      setText("m-s3-note", "Quota " + fmtBytes(quota));
    } else {
      setText("m-s3-quota", "—");
      setText("m-s3-free", "—");
      var s3bar2 = document.getElementById("m-s3-bar");
      if (s3bar2) s3bar2.style.width = "0%";
      setText("m-s3-note", "Set S3_QUOTA_BYTES to show free space");
    }
    if (d.s3_usage_error) {
      setText("m-s3-note", "Usage refresh error: " + d.s3_usage_error);
    }
  }
  async function tick() {
    try {
      var r = await fetch("/api/metrics", { headers: { Accept: "application/json" } });
      if (!r.ok) return;
      apply(await r.json());
    } catch (e) {}
  }
  setInterval(tick, 5000);
})();
