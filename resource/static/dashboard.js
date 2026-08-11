(function () {
  let LANG = {
    Add: "添加",
    Edit: "修改",
    Confirm: "确认",
    Cancel: "取消",
    Save: "保存",
    Server: "服务器",
    Monitor: "监控",
    Cron: "计划任务",
    Notification: "通知方式",
  };

  function updateLang(newLang) {
    if (newLang) {
      LANG = Object.assign({}, LANG, newLang);
    }
  }

  function t(key) {
    return LANG[key] || key;
  }

  function escapeHTML(value) {
    return String(value || "").replace(/[&<>"']/g, function (char) {
      return {
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      }[char];
    });
  }

  function highlightJSON(value) {
    const source = String(value || "");
    if (!source) {
      return "";
    }
    return source.replace(/("(\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g, function (match) {
      let type = "number";
      if (/^"/.test(match)) {
        type = /:$/.test(match) ? "key" : "string";
      } else if (match === "true" || match === "false") {
        type = "boolean";
      } else if (match === "null") {
        type = "null";
      }
      return '<span class="dashboard-json-token dashboard-json-token-' + type + '">' + escapeHTML(match) + "</span>";
    });
  }

  window.updateLang = updateLang;

  function csrfHeaders(headers) {
    if (window.santaiziCSRFHeaders) {
      return window.santaiziCSRFHeaders(headers || {});
    }
    headers = headers || {};
    if (window.SANTAIZI_CSRF_TOKEN) {
      headers["X-CSRF-Token"] = window.SANTAIZI_CSRF_TOKEN;
    }
    return headers;
  }

  async function requestJSON(url, options) {
    const resp = await fetch(url, options);
    const text = await resp.text();
    let data = {};
    if (text) {
      try {
        data = JSON.parse(text);
      } catch (error) {
        throw new Error(text);
      }
    }
    if (!resp.ok) {
      throw new Error(data.message || resp.statusText);
    }
    return data;
  }

  async function postJSON(url, data) {
    return requestJSON(url, {
      method: "POST",
      headers: csrfHeaders({
        "Content-Type": "application/json",
        Accept: "application/json",
      }),
      body: JSON.stringify(data),
    });
  }

  async function deleteJSON(url) {
    return requestJSON(url, {
      method: "DELETE",
      headers: csrfHeaders({ Accept: "application/json" }),
    });
  }

  window.requestJSON = requestJSON;
  window.ensureOK = ensureOK;
  window.deleteJSON = deleteJSON;

  function ensureOK(resp) {
    if (resp && resp.code && resp.code !== 200) {
      throw new Error(resp.message || ("Error " + resp.code));
    }
    return resp;
  }

  function message(type, content) {
    if (window.ElementPlus && ElementPlus.ElMessage) {
      ElementPlus.ElMessage({
        type: type,
        message: content,
        duration: 2600,
      });
    } else {
      alert(content);
    }
  }

  function parseIDArray(value) {
    if (Array.isArray(value)) {
      return value.map(Number).filter(Number.isFinite);
    }
    if (value === undefined || value === null || value === "") {
      return [];
    }
    if (typeof value === "string") {
      try {
        const parsed = JSON.parse(value);
        if (Array.isArray(parsed)) {
          return parsed.map(Number).filter(Number.isFinite);
        }
      } catch (error) {}
      return (value.match(/\d+/g) || []).map(Number).filter(Number.isFinite);
    }
    return [];
  }

  function toIDMap(ids) {
    const ret = {};
    parseIDArray(ids).forEach(function (id) {
      ret[id] = true;
    });
    return ret;
  }

  function clone(value) {
    return JSON.parse(JSON.stringify(value || {}));
  }

  function onOff(v) {
    return v === true || v === "on" ? "on" : "off";
  }

  function selectedServerIDs() {
    const ids = new Set();
    document.querySelectorAll('input.santaizi-servers[type="checkbox"]').forEach(function (cb) {
      if (cb.checked && cb.offsetParent !== null) {
        ids.add(Number(cb.value));
      }
    });
    return Array.from(ids).filter(Number.isFinite);
  }

  window.showConfirm = function (title, content, callFn, extData) {
    if (window.ElementPlus && ElementPlus.ElMessageBox) {
      ElementPlus.ElMessageBox.confirm(content, title, {
        confirmButtonText: t("Confirm"),
        cancelButtonText: t("Cancel"),
        type: "warning",
      }).then(function () {
        return callFn(extData);
      }).catch(function () {});
      return;
    }
    if (confirm(title + "\n" + content)) {
      callFn(extData);
    }
  };

  window.deleteRequest = async function (api) {
    try {
      const resp = ensureOK(await deleteJSON(api));
      message("success", resp.message || "删除成功");
      window.location.reload();
    } catch (error) {
      message("error", error.message || String(error));
    }
  };

  window.resetServerSecret = async function (serverId) {
    try {
      const resp = ensureOK(await postJSON("/api/server/" + serverId + "/reset-secret", {}));
      message("success", "密钥已重置：" + (resp.message || ""));
      window.location.reload();
    } catch (error) {
      message("error", error.message || String(error));
    }
  };

  window.logout = async function (id) {
    try {
      ensureOK(await postJSON("/api/logout", { id: id }));
      message("success", "注销成功");
      window.location.reload();
    } catch (error) {
      message("error", error.message || String(error));
    }
  };

  window.connectToServer = function (id) {
    postForm("/terminal", {
      Host: window.location.host,
      Protocol: window.location.protocol,
      ID: id,
    });
  };

  // PC 端以弹窗形式打开服务器可用性历史，弹窗内可独立页面打开；移动端直接打开独立页面。
  window.openServerOfflineHistory = function (serverId) {
    const url = "/server/offline-history?server_id=" + encodeURIComponent(serverId);
    const isMobile = window.matchMedia && window.matchMedia("(max-width: 768px)").matches;
    if (isMobile) {
      window.open(url, "_blank");
      return;
    }
    if (!window.Vue || !window.ElementPlus) {
      window.open(url, "_blank");
      return;
    }

    const mount = document.createElement("div");
    mount.id = "offline-history-modal-" + Date.now();
    document.body.appendChild(mount);

    // 弹窗内以 modal=1 打开：页面隐藏顶部导航与返回按钮，使用紧凑排版；“新窗口打开”仍用完整页面
    const iframeUrl = url + "&modal=1";

    const app = Vue.createApp({
      data: function () {
        return { visible: true, url: url, iframeUrl: iframeUrl };
      },
      mounted: function () {
        document.documentElement.classList.add("dashboard-modal-open");
        document.body.classList.add("dashboard-modal-open");
      },
      beforeUnmount: function () {
        document.documentElement.classList.remove("dashboard-modal-open");
        document.body.classList.remove("dashboard-modal-open");
      },
      watch: {
        visible: function (value) {
          if (!value) {
            this.$nextTick(function () {
              try {
                app.unmount();
              } catch (error) {}
              if (mount.parentNode) {
                mount.parentNode.removeChild(mount);
              }
            });
          }
        },
      },
      template:
        '<el-dialog v-model="visible" width="min(1100px, 96vw)" top="4vh" :close-on-click-modal="false" destroy-on-close append-to-body>' +
        '  <template #header>' +
        '    <div style="display:flex;justify-content:space-between;align-items:center;width:100%;padding-right:28px;box-sizing:border-box;">' +
        '      <span style="font-weight:600;font-size:16px;">服务器可用性历史</span>' +
        '      <a class="dashboard-button dashboard-button-small" :href="url" target="_blank"><i class="ri-external-link-line"></i> 新窗口打开</a>' +
        '    </div>' +
        '  </template>' +
        '  <iframe :src="iframeUrl" style="width:100%;height:70vh;border:none;display:block;"></iframe>' +
        '</el-dialog>',
    });
    app.use(ElementPlus);
    app.mount(mount);
  };

  function postForm(path, params, method) {
    const form = document.createElement("form");
    form.method = method || "post";
    form.action = path;
    form.target = "_blank";
    if (window.SANTAIZI_CSRF_TOKEN && !/^(get|head|options|trace)$/i.test(form.method)) {
      const csrfField = document.createElement("input");
      csrfField.type = "hidden";
      csrfField.name = "_csrf";
      csrfField.value = window.SANTAIZI_CSRF_TOKEN;
      form.appendChild(csrfField);
    }
    Object.keys(params || {}).forEach(function (key) {
      const field = document.createElement("input");
      field.type = "hidden";
      field.name = key;
      field.value = params[key];
      form.appendChild(field);
    });
    document.body.appendChild(form);
    form.submit();
    document.body.removeChild(form);
  }

  window.checkAllServer = function () {
    document.querySelectorAll('input.santaizi-servers[type="checkbox"]').forEach(function (cb) {
      if (cb.offsetParent !== null) {
        cb.checked = true;
      }
    });
  };

  window.batchEditServerGroup = async function () {
    const servers = selectedServerIDs();
    if (!servers.length) {
      message("warning", "请选择服务器");
      return;
    }
    const group = prompt(t("InputServerGroupName") || "Input server group name");
    if (group === null) {
      return;
    }
    try {
      ensureOK(await postJSON("/api/batch-update-server-group", { servers: servers, group: group }));
      message("success", "操作成功");
      window.location.reload();
    } catch (error) {
      message("error", error.message || String(error));
    }
  };

  window.batchDeleteServer = function () {
    const servers = selectedServerIDs();
    if (!servers.length) {
      message("warning", "请选择服务器");
      return;
    }
    window.showConfirm(t("BatchDeleteServer"), t("ConfirmToDeleteServer"), async function () {
      try {
        ensureOK(await postJSON("/api/batch-delete-server", servers));
        message("success", "操作成功");
        window.location.reload();
      } catch (error) {
        message("error", error.message || String(error));
      }
    });
  };

  window.manualTrigger = async function (btn, cronId) {
    if (btn) {
      btn.disabled = true;
    }
    try {
      const resp = ensureOK(await requestJSON("/api/cron/" + cronId + "/manual?_csrf=" + encodeURIComponent(window.SANTAIZI_CSRF_TOKEN || ""), {
        method: "GET",
        headers: csrfHeaders({ Accept: "application/json" }),
      }));
      message("success", resp.message || "触发成功，等待执行结果");
    } catch (error) {
      message("error", error.message || String(error));
    } finally {
      if (btn) {
        btn.disabled = false;
      }
    }
  };

  window.submitSettingForm = async function (event) {
    event.preventDefault();
    const form = event.target;
    const body = new URLSearchParams(new FormData(form));
    try {
      const resp = ensureOK(await requestJSON("/api/setting", {
        method: "POST",
        headers: csrfHeaders({
          "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
          Accept: "application/json",
        }),
        body: body.toString(),
      }));
      message("success", resp.message || t("ModifiedSuccessfully") || "Saved");
      window.location.reload();
    } catch (error) {
      message("error", error.message || String(error));
    }
    return false;
  };

  ["issueNewApiToken", "addOrEditNotification", "addOrEditDDNS", "addOrEditNAT", "addOrEditServer", "addOrEditMonitor", "addOrEditCron", "addOrEditAlertRule"].forEach(function (name) {
    window[name] = function (payload) {
      if (!window.dashboardModal) {
        message("error", "Dashboard modal is not ready");
        return;
      }
      const map = {
        issueNewApiToken: "api",
        addOrEditNotification: "notification",
        addOrEditDDNS: "ddns",
        addOrEditNAT: "nat",
        addOrEditServer: "server",
        addOrEditMonitor: "monitor",
        addOrEditCron: "cron",
        addOrEditAlertRule: "rule",
      };
      window.dashboardModal.open(map[name], payload || null);
    };
  });

  function installCopyHandler() {
    document.addEventListener("click", async function (event) {
      const btn = event.target.closest("[data-copy]");
      if (!btn) {
        return;
      }
      const text = btn.getAttribute("data-copy") || "";
      try {
        await navigator.clipboard.writeText(text);
        message("success", t("ClickToCopy") || "Copied");
      } catch (error) {
        const area = document.createElement("textarea");
        area.value = text;
        document.body.appendChild(area);
        area.select();
        document.execCommand("copy");
        document.body.removeChild(area);
        message("success", t("ClickToCopy") || "Copied");
      }
    });
  }

  function installMobileNav() {
    const toggle = document.getElementById("dashboard-mobile-nav-toggle");
    const nav = document.getElementById("dashboard-nav");
    if (!toggle || !nav) {
      return;
    }
    toggle.addEventListener("click", function () {
      nav.classList.toggle("open");
    });
  }

  function createModalApp() {
    if (!window.Vue || !window.ElementPlus || window.dashboardModal) {
      return;
    }
    function elementLocale() {
      const lang = document.documentElement.lang || "en-US";
      const localeMap = {
        "zh-CN": window.ElementPlusLocaleZhCn,
        "zh-TW": window.ElementPlusLocaleZhTw,
        "en-US": window.ElementPlusLocaleEn,
        "es-ES": window.ElementPlusLocaleEs,
      };
      const base = localeMap[lang] || window.ElementPlusLocaleEn || {};
      return Object.assign({}, base, {
        name: base.name || "santaizi",
        el: Object.assign({}, base.el || {}, {
          select: {
            loading: t("Loading"),
            noMatch: t("NoMatch"),
            noData: t("NoData"),
            placeholder: t("PleaseSelect"),
          },
          messagebox: {
            confirm: t("Confirm"),
            cancel: t("Cancel"),
          },
        }),
      });
    }

    function emptyPublicNoteState() {
      return {
        billingDataMod: { startDate: "", endDate: "", autoRenewal: "0", cycle: "", amount: "" },
        planDataMod: { bandwidth: "", trafficVol: "", trafficType: "", IPv4: "0", IPv6: "0", networkRoute: [], extra: [] },
        customData: { location: "", slogan: "", orderLink: "", buyBtnText: "", buyBtnIcon: "", flag: "", lat: "", lng: "", latlng: "", locationLabel: "" },
      };
    }

    const RULE_UNIT_FACTORS = {
      B: 1,
      KB: 1024,
      MB: 1048576,
      GB: 1073741824,
      TB: 1099511627776,
      PB: 1125899906842624,
      "B/s": 1,
      "KB/s": 1024,
      "MB/s": 1048576,
      "GB/s": 1073741824,
    };

    function ruleUnitMeta(type) {
      if (["cpu", "memory", "swap", "disk"].includes(type)) {
        return { category: "ratio", defaultUnit: "%", units: [{ value: "%", label: "%", factor: 1 }] };
      }
      if (["net_in_speed", "net_out_speed", "net_all_speed"].includes(type)) {
        const units = ["B/s", "KB/s", "MB/s", "GB/s"].map(function (u) {
          return { value: u, label: u, factor: RULE_UNIT_FACTORS[u] };
        });
        return { category: "speed", defaultUnit: "MB/s", units: units };
      }
      if (type && type.startsWith("transfer_")) {
        const units = ["B", "KB", "MB", "GB", "TB", "PB"].map(function (u) {
          return { value: u, label: u, factor: RULE_UNIT_FACTORS[u] };
        });
        return { category: "traffic", defaultUnit: "GB", units: units };
      }
      return null;
    }

    function ruleAutoUnit(type, raw) {
      const meta = ruleUnitMeta(type);
      if (!meta) {
        return "";
      }
      if (raw === null || raw === undefined || raw === "" || Number(raw) === 0) {
        return meta.defaultUnit;
      }
      const absRaw = Math.abs(Number(raw));
      for (let i = meta.units.length - 1; i >= 0; i--) {
        if (absRaw >= meta.units[i].factor) {
          return meta.units[i].value;
        }
      }
      return meta.units[0].value;
    }

    function ruleToDisplay(type, raw, unit) {
      if (raw === null || raw === undefined || raw === "") {
        return null;
      }
      const meta = ruleUnitMeta(type);
      if (!meta) {
        return Number(raw);
      }
      const factor = (meta.units.find(function (u) { return u.value === unit; }) || meta.units[0]).factor;
      return Number((Number(raw) / factor).toFixed(6));
    }

    function ruleToRaw(type, display, unit) {
      if (display === null || display === undefined || display === "") {
        return undefined;
      }
      const meta = ruleUnitMeta(type);
      if (!meta) {
        return Number(display);
      }
      const factor = (meta.units.find(function (u) { return u.value === unit; }) || meta.units[0]).factor;
      const raw = Number(display) * factor;
      if (meta.category === "speed" || meta.category === "traffic") {
        return Math.round(raw);
      }
      return Number(raw.toFixed(6));
    }

    const mount = document.createElement("div");
    mount.id = "dashboard-modal-root";
    mount.setAttribute("v-cloak", "");
    document.body.appendChild(mount);

    const app = Vue.createApp({
      data: function () {
        return {
          visible: false,
          kind: "",
          title: "",
          loading: false,
          error: "",
          form: {},
          providers: (window.SANTAIZI_DASHBOARD_CONTEXT && window.SANTAIZI_DASHBOARD_CONTEXT.providers) || [],
          remote: {
            servers: [],
            tasks: [],
            ddns: [],
          },
          activeCollapse: ["basic", "access", "note", "public"],
          publicNoteTab: "billing",
          publicNoteRaw: "",
          publicNoteBase: {},
          endDateUnlimited: false,
          publicNote: emptyPublicNoteState(),
          ruleTab: "visual",
          rules: [],
          ruleTypes: [
            "cpu", "memory", "swap", "disk", "net_in_speed", "net_out_speed", "net_all_speed",
            "transfer_in", "transfer_out", "transfer_all", "transfer_in_cycle", "transfer_out_cycle",
            "transfer_all_cycle", "offline", "load1", "load5", "load15", "tcp_conn_count",
            "udp_conn_count", "process_count", "temperature_max",
          ],
          cycleUnits: ["hour", "day", "week", "month", "year"],
          cycles: ["月", "month", "monthly", "m", "mo", "季", "quarterly", "q", "半年", "半", "half", "semi-annually", "h", "年", "year", "annually", "y", "yr"],
          trafficTypes: [
            { value: "1", label: t("TrafficOutboundOnly") },
            { value: "2", label: t("TrafficBidirectional") },
            { value: "3", label: t("TrafficMaxInOut") },
          ],
        };
      },
      computed: {
        dialogWidth: function () {
          return this.kind === "server" || this.kind === "rule" ? "min(980px, 94vw)" : "min(720px, 94vw)";
        },
        provider: function () {
          const providerID = Number(this.form.Provider || 0);
          return this.providers.find(function (item) { return Number(item.ID) === providerID; }) || {};
        },
        publicNotePreview: function () {
          const built = this.buildPublicNoteObject(true);
          if (built === false) {
            return this.publicNoteRaw || "";
          }
          if (!built || !Object.keys(built).length) {
            return "";
          }
          return JSON.stringify(built, null, 2);
        },
        highlightedPublicNotePreview: function () {
          return highlightJSON(this.publicNotePreview);
        },
        publicNoteInvalid: function () {
          return this.publicNoteTab === "raw" && this.publicNoteRaw.trim() && this.parsePublicNoteRaw() === false;
        },
        ruleTypeDesc: function () {
          const map = {
            cpu: t("RuleTypeDesc_cpu"),
            memory: t("RuleTypeDesc_memory"),
            swap: t("RuleTypeDesc_swap"),
            disk: t("RuleTypeDesc_disk"),
            net_in_speed: t("RuleTypeDesc_net_in_speed"),
            net_out_speed: t("RuleTypeDesc_net_out_speed"),
            net_all_speed: t("RuleTypeDesc_net_all_speed"),
            transfer_in: t("RuleTypeDesc_transfer_in"),
            transfer_out: t("RuleTypeDesc_transfer_out"),
            transfer_all: t("RuleTypeDesc_transfer_all"),
            transfer_in_cycle: t("RuleTypeDesc_transfer_in_cycle"),
            transfer_out_cycle: t("RuleTypeDesc_transfer_out_cycle"),
            transfer_all_cycle: t("RuleTypeDesc_transfer_all_cycle"),
            offline: t("RuleTypeDesc_offline"),
            load1: t("RuleTypeDesc_load1"),
            load5: t("RuleTypeDesc_load5"),
            load15: t("RuleTypeDesc_load15"),
            tcp_conn_count: t("RuleTypeDesc_tcp_conn_count"),
            udp_conn_count: t("RuleTypeDesc_udp_conn_count"),
            process_count: t("RuleTypeDesc_process_count"),
            temperature_max: t("RuleTypeDesc_temperature_max"),
          };
          return function (type) {
            return map[type] || type;
          };
        },
      },
      watch: {
        visible: function (value) {
          document.documentElement.classList.toggle("dashboard-modal-open", !!value);
          document.body.classList.toggle("dashboard-modal-open", !!value);
        },
      },
      methods: {
        t: t,
        showConfirm: window.showConfirm,
        resetServerSecret: window.resetServerSecret,
        confirmClose: function (done) {
          window.showConfirm(t("Confirm"), t("ConfirmCloseModal"), done);
        },
        emptyPublicNote: function () {
          return emptyPublicNoteState();
        },
        defaultForm: function (kind) {
          const defaults = {
            api: { Note: "" },
            notification: { ID: 0, Name: "", Tag: "default", URL: "", RequestMethod: 1, RequestType: 1, RequestHeader: "", RequestBody: "", VerifySSL: "on", SkipCheck: "off" },
            ddns: { ID: 0, Name: "", Provider: this.providers.length ? Number(this.providers[0].ID) : 0, DomainsRaw: "", AccessID: "", AccessSecret: "", MaxRetries: 3, WebhookURL: "", WebhookMethod: 1, WebhookRequestType: 1, WebhookHeaders: "", WebhookRequestBody: "", EnableIPv4: "on", EnableIPv6: "off" },
            nat: { ID: 0, Name: "", ServerID: 0, Host: "", Domain: "" },
            cron: { ID: 0, TaskType: 0, Name: "", Scheduler: "", Command: "", Servers: [], Cover: 0, PushSuccessful: "off", NotificationTag: "default" },
            monitor: { ID: 0, Name: "", Target: "", Type: 1, EnableShowInService: "off", Duration: 30, Cover: 0, SkipServers: [], NotificationTag: "default", Notify: "on", MaxLatency: null, MinLatency: null, LatencyNotify: "off", EnableTriggerTask: "off", FailTriggerTasks: [], RecoverTriggerTasks: [] },
            rule: { ID: 0, Name: "", RulesRaw: "[]", TriggerMode: 0, NotificationTag: "default", FailTriggerTasks: [], RecoverTriggerTasks: [], Enable: "on" },
            server: { id: 0, name: "", Tag: "", DisplayIndex: 0, secret: "", DDNSProfiles: [], EnableDDNS: "off", HideForGuest: "off", Note: "", PublicNote: "" },
          };
          return clone(defaults[kind] || {});
        },
        modalTitle: function (kind, item) {
          const editing = item && (item.ID || item.id);
          const labels = {
            api: "API Token",
            notification: t("Notification"),
            ddns: t("DDNS"),
            nat: "NAT",
            cron: t("Cron"),
            monitor: t("Monitor"),
            rule: t("AlarmRule"),
            server: t("Server"),
          };
          return (editing ? t("Edit") : t("Add")) + " " + (labels[kind] || kind);
        },
        open: function (kind, item) {
          this.kind = kind;
          this.error = "";
          this.form = this.defaultForm(kind);
          this.activeCollapse = item ? ["basic"] : ["basic", "access", "note", "public"];
          this.publicNoteTab = "billing";
          this.ruleTab = "visual";
          this.rules = [];
          this.title = this.modalTitle(kind, item);
          if (item) {
            this.applyItem(kind, item);
          } else if (kind === "rule") {
            this.addRule();
          } else if (kind === "server") {
            this.loadPublicNote("");
          }
          if (["cron", "monitor", "rule"].includes(kind)) {
            this.loadServerOptions();
          }
          this.visible = true;
        },
        applyItem: function (kind, item) {
          const data = clone(item);
          if (kind === "api") {
            this.form.Note = data.Note || "";
          } else if (kind === "notification") {
            Object.assign(this.form, data);
            this.form.VerifySSL = onOff(data.VerifySSL);
            this.form.SkipCheck = onOff(data.SkipCheck);
          } else if (kind === "ddns") {
            Object.assign(this.form, data);
            this.form.EnableIPv4 = onOff(data.EnableIPv4);
            this.form.EnableIPv6 = onOff(data.EnableIPv6);
          } else if (kind === "nat") {
            Object.assign(this.form, data);
          } else if (kind === "cron") {
            Object.assign(this.form, data);
            this.form.Servers = parseIDArray(data.ServersRaw || data.Servers);
            this.form.PushSuccessful = onOff(data.PushSuccessful);
            this.seedOptions("servers", this.form.Servers);
          } else if (kind === "monitor") {
            Object.assign(this.form, data);
            this.form.SkipServers = parseIDArray(data.SkipServersRaw);
            this.form.FailTriggerTasks = parseIDArray(data.FailTriggerTasksRaw);
            this.form.RecoverTriggerTasks = parseIDArray(data.RecoverTriggerTasksRaw);
            this.form.Notify = onOff(data.Notify);
            this.form.LatencyNotify = onOff(data.LatencyNotify);
            this.form.EnableTriggerTask = onOff(data.EnableTriggerTask);
            this.form.EnableShowInService = onOff(data.EnableShowInService);
            this.seedOptions("servers", this.form.SkipServers);
            this.seedOptions("tasks", this.form.FailTriggerTasks.concat(this.form.RecoverTriggerTasks));
          } else if (kind === "rule") {
            Object.assign(this.form, data);
            this.form.Enable = onOff(data.Enable);
            this.form.FailTriggerTasks = parseIDArray(data.FailTriggerTasksRaw);
            this.form.RecoverTriggerTasks = parseIDArray(data.RecoverTriggerTasksRaw);
            this.form.RulesRaw = data.RulesRaw || "[]";
            this.loadRulesFromRaw();
            this.seedOptions("servers", this.rules.flatMap(function (rule) { return rule.ignoreIds || []; }));
            this.seedOptions("tasks", this.form.FailTriggerTasks.concat(this.form.RecoverTriggerTasks));
          } else if (kind === "server") {
            this.form.id = data.ID || data.id || 0;
            this.form.name = data.Name || data.name || "";
            this.form.Tag = data.Tag || "";
            this.form.DisplayIndex = data.DisplayIndex || 0;
            this.form.secret = data.Secret || data.secret || "";
            this.form.DDNSProfiles = parseIDArray(data.DDNSProfilesRaw);
            this.form.EnableDDNS = onOff(data.EnableDDNS);
            this.form.HideForGuest = onOff(data.HideForGuest);
            this.form.Note = data.Note || "";
            this.form.PublicNote = data.PublicNote || "";
            this.loadPublicNote(this.form.PublicNote);
            this.seedOptions("ddns", this.form.DDNSProfiles);
          }
        },
        seedOptions: function (type, ids) {
          const existing = this.remote[type] || [];
          const map = new Map(existing.map(function (item) { return [Number(item.value), item]; }));
          parseIDArray(ids).forEach(function (id) {
            if (!map.has(id)) {
              map.set(id, { value: id, label: "ID:" + id });
            }
          });
          this.remote[type] = Array.from(map.values());
        },
        remoteSearch: async function (type, query) {
          const endpoints = {
            servers: "/api/search-server?word=",
            tasks: "/api/search-tasks?word=",
            ddns: "/api/search-ddns?word=",
          };
          try {
            const data = await requestJSON(endpoints[type] + encodeURIComponent(query || ""), {
              headers: csrfHeaders({ Accept: "application/json" }),
            });
            this.remote[type] = (data.results || []).map(function (item) {
              return {
                value: Number(item.value || item.Value),
                label: item.name || item.text || item.Name || item.Text || ("ID:" + (item.value || item.Value)),
              };
            });
          } catch (error) {
            this.remote[type] = [];
          }
        },
        loadServerOptions: async function () {
          const ids = [];
          if (this.kind === "cron") {
            ids.push.apply(ids, this.form.Servers || []);
          } else if (this.kind === "monitor") {
            ids.push.apply(ids, this.form.SkipServers || []);
          } else if (this.kind === "rule") {
            this.rules.forEach(function (rule) {
              ids.push.apply(ids, rule.ignoreIds || []);
            });
          }
          await this.remoteSearch("servers", "");
          if (ids.length) {
            this.seedOptions("servers", ids);
          }
        },
        loadPublicNote: function (raw) {
          this.publicNoteRaw = raw || "";
          this.publicNoteBase = {};
          this.endDateUnlimited = false;
          this.publicNote = this.emptyPublicNote();
          const parsed = this.parsePublicNoteRaw();
          if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
            this.publicNoteBase = clone(parsed);
            if (parsed.billingDataMod) {
              Object.assign(this.publicNote.billingDataMod, parsed.billingDataMod);
              this.publicNote.billingDataMod.autoRenewal = this.switchString(parsed.billingDataMod.autoRenewal);
              if (parsed.billingDataMod.endDate === "0000-00-00") {
                this.endDateUnlimited = true;
                this.publicNote.billingDataMod.endDate = "";
              }
            }
            if (parsed.planDataMod) {
              Object.assign(this.publicNote.planDataMod, parsed.planDataMod);
              this.publicNote.planDataMod.IPv4 = this.switchString(parsed.planDataMod.IPv4);
              this.publicNote.planDataMod.IPv6 = this.switchString(parsed.planDataMod.IPv6);
              this.publicNote.planDataMod.networkRoute = this.splitTags(parsed.planDataMod.networkRoute);
              this.publicNote.planDataMod.extra = this.splitTags(parsed.planDataMod.extra);
            }
            if (parsed.customData) {
              Object.assign(this.publicNote.customData, parsed.customData);
            }
          }
        },
        parsePublicNoteRaw: function () {
          const raw = (this.publicNoteRaw || "").trim();
          if (!raw) {
            return {};
          }
          try {
            const parsed = JSON.parse(raw);
            return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : false;
          } catch (error) {
            return false;
          }
        },
        splitTags: function (value) {
          if (Array.isArray(value)) {
            return value.filter(Boolean);
          }
          if (!value) {
            return [];
          }
          return String(value).split(",").map(function (item) { return item.trim(); }).filter(Boolean);
        },
        switchString: function (value) {
          return value === "1" || value === 1 || value === true ? "1" : "0";
        },
        filterEmpty: function (obj) {
          const ret = {};
          Object.keys(obj).forEach(function (key) {
            const value = obj[key];
            if (Array.isArray(value) && value.length) {
              ret[key] = value;
            } else if (value !== undefined && value !== null && String(value).trim() !== "") {
              ret[key] = value;
            }
          });
          return ret;
        },
        buildPublicNoteObject: function (silent) {
          if (this.publicNoteTab === "raw") {
            const parsed = this.parsePublicNoteRaw();
            if (parsed === false) {
              if (!silent) {
                this.error = t("PleaseEnterValidJSON");
              }
              return false;
            }
            return parsed;
          }
          const data = clone(this.publicNoteBase);
          const billing = this.filterEmpty(Object.assign({}, this.publicNote.billingDataMod, {
            autoRenewal: this.switchString(this.publicNote.billingDataMod.autoRenewal),
            endDate: this.endDateUnlimited ? "0000-00-00" : this.publicNote.billingDataMod.endDate,
          }));
          const billingHasFields = Object.keys(billing).some(function (key) { return key !== "autoRenewal"; });
          const billingHadSwitch = !!(this.publicNoteBase.billingDataMod && Object.prototype.hasOwnProperty.call(this.publicNoteBase.billingDataMod, "autoRenewal"));
          if (billingHasFields || billing.autoRenewal === "1" || billingHadSwitch) {
            data.billingDataMod = billing;
          } else {
            delete data.billingDataMod;
          }
          const plan = this.filterEmpty(Object.assign({}, this.publicNote.planDataMod, {
            IPv4: this.switchString(this.publicNote.planDataMod.IPv4),
            IPv6: this.switchString(this.publicNote.planDataMod.IPv6),
            networkRoute: this.publicNote.planDataMod.networkRoute.join(","),
            extra: this.publicNote.planDataMod.extra.join(","),
          }));
          const planHasFields = Object.keys(plan).some(function (key) { return key !== "IPv4" && key !== "IPv6"; });
          const planHadSwitch = !!(this.publicNoteBase.planDataMod && (
            Object.prototype.hasOwnProperty.call(this.publicNoteBase.planDataMod, "IPv4") ||
            Object.prototype.hasOwnProperty.call(this.publicNoteBase.planDataMod, "IPv6")
          ));
          if (planHasFields || plan.IPv4 === "1" || plan.IPv6 === "1" || planHadSwitch) {
            data.planDataMod = plan;
          } else {
            delete data.planDataMod;
          }
          const customData = this.filterEmpty(this.publicNote.customData);
          if (Object.keys(customData).length) {
            data.customData = customData;
          } else {
            delete data.customData;
          }
          return data;
        },
        loadRulesFromRaw: function () {
          let parsed = [];
          try {
            parsed = JSON.parse(this.form.RulesRaw || "[]");
          } catch (error) {
            parsed = [];
          }
          this.rules = parsed.map(this.normalizeRule);
          if (!this.rules.length) {
            this.addRule();
          }
        },
        normalizeRule: function (rule) {
          const ignoreIds = rule && rule.ignore ? Object.keys(rule.ignore).map(Number).filter(Number.isFinite) : [];
          const unit = ruleAutoUnit(rule.type, rule.max !== undefined ? rule.max : rule.min);
          return {
            type: rule.type || "cpu",
            unit: unit,
            min: ruleToDisplay(rule.type, rule.min, unit),
            max: ruleToDisplay(rule.type, rule.max, unit),
            duration: rule.duration || 60,
            cover: rule.cover || 0,
            ignoreIds: ignoreIds,
            cycle_start: rule.cycle_start ? this.rfc3339ToLocal(rule.cycle_start) : "",
            cycle_interval: rule.cycle_interval || 1,
            cycle_unit: rule.cycle_unit || "hour",
          };
        },
        addRule: function () {
          this.rules.push(this.normalizeRule({}));
        },
        removeRule: function (index) {
          this.rules.splice(index, 1);
          if (!this.rules.length) {
            this.addRule();
          }
        },
        isCycleRule: function (type) {
          return type && type.endsWith("_cycle");
        },
        ruleUnitMeta: ruleUnitMeta,
        ruleUnitOptions: function (type) {
          const meta = ruleUnitMeta(type);
          return meta ? meta.units : [];
        },
        ruleUnitLabel: function (type) {
          const meta = ruleUnitMeta(type);
          return meta && meta.units.length === 1 ? meta.units[0].label : "";
        },
        onThresholdUnitChange: function (rule) {
          const meta = ruleUnitMeta(rule.type);
          const prev = rule._prevUnit;
          if (!meta || !prev || prev === rule.unit) {
            return;
          }
          const prevUnit = meta.units.find(function (u) { return u.value === prev; });
          const newUnit = meta.units.find(function (u) { return u.value === rule.unit; });
          if (!prevUnit || !newUnit) {
            return;
          }
          const ratio = prevUnit.factor / newUnit.factor;
          if (rule.min !== null && rule.min !== "" && rule.min !== undefined) {
            rule.min = Number((Number(rule.min) * ratio).toFixed(6));
          }
          if (rule.max !== null && rule.max !== "" && rule.max !== undefined) {
            rule.max = Number((Number(rule.max) * ratio).toFixed(6));
          }
        },
        onRuleTypeChange: function (rule) {
          const prevType = rule._prevType;
          if (!prevType || prevType === rule.type) {
            return;
          }
          const prevMeta = ruleUnitMeta(prevType);
          const prevUnit = rule._prevUnit || "";
          let rawMin, rawMax;
          if (prevMeta) {
            rawMin = ruleToRaw(prevType, rule.min, prevUnit);
            rawMax = ruleToRaw(prevType, rule.max, prevUnit);
          } else {
            rawMin = rule.min;
            rawMax = rule.max;
          }
          const unit = ruleAutoUnit(rule.type, rawMax !== undefined ? rawMax : rawMin);
          rule.unit = unit;
          rule.min = ruleToDisplay(rule.type, rawMin, unit);
          rule.max = ruleToDisplay(rule.type, rawMax, unit);
        },
        localToRFC3339: function (value) {
          if (!value) {
            return "";
          }
          const normalized = String(value).replace(" ", "T");
          const d = new Date(normalized);
          if (Number.isNaN(d.getTime())) {
            return "";
          }
          const pad = function (n) { return String(n).padStart(2, "0"); };
          const offset = -d.getTimezoneOffset();
          const sign = offset >= 0 ? "+" : "-";
          return normalized + ":00" + sign + pad(Math.floor(Math.abs(offset) / 60)) + ":" + pad(Math.abs(offset) % 60);
        },
        rfc3339ToLocal: function (value) {
          const d = new Date(value);
          if (Number.isNaN(d.getTime())) {
            return "";
          }
          const pad = function (n) { return String(n).padStart(2, "0"); };
          return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) + " " + pad(d.getHours()) + ":" + pad(d.getMinutes());
        },
        buildRulesRaw: function () {
          if (this.ruleTab === "raw") {
            JSON.parse(this.form.RulesRaw || "[]");
            return this.form.RulesRaw || "[]";
          }
          const rules = this.rules.map((rule) => {
            const item = {
              type: rule.type,
              cover: Number(rule.cover || 0),
              ignore: toIDMap(rule.ignoreIds),
            };
            if (rule.min !== null && rule.min !== "" && rule.min !== undefined) item.min = ruleToRaw(rule.type, rule.min, rule.unit);
            if (rule.max !== null && rule.max !== "" && rule.max !== undefined) item.max = ruleToRaw(rule.type, rule.max, rule.unit);
            if (this.isCycleRule(rule.type)) {
              item.cycle_start = this.localToRFC3339(rule.cycle_start);
              item.cycle_interval = Number(rule.cycle_interval || 1);
              item.cycle_unit = rule.cycle_unit || "hour";
            } else {
              item.duration = Number(rule.duration || 60);
            }
            if (!Object.keys(item.ignore).length) {
              delete item.ignore;
            }
            return item;
          });
          return JSON.stringify(rules);
        },
        preparePayload: function () {
          const f = this.form;
          if (this.kind === "api") {
            return { Note: f.Note || "" };
          }
          if (this.kind === "notification") {
            return Object.assign({}, f, { VerifySSL: onOff(f.VerifySSL), SkipCheck: onOff(f.SkipCheck) });
          }
          if (this.kind === "ddns") {
            return Object.assign({}, f, {
              MaxRetries: Number(f.MaxRetries || 0),
              Provider: Number(f.Provider || 0),
              WebhookMethod: Number(f.WebhookMethod || 1),
              WebhookRequestType: Number(f.WebhookRequestType || 1),
              EnableIPv4: onOff(f.EnableIPv4),
              EnableIPv6: onOff(f.EnableIPv6),
            });
          }
          if (this.kind === "nat") {
            return Object.assign({}, f, { ServerID: Number(f.ServerID || 0) });
          }
          if (this.kind === "cron") {
            return {
              ID: Number(f.ID || 0),
              TaskType: Number(f.TaskType || 0),
              Name: f.Name || "",
              Scheduler: f.Scheduler || "",
              Command: f.Command || "",
              ServersRaw: JSON.stringify(parseIDArray(f.Servers)),
              Cover: Number(f.Cover || 0),
              PushSuccessful: onOff(f.PushSuccessful),
              NotificationTag: f.NotificationTag || "default",
            };
          }
          if (this.kind === "monitor") {
            return {
              ID: Number(f.ID || 0),
              Name: f.Name || "",
              Target: f.Target || "",
              Type: Number(f.Type || 1),
              Cover: Number(f.Cover || 0),
              Notify: onOff(f.Notify),
              NotificationTag: f.NotificationTag || "default",
              SkipServersRaw: JSON.stringify(parseIDArray(f.SkipServers)),
              Duration: Number(f.Duration || 30),
              MinLatency: Number(f.MinLatency || 0),
              MaxLatency: Number(f.MaxLatency || 0),
              LatencyNotify: onOff(f.LatencyNotify),
              EnableTriggerTask: onOff(f.EnableTriggerTask),
              EnableShowInService: onOff(f.EnableShowInService),
              FailTriggerTasksRaw: JSON.stringify(parseIDArray(f.FailTriggerTasks)),
              RecoverTriggerTasksRaw: JSON.stringify(parseIDArray(f.RecoverTriggerTasks)),
            };
          }
          if (this.kind === "rule") {
            const rulesRaw = this.buildRulesRaw();
            return {
              ID: Number(f.ID || 0),
              Name: f.Name || "",
              RulesRaw: rulesRaw,
              FailTriggerTasksRaw: JSON.stringify(parseIDArray(f.FailTriggerTasks)),
              RecoverTriggerTasksRaw: JSON.stringify(parseIDArray(f.RecoverTriggerTasks)),
              NotificationTag: f.NotificationTag || "default",
              TriggerMode: Number(f.TriggerMode || 0),
              Enable: onOff(f.Enable),
            };
          }
          if (this.kind === "server") {
            const noteObject = this.buildPublicNoteObject(false);
            if (noteObject === false) {
              return false;
            }
            return {
              ID: Number(f.id || f.ID || 0),
              Name: f.name || f.Name || "",
              DisplayIndex: Number(f.DisplayIndex || 0),
              Secret: f.secret || f.Secret || "",
              Tag: f.Tag || "",
              Note: f.Note || "",
              PublicNote: noteObject && Object.keys(noteObject).length ? JSON.stringify(noteObject, null, 2) : "",
              HideForGuest: onOff(f.HideForGuest),
              EnableDDNS: onOff(f.EnableDDNS),
              DDNSProfilesRaw: JSON.stringify(parseIDArray(f.DDNSProfiles)),
            };
          }
          return f;
        },
        endpoint: function () {
          return {
            api: "/api/token",
            notification: "/api/notification",
            ddns: "/api/ddns",
            nat: "/api/nat",
            cron: "/api/cron",
            monitor: "/api/monitor",
            rule: "/api/alert-rule",
            server: "/api/server",
          }[this.kind];
        },
        submit: async function () {
          this.error = "";
          let payload;
          try {
            payload = this.preparePayload();
          } catch (error) {
            this.error = error.message || String(error);
            return;
          }
          if (payload === false) {
            return;
          }
          this.loading = true;
          try {
            ensureOK(await postJSON(this.endpoint(), payload));
            this.visible = false;
            window.location.reload();
          } catch (error) {
            this.error = error.message || String(error);
          } finally {
            this.loading = false;
          }
        },
      },
      template: `
        <el-dialog v-model="visible" :title="title" :width="dialogWidth" :lock-scroll="true" :before-close="confirmClose" append-to-body destroy-on-close>
          <div class="dashboard-dialog-body-inner">
            <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" class="dashboard-dialog-error" />
            <el-form label-position="top" class="dashboard-dialog-form" @submit.prevent>
              <template v-if="kind === 'api'">
                <el-form-item :label="t('Note')"><el-input v-model="form.Note" type="textarea" :rows="4" /></el-form-item>
              </template>

            <template v-if="kind === 'notification'">
              <div class="dashboard-dialog-grid">
                <el-form-item :label="t('Name')"><el-input v-model="form.Name" /></el-form-item>
                <el-form-item :label="t('Tag')"><el-input v-model="form.Tag" placeholder="default" /></el-form-item>
                <el-form-item class="dashboard-dialog-grid-full" label="URL"><el-input v-model="form.URL" /></el-form-item>
                <el-form-item :label="t('RequestMethod')"><el-select v-model="form.RequestMethod"><el-option label="GET" :value="1" /><el-option label="POST" :value="2" /></el-select></el-form-item>
                <el-form-item :label="t('RequestType')"><el-select v-model="form.RequestType"><el-option label="JSON" :value="1" /><el-option label="FORM" :value="2" /></el-select></el-form-item>
                <el-form-item class="dashboard-dialog-grid-full" label="Header"><el-input v-model="form.RequestHeader" type="textarea" :rows="4" placeholder='{"User-Agent":"Santaizi-Agent"}' /></el-form-item>
                <el-form-item class="dashboard-dialog-grid-full" label="Body"><el-input v-model="form.RequestBody" type="textarea" :rows="6" /></el-form-item>
                <el-form-item :label="t('VerifySSL')"><el-switch v-model="form.VerifySSL" active-value="on" inactive-value="off" /></el-form-item>
                <el-form-item :label="t('DoNotSendTestMessages')"><el-switch v-model="form.SkipCheck" active-value="on" inactive-value="off" /></el-form-item>
              </div>
            </template>

            <template v-if="kind === 'ddns'">
              <div class="dashboard-dialog-grid">
                <el-form-item :label="t('Name')"><el-input v-model="form.Name" /></el-form-item>
                <el-form-item :label="t('DDNSProvider')"><el-select v-model="form.Provider"><el-option v-for="item in providers" :key="item.ID" :label="item.Name" :value="Number(item.ID)" /></el-select></el-form-item>
                <el-form-item class="dashboard-dialog-grid-full" :label="t('DDNSDomains')"><el-input v-model="form.DomainsRaw" placeholder="www.example.com" /></el-form-item>
                <el-form-item v-if="provider.AccessID" :label="t('DDNSAccessID')"><el-input v-model="form.AccessID" /></el-form-item>
                <el-form-item v-if="provider.AccessSecret" :label="t('DDNSAccessSecret')"><el-input v-model="form.AccessSecret" /></el-form-item>
                <el-form-item :label="t('MaxRetries')"><el-input-number v-model="form.MaxRetries" :min="1" :max="10" /></el-form-item>
                <el-form-item :label="t('EnableIPv4')"><el-switch v-model="form.EnableIPv4" active-value="on" inactive-value="off" /></el-form-item>
                <el-form-item :label="t('EnableIPv6')"><el-switch v-model="form.EnableIPv6" active-value="on" inactive-value="off" /></el-form-item>
                <el-form-item v-if="provider.WebhookURL" class="dashboard-dialog-grid-full" :label="t('WebhookURL')"><el-input v-model="form.WebhookURL" /></el-form-item>
                <el-form-item v-if="provider.WebhookMethod" :label="t('WebhookMethod')"><el-select v-model="form.WebhookMethod"><el-option label="GET" :value="1" /><el-option label="POST" :value="2" /><el-option label="PATCH" :value="3" /><el-option label="DELETE" :value="4" /><el-option label="PUT" :value="5" /></el-select></el-form-item>
                <el-form-item v-if="provider.WebhookRequestType" :label="t('WebhookRequestType')"><el-select v-model="form.WebhookRequestType"><el-option label="JSON" :value="1" /><el-option label="Form" :value="2" /></el-select></el-form-item>
                <el-form-item v-if="provider.WebhookHeaders" class="dashboard-dialog-grid-full" :label="t('WebhookHeaders')"><el-input v-model="form.WebhookHeaders" type="textarea" :rows="4" /></el-form-item>
                <el-form-item v-if="provider.WebhookRequestBody" class="dashboard-dialog-grid-full" :label="t('WebhookRequestBody')"><el-input v-model="form.WebhookRequestBody" type="textarea" :rows="5" /></el-form-item>
              </div>
            </template>

            <template v-if="kind === 'nat'">
              <div class="dashboard-dialog-grid">
                <el-form-item :label="t('Name')"><el-input v-model="form.Name" /></el-form-item>
                <el-form-item label="Agent ID"><el-input-number v-model="form.ServerID" :min="0" /></el-form-item>
                <el-form-item :label="t('LocalService')"><el-input v-model="form.Host" /></el-form-item>
                <el-form-item :label="t('BindHostname')"><el-input v-model="form.Domain" /></el-form-item>
              </div>
            </template>

            <template v-if="kind === 'cron'">
              <div class="dashboard-dialog-grid">
                <el-form-item :label="t('Name')"><el-input v-model="form.Name" /></el-form-item>
                <el-form-item :label="t('TaskType')"><el-select v-model="form.TaskType"><el-option :label="t('CronTask')" :value="0" /><el-option :label="t('TriggerTask')" :value="1" /></el-select></el-form-item>
                <el-form-item :label="t('Scheduler')"><el-input v-model="form.Scheduler" placeholder="0 0 3 * * *" /></el-form-item>
                <el-form-item :label="t('Coverage')"><el-select v-model="form.Cover"><el-option :label="t('IgnoreAllAndExecuteOnlyThroughSpecificServers')" :value="0" /><el-option :label="t('AllIncludedOnlySpecificServersAreNotExecuted')" :value="1" /><el-option :label="t('ExecuteByTriggerServer')" :value="2" /></el-select></el-form-item>
                <el-form-item class="dashboard-dialog-grid-full" :label="t('Command')"><el-input v-model="form.Command" type="textarea" :rows="5" /></el-form-item>
                <el-form-item class="dashboard-dialog-grid-full" :label="t('SpecificServers')"><el-transfer v-model="form.Servers" :data="remote.servers" :props="{key:'value', label:'label'}" filterable :titles="[t('Server'), t('SpecificServers')]" class="dashboard-server-transfer" /></el-form-item>
                <el-form-item :label="t('NotificationMethodGroup')"><el-input v-model="form.NotificationTag" placeholder="default" /></el-form-item>
                <el-form-item :label="t('PushSuccessMessages')"><el-switch v-model="form.PushSuccessful" active-value="on" inactive-value="off" /></el-form-item>
              </div>
            </template>

            <template v-if="kind === 'monitor'">
              <div class="dashboard-monitor-editor">
                <section class="dashboard-editor-section">
                  <div class="dashboard-section-title"><i class="ri-server-line"></i>{{t('ServerBasicInfo')}}</div>
                  <div class="dashboard-dialog-grid">
                    <el-form-item :label="t('Name')"><el-input v-model="form.Name" /></el-form-item>
                    <el-form-item :label="t('Type')"><el-select v-model="form.Type"><el-option :label="'HTTP-GET ' + t('SslExpirationOrChange')" :value="1" /><el-option label="ICMP-Ping" :value="2" /><el-option label="TCP-Ping" :value="3" /></el-select></el-form-item>
                    <el-form-item class="dashboard-dialog-grid-full" :label="t('Target')"><el-input v-model="form.Target" placeholder="HTTP (https://t.tt)｜Ping (t.tt)｜TCP (t.tt:80)" /></el-form-item>
                  </div>
                </section>

                <section class="dashboard-editor-section">
                  <div class="dashboard-section-title"><i class="ri-timer-line"></i>{{t('Monitor')}}</div>
                  <div class="dashboard-dialog-grid">
                    <el-form-item :label="t('Duration')"><el-input-number v-model="form.Duration" :min="1" /></el-form-item>
                    <el-form-item :label="t('Coverage')"><el-select v-model="form.Cover"><el-option :label="t('AllIncludedOnlySpecificServersAreNotRequest')" :value="0" /><el-option :label="t('IgnoreAllRequestOnlyThroughSpecificServers')" :value="1" /></el-select></el-form-item>
                    <el-form-item class="dashboard-dialog-grid-full" :label="t('SpecificServers')"><el-transfer v-model="form.SkipServers" :data="remote.servers" :props="{key:'value', label:'label'}" filterable :titles="[t('Server'), t('SpecificServers')]" class="dashboard-server-transfer" /></el-form-item>
                  </div>
                </section>

                <section class="dashboard-editor-section">
                  <div class="dashboard-section-title"><i class="ri-notification-3-line"></i>{{t('NotificationMethod')}}</div>
                  <div class="dashboard-dialog-grid">
                    <el-form-item :label="t('NotificationMethodGroup')"><el-input v-model="form.NotificationTag" placeholder="default" /></el-form-item>
                    <el-form-item :label="t('EnableFailureNotification')"><el-switch v-model="form.Notify" active-value="on" inactive-value="off" /></el-form-item>
                    <el-form-item :label="t('EnableLatencyNotification')"><el-switch v-model="form.LatencyNotify" active-value="on" inactive-value="off" /></el-form-item>
                    <el-form-item :label="t('MaxLatency')"><el-input-number v-model="form.MaxLatency" :min="0" /></el-form-item>
                    <el-form-item :label="t('MinLatency')"><el-input-number v-model="form.MinLatency" :min="0" /></el-form-item>
                  </div>
                </section>

                <section class="dashboard-editor-section">
                  <div class="dashboard-section-title"><i class="ri-settings-3-line"></i>{{t('Settings')}}</div>
                  <div class="dashboard-dialog-grid">
                    <el-form-item :label="t('EnableShowInService')"><el-switch v-model="form.EnableShowInService" active-value="on" inactive-value="off" /></el-form-item>
                    <el-form-item :label="t('EnableTriggerTask')"><el-switch v-model="form.EnableTriggerTask" active-value="on" inactive-value="off" /></el-form-item>
                    <el-form-item :label="t('FailTriggerTasks')"><el-select v-model="form.FailTriggerTasks" multiple filterable remote clearable :remote-method="q => remoteSearch('tasks', q)"><el-option v-for="item in remote.tasks" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
                    <el-form-item :label="t('RecoverTriggerTasks')"><el-select v-model="form.RecoverTriggerTasks" multiple filterable remote clearable :remote-method="q => remoteSearch('tasks', q)"><el-option v-for="item in remote.tasks" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
                  </div>
                </section>
              </div>
              <div class="dashboard-dialog-tip" v-html="t('IntroductionOfMonitor')"></div>
            </template>

            <template v-if="kind === 'rule'">
              <div class="dashboard-dialog-grid">
                <el-form-item :label="t('Name')"><el-input v-model="form.Name" /></el-form-item>
                <el-form-item :label="t('NotificationTriggerMode')"><el-select v-model="form.TriggerMode"><el-option :label="t('ModeAlwaysTrigger')" :value="0" /><el-option :label="t('ModeOnetimeTrigger')" :value="1" /></el-select></el-form-item>
                <el-form-item :label="t('NotificationMethodGroup')"><el-input v-model="form.NotificationTag" placeholder="default" /></el-form-item>
                <el-form-item :label="t('Enable')"><el-switch v-model="form.Enable" active-value="on" inactive-value="off" /></el-form-item>
                <el-form-item :label="t('FailTriggerTasks')"><el-select v-model="form.FailTriggerTasks" multiple filterable remote clearable :remote-method="q => remoteSearch('tasks', q)"><el-option v-for="item in remote.tasks" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
                <el-form-item :label="t('RecoverTriggerTasks')"><el-select v-model="form.RecoverTriggerTasks" multiple filterable remote clearable :remote-method="q => remoteSearch('tasks', q)"><el-option v-for="item in remote.tasks" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
              </div>
              <el-tabs v-model="ruleTab">
                <el-tab-pane :label="t('VisualEditor')" name="visual">
                  <div class="dashboard-dialog-form">
                    <div v-for="(rule, index) in rules" :key="index" class="dashboard-rule-item">
                      <div class="dashboard-rule-header">
                        <span class="dashboard-rule-title">#{{index + 1}}</span>
                        <button type="button" class="dashboard-icon-button dashboard-icon-button-danger" @click="removeRule(index)" :title="t('RemoveRule')"><i class="ri-delete-bin-6-line"></i></button>
                      </div>
                      <div class="dashboard-dialog-grid">
                        <el-form-item :label="t('RuleType')">
                          <el-select v-model="rule.type" class="dashboard-rule-type-select" @focus="rule._prevType = rule.type; rule._prevUnit = rule.unit" @change="onRuleTypeChange(rule)">
                            <el-option v-for="type in ruleTypes" :key="type" :label="type" :value="type">
                              <span class="dashboard-rule-type-option-label">{{type}}</span>
                              <span class="dashboard-rule-type-option-desc">{{ruleTypeDesc(type)}}</span>
                            </el-option>
                          </el-select>
                        </el-form-item>
                        <el-form-item :label="t('Coverage')"><el-select v-model="rule.cover"><el-option :label="t('AllIncludedOnlySpecificServersAreNotAlerted')" :value="0" /><el-option :label="t('IgnoreAllOnlyAlertSpecificServers')" :value="1" /></el-select></el-form-item>
                        <el-form-item :label="t('MinThreshold')">
                          <div class="dashboard-threshold-row">
                            <el-input-number v-model="rule.min" />
                            <template v-if="ruleUnitMeta(rule.type)">
                              <el-select v-if="ruleUnitOptions(rule.type).length > 1" v-model="rule.unit" class="dashboard-threshold-unit" @focus="rule._prevUnit = rule.unit" @change="onThresholdUnitChange(rule)"><el-option v-for="u in ruleUnitOptions(rule.type)" :key="u.value" :label="u.label" :value="u.value" /></el-select>
                              <span v-else class="dashboard-threshold-unit-text">{{ruleUnitLabel(rule.type)}}</span>
                            </template>
                          </div>
                        </el-form-item>
                        <el-form-item :label="t('MaxThreshold')">
                          <div class="dashboard-threshold-row">
                            <el-input-number v-model="rule.max" />
                            <template v-if="ruleUnitMeta(rule.type)">
                              <el-select v-if="ruleUnitOptions(rule.type).length > 1" v-model="rule.unit" class="dashboard-threshold-unit" @focus="rule._prevUnit = rule.unit" @change="onThresholdUnitChange(rule)"><el-option v-for="u in ruleUnitOptions(rule.type)" :key="u.value" :label="u.label" :value="u.value" /></el-select>
                              <span v-else class="dashboard-threshold-unit-text">{{ruleUnitLabel(rule.type)}}</span>
                            </template>
                          </div>
                        </el-form-item>
                        <template v-if="isCycleRule(rule.type)">
                          <el-form-item :label="t('CycleStart')"><el-date-picker v-model="rule.cycle_start" type="datetime" value-format="YYYY-MM-DD HH:mm" format="YYYY-MM-DD HH:mm" /></el-form-item>
                          <el-form-item :label="t('CycleInterval')"><el-input-number v-model="rule.cycle_interval" :min="1" /></el-form-item>
                          <el-form-item :label="t('CycleUnit')"><el-select v-model="rule.cycle_unit"><el-option v-for="unit in cycleUnits" :key="unit" :label="unit" :value="unit" /></el-select></el-form-item>
                        </template>
                        <template v-else>
                          <el-form-item :label="t('Duration')"><el-input-number v-model="rule.duration" :min="3" /></el-form-item>
                        </template>
                        <el-form-item class="dashboard-dialog-grid-full" :label="t('SpecificServers')"><el-transfer v-model="rule.ignoreIds" :data="remote.servers" :props="{key:'value', label:'label'}" filterable :titles="[t('Server'), t('SpecificServers')]" class="dashboard-server-transfer" /></el-form-item>
                      </div>
                    </div>
                    <button type="button" class="dashboard-button dashboard-button-small" @click="addRule"><i class="ri-add-line"></i>{{t('AddRule')}}</button>
                  </div>
                </el-tab-pane>
                <el-tab-pane :label="t('RawJSON')" name="raw"><el-input v-model="form.RulesRaw" type="textarea" :rows="10" /></el-tab-pane>
              </el-tabs>
            </template>

            <template v-if="kind === 'server'">
              <div class="dashboard-server-editor">
                <el-collapse v-model="activeCollapse" class="dashboard-server-collapse">
                  <el-collapse-item :title="t('ServerBasicInfo')" name="basic">
                    <section class="dashboard-editor-section">
                      <div class="dashboard-server-basic-grid">
                        <el-form-item :label="t('Name')"><el-input v-model="form.name" /></el-form-item>
                        <el-form-item :label="t('ServerGroup')"><el-input v-model="form.Tag" /></el-form-item>
                        <el-form-item class="dashboard-compact-field" :label="t('DisplayIndex')"><el-input-number v-model="form.DisplayIndex" /></el-form-item>
                      </div>
                    </section>
                  </el-collapse-item>

                  <el-collapse-item :title="t('AccessAndVisibility')" name="access">
                    <section class="dashboard-editor-section">
                      <div class="dashboard-server-access-grid">
                        <el-form-item v-if="form.id" class="dashboard-server-secret-field" :label="t('Secret')"><el-input v-model="form.secret"><template #append><el-button @click="showConfirm(t('ResetSecret'), t('ConfirmToResetSecret'), resetServerSecret, form.id)"><i class="ri-key-2-line"></i></el-button></template></el-input></el-form-item>
                        <el-form-item class="dashboard-server-ddns-field" :label="t('DDNSProfiles')"><el-select v-model="form.DDNSProfiles" multiple filterable remote clearable :remote-method="q => remoteSearch('ddns', q)"><el-option v-for="item in remote.ddns" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
                        <div class="dashboard-server-toggle-row">
                          <div class="dashboard-toggle-card">
                            <span>{{t('EnableDDNS')}}</span>
                            <el-switch v-model="form.EnableDDNS" active-value="on" inactive-value="off" />
                          </div>
                          <div class="dashboard-toggle-card">
                            <span>{{t('HideForGuest')}}</span>
                            <el-switch v-model="form.HideForGuest" active-value="on" inactive-value="off" />
                          </div>
                        </div>
                      </div>
                    </section>
                  </el-collapse-item>

                  <el-collapse-item :title="t('InternalNote')" name="note">
                    <section class="dashboard-editor-section">
                      <el-form-item :label="t('Note')"><el-input v-model="form.Note" type="textarea" :rows="6" /></el-form-item>
                    </section>
                  </el-collapse-item>

                  <el-collapse-item :title="t('PublicNote')" name="public">
                    <section class="dashboard-editor-section dashboard-public-note-editor">
                      <el-tabs v-model="publicNoteTab" class="dashboard-public-note-tabs">
                        <el-tab-pane :label="t('BillingInfo')" name="billing">
                          <div class="dashboard-public-note-grid">
                            <el-form-item :label="t('StartDate')"><el-date-picker v-model="publicNote.billingDataMod.startDate" type="date" value-format="YYYY-MM-DD" format="YYYY-MM-DD" clearable /></el-form-item>
                            <el-form-item :label="t('EndDate')" class="dashboard-date-with-toggle"><div class="dashboard-date-row"><el-checkbox v-model="endDateUnlimited">{{t('UnlimitedDuration')}}</el-checkbox><el-date-picker v-model="publicNote.billingDataMod.endDate" type="date" value-format="YYYY-MM-DD" format="YYYY-MM-DD" :disabled="endDateUnlimited" clearable /></div></el-form-item>
                            <el-form-item :label="t('Amount')" class="dashboard-public-note-span-2"><el-input v-model="publicNote.billingDataMod.amount" :placeholder="t('AmountPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('BillingCycle')" class="dashboard-public-note-span-2"><el-autocomplete v-model="publicNote.billingDataMod.cycle" :fetch-suggestions="(q, cb) => cb(cycles.filter(i => !q || i.includes(q)).map(i => ({ value: i })))" :placeholder="t('BillingCyclePlaceholder')" clearable /></el-form-item>
                            <div class="dashboard-toggle-card dashboard-public-note-toggle">
                              <span>{{t('AutoRenewal')}}</span>
                              <el-switch v-model="publicNote.billingDataMod.autoRenewal" active-value="1" inactive-value="0" />
                            </div>
                          </div>
                        </el-tab-pane>
                        <el-tab-pane :label="t('PlanInfo')" name="plan">
                          <div class="dashboard-public-note-grid dashboard-public-note-grid-2col">
                            <el-form-item :label="t('Bandwidth')"><el-input v-model="publicNote.planDataMod.bandwidth" :placeholder="t('BandwidthPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('TrafficVol')"><el-input v-model="publicNote.planDataMod.trafficVol" :placeholder="t('TrafficVolPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('TrafficType')"><el-select v-model="publicNote.planDataMod.trafficType" clearable><el-option v-for="item in trafficTypes" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item>
                            <div class="dashboard-switch-pair">
                              <el-form-item label="IPv4" class="dashboard-form-item-switch"><el-switch v-model="publicNote.planDataMod.IPv4" active-value="1" inactive-value="0" /></el-form-item>
                              <el-form-item label="IPv6" class="dashboard-form-item-switch"><el-switch v-model="publicNote.planDataMod.IPv6" active-value="1" inactive-value="0" /></el-form-item>
                            </div>
                            <el-form-item :label="t('NetworkRoute')" class="dashboard-public-note-span-2"><el-input-tag v-model="publicNote.planDataMod.networkRoute" :placeholder="t('NetworkRoutePlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('Extra')" class="dashboard-public-note-span-2"><el-input-tag v-model="publicNote.planDataMod.extra" :placeholder="t('ExtraPlaceholder')" clearable /></el-form-item>
                          </div>
                        </el-tab-pane>
                        <el-tab-pane label="CustomData" name="custom">
                          <div class="dashboard-public-note-grid">
                            <el-form-item :label="t('LocationCode')"><el-input v-model="publicNote.customData.location" :placeholder="t('LocationCodePlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('Flag')"><el-input v-model="publicNote.customData.flag" :placeholder="t('FlagPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('OrderLink')" class="dashboard-public-note-span-2"><el-input v-model="publicNote.customData.orderLink" :placeholder="t('OrderLinkPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('BuyBtnText')"><el-input v-model="publicNote.customData.buyBtnText" :placeholder="t('BuyBtnTextPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('BuyBtnIcon')"><el-input v-model="publicNote.customData.buyBtnIcon" :placeholder="t('BuyBtnIconPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('Slogan')" class="dashboard-public-note-span-2"><el-input v-model="publicNote.customData.slogan" :placeholder="t('SloganPlaceholder')" clearable /></el-form-item>
                            <el-form-item :label="t('Latitude')"><el-input v-model="publicNote.customData.lat" clearable /></el-form-item>
                            <el-form-item :label="t('Longitude')"><el-input v-model="publicNote.customData.lng" clearable /></el-form-item>
                            <el-form-item :label="t('LatLng')"><el-input v-model="publicNote.customData.latlng" clearable /></el-form-item>
                            <el-form-item :label="t('LocationLabel')"><el-input v-model="publicNote.customData.locationLabel" clearable /></el-form-item>
                          </div>
                        </el-tab-pane>
                        <el-tab-pane :label="t('RawJSON')" name="raw"><el-input v-model="publicNoteRaw" type="textarea" :rows="6" placeholder="{}" /></el-tab-pane>
                      </el-tabs>
                      <el-form-item class="dashboard-json-preview-field" :label="t('FinalJSONPreview')"><pre class="dashboard-public-note-preview"><code v-html="highlightedPublicNotePreview"></code></pre></el-form-item>
                      <el-alert v-if="publicNoteInvalid" :title="t('PleaseEnterValidJSON')" type="error" show-icon :closable="false" />
                    </section>
                  </el-collapse-item>
                </el-collapse>
              </div>
            </template>
            </el-form>
          </div>
          <template #footer>
            <el-button @click="visible = false">{{t('Cancel')}}</el-button>
            <el-button type="primary" :loading="loading" @click="submit">{{t('Confirm')}}</el-button>
          </template>
        </el-dialog>
      `,
    });

    app.use(ElementPlus, { locale: elementLocale() });
    window.dashboardModal = app.mount("#dashboard-modal-root");
  }

  document.addEventListener("DOMContentLoaded", function () {
    installCopyHandler();
    installMobileNav();
    createModalApp();
  });
})();
