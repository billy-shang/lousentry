<template>
  <div>
    <h2 class="page-title">系统设置</h2>
    <StatsBar :stats="stats" />

    <div class="card form-shell">
      <div class="tabs">
        <button type="button" :class="{ active: tab === 'monitor' }" @click="tab = 'monitor'">监控设置</button>
        <button type="button" :class="{ active: tab === 'push' }" @click="tab = 'push'">推送设置</button>
        <button type="button" :class="{ active: tab === 'account' }" @click="tab = 'account'">账户安全</button>
      </div>

      <div v-if="tab === 'monitor'" class="form-card">
        <h3>监控设置</h3>
        <p class="hint">启动时先把当前漏洞入库，不推送历史数据。之后按下面的周期增量检查；发现符合策略的新漏洞（或等级/标签升级）会自动推送到已启用的 Webhook。00:00-07:00 默认休眠。</p>
        <label class="field">
          <span>检查间隔</span>
          <select v-model="monitor.interval" class="select compact">
            <option value="10m">10 分钟</option>
            <option value="30m">30 分钟</option>
            <option value="1h">1 小时</option>
            <option value="2h">2 小时</option>
            <option value="6h">6 小时</option>
            <option value="12h">12 小时</option>
            <option value="24h">24 小时</option>
          </select>
        </label>
        <div class="field">
          <span>数据源</span>
          <div class="checks">
            <label v-for="s in sources" :key="s.id" class="chip">
              <input type="checkbox" :value="s.id" v-model="monitor.sources" />
              {{ s.display_name }}
            </label>
          </div>
        </div>
        <label class="field">
          <span>白名单关键字（每行一个）</span>
          <textarea v-model="whiteText" class="textarea" placeholder="例如：Apache" />
        </label>
        <label class="field">
          <span>黑名单关键字（每行一个）</span>
          <textarea v-model="blackText" class="textarea" placeholder="例如：无意义的产品名" />
        </label>
        <div class="options">
          <label class="check"><input type="checkbox" v-model="monitor.no_filter" />不过滤，推送全部新发现漏洞</label>
          <label class="check"><input type="checkbox" v-model="monitor.no_github_search" />不搜索 GitHub POC</label>
          <label class="check"><input type="checkbox" v-model="monitor.enable_cve_filter" />启用 CVE 去重（相同 CVE 只推送一次）</label>
          <label class="check"><input type="checkbox" v-model="monitor.no_sleep" />夜间不休眠（00:00-07:00 也检查）</label>
          <label class="check"><input type="checkbox" v-model="monitor.no_start_message" />禁用启动提示推送</label>
          <label class="check"><input type="checkbox" v-model="monitor.skip_tls_verify" />跳过 TLS 校验</label>
        </div>
        <label class="field">
          <span>代理地址（支持 http(s):// 或 socks5://）</span>
          <input v-model.trim="monitor.proxy" class="input" placeholder="例如 http://127.0.0.1:7890" />
        </label>
        <div v-if="isAdmin" class="actions">
          <button class="btn" type="button" :disabled="busy" @click="saveMonitor">保存配置</button>
          <button class="btn ghost" type="button" :disabled="busy" @click="checkNow">立即检查</button>
        </div>
      </div>

      <div v-if="tab === 'push'" class="form-card">
        <h3>推送渠道</h3>
        <p class="hint">Webhook 保存在 SQLite。保存后立即生效：之后自动检查到的新漏洞、以及首页的「立即检查 / 手动推送」，都会发到已启用的渠道。</p>
        <div class="channel">
          <label class="check channel-check">
            <input type="checkbox" :disabled="!isAdmin" v-model="push.dingding.enabled" @change="onChannelToggle('dingding')" />
            钉钉
          </label>
          <div v-if="push.dingding.enabled" class="channel-body">
            <input v-model.trim="push.dingding.access_token" class="input" :disabled="!isAdmin" placeholder="Access Token" />
            <input v-model.trim="push.dingding.sign_secret" class="input" :disabled="!isAdmin" placeholder="加签 Secret" />
          </div>
        </div>
        <div class="channel">
          <label class="check channel-check">
            <input type="checkbox" :disabled="!isAdmin" v-model="push.lark.enabled" @change="onChannelToggle('lark')" />
            飞书
          </label>
          <div v-if="push.lark.enabled" class="channel-body">
            <input v-model.trim="push.lark.access_token" class="input" :disabled="!isAdmin" placeholder="Access Token / Webhook URL" />
            <input v-model.trim="push.lark.sign_secret" class="input" :disabled="!isAdmin" placeholder="加签 Secret" />
          </div>
        </div>
        <div class="channel">
          <label class="check channel-check">
            <input type="checkbox" :disabled="!isAdmin" v-model="push.wechatwork.enabled" @change="onChannelToggle('wechatwork')" />
            企业微信
          </label>
          <div v-if="push.wechatwork.enabled" class="channel-body">
            <input v-model.trim="push.wechatwork.key" class="input" :disabled="!isAdmin" placeholder="机器人 Key" />
          </div>
        </div>
        <div v-if="isAdmin" class="actions">
          <button class="btn" type="button" :disabled="busy" @click="savePush">保存推送配置</button>
          <button class="btn ghost" type="button" :disabled="busy" @click="testPush">测试推送</button>
        </div>
      </div>

      <div v-if="tab === 'account'" class="form-card">
        <div class="account-head">
          <h3>账号列表</h3>
          <button v-if="isAdmin" class="btn" type="button" @click="openAddUser">添加账号</button>
        </div>
        <div class="user-table">
          <div class="user-row head">
            <span>用户名</span>
            <span>权限</span>
            <span>操作</span>
          </div>
          <div v-for="u in users" :key="u.username" class="user-row">
            <span>{{ u.username }}<em v-if="u.username === me">当前</em></span>
            <span class="role-tag" :class="u.role">{{ roleLabel(u.role) }}</span>
            <button
              v-if="isAdmin"
              class="btn ghost mini"
              type="button"
              :disabled="busy || u.username === me || users.length <= 1"
              @click="removeUser(u.username)"
            >删除</button>
            <span v-else class="dash">-</span>
          </div>
        </div>

        <h3 class="sub-title">修改当前密码</h3>
        <label class="field">
          <span>原密码</span>
          <input v-model="pwd.old_password" class="input" type="password" />
        </label>
        <label class="field">
          <span>新密码</span>
          <input v-model="pwd.new_password" class="input" type="password" />
        </label>
        <div class="actions">
          <button class="btn danger" type="button" :disabled="busy" @click="changePwd">修改密码</button>
        </div>
      </div>
    </div>

    <div v-if="showAddUser" class="modal-mask" @click.self="showAddUser = false">
      <div class="modal add-modal">
        <h3>添加账号</h3>
        <label class="field">
          <span>用户名</span>
          <input v-model.trim="newbie.username" class="input" placeholder="2-32 位" />
        </label>
        <label class="field">
          <span>密码</span>
          <input v-model="newbie.password" class="input" type="password" placeholder="至少 6 位" />
        </label>
        <div class="field">
          <span>权限</span>
          <div class="role-options">
            <label class="role-card" :class="{ active: newbie.role === 'admin' }">
              <input type="radio" value="admin" v-model="newbie.role" />
              <strong>管理员</strong>
              <em>可改配置、推送和管理账号</em>
            </label>
            <label class="role-card" :class="{ active: newbie.role === 'readonly' }">
              <input type="radio" value="readonly" v-model="newbie.role" />
              <strong>只读</strong>
              <em>只能查看漏洞、日志和当前配置</em>
            </label>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn ghost" type="button" @click="showAddUser = false">取消</button>
          <button class="btn" type="button" :disabled="busy" @click="addUser">确定添加</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { inject, onMounted, reactive, ref } from "vue";
import http, { errMsg } from "../api";
import StatsBar from "../components/StatsBar.vue";

const toast = inject("toast");
const isAdmin = inject("isAdmin", ref(true));
const tab = ref("monitor");
const busy = ref(false);
const users = ref([]);
const me = ref("");
const showAddUser = ref(false);
const newbie = reactive({ username: "", password: "", role: "readonly" });
const stats = ref({});
const sources = ref([]);
const whiteText = ref("");
const blackText = ref("");
const monitor = reactive({
  sources: [],
  interval: "30m",
  enable_cve_filter: true,
  no_github_search: false,
  no_start_message: false,
  no_sleep: false,
  no_filter: false,
  white_keywords: [],
  black_keywords: [],
  proxy: "",
  skip_tls_verify: false,
});
const push = reactive({
  dingding: { enabled: false, access_token: "", sign_secret: "" },
  lark: { enabled: false, access_token: "", sign_secret: "" },
  wechatwork: { enabled: false, key: "" },
});
const pwd = reactive({ old_password: "", new_password: "" });

function splitLines(text) {
  return text.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
}

async function load() {
  const [st, src, mon, pu] = await Promise.all([
    http.get("/stats"),
    http.get("/sources"),
    http.get("/settings"),
    http.get("/push"),
  ]);
  stats.value = st.data || {};
  sources.value = src.data || [];
  Object.assign(monitor, mon.data || {});
  if (!Array.isArray(monitor.sources)) monitor.sources = [];
  whiteText.value = (monitor.white_keywords || []).join("\n");
  blackText.value = (monitor.black_keywords || []).join("\n");
  applyPush(pu.data || {});
  await loadUsers();
}

function applyPush(data) {
  push.dingding.enabled = !!(data.dingding && data.dingding.enabled);
  push.dingding.access_token = (data.dingding && data.dingding.access_token) || "";
  push.dingding.sign_secret = (data.dingding && data.dingding.sign_secret) || "";
  push.lark.enabled = !!(data.lark && data.lark.enabled);
  push.lark.access_token = (data.lark && data.lark.access_token) || "";
  push.lark.sign_secret = (data.lark && data.lark.sign_secret) || "";
  push.wechatwork.enabled = !!(data.wechatwork && data.wechatwork.enabled);
  push.wechatwork.key = (data.wechatwork && data.wechatwork.key) || "";
}

function clearChannel(name) {
  if (name === "wechatwork") {
    push.wechatwork.key = "";
    return;
  }
  push[name].access_token = "";
  push[name].sign_secret = "";
}

async function onChannelToggle(name) {
  if (push[name].enabled) return;
  clearChannel(name);
  if (!isAdmin.value) return;
  await savePush(true);
  toast("已删除该渠道的 Webhook 配置");
  console.log("[settings] 已删除渠道配置", name);
}

function roleLabel(role) {
  return role === "readonly" ? "只读" : "管理员";
}

function openAddUser() {
  newbie.username = "";
  newbie.password = "";
  newbie.role = "readonly";
  showAddUser.value = true;
}

async function loadUsers() {
  const [list, profile] = await Promise.all([http.get("/users"), http.get("/me")]);
  users.value = list.data || [];
  me.value = (profile.data && profile.data.username) || "";
}

async function saveMonitor() {
  busy.value = true;
  try {
    monitor.white_keywords = splitLines(whiteText.value);
    monitor.black_keywords = splitLines(blackText.value);
    await http.put("/settings", monitor);
    toast("监控设置已保存");
    console.log("[settings] 监控设置已保存", monitor);
    stats.value = (await http.get("/stats")).data || {};
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function savePush(silent) {
  busy.value = true;
  try {
    await http.put("/push", push);
    const latest = await http.get("/push");
    applyPush(latest.data || {});
    if (!silent) toast("推送设置已保存到 SQLite");
    console.log("[settings] 推送设置已写入 sqlite");
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function testPush() {
  busy.value = true;
  try {
    await http.post("/push/test");
    toast("测试推送已发送");
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function checkNow() {
  busy.value = true;
  try {
    await http.post("/check");
    toast("已开始立即检查，发现符合策略的新漏洞会自动推送");
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function addUser() {
  if (!newbie.username || !newbie.password) {
    toast("请填写用户名和密码", "error");
    return;
  }
  busy.value = true;
  try {
    await http.post("/users", newbie);
    toast("账号已添加");
    console.log("[settings] 新增账号", newbie.username, newbie.role);
    showAddUser.value = false;
    newbie.username = "";
    newbie.password = "";
    newbie.role = "readonly";
    await loadUsers();
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function removeUser(username) {
  if (!confirm(`确定删除账号 ${username}？`)) return;
  busy.value = true;
  try {
    await http.delete(`/users/${encodeURIComponent(username)}`);
    toast("账号已删除");
    await loadUsers();
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function changePwd() {
  if (!pwd.old_password || !pwd.new_password) {
    toast("请填写原密码和新密码", "error");
    return;
  }
  busy.value = true;
  try {
    await http.post("/password", pwd);
    toast("密码已更新");
    pwd.old_password = "";
    pwd.new_password = "";
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

onMounted(async () => {
  try {
    await load();
  } catch (e) {
    toast(errMsg(e), "error");
  }
});
</script>

<style scoped>
.form-shell { overflow: hidden; }
.tabs {
  display: flex;
  gap: 6px;
  padding: 12px 16px 0;
  border-bottom: 1px solid #ebeef5;
}
.tabs button {
  height: 36px;
  padding: 0 14px;
  border: 0;
  border-radius: 8px 8px 0 0;
  background: transparent;
  color: #8a8f99;
  cursor: pointer;
  font-size: 14px;
}
.tabs button.active {
  color: #2f6bff;
  font-weight: 650;
  box-shadow: inset 0 -2px 0 #2f6bff;
}
.form-card { padding: 18px 20px 22px; }
.form-card h3 { margin: 0 0 16px; font-size: 16px; }
.account-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.account-head h3 { margin: 0; }
.hint { margin: -8px 0 16px; color: #64748b; font-size: 13px; }
.field { display: block; margin-bottom: 14px; }
.field > span {
  display: block;
  margin-bottom: 6px;
  color: #64748b;
  font-size: 13px;
}
.compact { max-width: 240px; }
.checks { display: flex; flex-wrap: wrap; gap: 8px; }
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border-radius: 8px;
  background: #f5f7fa;
  border: 1px solid #ebeef5;
  color: #334155;
  font-size: 13px;
}
.options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 16px;
  margin: 4px 0 14px;
}
.check {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #334155;
  font-size: 13px;
}
.channel-check {
  font-size: 15px;
  font-weight: 600;
}
.channel-check input {
  width: 18px;
  height: 18px;
  accent-color: #2f6bff;
  cursor: pointer;
}
.channel {
  background: #f8fafc;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  padding: 14px 16px;
  margin-bottom: 10px;
}
.channel-body { display: grid; gap: 8px; margin-top: 10px; }
.actions { display: flex; gap: 10px; margin-top: 8px; }
.sub-title { margin: 28px 0 14px; padding-top: 18px; border-top: 1px solid #eef0f5; }
.add-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.user-table { border: 1px solid #eef0f5; border-radius: 12px; overflow: hidden; }
.user-row {
  display: grid;
  grid-template-columns: 1fr 100px auto;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-top: 1px solid #eef0f5;
}
.role-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 12px;
  background: #e8f1ff;
  color: #2f6bff;
}
.role-tag.readonly { background: #f1f5f9; color: #64748b; }
.dash { color: #cbd5e1; }
.add-modal { width: min(480px, 100%); }
.role-options { display: grid; gap: 10px; }
.role-card {
  display: grid;
  grid-template-columns: 18px 1fr;
  grid-template-rows: auto auto;
  column-gap: 10px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  cursor: pointer;
}
.role-card input { grid-row: 1 / span 2; margin-top: 3px; width: 16px; height: 16px; accent-color: #2f6bff; }
.role-card strong { font-size: 14px; color: #1f2329; }
.role-card em { grid-column: 2; font-style: normal; color: #8a8f99; font-size: 12px; margin-top: 4px; }
.role-card.active { border-color: #2f6bff; background: #f4f8ff; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px; }
.user-row.head {
  background: #f8fafc;
  color: #94a3b8;
  font-size: 13px;
  border-top: 0;
}
.user-row em {
  margin-left: 8px;
  font-style: normal;
  color: #2f6bff;
  font-size: 12px;
}
.mini { height: 30px; padding: 0 12px; }

@media (max-width: 720px) {
  .add-grid { grid-template-columns: 1fr; }
  .options { grid-template-columns: 1fr; }
  .compact { max-width: 100%; }
}
</style>
