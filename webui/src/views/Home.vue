<template>
  <div>
    <h2 class="page-title">漏洞总览</h2>
    <StatsBar :stats="stats" />

    <div class="status-strip card">
      <div class="status-item">
        <strong>自动检查</strong>
        <span v-if="stats.checking">正在检查各数据源...</span>
        <span v-else>
          每 {{ formatInterval(stats.interval) }} 抓取一次
          <em v-if="stats.next_check_at">，下次 {{ formatDateTime(stats.next_check_at) }}</em>
          <em v-if="stats.last_check_at">，上次 {{ formatDateTime(stats.last_check_at) }}</em>
        </span>
      </div>
      <div class="status-item">
        <strong>自动推送</strong>
        <span v-if="stats.auto_push">已启用 {{ (stats.pushers || []).join("、") }}，发现符合策略的新漏洞会立刻发到 Webhook</span>
        <span v-else>尚未启用推送渠道，新漏洞只会入库，可在设置里配置钉钉 / 飞书 / 企业微信</span>
      </div>
      <button v-if="isAdmin" class="btn" type="button" :disabled="busy || stats.checking" @click="checkNow">
        {{ stats.checking ? "检查中..." : "立即检查" }}
      </button>
    </div>

    <div class="toolbar card">
      <input v-model.trim="query.keyword" class="input grow" placeholder="搜索 CVE / 漏洞名称..." @keyup.enter="search" />
      <select v-model="query.source" class="select" title="数据来源">
        <option value="">全部来源</option>
        <option v-for="s in sources" :key="s.id" :value="s.id">{{ s.display_name }}</option>
      </select>
      <select v-model="query.status" class="select" title="推送状态">
        <option value="">全部状态</option>
        <option value="unpushed">未推送</option>
        <option value="pushed">已推送</option>
      </select>
      <select v-model="query.severity" class="select" title="危害等级">
        <option value="">全部等级</option>
        <option value="严重">严重</option>
        <option value="高危">高危</option>
        <option value="中危">中危</option>
        <option value="低危">低危</option>
      </select>
      <button class="btn" type="button" @click="search">搜索</button>
      <button v-if="isAdmin" class="btn ghost" type="button" :disabled="busy" @click="pushUnpushed">手动推送</button>
      <button v-if="isAdmin" class="btn ghost" type="button" :disabled="busy" @click="dailyReport">每日报告</button>
    </div>

    <div class="table-card card">
      <div class="table-head">
        <h3>漏洞列表</h3>
        <span class="muted">共 {{ total }} 条</span>
      </div>
      <div class="table-wrap">
        <table>
          <colgroup>
            <col class="col-title" />
            <col class="col-cve" />
            <col class="col-sev" />
            <col class="col-src" />
            <col class="col-tag" />
            <col class="col-date" />
            <col class="col-time" />
            <col class="col-status" />
          </colgroup>
          <thead>
            <tr>
              <th>漏洞标题</th>
              <th>CVE</th>
              <th>危害等级</th>
              <th>来源</th>
              <th>标签</th>
              <th>披露时间</th>
              <th>更新时间</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading"><td colspan="8" class="empty">加载中...</td></tr>
            <tr v-else-if="!items.length"><td colspan="8" class="empty">暂无漏洞数据</td></tr>
            <tr v-for="item in items" :key="item.id" @click="open(item)">
              <td class="title" :title="item.title">
                <span v-if="isToday(item.create_time)" class="tag">今日</span>
                {{ item.title }}
              </td>
              <td class="mono" :title="item.cve">{{ item.cve || "-" }}</td>
              <td><span class="tag" :class="sevClass(item.severity)">{{ item.severity || "-" }}</span></td>
              <td class="ellipsis" :title="item.source_name">{{ sourceShort(item) }}</td>
              <td class="tags">
                <div class="tag-cell">
                  <span v-for="tag in (item.tags || [])" :key="tag" class="tag">{{ tag }}</span>
                  <span v-if="!item.tags || !item.tags.length" class="dash">-</span>
                </div>
              </td>
              <td class="nowrap">{{ item.disclosure || "-" }}</td>
              <td class="nowrap">{{ formatDateTime(item.update_time) }}</td>
              <td><span class="tag" :class="item.pushed ? 'ok' : 'warn'">{{ item.pushed ? "已推送" : "未推送" }}</span></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="pager">
        <span>共 {{ pages }} 页，第 {{ query.page }} 页</span>
        <div class="pager-btns">
          <button class="btn page" type="button" :disabled="query.page <= 1" @click="go(query.page - 1)">上一页</button>
          <button
            v-for="(p, idx) in visiblePages"
            :key="`${p}-${idx}`"
            class="btn page"
            type="button"
            :class="{ active: p === query.page }"
            :disabled="p === '...'"
            @click="typeof p === 'number' && go(p)"
          >{{ p }}</button>
          <button class="btn page" type="button" :disabled="query.page >= pages" @click="go(query.page + 1)">下一页</button>
        </div>
      </div>
    </div>

    <div v-if="current" class="modal-mask" @click.self="current = null">
      <div class="modal">
        <h3>{{ current.title }}</h3>
        <p class="meta">
          <span class="tag" :class="sevClass(current.severity)">{{ current.severity }}</span>
          <span>{{ current.cve || "暂无 CVE" }}</span>
          <span>{{ sourceShort(current) }}</span>
          <a v-if="current.from" :href="current.from" target="_blank" rel="noreferrer">查看来源</a>
        </p>
        <section>
          <h4>漏洞描述</h4>
          <p>{{ current.description || "暂无描述" }}</p>
        </section>
        <section v-if="current.solutions">
          <h4>修复方案</h4>
          <p class="pre">{{ current.solutions }}</p>
        </section>
        <section v-if="current.references && current.references.length">
          <h4>参考链接</h4>
          <ul>
            <li v-for="ref in current.references" :key="ref"><a :href="ref" target="_blank" rel="noreferrer">{{ ref }}</a></li>
          </ul>
        </section>
        <div class="modal-actions">
          <button class="btn ghost" type="button" @click="current = null">关闭</button>
          <button v-if="isAdmin" class="btn" type="button" :disabled="busy" @click="pushOne(current)">推送此漏洞</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, inject, onMounted, reactive, ref } from "vue";
import http, { errMsg } from "../api";
import StatsBar from "../components/StatsBar.vue";
import { formatDateTime, formatInterval, isToday, pageNumbers, sevClass, sourceShort } from "../format";

const toast = inject("toast");
const isAdmin = inject("isAdmin", ref(true));
const stats = ref({});
const sources = ref([]);
const items = ref([]);
const total = ref(0);
const loading = ref(false);
const busy = ref(false);
const current = ref(null);
const query = reactive({
  keyword: "",
  source: "",
  status: "",
  severity: "",
  page: 1,
  size: 20,
});

const pages = computed(() => Math.max(1, Math.ceil(total.value / query.size)));
const visiblePages = computed(() => pageNumbers(query.page, pages.value));

async function loadStats() {
  const res = await http.get("/stats");
  stats.value = res.data || {};
}

async function loadSources() {
  const res = await http.get("/sources");
  sources.value = res.data || [];
}

async function loadList() {
  loading.value = true;
  try {
    const res = await http.get("/vulns", { params: query });
    items.value = res.data.items || [];
    total.value = res.data.total || 0;
    console.log("[home] 加载漏洞列表", { page: query.page, total: total.value });
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    loading.value = false;
  }
}

function search() {
  query.page = 1;
  loadList();
}

function go(page) {
  query.page = page;
  loadList();
}

async function open(item) {
  try {
    const res = await http.get(`/vulns/${item.id}`);
    current.value = res.data;
  } catch (e) {
    toast(errMsg(e), "error");
  }
}

async function pushOne(item) {
  busy.value = true;
  try {
    await http.post(`/vulns/${item.id}/push`);
    toast("推送成功");
    current.value = null;
    await Promise.all([loadList(), loadStats()]);
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function pushUnpushed() {
  if (!confirm("将推送最近 20 条未推送漏洞，确认继续？")) return;
  busy.value = true;
  try {
    const res = await http.post("/vulns/push-unpushed");
    toast(`已推送 ${res.data.pushed} 条`);
    await Promise.all([loadList(), loadStats()]);
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
    console.log("[home] 触发立即检查");
    await loadStats();
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

async function dailyReport() {
  busy.value = true;
  try {
    await http.post("/report/daily");
    toast("每日报告已推送");
  } catch (e) {
    toast(errMsg(e), "error");
  } finally {
    busy.value = false;
  }
}

onMounted(async () => {
  await Promise.all([loadStats(), loadSources(), loadList()]);
});
</script>

<style scoped>
.status-strip {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.4fr) auto;
  gap: 16px;
  align-items: center;
  padding: 16px 18px;
  margin-bottom: 16px;
  border-left: 3px solid #2f6bff;
}
.status-item {
  min-width: 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.55;
}
.status-item strong {
  display: block;
  margin-bottom: 4px;
  color: #1f2329;
  font-size: 14px;
}
.status-item em { font-style: normal; color: #475569; }
.toolbar {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) 150px 120px 120px auto auto auto;
  gap: 10px;
  padding: 14px 16px;
  margin-bottom: 16px;
  align-items: center;
}
.table-card { overflow: hidden; }
.table-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #f1f5f9;
}
.table-head h3 { margin: 0; font-size: 16px; font-weight: 650; }
.muted { color: #94a3b8; font-size: 13px; }
.table-wrap { overflow-x: auto; }
table {
  width: 100%;
  min-width: 1080px;
  border-collapse: collapse;
  table-layout: fixed;
}
.col-title { width: auto; }
.col-cve { width: 150px; }
.col-sev { width: 84px; }
.col-src { width: 88px; }
.col-tag { width: 176px; }
.col-date { width: 112px; }
.col-time { width: 148px; }
.col-status { width: 84px; }
th, td {
  padding: 11px 12px;
  border-bottom: 1px solid #f1f5f9;
  text-align: left;
  font-size: 13px;
  vertical-align: middle;
  overflow: hidden;
}
th {
  color: #8a8f99;
  font-weight: 500;
  white-space: nowrap;
  background: #f8fafc;
}
tbody tr { cursor: pointer; }
tbody tr:hover { background: #f7faff; }
.title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #1f2937;
}
.title .tag { margin-right: 6px; vertical-align: 1px; }
.ellipsis, .mono, .nowrap {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.mono { font-variant-numeric: tabular-nums; }
.tags { overflow: hidden; }
.tag-cell {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  min-width: 0;
  max-width: 100%;
}
.tag-cell .tag {
  max-width: 100%;
}
.dash { color: #cbd5e1; }
.empty { text-align: center; color: #94a3b8; padding: 36px 0; }
.pager {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 12px 18px 16px;
  color: #94a3b8;
  font-size: 13px;
}
.pager-btns { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
.meta { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; color: #64748b; }
.meta a { color: #2f6bff; }
section { margin-top: 16px; }
section h4 { margin: 0 0 8px; font-size: 14px; }
section p, section li { color: #475569; line-height: 1.7; font-size: 14px; word-break: break-all; }
.pre { white-space: pre-wrap; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }

@media (max-width: 1100px) {
  .status-strip { grid-template-columns: 1fr; }
  .toolbar { grid-template-columns: 1fr 1fr; }
  .toolbar .grow { grid-column: 1 / -1; }
}
</style>
