(function () {
  "use strict";

  // 注入通用进度条样式，各主题只需覆盖容器级边距/背景即可
  (function injectStyles() {
    var id = "nezha-availability-styles";
    if (document.getElementById(id)) return;
    var style = document.createElement("style");
    style.id = id;
    style.textContent =
      ".nezha-availability-bar{font-size:12px}" +
      ".nezha-availability-bar .availability-row{display:flex;justify-content:space-between;align-items:center;margin-bottom:4px}" +
      ".nezha-availability-bar .availability-label{opacity:.7}" +
      ".nezha-availability-bar .availability-percent{font-weight:bold}" +
      ".nezha-availability-bar .availability-percent.good{color:#4ba242}" +
      ".nezha-availability-bar .availability-percent.warning{color:#f7c709}" +
      ".nezha-availability-bar .availability-percent.danger{color:#e74c3c}" +
      ".nezha-availability-bar .availability-bar-bg{height:6px;background:rgba(128,128,128,.2);border-radius:3px;overflow:hidden;margin-bottom:4px}" +
      ".nezha-availability-bar .availability-bar-fill{height:100%;border-radius:3px;transition:width .3s ease}" +
      ".nezha-availability-bar .availability-bar-fill.good{background:#4ba242}" +
      ".nezha-availability-bar .availability-bar-fill.warning{background:#f7c709}" +
      ".nezha-availability-bar .availability-bar-fill.danger{background:#e74c3c}" +
      ".nezha-availability-bar .availability-detail{font-size:11px;opacity:.7;text-align:right}";
    document.head.appendChild(style);
  })();

  var debounceTimer = null;

  function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return "0秒";
    var s = Number(seconds);
    var parts = [];
    var days = Math.floor(s / 86400);
    if (days) parts.push(days + "天");
    var hours = Math.floor((s % 86400) / 3600);
    if (hours) parts.push(hours + "小时");
    var minutes = Math.floor((s % 3600) / 60);
    if (minutes) parts.push(minutes + "分");
    var secs = Math.floor(s % 60);
    if (secs || parts.length === 0) parts.push(secs + "秒");
    return parts.join("");
  }

  function availabilityClass(percent) {
    if (percent >= 99) return "good";
    if (percent >= 95) return "warning";
    return "danger";
  }

  function buildText(summary) {
    var percent = summary.availability_percent != null ? summary.availability_percent.toFixed(2) : "100.00";
    var count = summary.offline_count || 0;
    var duration = formatDuration(summary.total_offline_seconds);
    var longest = formatDuration(summary.longest_offline_seconds);
    var text = percent + "% 可用率";
    if (count > 0) {
      text += " · 离线 " + count + " 次 / 共 " + duration + "（最长 " + longest + "）";
    } else {
      text += " · 无离线";
    }
    return text;
  }

  function renderBar(el, summary) {
    var percent = summary.availability_percent != null ? summary.availability_percent.toFixed(2) : "100.00";
    var count = summary.offline_count || 0;
    var duration = formatDuration(summary.total_offline_seconds);
    var longest = formatDuration(summary.longest_offline_seconds);
    var detail = count > 0 ? "离线 " + count + " 次 / 共 " + duration + "（最长 " + longest + "）" : "无离线";
    var cls = availabilityClass(Number(percent));

    el.innerHTML =
      '<div class="availability-row">' +
        '<span class="availability-label">可用性</span>' +
        '<span class="availability-percent ' + cls + '">' + percent + '%</span>' +
      '</div>' +
      '<div class="availability-bar-bg">' +
        '<div class="availability-bar-fill ' + cls + '" style="width:' + percent + '%;"></div>' +
      '</div>' +
      '<div class="availability-detail">' + detail + '</div>';
  }

  function renderOne(summary) {
    var text = buildText(summary);
    var title = "最近 " + (summary.days || 30) + " 天可用性：" + text;

    var containers = document.querySelectorAll('[data-availability-id="' + summary.server_id + '"]');
    for (var i = 0; i < containers.length; i++) {
      var el = containers[i];
      el.setAttribute("title", title);
      if (el.classList.contains('nezha-availability-bar')) {
        renderBar(el, summary);
      } else if (el.classList.contains('nezha-availability')) {
        el.textContent = text;
      }
      el.setAttribute("data-availability-loaded", "true");
    }
  }

  function collectPendingIds() {
    var ids = [];
    var seen = {};
    var containers = document.querySelectorAll('[data-availability-id]:not([data-availability-loaded="true"])');
    for (var i = 0; i < containers.length; i++) {
      var id = containers[i].getAttribute("data-availability-id");
      if (!id || seen[id]) continue;
      seen[id] = true;
      ids.push(id);
    }
    return ids;
  }

  function chunk(arr, size) {
    var result = [];
    for (var i = 0; i < arr.length; i += size) {
      result.push(arr.slice(i, i + size));
    }
    return result;
  }

  function markError(batch) {
    for (var i = 0; i < batch.length; i++) {
      var containers = document.querySelectorAll('[data-availability-id="' + batch[i] + '"]');
      for (var j = 0; j < containers.length; j++) {
        var el = containers[j];
        if (el.classList.contains('nezha-availability-bar')) {
          el.innerHTML = '<div class="availability-detail">可用性数据加载失败</div>';
        } else {
          el.textContent = "可用性数据加载失败";
        }
        el.setAttribute("data-availability-loaded", "true");
      }
    }
  }

  function fetchBatch(batch) {
    var url = "/api/v1/server/availability?id=" + encodeURIComponent(batch.join(","));
    $.ajax({
      url: url,
      type: "GET",
      dataType: "json",
    }).done(function (resp) {
      if (resp.code !== 200 || !resp.result) {
        markError(batch);
        return;
      }
      for (var i = 0; i < resp.result.length; i++) {
        renderOne(resp.result[i]);
      }
      var returned = {};
      for (var k = 0; k < resp.result.length; k++) {
        returned[resp.result[k].server_id] = true;
      }
      for (var m = 0; m < batch.length; m++) {
        if (!returned[batch[m]]) markError([batch[m]]);
      }
    }).fail(function () {
      markError(batch);
    });
  }

  function fetchAndRender() {
    var ids = collectPendingIds();
    if (ids.length === 0) return;
    var batches = chunk(ids, 50);
    for (var i = 0; i < batches.length; i++) {
      fetchBatch(batches[i]);
    }
  }

  function scheduleFetch() {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(fetchAndRender, 300);
  }

  scheduleFetch();

  if (typeof MutationObserver !== "undefined") {
    var observer = new MutationObserver(function (mutations) {
      var hasNew = false;
      for (var i = 0; i < mutations.length; i++) {
        var nodes = mutations[i].addedNodes;
        for (var j = 0; j < nodes.length; j++) {
          if (nodes[j].nodeType === 1 && (
              nodes[j].hasAttribute && nodes[j].hasAttribute("data-availability-id") ||
              (nodes[j].querySelector && nodes[j].querySelector("[data-availability-id]"))
            )) {
            hasNew = true;
            break;
          }
        }
        if (hasNew) break;
      }
      if (hasNew) scheduleFetch();
    });
    observer.observe(document.body, { childList: true, subtree: true });
  }
})();
