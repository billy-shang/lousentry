<template>
  <div>
    <h2 class="page-title">系统日志</h2>
    <div class="toolbar card">
      <button class="btn" type="button" @click="load">刷新</button>
      <select v-model="level" class="select" @change="load">
        <option value="">全部级别</option>
        <option value="info">信息</option>
        <option value="warn">警告</option>
        <option value="error">错误</option>
        <option value="debug">调试</option>
      </select>
      <select v-model.number="size" class="select" @change="load">
        <option :value="50">50 条</option>
        <option :value="100">100 条</option>
        <option :value="200">200 条</option>
      </select>
      <span class="spacer"></span>
      <span class="muted">共 {{ total }} 条</span>
    </div>
    <div class="log-card card">
      <div class="log-head">
        <strong>运行日志</strong>
        <span>当前显示 {{ items.length }} 条</span>
      </div>
      <div class="terminal">
        <div v-if="!items.length" class="empty">暂无日志</div>
        <div v-for="(item, idx) in items" :key="idx" class="line" :class="item.level">
          <span class="time">{{ item.time }}</span>
          <span class="lv">{{ item.level }}</span>
          <span class="msg">{{ item.message }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { inject, onMounted, ref } from "vue";
import http, { errMsg } from "../api";

const toast = inject("toast");
const items = ref([]);
const total = ref(0);
const level = ref("");
const size = ref(100);

async function load() {
  try {
    const res = await http.get("/logs", { params: { level: level.value, page: 1, size: size.value } });
    items.value = res.data.items || [];
    total.value = res.data.total || 0;
    console.log("[logs] 刷新日志", total.value);
  } catch (e) {
    toast(errMsg(e), "error");
  }
}

onMounted(load);
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  margin-bottom: 16px;
}
.select { width: 120px; }
.spacer { flex: 1; }
.muted { color: #94a3b8; font-size: 13px; }
.log-card { padding: 16px; }
.log-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  color: #8a8f99;
  font-size: 13px;
}
.log-head strong { color: #1f2329; font-size: 14px; }
.terminal {
  background: #0f172a;
  color: #e5e7eb;
  border-radius: 10px;
  padding: 12px 14px;
  min-height: 420px;
  max-height: calc(100vh - 280px);
  overflow: auto;
  font-family: Consolas, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.7;
}
.line {
  display: grid;
  grid-template-columns: 160px 56px 1fr;
  gap: 10px;
  align-items: start;
}
.line.error { color: #fca5a5; }
.line.warn { color: #fcd34d; }
.line.debug { color: #93c5fd; }
.time, .lv { color: #93c5fd; white-space: nowrap; }
.msg { word-break: break-all; }
.empty { color: #94a3b8; }

@media (max-width: 720px) {
  .line { grid-template-columns: 1fr; gap: 2px; margin-bottom: 8px; }
}
</style>
