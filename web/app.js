
    const DEFAULT_ACCOUNT_FIELDS = {
      email: "email",
      password: "password",
      mailbox_url: "mailbox_url"
    };
    const DEFAULT_CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann";
    const DEFAULT_PRIVACY_MODE = "training_off";
    const AUTH_STATUS_LABELS = {
      active: "正常",
      disabled: "已禁用",
      unavailable: "不可用"
    };
    const textEncoder = new TextEncoder();
    const textDecoder = new TextDecoder();
    const CPA_PLUGIN_API = "/v0/management/plugins/cpa-account-pilot";

    function readHostStorage() {
      try { return window.parent !== window ? window.parent.localStorage : localStorage; } catch (_) { return null; }
    }

    function decodeHostValue(value) {
      const encryptedPrefix = "enc::v1::";
      const secureSalt = "cli-proxy-api-webui::secure-storage";
      if (typeof value !== "string" || !value.startsWith(encryptedPrefix)) return value;
      try {
        const raw = atob(value.slice(encryptedPrefix.length));
        const bytes = new TextEncoder().encode(`${secureSalt}|${location.host}|${navigator.userAgent}`);
        const decoded = Uint8Array.from(raw, (_, index) => raw.charCodeAt(index) ^ bytes[index % bytes.length]);
        return new TextDecoder().decode(decoded);
      } catch (_) { return value; }
    }

    function getCpaApiSettings() {
      try {
        const storage = readHostStorage();
        if (!storage) return null;
        const read = (name) => {
          const raw = storage.getItem(name);
          if (!raw) return null;
          const decoded = decodeHostValue(raw);
          try { return JSON.parse(decoded); } catch (_) { return decoded; }
        };
        const auth = read("cli-proxy-auth") || {};
        const value = auth && auth.state ? auth.state : auth;
        const base = stringifyValue(value.apiBase || read("apiBase") || location.origin).trim().replace(/\/+$/, "");
        const key = stringifyValue(value.managementKey || read("managementKey")).trim();
        return base && key ? { base, key } : null;
      } catch (error) {
        return null;
      }
    }

    function getApiSettings() {
      return getCpaApiSettings() || { base: "", key: "" };
    }

    function getCpaPreference(names, fallback) {
      const storage = readHostStorage();
      for (const name of names) {
        const raw = storage && storage.getItem(name);
        if (!raw) continue;
        const decoded = decodeHostValue(raw);
        try {
          const parsed = JSON.parse(decoded);
          if (typeof parsed === "string" || typeof parsed === "number") {
            return String(parsed);
          }
          if (parsed && typeof parsed === "object") {
            const value = parsed.value || parsed.mode || parsed.theme;
            if (value) return String(value);
          }
        } catch (_) {
          return decoded;
        }
      }
      try {
        const root = window.parent !== window ? window.parent.document.documentElement : document.documentElement;
        return root.dataset.themeMode || root.dataset.theme || fallback;
      } catch (_) { return fallback; }
    }

    function getCpaTheme() {
      const value = String(getCpaPreference(["theme", "themeMode", "ui_theme"], "auto")).toLowerCase();
      return ["light", "dark", "auto"].includes(value) ? value : "auto";
    }

    const state = {
      accounts: [],
      accountFileText: "",
      accountObjectRanges: [],
      accountFields: getStoredAccountFields(),
      authFileStatuses: new Map(),
      fileHandle: null,
      fileName: "account",
      isLoadingPersistedAccounts: false,
      isReloadingFile: false,
      isRefreshing: false,
      isSettingsOpen: false,
      isStartingLogin: false,
      loginTabWindow: null,
      mailboxFilter: "all",
      statusFilter: "all",
      themeMode: getCpaTheme(),
      lastChanged: 0,
      messageKey: "selectAccountFile",
      messageParams: {},
      messageText: "",
      messageType: "",
      statuses: new Map()
    };

    const elements = {
      pageTitle: document.getElementById("pageTitle"),
      cpaBackBtn: document.getElementById("cpaBackBtn"),
      cpaBackText: document.getElementById("cpaBackText"),
      pageSubtitle: document.getElementById("pageSubtitle"),
      settingsWrap: document.getElementById("settingsWrap"),
      settingsBtn: document.getElementById("settingsBtn"),
      settingsPanel: document.getElementById("settingsPanel"),
      settingsPanelTitle: document.getElementById("settingsPanelTitle"),
      openFileBtn: document.getElementById("openFileBtn"),
      fileInput: document.getElementById("fileInput"),
      reloadFileBtn: document.getElementById("reloadFileBtn"),
      refreshBtn: document.getElementById("refreshBtn"),
      mailboxFilter: document.getElementById("mailboxFilter"),
      mailboxFilterOptions: Array.from(document.querySelectorAll(".mailbox-filter-option")),
      statusFilter: document.getElementById("statusFilter"),
      statusFilterOptions: [],
      toolbar: document.querySelector(".toolbar"),
      accountFieldsTitle: document.getElementById("accountFieldsTitle"),
      emailFieldLabel: document.getElementById("emailFieldLabel"),
      emailFieldInput: document.getElementById("emailFieldInput"),
      passwordFieldLabel: document.getElementById("passwordFieldLabel"),
      passwordFieldInput: document.getElementById("passwordFieldInput"),
      mailboxUrlFieldLabel: document.getElementById("mailboxUrlFieldLabel"),
      mailboxUrlFieldInput: document.getElementById("mailboxUrlFieldInput"),
      searchInput: document.getElementById("searchInput"),
      message: document.getElementById("message"),
      emptyState: document.getElementById("emptyState"),
      tableShell: document.getElementById("tableShell"),
      emailHeader: document.getElementById("emailHeader"),
      passwordHeader: document.getElementById("passwordHeader"),
      mailInfoHeader: document.getElementById("mailInfoHeader"),
      authStatusHeader: document.getElementById("authStatusHeader"),
      authDescriptionHeader: document.getElementById("authDescriptionHeader"),
      actionsHeader: document.getElementById("actionsHeader"),
      accountRows: document.getElementById("accountRows")
    };

    const darkModeQuery = window.matchMedia
      ? window.matchMedia("(prefers-color-scheme: dark)")
      : null;

    elements.settingsBtn.addEventListener("click", handleSettingsClick);
    elements.openFileBtn.addEventListener("click", openJsonFile);
    elements.fileInput.addEventListener("change", handleFileInput);
    elements.reloadFileBtn.addEventListener("click", reloadAccountFile);
    elements.refreshBtn.addEventListener("click", refreshAllAccounts);
    elements.mailboxFilter.addEventListener("click", handleMailboxFilterClick);
    elements.statusFilter.addEventListener("click", handleStatusFilterClick);
    elements.emailFieldInput.addEventListener("change", handleAccountFieldsChange);
    elements.passwordFieldInput.addEventListener("change", handleAccountFieldsChange);
    elements.mailboxUrlFieldInput.addEventListener("change", handleAccountFieldsChange);
    elements.searchInput.addEventListener("input", render);
    elements.accountRows.addEventListener("click", handleRowClick);
    document.addEventListener("click", handleDocumentClick);
    document.addEventListener("keydown", handleDocumentKeydown);
    if (darkModeQuery) {
      if (darkModeQuery.addEventListener) {
        darkModeQuery.addEventListener("change", handleSystemThemeChange);
      } else if (darkModeQuery.addListener) {
        darkModeQuery.addListener(handleSystemThemeChange);
      }
    }

    loadAccountFieldSettings();
    applyTheme();
    applyLocale();
    loadPersistedAccounts();
    window.setInterval(() => {
      const nextTheme = getCpaTheme();
      if (nextTheme !== state.themeMode) {
        state.themeMode = nextTheme;
        applyTheme();
      }
      if (!state.accounts.length) {
        void loadPersistedAccounts();
      }
    }, 1000);

    function getStoredAccountFields() {
      return {
        email: getStoredAccountField("email"),
        password: getStoredAccountField("password"),
        mailbox_url: getStoredAccountField("mailbox_url")
      };
    }

    function getStoredAccountField(key) {
      return localStorage.getItem(`account_mgt_field_${key}`) || DEFAULT_ACCOUNT_FIELDS[key];
    }

    function handleMailboxFilterClick(event) {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }

      const button = target.closest(".mailbox-filter-option");
      if (!button) {
        return;
      }

      const nextFilter = ["all", "with", "without"].includes(button.dataset.mailboxFilter)
        ? button.dataset.mailboxFilter
        : "all";
      if (nextFilter === state.mailboxFilter) {
        return;
      }

      state.mailboxFilter = nextFilter;
      render();
    }

    function handleStatusFilterClick(event) {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }

      const button = target.closest(".status-filter-option");
      if (!button || button.disabled) {
        return;
      }

      state.statusFilter = button.dataset.statusFilter || "all";
      render();
    }

    function handleSettingsClick(event) {
      event.stopPropagation();
      setSettingsOpen(!state.isSettingsOpen);
    }

    function handleDocumentClick(event) {
      const target = event.target;
      if (!state.isSettingsOpen || !(target instanceof Element)) {
        return;
      }

      if (!elements.settingsWrap.contains(target)) {
        setSettingsOpen(false);
      }
    }

    function handleDocumentKeydown(event) {
      if (event.key === "Escape" && state.isSettingsOpen) {
        setSettingsOpen(false);
        elements.settingsBtn.focus();
      }
    }

    function setSettingsOpen(open) {
      state.isSettingsOpen = open;
      elements.settingsPanel.classList.toggle("hidden", !open);
      elements.settingsBtn.classList.toggle("is-active", open);
      elements.settingsBtn.setAttribute("aria-expanded", open ? "true" : "false");
      elements.settingsPanel.setAttribute("aria-hidden", open ? "false" : "true");
    }

    function handleAccountFieldsChange() {
      saveAccountFieldSettings();
    }

    function handleSystemThemeChange() {
      if (state.themeMode === "auto") {
        applyTheme();
      }
    }

    function applyTheme() {
      const effectiveTheme = state.themeMode === "auto" && darkModeQuery && darkModeQuery.matches
        ? "dark"
        : state.themeMode === "dark" ? "dark" : "light";
      document.documentElement.dataset.theme = effectiveTheme;
      document.documentElement.dataset.themeMode = state.themeMode;
    }

    function translate(key, params) {
      const templates = {
        unknownError: "未知错误", accountReadFailed: "账户文件读取失败。", noAccountObjects: "未读取到账户对象。",
        statusReadFailed: "读取失败", browserWritePermissionMissing: "浏览器未授权写入", reloadingList: "正在刷新认证文件状态...",
        reloadList: "刷新认证", refreshing: "刷新中...", refreshFiltered: "刷新筛选数据", refreshAll: "刷新邮箱",
        noMatchingAccounts: "没有匹配的账户。", cpaDownloadAction: "账户文件下载", mailboxAction: "邮箱", loginAction: "登陆",
        missingStatusLabel: "缺失", passwordMaskedTitle: "已脱敏，点击图标复制原始值", copyLabel: "{label}",
        invalidCodexAuthUrl: "管理端未返回有效的 Codex OAuth 授权链接", emailLabel: "邮箱", passwordLabel: "密码", contentLabel: "内容"
      };
      const template = templates[key] || key;
      return template.replace(/\{(\w+)\}/g, (match, name) => {
        return Object.prototype.hasOwnProperty.call(params || {}, name)
          ? stringifyValue(params[name])
          : match;
      });
    }

    function applyLocale() {
      document.documentElement.lang = "zh-CN";
      updateMailboxFilterControl();
      updateStatusFilterControl();
      updateMessage();
      render();
    }

    function updateMailboxFilterControl() {
      const accounts = state.accounts.filter(({ email }) => {
        const status = getAuthFileStatus({ email });
        return state.statusFilter === "all" || matchesStatusFilter(status);
      });
      const labels = {
        all: `全部:${accounts.length}`,
        with: `有邮箱:${accounts.filter(accountHasMailboxUrl).length}`,
        without: `无邮箱:${accounts.filter((account) => !accountHasMailboxUrl(account)).length}`
      };
      elements.mailboxFilterOptions.forEach((button) => {
        const filter = button.dataset.mailboxFilter || "all";
        const active = filter === state.mailboxFilter;
        const label = labels[filter] || filter;
        button.textContent = label;
        button.classList.toggle("is-active", active);
        button.setAttribute("aria-pressed", active ? "true" : "false");
        button.setAttribute("aria-label", label);
        button.title = label;
      });
    }

    function updateStatusFilterControl() {
      const options = getStatusFilterOptions();
      const hasCurrent = state.statusFilter === "all" ||
        options.some((option) => option.value === state.statusFilter);
      if (!hasCurrent) {
        state.statusFilter = "all";
      }

      elements.statusFilter.innerHTML = [
        `<button class="segmented-option status-filter-option" type="button" data-status-filter="all">${escapeHtml(`全部:${getAuthStatusCount()}`)}</button>`,
        ...options.map((option) => {
          return `<button class="segmented-option status-filter-option" type="button" data-status-filter="${escapeHtml(option.value)}">${escapeHtml(`${option.label}:${option.count}`)}</button>`;
        })
      ].join("");
      elements.statusFilterOptions = Array.from(elements.statusFilter.querySelectorAll(".status-filter-option"));
      elements.statusFilterOptions.forEach((button) => {
        const active = button.dataset.statusFilter === state.statusFilter;
        button.classList.toggle("is-active", active);
        button.setAttribute("aria-pressed", active ? "true" : "false");
        button.setAttribute("aria-label", button.textContent || "");
        button.title = button.textContent || "";
      });
    }

    function getAuthStatusCount() {
      return state.accounts.filter((account) => {
        return !isEmptyStatus(getAuthFileStatus(account)) && matchesMailboxFilter(account);
      }).length;
    }

    function setMessageKey(key, type, params) {
      state.messageKey = key;
      state.messageParams = params || {};
      state.messageText = "";
      state.messageType = type || "";
      updateMessage();
    }

    function setStatus(index, type, key, params) {
      state.statuses.set(index, {
        type,
        key,
        params: params || {}
      });
    }

    function getStatusText(status) {
      if (!status) {
        return "";
      }
      if (status.key) {
        return translate(status.key, status.params);
      }
      return status.statusCode
        ? getAuthStatusLabel(status.statusCode)
        : stringifyValue(status.text);
    }

    function openJsonFile() {
      elements.fileInput.value = "";
      elements.fileInput.click();
    }

    async function handleFileInput(event) {
      const file = event.target.files && event.target.files[0];
      if (!file) {
        return;
      }
      state.fileHandle = null;
      state.fileName = file.name || state.fileName;
      await loadFile(file);
    }

    async function reloadAccountFile() {
      if (state.isReloadingFile) {
        return;
      }
      state.isReloadingFile = true;
      render();
      try {
        await loadPersistedAccounts();
      } catch (error) {
        setMessageKey("reloadListFailed", "error", {
          error: error.message || translate("unknownError")
        });
      } finally {
        state.isReloadingFile = false;
        render();
      }
    }

    async function loadPersistedAccounts() {
      const api = getApiSettings();
      if (!api.base || !api.key || state.isLoadingPersistedAccounts) return;
      state.isLoadingPersistedAccounts = true;
      try {
        const response = await fetch(`${api.base}${CPA_PLUGIN_API}/account-files`, {
          method: "GET", cache: "no-store",
          headers: { "Accept": "application/json", "Authorization": `Bearer ${api.key}` }
        });
        const payload = await readJsonResponse(response);
        if (!response.ok) throw new Error(payload.error || `HTTP ${response.status}`);
        const accounts = Array.isArray(payload.accounts) ? payload.accounts : [];
        if (accounts.length) {
          state.accounts = accounts.map(normalizeAccount);
          state.accountFileText = accounts.map((account) => JSON.stringify(prepareAccountForWrite(account))).join("\n") + "\n";
          state.accountObjectRanges = [];
          setMessageKey("loadedAccounts", "success", { count: accounts.length, fileName: "cpa-account-pilot" });
        } else if (!state.fileHandle) {
          state.accounts = [];
          state.accountFileText = "";
          state.accountObjectRanges = [];
          state.authFileStatuses.clear();
        }
        render();
        await refreshAuthFileStatuses();
      } catch (error) {
        setMessage(error.message || translate("accountReadFailed"), "error");
      } finally {
        state.isLoadingPersistedAccounts = false;
      }
    }

    async function loadFile(file) {
      try {
        saveAccountFieldSettings();
        const text = await file.text();
        const records = extractAccountRecords(text);
        if (!records.length) {
          throw new Error(translate("noAccountObjects"));
        }

        state.accounts = records.map((record) => normalizeAccount(record.account));
        state.accountFileText = text;
        state.accountObjectRanges = records.map((record) => ({
          start: record.start,
          end: record.end
        }));
        state.statuses.clear();
        state.authFileStatuses.clear();
        state.lastChanged = 0;
        setMessageKey("loadedAccounts", "success", {
          count: state.accounts.length,
          fileName: state.fileName
        });
        await persistAccountsToCpa();
        render();
        await refreshAuthFileStatuses();
      } catch (error) {
        state.accounts = [];
        state.statuses.clear();
        state.authFileStatuses.clear();
        setMessage(error.message || translate("accountReadFailed"), "error");
        render();
      }
    }

    function extractAccountRecords(text) {
      const records = [];
      const source = stringifyValue(text);
      let cursor = 0;

      while (cursor < source.length) {
        const start = source.indexOf("{", cursor);
        if (start === -1) {
          break;
        }

        const end = findObjectEnd(source, start);
        if (end === -1) {
          break;
        }

        const chunk = source.slice(start, end + 1);
        try {
          const parsed = JSON.parse(chunk);
          if (isAccountObject(parsed)) {
            records.push({
              account: parsed,
              start,
              end
            });
          }
        } catch (error) {
          // Ignore non-account JSON fragments and continue scanning.
        }

        cursor = end + 1;
      }

      return records;
    }

    function findObjectEnd(source, start) {
      let depth = 0;
      let inString = false;
      let escaped = false;

      for (let index = start; index < source.length; index += 1) {
        const char = source[index];

        if (inString) {
          if (escaped) {
            escaped = false;
          } else if (char === "\\") {
            escaped = true;
          } else if (char === "\"") {
            inString = false;
          }
          continue;
        }

        if (char === "\"") {
          inString = true;
        } else if (char === "{") {
          depth += 1;
        } else if (char === "}") {
          depth -= 1;
          if (depth === 0) {
            return index;
          }
        }
      }

      return -1;
    }

    function isAccountObject(value) {
      const fields = getAccountFields();
      return Boolean(
        value &&
        typeof value === "object" &&
        !Array.isArray(value) &&
        hasConfiguredField(value, fields.email, "email") &&
        hasConfiguredField(value, fields.password, "password")
      );
    }

    function hasConfiguredField(object, fieldName, fallbackFieldName) {
      return Object.prototype.hasOwnProperty.call(object, fieldName) ||
        Object.prototype.hasOwnProperty.call(object, fallbackFieldName);
    }

    function normalizeAccount(account) {
      const source = account && typeof account === "object" ? account : {};
      const fields = getAccountFields();
      const normalized = {
        ...source,
        email: readConfiguredField(source, fields.email, "email"),
        password: readConfiguredField(source, fields.password, "password"),
        mailbox_url: readConfiguredField(source, fields.mailbox_url, "mailbox_url"),
        mail_subject: stringifyValue(source.mail_subject || source.subject || ""),
        read_at: stringifyValue(source.read_at || source.time || "")
      };
      delete normalized.code;
      delete normalized.verification_code;
      delete normalized.captcha;
      delete normalized.otp;
      delete normalized.pin;
      delete normalized.code_type;
      Object.defineProperty(normalized, "created_date", {
        value: formatDateOnly(source.created_at),
        enumerable: false
      });
      Object.defineProperty(normalized, "__accountFieldPresence", {
        value: {
          email: Object.prototype.hasOwnProperty.call(source, "email"),
          password: Object.prototype.hasOwnProperty.call(source, "password"),
          mailbox_url: Object.prototype.hasOwnProperty.call(source, "mailbox_url")
        },
        enumerable: false
      });
      Object.defineProperty(normalized, "__accountSourceFieldPresence", {
        value: {
          mailbox_url: hasConfiguredField(source, fields.mailbox_url, "mailbox_url")
        },
        enumerable: false
      });
      return normalized;
    }

    function readConfiguredField(source, fieldName, fallbackFieldName) {
      if (Object.prototype.hasOwnProperty.call(source, fieldName)) {
        return stringifyValue(source[fieldName]);
      }
      return stringifyValue(source[fallbackFieldName]);
    }

    function getAccountFields() {
      return {
        email: normalizeFieldName(state.accountFields.email, DEFAULT_ACCOUNT_FIELDS.email),
        password: normalizeFieldName(state.accountFields.password, DEFAULT_ACCOUNT_FIELDS.password),
        mailbox_url: normalizeFieldName(state.accountFields.mailbox_url, DEFAULT_ACCOUNT_FIELDS.mailbox_url)
      };
    }

    function normalizeFieldName(value, fallback) {
      return stringifyValue(value).trim() || fallback;
    }

    async function refreshAllAccounts() {
      if (state.isRefreshing) {
        setMessageKey("refreshRunning", "error");
        return;
      }
      if (!state.accounts.length) {
        return;
      }

      const keywords = getSearchKeywords();
      const filterActive = isFilterActive(keywords);
      const candidates = filterActive
        ? getVisibleAccounts()
        : state.accounts.map((account, index) => ({ account, index }));
      const targets = candidates.filter(({ account }) => accountHasMailboxUrl(account));
      if (!targets.length) {
        setMessageKey("noMailboxApiTargets", "error");
        return;
      }

      state.isRefreshing = true;
      state.lastChanged = 0;
      setMessageKey(filterActive ? "refreshingFilteredData" : "refreshingAllMailboxes", "");
      render();

      try {
        const results = await runWithConcurrencyLimit(
          targets,
          5,
          ({ account, index }) => refreshAccount(account, index)
        );

        const changed = results.reduce((count, result) => {
          return count + (result.status === "fulfilled" && result.value ? 1 : 0);
        }, 0);

        state.lastChanged = changed;
        setMessageKey("refreshCompleteChanged", changed ? "success" : "", { changed });
        await persistAccountFile();
      } finally {
        state.isRefreshing = false;
        render();
      }
    }

    async function runWithConcurrencyLimit(items, limit, worker) {
      const results = new Array(items.length);
      let nextIndex = 0;

      async function runNext() {
        const currentIndex = nextIndex;
        nextIndex += 1;

        if (currentIndex >= items.length) {
          return;
        }

        try {
          results[currentIndex] = {
            status: "fulfilled",
            value: await worker(items[currentIndex])
          };
        } catch (error) {
          results[currentIndex] = {
            status: "rejected",
            reason: error
          };
        }

        await runNext();
      }

      const workerCount = Math.min(limit, items.length);
      await Promise.all(Array.from({ length: workerCount }, runNext));
      return results;
    }

    async function refreshAuthFileStatuses() {
      if (!state.accounts.length) {
        state.authFileStatuses.clear();
        return;
      }

      const api = getApiSettings();
      if (!api.base || !api.key) {
        state.authFileStatuses.clear();
        render();
        return;
      }

      try {
        const response = await fetch(`${api.base}${CPA_PLUGIN_API}/auth-files`, {
          method: "GET",
          cache: "no-store",
          headers: {
            "Accept": "application/json",
            "Authorization": `Bearer ${api.key}`
          }
        });
        const payload = await readJsonResponse(response);
        if (!response.ok) {
          throw new Error(payload.error || `HTTP ${response.status}`);
        }

        const nextStatuses = new Map();
        extractAuthFileRecords(payload).forEach((record) => {
          const email = normalizeEmailKey(record.email);
          const statusText = stringifyValue(record.status).trim();
          const exceptionText = stringifyValue(record.status_message).trim();
          if (email && statusText) {
            nextStatuses.set(email, {
              type: "muted",
              statusCode: statusText,
              exception: exceptionText
            });
          }
        });
        state.authFileStatuses = nextStatuses;
      } catch (error) {
        state.authFileStatuses.clear();
      }

      render();
    }

    function extractAuthFileRecords(payload) {
      const records = [];

      function visit(value) {
        if (Array.isArray(value)) {
          value.forEach(visit);
          return;
        }
        if (!value || typeof value !== "object") {
          return;
        }

        if (
          Object.prototype.hasOwnProperty.call(value, "email") &&
          Object.prototype.hasOwnProperty.call(value, "status")
        ) {
          records.push({
            email: value.email,
            status: value.status,
            status_message: value.status_message
          });
        }

        Object.keys(value).forEach((key) => visit(value[key]));
      }

      visit(payload);
      return records;
    }


    async function refreshAccount(account, index) {
      if (!account.mailbox_url) {
        return false;
      }

      try {
        setStatus(index, "warn", "statusReading");
        renderRows();

        const response = await fetch(account.mailbox_url, {
          method: "GET",
          cache: "no-store"
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        const mailboxPayload = await readMailboxPayload(response);
        const subject = extractMailboxSubject(mailboxPayload);
        const previousSubject = stringifyValue(account.mail_subject);
        const nextReadAt = extractReceivedAt(mailboxPayload) || formatDateTime(new Date());
        const changed = previousSubject !== subject || stringifyValue(account.read_at) !== nextReadAt;
        account.mail_subject = subject;
        account.read_at = nextReadAt;
        if (subject.includes("Access Deactivated")) {
          setStatus(index, "bad", "statusMailboxDeactivated");
          return changed;
        }

        if (!changed) {
          setStatus(index, "unchanged", "statusMailInfoUnchanged");
          return false;
        }

        setStatus(index, "ok", "statusUpdated");
        return true;
      } catch (error) {
        state.statuses.set(index, {
          type: "bad",
          text: error.message || translate("statusReadFailed")
        });
        return false;
      }
    }

    async function readMailboxPayload(response) {
      const contentType = response.headers.get("content-type") || "";
      const text = await response.text();

      if (!contentType.includes("application/json") && !looksLikeJson(text)) {
        return text;
      }

      try {
        return JSON.parse(text);
      } catch (error) {
        return text;
      }
    }

    function looksLikeJson(text) {
      const trimmed = stringifyValue(text).trim();
      return trimmed.startsWith("{") || trimmed.startsWith("[");
    }

    function extractMailboxSubject(payload) {
      if (payload && typeof payload === "object") {
        const subject = decodeHtmlEntities(stringifyValue(
          payload.subject || payload.title || payload.mail_subject || payload.email_subject || ""
        ));
        if (subject) {
          return subject;
        }

        if (payload.data && typeof payload.data === "object") {
          return extractMailboxSubject(payload.data);
        }

        return "";
      }

      return "";
    }

    function extractReceivedAt(payload) {
      if (!payload || typeof payload !== "object") {
        return "";
      }

      const receivedAt = payload.received_at || payload.receivedAt || payload.read_at || payload.time || "";
      if (receivedAt) {
        return formatPayloadDate(receivedAt);
      }

      if (payload.data && typeof payload.data === "object") {
        return extractReceivedAt(payload.data);
      }

      return "";
    }

    async function persistAccountFile() {
      try {
        await persistAccountsToCpa();
        setMessageKey("accountFileWritten", state.lastChanged ? "success" : "");
      } catch (error) {
        setMessageKey("accountFileWriteFailed", "error", {
          error: error.message || translate("browserWritePermissionMissing")
        });
      }
    }

    async function persistAccountsToCpa() {
      const api = getApiSettings();
      if (!api.base || !api.key) {
        throw new Error("未找到 CPA API 会话");
      }
      const payload = state.accounts.map((account) => JSON.stringify(prepareAccountForWrite(account))).join("\n");
      const response = await fetch(`${api.base}${CPA_PLUGIN_API}/account-files`, {
        method: "POST",
        cache: "no-store",
        headers: { "Accept": "application/json", "Content-Type": "application/json", "Authorization": `Bearer ${api.key}` },
        body: payload
      });
      const result = await readJsonResponse(response);
      if (!response.ok) {
        throw new Error(result.error || `HTTP ${response.status}`);
      }
      return result;
    }

    async function requestWritePermission() {
      if (!state.fileHandle || !state.fileHandle.queryPermission || !state.fileHandle.requestPermission) {
        return false;
      }

      const options = { mode: "readwrite" };
      if ((await state.fileHandle.queryPermission(options)) === "granted") {
        return true;
      }

      return (await state.fileHandle.requestPermission(options)) === "granted";
    }

    function serializeAccountFile() {
      if (state.accountObjectRanges.length !== state.accounts.length) {
        return `${state.accounts.map((account) => JSON.stringify(prepareAccountForWrite(account))).join("\n")}\n`;
      }

      let cursor = 0;
      let content = "";
      state.accountObjectRanges.forEach((range, index) => {
        content += state.accountFileText.slice(cursor, range.start);
        content += JSON.stringify(prepareAccountForWrite(state.accounts[index]));
        cursor = range.end + 1;
      });
      content += state.accountFileText.slice(cursor);
      return content;
    }

    function prepareAccountForWrite(account) {
      const fields = getAccountFields();
      const fieldPresence = account.__accountFieldPresence || {
        email: true,
        password: true,
        mailbox_url: true
      };
      const nextAccount = {
        ...account
      };
      ["code", "verification_code", "captcha", "otp", "pin", "code_type"].forEach((field) => {
        delete nextAccount[field];
      });
      const sourceFieldPresence = account.__accountSourceFieldPresence || fieldPresence;
      const mailboxUrl = stringifyValue(account.mailbox_url);
      nextAccount[fields.email] = stringifyValue(account.email);
      nextAccount[fields.password] = stringifyValue(account.password);
      if (sourceFieldPresence.mailbox_url || mailboxUrl) {
        nextAccount[fields.mailbox_url] = mailboxUrl;
      } else {
        delete nextAccount[fields.mailbox_url];
        delete nextAccount.mailbox_url;
      }
      if (fields.email !== DEFAULT_ACCOUNT_FIELDS.email && !fieldPresence.email) {
        delete nextAccount.email;
      }
      if (fields.password !== DEFAULT_ACCOUNT_FIELDS.password && !fieldPresence.password) {
        delete nextAccount.password;
      }
      if (fields.mailbox_url !== DEFAULT_ACCOUNT_FIELDS.mailbox_url && !fieldPresence.mailbox_url) {
        delete nextAccount.mailbox_url;
      }
      return nextAccount;
    }

    function syncAccountFileState(content) {
      state.accountFileText = content;
      const records = extractAccountRecords(content);
      state.accountObjectRanges = records.map((record) => ({
        start: record.start,
        end: record.end
      }));
    }

    function render() {
      const hasAccounts = state.accounts.length > 0;
      elements.refreshBtn.disabled = !hasAccounts || state.isRefreshing;
      elements.reloadFileBtn.disabled = !canRefreshAuthFileStatuses() || state.isRefreshing || state.isReloadingFile;
      elements.reloadFileBtn.textContent = state.isReloadingFile ? translate("reloadingList") : translate("reloadList");
      elements.emptyState.classList.toggle("hidden", hasAccounts);
      elements.tableShell.classList.toggle("hidden", !hasAccounts);
      updateStatusFilterControl();
      const keywords = getSearchKeywords();
      const filterActive = isFilterActive(keywords);
      elements.refreshBtn.classList.toggle("is-loading", state.isRefreshing);
      elements.refreshBtn.textContent = state.isRefreshing
        ? translate("refreshing")
        : filterActive ? translate("refreshFiltered") : translate("refreshAll");
      updateMailboxFilterControl();
      renderRows();
    }

    function canRefreshAuthFileStatuses() {
      const api = getApiSettings();
      return state.accounts.length > 0 && Boolean(api.base && api.key);
    }

    function renderRows() {
      const rows = getVisibleAccounts();
      if (!rows.length && state.accounts.length) {
        elements.accountRows.innerHTML = `<tr><td class="no-results" colspan="6">${escapeHtml(translate("noMatchingAccounts"))}</td></tr>`;
        return;
      }

      let currentGroup = "";
      const groupSummaries = getGroupStatusSummaries(rows);
      elements.accountRows.innerHTML = rows.map(({ account, index, authFileStatus, mailboxApiStatus }, rowIndex) => {
        const group = getCreatedDateGroup(account);
        const summary = groupSummaries.get(group) || [];
        const groupMarkup = group !== currentGroup
          ? `<tr class="group-row"><td colspan="6">
              <div class="group-content">
                <span>${escapeHtml(group)}</span>
                <button class="group-download-btn cpa-download-btn" type="button" data-cpa-group="${escapeHtml(group)}" title="${escapeHtml(translate("cpaDownloadAction"))}">${escapeHtml(translate("cpaDownloadAction"))}</button>
                ${summary.length ? `<span class="group-summary">${renderGroupSummary(summary)}</span>` : ""}
              </div>
            </td></tr>`
          : "";
        const hasMailboxUrl = accountHasMailboxUrl(account);
        const actionsMarkup = `<div class="action-buttons">
            ${renderCopyButton(account.email, index, "email", true)}
            ${renderCopyButton(account.password, index, "password", true)}
            ${hasMailboxUrl ? `
              <button class="mail-btn" type="button" data-mail-index="${index}">${escapeHtml(translate("mailboxAction"))}</button>
              <button class="login-btn" type="button" data-login-index="${index}"${state.isStartingLogin ? " disabled" : ""}>${escapeHtml(translate("loginAction"))}</button>` : ""}
          </div>`;
        currentGroup = group;
        return `
          ${groupMarkup}
          <tr class="account-row${rowIndex % 2 ? " is-striped" : ""}">
            <td>${renderCopyCell(account.email, index, "email", "", false)}</td>
            <td>${renderCopyCell(account.password, index, "password", maskPassword(account.password), false)}</td>
            <td>${renderMailboxInfoCell(account.mail_subject, account.read_at)}</td>
            <td class="status-cell ${escapeHtml(authFileStatus.type)}"><span class="status-text auth-status-text">${escapeHtml(getAuthFileStatusLabel(authFileStatus) || "/")}</span></td>
            <td class="status-cell ${escapeHtml(authFileStatus.type)}">${getAuthFileMessageText(authFileStatus) ? `<span class="auth-status-message">${escapeHtml(getAuthFileMessageText(authFileStatus))}</span>` : `<span class="muted">/</span>`}</td>
            <td class="action-cell">
              ${actionsMarkup}
            </td>
          </tr>
        `;
      }).join("");
    }

    function getCreatedDateGroup(account) {
      return account.created_date || "/";
    }

    function getGroupStatusSummaries(rows) {
      const useAuthFileStatus = rows.some(({ authFileStatus }) => {
        return !isEmptyStatus(authFileStatus);
      });
      const groups = new Map();
      rows.forEach(({ account, authFileStatus, mailboxApiStatus }) => {
        const group = getCreatedDateGroup(account);
        const status = useAuthFileStatus ? authFileStatus : mailboxApiStatus;
        const statusText = getStatusText(status) || "/";
        const statusType = status && status.type ? status.type : "muted";
        if (!groups.has(group)) {
          groups.set(group, new Map());
        }
        const statusCounts = groups.get(group);
        if (!statusCounts.has(statusText)) {
          statusCounts.set(statusText, {
            count: 0,
            type: statusType
          });
        }
        statusCounts.get(statusText).count += 1;
      });

      return new Map(Array.from(groups, ([group, statusCounts]) => {
        const summary = Array.from(statusCounts, ([statusText, item]) => ({
          text: statusText,
          count: item.count,
          type: item.type
        }));
        return [group, summary];
      }));
    }

    function renderGroupSummary(summary) {
      return summary.map((item) => {
        const type = item.type || "muted";
        const label = item.text === "/" ? translate("missingStatusLabel") : item.text;
        return `<span class="code group-status-code ${escapeHtml(type)}"><span class="group-status-label">${escapeHtml(label)}</span>:<span>${escapeHtml(item.count)}</span></span>`;
      }).join("");
    }

    function getVisibleAccounts() {
      const keywords = getSearchKeywords();
      return state.accounts
        .map((account, index) => ({
          account,
          index,
          authFileStatus: getAuthFileStatus(account) || getEmptyStatus(),
          mailboxApiStatus: getMailboxApiStatus(account, index)
        }))
        .filter(({ account }) => matchesMailboxFilter(account))
        .filter(({ authFileStatus }) => matchesStatusFilter(authFileStatus))
        .filter(({ account, authFileStatus, mailboxApiStatus }) => matchesSearchKeywords(account, authFileStatus, mailboxApiStatus, keywords))
        .sort(compareAccountsByTimeDesc);
    }

    function getMailboxApiStatus(account, index) {
      if (!accountHasMailboxUrl(account)) {
        return getEmptyStatus();
      }
      return state.statuses.get(index) || { type: "muted", key: "statusPending" };
    }

    function getAuthFileStatus(account) {
      return state.authFileStatuses.get(normalizeEmailKey(account && account.email));
    }

    function getEmptyStatus() {
      return { type: "muted", text: "/" };
    }

    function isEmptyStatus(status) {
      return !status || getStatusText(status) === "/";
    }

    function isFilterActive(keywords) {
      return Boolean((keywords && keywords.length) || state.mailboxFilter !== "all" || state.statusFilter !== "all");
    }

    function getStatusFilterOptions() {
      const options = new Map();
      state.accounts.filter(matchesMailboxFilter).forEach((account) => {
        const status = getAuthFileStatus(account);
        if (isEmptyStatus(status)) {
          return;
        }
        const value = getStatusFilterValue(status);
        if (value) {
          const current = options.get(value);
          options.set(value, {
            label: getAuthFileStatusLabel(status),
            count: (current ? current.count : 0) + 1
          });
        }
      });
      const result = Array.from(options, ([value, item]) => ({ value, ...item }))
        .filter((item) => item.count > 0);
      return result;
    }

    function matchesStatusFilter(status) {
      return state.statusFilter === "all" ||
        (!isEmptyStatus(status) && getStatusFilterValue(status) === state.statusFilter);
    }

    function getAuthFileExceptionText(status) {
      return stringifyValue(status && status.exception).trim();
    }

    function getAuthFileMessageText(status) {
      const raw = getAuthFileExceptionText(status);
      if (!raw) {
        return "";
      }
      try {
        const parsed = JSON.parse(raw);
        if (
          parsed &&
          typeof parsed === "object" &&
          parsed.error &&
          typeof parsed.error === "object" &&
          parsed.error.type !== undefined
        ) {
          return stringifyValue(parsed.error.type);
        }
      } catch (_) {
        // Keep non-JSON status_message unchanged.
      }
      return raw;
    }

    function getStatusFilterValue(status) {
      const value = stringifyValue(status && status.statusCode).trim().toLowerCase();
      return value ? `code:${value}` : "";
    }

    function getAuthFileStatusLabel(status) {
      return getStatusText(status);
    }

    function getAuthStatusLabel(statusCode) {
      const value = stringifyValue(statusCode).trim();
      return AUTH_STATUS_LABELS[value.toLowerCase()] || value;
    }

    function matchesMailboxFilter(account) {
      if (state.mailboxFilter === "with") {
        return accountHasMailboxUrl(account);
      }
      if (state.mailboxFilter === "without") {
        return !accountHasMailboxUrl(account);
      }
      return true;
    }

    function accountHasMailboxUrl(account) {
      return Boolean(stringifyValue(account && account.mailbox_url).trim());
    }

    function normalizeEmailKey(value) {
      return stringifyValue(value).trim().toLowerCase();
    }

    function compareAccountsByTimeDesc(left, right) {
      const createdDateCompare = parseCreatedDate(right.account.created_date) - parseCreatedDate(left.account.created_date);
      if (createdDateCompare) {
        return createdDateCompare;
      }
      return parseAccountTime(right.account.read_at) - parseAccountTime(left.account.read_at);
    }

    function parseCreatedDate(value) {
      const text = stringifyValue(value).trim();
      if (!text || text === "/") {
        return 0;
      }
      const timestamp = Date.parse(`${text}T00:00:00`);
      return Number.isNaN(timestamp) ? 0 : timestamp;
    }

    function parseAccountTime(value) {
      const text = stringifyValue(value).trim();
      if (!text) {
        return 0;
      }

      const normalized = text.includes("T") ? text : text.replace(" ", "T");
      const timestamp = Date.parse(normalized);
      return Number.isNaN(timestamp) ? 0 : timestamp;
    }

    function getSearchKeywords() {
      return stringifyValue(elements.searchInput.value)
        .trim()
        .toLowerCase()
        .split(/[\s,，;；]+/)
        .filter(Boolean);
    }

    function matchesSearchKeywords(account, authFileStatus, mailboxApiStatus, keywords) {
      if (!keywords.length) {
        return true;
      }

      const searchableText = [
        account.email,
        account.password,
        account.mail_subject,
        account.read_at,
        account.created_date,
        account.created_at,
        account.mailbox_url,
        getStatusText(authFileStatus),
        getStatusText(mailboxApiStatus)
      ].map(stringifyValue).join(" ").toLowerCase();

      return keywords.some((keyword) => searchableText.includes(keyword));
    }

    function renderCopyCell(value, index, field, displayValue, showCopyButton = true) {
      const text = stringifyValue(value);
      const displayText = stringifyValue(displayValue || text);
      const valueMarkup = text
        ? `<span class="copy-value" title="${escapeHtml(field === "password" ? translate("passwordMaskedTitle") : text)}">${escapeHtml(displayText)}</span>`
        : `<span class="muted">-</span>`;
      return `
        <div class="copy-cell">
          ${showCopyButton ? renderCopyButton(text, index, field) : ""}
          ${valueMarkup}
        </div>
      `;
    }

    function renderMailboxInfoCell(subject, readAt) {
      const subjectText = stringifyValue(subject) || "-";
      const timeText = stringifyValue(readAt) || "-";
      return `<div class="mail-info">
        <span class="mail-subject" title="${escapeHtml(subjectText)}">${escapeHtml(subjectText)}</span>
        <span class="mail-time" title="${escapeHtml(timeText)}">${escapeHtml(timeText)}</span>
      </div>`;
    }

    function renderCopyButton(value, index, field, showLabel = false) {
      const label = fieldLabel(field);
      const copyText = translate("copyLabel", { label });
      const disabled = stringifyValue(value) ? "" : " disabled";
      return `<button class="copy-btn${showLabel ? " copy-btn-labeled" : ""}" type="button" data-copy-index="${index}" data-copy-field="${field}" aria-label="${escapeHtml(copyText)}" title="${escapeHtml(copyText)}"${disabled}>${copyIconMarkup()}${showLabel ? `<span>${escapeHtml(copyText)}</span>` : ""}</button>`;
    }

    function maskPassword(value) {
      const text = stringifyValue(value);
      if (!text) {
        return "";
      }

      if (text.length <= 4) {
        return "****";
      }

      return `${text.slice(0, 2)}${"*".repeat(Math.min(8, text.length - 4))}${text.slice(-2)}`;
    }

    function maskEmail(value) {
      const text = stringifyValue(value).trim();
      const parts = text.split("@");
      if (parts.length !== 2) {
        return text || "-";
      }

      const local = parts[0];
      const domain = parts[1];
      if (local.length <= 2) {
        return `${"*".repeat(local.length)}@${domain}`;
      }

      return `${local.slice(0, 1)}${"*".repeat(Math.min(8, local.length - 2))}${local.slice(-1)}@${domain}`;
    }

    function copyIconMarkup() {
      return `
        <svg class="copy-icon" viewBox="0 0 24 24" aria-hidden="true">
          <rect x="9" y="9" width="11" height="11" rx="2"></rect>
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
        </svg>
      `;
    }

    async function handleRowClick(event) {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }

      const cpaDownloadButton = target.closest(".cpa-download-btn");
      if (cpaDownloadButton) {
        handleCpaDownloadClick(cpaDownloadButton);
        return;
      }

      const mailButton = target.closest(".mail-btn");
      if (mailButton) {
        await handleMailClick(mailButton);
        return;
      }

      const loginButton = target.closest(".login-btn");
      if (loginButton) {
        await handleLoginClick(loginButton);
        return;
      }

      await handleCopyClick(event);
    }

    function handleCpaDownloadClick(button) {
      const group = button.dataset.cpaGroup || "";
      const rows = getVisibleAccounts().filter(({ account }) => {
        return getCreatedDateGroup(account) === group;
      });
      if (!rows.length) {
        setMessageKey("cpaNoRows", "error");
        return;
      }

      try {
        const output = buildAccountDownloadOutput(rows, group);
        downloadBlobParts(output.parts, output.name, output.mime);
        setMessageKey("cpaDownloadReady", "success", { count: output.count });
      } catch (error) {
        setMessageKey("cpaDownloadFailed", "error", {
          error: error.message || translate("unknownError")
        });
      }
    }

    function buildAccountDownloadOutput(rows, group) {
      // Downloads intentionally use the decrypted password held by the
      // authorized management page, never the on-disk password_cipher value.
      const accounts = rows.map(({ account }) => prepareAccountForWrite(account));
      if (accounts.length === 1) {
        const source = rows[0].account;
        const text = JSON.stringify(accounts[0], null, 2);
        return {
          parts: [text],
          name: `${sanitizeFilename(source.email, "account")}.json`,
          mime: "application/json;charset=utf-8",
          count: 1
        };
      }

      const files = accounts.map((account, index) => ({
        name: `${sanitizeFilename(rows[index].account.email, "account")}.json`,
        text: JSON.stringify(account, null, 2)
      }));
      const groupName = group && group !== "/" ? group : "unknown-date";
      return {
        parts: [createTarArchive(files)],
        name: `${sanitizeFilename(groupName, "accounts")}-accounts.tar`,
        mime: "application/x-tar",
        count: accounts.length
      };
    }

    function downloadBlobParts(parts, name, mime) {
      const blob = new Blob(parts, { type: mime });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = name;
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    }

    function firstText(...values) {
      for (const value of values) {
        const text = String(value ?? "").trim();
        if (text) {
          return text;
        }
      }
      return "";
    }

    function coerceTs(value) {
      if (typeof value === "number" && Number.isFinite(value)) {
        return Math.max(0, Math.trunc(value));
      }
      const text = String(value ?? "").trim();
      if (!text) {
        return 0;
      }
      if (/^-?\d+$/.test(text)) {
        return Math.max(0, Number.parseInt(text, 10));
      }
      const parsed = Date.parse(text);
      return Number.isNaN(parsed) ? 0 : Math.max(0, Math.trunc(parsed / 1000));
    }

    function looksLikeEmail(value) {
      const text = String(value ?? "").trim();
      if (!text || /\s/.test(text)) {
        return false;
      }
      const parts = text.split("@");
      return parts.length === 2 && Boolean(parts[0]) && Boolean(parts[1]);
    }

    function b64uToText(text) {
      let value = String(text ?? "").replace(/-/g, "+").replace(/_/g, "/");
      const remainder = value.length % 4;
      if (remainder) {
        value += "=".repeat(4 - remainder);
      }
      const binary = atob(value);
      const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
      return textDecoder.decode(bytes);
    }

    function decodeJwtPayload(token) {
      try {
        const parts = String(token ?? "").split(".");
        if (parts.length < 2) {
          return {};
        }
        const parsed = JSON.parse(b64uToText(parts[1]));
        return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
      } catch (error) {
        return {};
      }
    }

    function b64uBytes(bytes) {
      let binary = "";
      for (const byte of bytes) {
        binary += String.fromCharCode(byte);
      }
      return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
    }

    function b64uJson(value) {
      return b64uBytes(textEncoder.encode(JSON.stringify(value)));
    }

    function extractAuth(payload) {
      const value = payload && payload["https://api.openai.com/auth"];
      return value && typeof value === "object" && !Array.isArray(value) ? value : {};
    }

    function extractProfile(payload) {
      const value = payload && payload["https://api.openai.com/profile"];
      return value && typeof value === "object" && !Array.isArray(value) ? value : {};
    }

    function extractAccountIdFromAuth(auth) {
      const accountId = firstText(auth && auth.chatgpt_account_id, auth && auth.account_id);
      if (accountId) {
        return accountId;
      }
      const accountUserId = firstText(auth && auth.chatgpt_account_user_id);
      if (accountUserId.includes("__")) {
        const suffix = accountUserId.split("__").pop().trim();
        if (suffix) {
          return suffix;
        }
      }
      return "";
    }

    function extractOrganizationId(idAuth, accessAuth) {
      const organizationId = firstText(idAuth && idAuth.organization_id, accessAuth && accessAuth.organization_id);
      if (organizationId) {
        return organizationId;
      }
      const organizations = (idAuth && idAuth.organizations) || (accessAuth && accessAuth.organizations) || [];
      if (Array.isArray(organizations)) {
        for (const item of organizations) {
          const value = firstText(item && item.id);
          if (value) {
            return value;
          }
        }
      }
      return "";
    }

    function sanitizeFilename(name, fallback) {
      const cleaned = String(name ?? "").trim().replace(/[<>:"/\\|?*\u0000-\u001f]+/g, "_");
      return cleaned || fallback;
    }

    function toIso8(date) {
      const utc = date.getTime() + (date.getTimezoneOffset() * 60000);
      const shifted = new Date(utc + 8 * 3600000);
      return shifted.toISOString().replace("Z", "+08:00");
    }

    function compatSeeds(accountId, userId, email) {
      const seed = firstText(accountId, userId, email, "unknown")
        .replace(/[^a-zA-Z0-9]/g, "")
        .slice(0, 24) || "unknown";
      return {
        org: `org-${seed}`,
        proj: `proj_${seed}`,
        sid: `compat_session_${seed}`
      };
    }

    function buildLocalCompatIdToken(args) {
      const accountId = firstText(args.accountId);
      const raw = firstText(args.idToken);
      if (!accountId) {
        return raw;
      }

      const idPayload = decodeJwtPayload(raw);
      const accessPayload = decodeJwtPayload(args.accessToken);
      const basePayload = Object.keys(idPayload).length ? idPayload : accessPayload;
      if (!Object.keys(basePayload).length) {
        return raw;
      }

      const baseAuth = extractAuth(basePayload);
      const profile = extractProfile(basePayload);
      const email = firstText(profile.email, basePayload.email, args.email);
      const userId = firstText(args.userId, baseAuth.chatgpt_user_id, baseAuth.user_id, basePayload.sub);
      const seeds = compatSeeds(accountId, userId, email);
      const organizationId = firstText(
        args.organizationId,
        baseAuth.organization_id,
        extractOrganizationId(baseAuth, baseAuth),
        seeds.org
      );
      const projectId = firstText(args.projectId, baseAuth.project_id, seeds.proj);
      const auth = { ...baseAuth };
      auth.chatgpt_account_id = firstText(auth.chatgpt_account_id, auth.account_id, accountId);
      auth.account_id = firstText(auth.account_id, auth.chatgpt_account_id, accountId);
      if (userId) {
        auth.chatgpt_user_id = firstText(auth.chatgpt_user_id, auth.user_id, userId);
        auth.user_id = firstText(auth.user_id, auth.chatgpt_user_id, userId);
      }
      auth.chatgpt_plan_type = firstText(auth.chatgpt_plan_type, args.planType, "free");
      if (!firstText(auth.organization_id)) {
        auth.organization_id = organizationId;
      }
      if (!Array.isArray(auth.organizations) || !auth.organizations.length) {
        auth.organizations = [{ id: organizationId, is_default: true, role: "owner", title: "Personal" }];
      }
      if (!firstText(auth.project_id)) {
        auth.project_id = projectId;
      }
      if (!("completed_platform_onboarding" in auth)) {
        auth.completed_platform_onboarding = false;
      }
      if (!Array.isArray(auth.groups)) {
        auth.groups = [];
      }
      if (!("is_org_owner" in auth)) {
        auth.is_org_owner = true;
      }
      if (!("localhost" in auth)) {
        auth.localhost = true;
      }

      const payload = { ...basePayload, "https://api.openai.com/auth": auth };
      if (email && !firstText(payload.email)) {
        payload.email = email;
      }
      if (!("email_verified" in payload)) {
        payload.email_verified = true;
      }
      if (!firstText(payload.iss)) {
        payload.iss = "https://auth.openai.com";
      }
      if (!payload.aud) {
        payload.aud = [DEFAULT_CLIENT_ID];
      }
      if (!firstText(payload.auth_provider)) {
        payload.auth_provider = "password";
      }
      const authTime = coerceTs(payload.pwd_auth_time || payload.auth_time || payload.rat || payload.iat);
      if (authTime && !coerceTs(payload.auth_time)) {
        payload.auth_time = authTime;
      }
      const sid = firstText(payload.sid, payload.session_id, seeds.sid);
      if (sid && !firstText(payload.sid)) {
        payload.sid = sid;
      }
      if (sid && !firstText(payload.session_id)) {
        payload.session_id = sid;
      }
      if (!firstText(payload.sub) && userId) {
        payload.sub = userId;
      }
      if (!firstText(payload.jti)) {
        const compact = firstText(args.accessToken, raw, accountId, userId, email)
          .replace(/[^a-zA-Z0-9]/g, "")
          .slice(0, 32) || "compat";
        payload.jti = `compat-${compact}`;
      }
      if (!firstText(payload.name) && email) {
        payload.name = email.split("@")[0] || "OpenAI User";
      }

      return `${b64uJson({ alg: "RS256", typ: "JWT", kid: "compat" })}.${b64uJson(payload)}.${b64uBytes(textEncoder.encode("compat_signature_for_local_parsing_only"))}`;
    }

    function ensureIdTokenClaims(args) {
      const token = firstText(args.idToken);
      const accountId = firstText(args.accountId);
      if (!accountId) {
        return token;
      }

      const payload = decodeJwtPayload(token);
      if (!Object.keys(payload).length) {
        return buildLocalCompatIdToken(args);
      }

      const auth = { ...extractAuth(payload) };
      const existingChatgpt = firstText(auth.chatgpt_account_id);
      const existingAccount = firstText(auth.account_id);
      const resolved = firstText(existingChatgpt, existingAccount, accountId);
      if (existingChatgpt && existingAccount) {
        return token;
      }

      auth.chatgpt_account_id = firstText(existingChatgpt, resolved);
      auth.account_id = firstText(existingAccount, resolved);
      if (args.userId) {
        auth.chatgpt_user_id = firstText(auth.chatgpt_user_id, auth.user_id, args.userId);
        auth.user_id = firstText(auth.user_id, auth.chatgpt_user_id, args.userId);
      }
      if (args.organizationId && !firstText(auth.organization_id)) {
        auth.organization_id = args.organizationId;
      }
      if (args.projectId && !firstText(auth.project_id)) {
        auth.project_id = args.projectId;
      }
      if (args.planType && !firstText(auth.chatgpt_plan_type)) {
        auth.chatgpt_plan_type = args.planType;
      }

      const updated = { ...payload, "https://api.openai.com/auth": auth };
      const parts = token.split(".");
      const head = parts[0] || b64uJson({ alg: "RS256", typ: "JWT", kid: "compat" });
      const sig = parts[2] || b64uBytes(textEncoder.encode("compat_signature_for_local_parsing_only"));
      return `${head}.${b64uJson(updated)}.${sig}`;
    }

    function finalizeRecord(record) {
      const item = { ...record };
      item.chatgpt_account_id = firstText(item.chatgpt_account_id, item.account_id);
      item.project_id = firstText(item.project_id, item.workspace_id);
      item.workspace_id = firstText(item.workspace_id, item.project_id);
      if (!item.client_id) {
        item.client_id = DEFAULT_CLIENT_ID;
      }
      if (!item.privacy_mode) {
        item.privacy_mode = DEFAULT_PRIVACY_MODE;
      }
      if (!("openai_oauth_responses_websockets_v2_enabled" in item)) {
        item.openai_oauth_responses_websockets_v2_enabled = false;
      }
      if (!item.openai_oauth_responses_websockets_v2_mode) {
        item.openai_oauth_responses_websockets_v2_mode = "off";
      }
      item.id_token = ensureIdTokenClaims({
        idToken: firstText(item.id_token),
        accessToken: firstText(item.access_token),
        accountId: firstText(item.chatgpt_account_id),
        userId: firstText(item.chatgpt_user_id),
        organizationId: firstText(item.organization_id),
        projectId: firstText(item.project_id, item.workspace_id),
        email: firstText(item.email, item.account_claims_email),
        planType: firstText(item.plan_type, "free")
      });
      return item;
    }

    function createTarArchive(files) {
      function put(destination, offset, text) {
        const bytes = textEncoder.encode(String(text ?? ""));
        destination.set(bytes.slice(0, Math.max(0, destination.length - offset)), offset);
      }

      function oct(value, length) {
        const text = Math.max(0, Math.trunc(value)).toString(8);
        return `${text}`.padStart(length - 1, "0") + "\0";
      }

      function checksum(header) {
        let sum = 0;
        for (const byte of header) {
          sum += byte;
        }
        return `${sum.toString(8).padStart(6, "0")}\0 `;
      }

      const blocks = [];
      for (const file of files) {
        const name = sanitizeFilename(file.name, "file.json").slice(0, 99);
        const bytes = file.bytes instanceof Uint8Array ? file.bytes : textEncoder.encode(String(file.text ?? ""));
        const header = new Uint8Array(512);
        put(header, 0, name);
        put(header, 100, "0000777\0");
        put(header, 108, "0000000\0");
        put(header, 116, "0000000\0");
        put(header, 124, oct(bytes.length, 12));
        put(header, 136, oct(Math.trunc(Date.now() / 1000), 12));
        put(header, 148, "        ");
        put(header, 156, "0");
        put(header, 257, "ustar\0");
        put(header, 263, "00");
        put(header, 148, checksum(header));
        blocks.push(header, bytes);
        const pad = (512 - (bytes.length % 512)) % 512;
        if (pad) {
          blocks.push(new Uint8Array(pad));
        }
      }
      blocks.push(new Uint8Array(1024));

      const total = blocks.reduce((size, block) => size + block.length, 0);
      const output = new Uint8Array(total);
      let offset = 0;
      for (const block of blocks) {
        output.set(block, offset);
        offset += block.length;
      }
      return output;
    }

    function normalizeRecord(item) {
      if (!item || typeof item !== "object" || Array.isArray(item) || Array.isArray(item.accounts)) {
        return null;
      }

      let email = "";
      let password = "";
      let loginIdentity = "";
      let phone = "";
      let accessToken = "";
      let refreshToken = "";
      let idToken = "";
      let sessionToken = "";
      let clientId = "";
      let chatgptAccountId = "";
      let chatgptUserId = "";
      let organizationId = "";
      let projectId = "";
      let workspaceId = "";
      let createdAt = 0;
      let lastUsed = 0;
      let status = "";
      let source = "";
      let disabled = false;
      let accountClaimsEmail = "";
      let privacyMode = "";
      let wsEnabled = null;
      let wsMode = "";

      if (item.tokens && typeof item.tokens === "object" && !Array.isArray(item.tokens)) {
        const tokens = item.tokens;
        email = firstText(item.email);
        accessToken = firstText(tokens.access_token);
        refreshToken = firstText(tokens.refresh_token);
        idToken = firstText(tokens.id_token);
        chatgptAccountId = firstText(item.chatgpt_account_id, item.account_id);
        createdAt = coerceTs(item.created_at);
        lastUsed = coerceTs(item.last_used);
        source = "codex_input";
      } else if (item.credentials && typeof item.credentials === "object" && !Array.isArray(item.credentials)) {
        const credentials = item.credentials;
        const extra = item.extra && typeof item.extra === "object" && !Array.isArray(item.extra) ? item.extra : {};
        email = firstText(extra.email, credentials.email, item.name);
        accessToken = firstText(credentials.access_token);
        refreshToken = firstText(credentials.refresh_token);
        idToken = firstText(credentials.id_token);
        sessionToken = firstText(credentials.session_token);
        clientId = firstText(credentials.client_id, DEFAULT_CLIENT_ID);
        chatgptAccountId = firstText(credentials.chatgpt_account_id, credentials.account_id, item.chatgpt_account_id, item.account_id);
        chatgptUserId = firstText(credentials.chatgpt_user_id);
        organizationId = firstText(credentials.organization_id);
        projectId = firstText(credentials.project_id);
        workspaceId = firstText(projectId);
        createdAt = coerceTs(item.created_at);
        lastUsed = coerceTs(item.last_used);
        status = firstText(item.status);
        source = firstText(item.notes, "sub_bundle_input");
        disabled = Boolean(item.disabled);
        accountClaimsEmail = firstText(extra.email);
        privacyMode = firstText(extra.privacy_mode);
        wsEnabled = extra.openai_oauth_responses_websockets_v2_enabled;
        wsMode = firstText(extra.openai_oauth_responses_websockets_v2_mode);
      } else {
        email = firstText(item.email);
        password = firstText(item.password);
        loginIdentity = firstText(item.login_identity);
        phone = firstText(item.phone);
        accessToken = firstText(item.access_token);
        refreshToken = firstText(item.refresh_token);
        idToken = firstText(item.id_token);
        sessionToken = firstText(item.session_token);
        clientId = firstText(item.client_id, DEFAULT_CLIENT_ID);
        chatgptAccountId = firstText(item.chatgpt_account_id, item.account_id);
        chatgptUserId = firstText(item.chatgpt_user_id);
        organizationId = firstText(item.organization_id);
        projectId = firstText(item.project_id);
        workspaceId = firstText(item.workspace_id, projectId);
        createdAt = coerceTs(item.created_at);
        lastUsed = coerceTs(item.last_used);
        status = firstText(item.status);
        source = firstText(item.source, "unified_input");
        disabled = Boolean(item.disabled);
        accountClaimsEmail = firstText(item.account_claims_email);
        privacyMode = firstText(item.privacy_mode);
        wsEnabled = item.openai_oauth_responses_websockets_v2_enabled;
        wsMode = firstText(item.openai_oauth_responses_websockets_v2_mode);
      }
      if (!email) {
        return null;
      }

      const idPayload = decodeJwtPayload(idToken);
      const accessPayload = decodeJwtPayload(accessToken);
      const idAuth = extractAuth(idPayload);
      const accessAuth = extractAuth(accessPayload);
      const accessProfile = extractProfile(accessPayload);
      const record = {
        version: Number.parseInt(item.version || 1, 10) || 1,
        platform: firstText(item.platform, "chatgpt"),
        email,
        password,
        login_identity: firstText(loginIdentity),
        phone: firstText(phone),
        access_token: accessToken,
        refresh_token: refreshToken,
        id_token: idToken,
        session_token: sessionToken,
        client_id: firstText(clientId, DEFAULT_CLIENT_ID),
        chatgpt_account_id: firstText(chatgptAccountId, extractAccountIdFromAuth(idAuth), extractAccountIdFromAuth(accessAuth)),
        chatgpt_user_id: firstText(
          chatgptUserId,
          idAuth.chatgpt_user_id,
          idAuth.user_id,
          idAuth.chatgpt_account_user_id,
          accessAuth.chatgpt_user_id,
          accessAuth.user_id,
          accessAuth.chatgpt_account_user_id
        ),
        organization_id: firstText(organizationId, extractOrganizationId(idAuth, accessAuth)),
        project_id: firstText(projectId, workspaceId, idAuth.project_id, accessAuth.project_id),
        workspace_id: firstText(workspaceId, projectId, idAuth.project_id, accessAuth.project_id),
        created_at: createdAt,
        last_used: lastUsed,
        status,
        source,
        disabled,
        account_claims_email: firstText(accountClaimsEmail, idPayload.email, accessProfile.email),
        plan_type: firstText(item.plan_type, idAuth.chatgpt_plan_type, accessAuth.chatgpt_plan_type, "free"),
        privacy_mode: firstText(privacyMode, DEFAULT_PRIVACY_MODE),
        openai_oauth_responses_websockets_v2_enabled: wsEnabled !== null ? Boolean(wsEnabled) : false,
        openai_oauth_responses_websockets_v2_mode: firstText(wsMode, "off")
      };
      if (record.login_identity && !record.phone && !looksLikeEmail(record.login_identity)) {
        record.phone = record.login_identity;
      }
      return finalizeRecord(record);
    }

    function buildCpaPayload(record) {
      const item = finalizeRecord(record);
      const exp = coerceTs(decodeJwtPayload(item.access_token).exp);
      return {
        type: "codex",
        email: item.email,
        expired: exp ? toIso8(new Date(exp * 1000)) : "",
        id_token: item.id_token,
        account_id: firstText(item.chatgpt_account_id),
        disabled: Boolean(item.disabled),
        access_token: item.access_token,
        last_refresh: toIso8(new Date()),
        refresh_token: item.refresh_token
      };
    }

    async function handleMailClick(button) {
      const index = Number.parseInt(button.dataset.mailIndex, 10);
      const account = state.accounts[index];
      const mailboxUrl = account ? decodeHtmlEntities(stringifyValue(account.mailbox_url).trim()) : "";
      if (!mailboxUrl) {
        setMessageKey("mailboxMissing", "error");
        return;
      }

      const opened = window.open(mailboxUrl, "_blank");
      if (!opened) {
        await copyText(mailboxUrl);
        setMessageKey("mailboxBlockedCopied", "error");
        return;
      }

      opened.opener = null;
      setMessageKey("mailboxOpened", "success", { email: maskEmail(account.email) });
    }

    async function handleLoginClick(button) {
      if (state.isStartingLogin) {
        setMessageKey("loginRunning", "error");
        return;
      }

      const index = Number.parseInt(button.dataset.loginIndex, 10);
      const account = state.accounts[index];
      if (!account) {
        return;
      }

      state.isStartingLogin = true;
      render();
      try {
        const api = getApiSettings();
        const managementBaseUrl = api.base;
        const managementKey = api.key;
        if (!managementBaseUrl) {
          setMessage("未找到 CPA 管理会话。", "error");
          return;
        }
        if (!managementKey) {
          setMessage("未找到 CPA 管理凭证。", "error");
          return;
        }

        setStatus(index, "warn", "statusLoggingIn");
        setMessageKey("creatingLoginLink", "", { email: maskEmail(account.email) });
        render();

        const response = await fetch(`${managementBaseUrl}/v0/management/codex-auth-url?is_webui=true`, {
          method: "GET",
          cache: "no-store",
          headers: {
            "Accept": "application/json",
            "Authorization": `Bearer ${managementKey}`
          }
        });
        const payload = await readJsonResponse(response);
        if (!response.ok) {
          throw new Error(payload.error || `HTTP ${response.status}`);
        }

        const authUrl = stringifyValue(payload.url);
        if (!isCodexAuthorizeUrl(authUrl)) {
          throw new Error(translate("invalidCodexAuthUrl"));
        }

        const opened = openLoginTab(authUrl);
        if (!opened) {
          const copied = await copyText(authUrl);
          if (copied) {
            setMessageKey("loginBlockedCopied", "error");
          } else {
            setMessageKey("loginBlockedCopyFailed", "error");
          }
          setStatus(index, "bad", "statusOpenFailed");
          return;
        }

        setMessageKey("fixedLoginTabOpened", "success", { email: maskEmail(account.email) });
        setStatus(index, "ok", "statusLoginOpened");
      } catch (error) {
        setStatus(index, "bad", "statusLoginFailed");
        setMessageKey("codexLoginFailed", "error", {
          error: error.message || translate("unknownError")
        });
      } finally {
        state.isStartingLogin = false;
        render();
      }
    }

    function openLoginTab(authUrl) {
      const currentTab = state.loginTabWindow;
      if (currentTab && !currentTab.closed) {
        try {
          currentTab.location.href = authUrl;
          currentTab.focus();
          return currentTab;
        } catch (error) {
          state.loginTabWindow = null;
        }
      }

      const opened = window.open(authUrl, "accountMgtLoginTab");
      if (opened) {
        state.loginTabWindow = opened;
        try {
          opened.opener = null;
          opened.focus();
        } catch (error) {
          // Browser restrictions may block cross-window property access; the tab is still open.
        }
      }
      return opened;
    }

    async function readJsonResponse(response) {
      const text = await response.text();
      if (!text) {
        return {};
      }

      try {
        return JSON.parse(text);
      } catch (error) {
        return { error: text };
      }
    }

    function isCodexAuthorizeUrl(rawUrl) {
      try {
        const url = new URL(rawUrl);
        return url.protocol === "https:" &&
          url.host === "auth.openai.com" &&
          url.pathname === "/oauth/authorize" &&
          Boolean(url.searchParams.get("state")) &&
          Boolean(url.searchParams.get("code_challenge"));
      } catch (error) {
        return false;
      }
    }

    function normalizeManagementBaseUrl(value) {
      const text = stringifyValue(value).trim().replace(/\/+$/, "");
      if (!text) {
        return "";
      }
      if (!/^https?:\/\//i.test(text)) {
        return "";
      }
      return text;
    }

    function loadAccountFieldSettings() {
      elements.emailFieldInput.value = state.accountFields.email;
      elements.passwordFieldInput.value = state.accountFields.password;
      elements.mailboxUrlFieldInput.value = state.accountFields.mailbox_url;
    }

    function saveAccountFieldSettings() {
      state.accountFields = {
        email: normalizeFieldName(elements.emailFieldInput.value, DEFAULT_ACCOUNT_FIELDS.email),
        password: normalizeFieldName(elements.passwordFieldInput.value, DEFAULT_ACCOUNT_FIELDS.password),
        mailbox_url: normalizeFieldName(elements.mailboxUrlFieldInput.value, DEFAULT_ACCOUNT_FIELDS.mailbox_url)
      };
      elements.emailFieldInput.value = state.accountFields.email;
      elements.passwordFieldInput.value = state.accountFields.password;
      elements.mailboxUrlFieldInput.value = state.accountFields.mailbox_url;
      localStorage.setItem("account_mgt_field_email", state.accountFields.email);
      localStorage.setItem("account_mgt_field_password", state.accountFields.password);
      localStorage.setItem("account_mgt_field_mailbox_url", state.accountFields.mailbox_url);
    }

    async function handleCopyClick(event) {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }

      const button = target.closest(".copy-btn");
      if (!button) {
        return;
      }

      const index = Number.parseInt(button.dataset.copyIndex, 10);
      const field = button.dataset.copyField;
      const account = state.accounts[index];
      if (!account || !field) {
        return;
      }

      const value = stringifyValue(account[field]);
      if (!value) {
        return;
      }

      const copied = await copyText(value);
      if (copied) {
        setMessageKey("copySuccess", "success", { label: fieldLabel(field) });
      } else {
        setMessageKey("copyFailed", "error", { label: fieldLabel(field) });
      }
    }

    async function copyText(value) {
      if (navigator.clipboard && window.isSecureContext) {
        try {
          await navigator.clipboard.writeText(value);
          return true;
        } catch (error) {
          return fallbackCopyText(value);
        }
      }

      return fallbackCopyText(value);
    }

    function fallbackCopyText(value) {
      const textarea = document.createElement("textarea");
      textarea.value = value;
      textarea.setAttribute("readonly", "");
      textarea.style.position = "fixed";
      textarea.style.left = "-9999px";
      document.body.appendChild(textarea);
      textarea.select();

      try {
        return document.execCommand("copy");
      } catch (error) {
        return false;
      } finally {
        textarea.remove();
      }
    }

    function fieldLabel(field) {
      if (field === "email") {
        return translate("emailLabel");
      }
      if (field === "password") {
        return translate("passwordLabel");
      }
      return translate("contentLabel");
    }

    function setMessage(text, type) {
      state.messageKey = "";
      state.messageParams = {};
      state.messageText = text || "";
      state.messageType = type || "";
      updateMessage();
    }

    function updateMessage() {
      elements.message.textContent = state.messageKey
        ? translate(state.messageKey, state.messageParams)
        : state.messageText;
      elements.message.className = `message${state.messageType ? ` ${state.messageType}` : ""}`;
    }

    function stringifyValue(value) {
      if (value === null || value === undefined) {
        return "";
      }
      return String(value);
    }

    function decodeHtmlEntities(value) {
      const text = stringifyValue(value);
      if (!/&(?:[a-zA-Z][a-zA-Z0-9]+|#\d+|#x[0-9a-fA-F]+);/.test(text)) {
        return text;
      }

      const textarea = document.createElement("textarea");
      textarea.innerHTML = text;
      return textarea.value;
    }

    function formatPayloadDate(value) {
      const text = stringifyValue(value).trim();
      if (!text) {
        return "";
      }

      const parsedDate = new Date(text);
      if (!Number.isNaN(parsedDate.getTime())) {
        return formatDateTime(parsedDate);
      }

      return text;
    }

    function formatDateTime(date) {
      const pad = (value) => String(value).padStart(2, "0");
      return [
        date.getFullYear(),
        "-",
        pad(date.getMonth() + 1),
        "-",
        pad(date.getDate()),
        " ",
        pad(date.getHours()),
        ":",
        pad(date.getMinutes()),
        ":",
        pad(date.getSeconds())
      ].join("");
    }

    function formatDateOnly(value) {
      const text = stringifyValue(value).trim();
      if (!text) {
        return "";
      }

      if (/^\d+$/.test(text)) {
        const numericTime = Number(text);
        const parsedDate = new Date(text.length >= 13 ? numericTime : numericTime * 1000);
        if (!Number.isNaN(parsedDate.getTime())) {
          return formatDateParts(parsedDate.getFullYear(), parsedDate.getMonth() + 1, parsedDate.getDate());
        }
      }

      const parsedDate = new Date(text);
      if (!Number.isNaN(parsedDate.getTime())) {
        return formatDateParts(parsedDate.getFullYear(), parsedDate.getMonth() + 1, parsedDate.getDate());
      }

      const match = text.match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})/);
      if (match) {
        return formatDateParts(match[1], match[2], match[3]);
      }

      return "";
    }

    function formatDateParts(year, month, day) {
      const pad = (value) => String(value).padStart(2, "0");
      return `${year}-${pad(month)}-${pad(day)}`;
    }

    function escapeHtml(value) {
      return stringifyValue(value)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
    }
