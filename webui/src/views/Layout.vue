<template>
  <div class="shell">
    <header class="topbar">
      <div class="page-wrap bar-inner">
        <router-link to="/" class="brand-link" title="返回首页">
          <BrandLockup size="sm" />
        </router-link>
        <div class="account">
          <button type="button" class="account-btn">
            <span class="avatar">{{ avatarText }}</span>
            <span class="meta">
              <strong>{{ username }}</strong>
              <em>{{ isAdmin ? "管理员" : "只读" }}</em>
            </span>
            <svg class="caret" viewBox="0 0 12 12" aria-hidden="true">
              <path d="M3 4.5L6 8l3-3.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </button>
          <div class="account-menu">
            <router-link to="/logs">日志</router-link>
            <router-link to="/settings">设置</router-link>
            <button type="button" class="logout" @click="logout">退出</button>
          </div>
        </div>
      </div>
    </header>
    <main class="main">
      <div class="page-wrap">
        <router-view />
      </div>
    </main>
    <AppFooter />
    <div v-if="toast" class="toast-wrap">
      <div class="toast" :class="toast.type">{{ toast.text }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, provide, ref } from "vue";
import { useRouter } from "vue-router";
import http from "../api";
import { getStore, setStore, removeStore } from "../storage";
import BrandLockup from "../components/BrandLockup.vue";
import AppFooter from "../components/AppFooter.vue";

const router = useRouter();
const username = ref("");
const role = ref("admin");
const isAdmin = computed(() => role.value === "admin");
const avatarText = computed(() => {
  const name = String(username.value || "U").trim();
  return name ? name.slice(0, 1).toUpperCase() : "U";
});
try {
  const cached = JSON.parse(getStore("user") || "{}");
  username.value = cached.username || "";
  role.value = cached.role || "admin";
} catch {
  username.value = "";
}
const toast = ref(null);
let timer = null;

function showToast(text, type = "success") {
  toast.value = { text, type };
  clearTimeout(timer);
  timer = setTimeout(() => { toast.value = null; }, 3600);
}

provide("toast", showToast);
provide("isAdmin", isAdmin);

onMounted(async () => {
  try {
    const res = await http.get("/me");
    username.value = (res.data && res.data.username) || username.value;
    role.value = (res.data && res.data.role) || "admin";
    setStore("user", JSON.stringify(res.data || {}));
  } catch (e) {
    console.log("[layout] 读取当前用户失败", e);
  }
});

function logout() {
  removeStore("token");
  removeStore("user");
  router.push("/login");
}
</script>

<style scoped>
.shell { min-height: 100vh; display: flex; flex-direction: column; }
.topbar {
  height: 60px;
  background: #fff;
  color: #1f2329;
  border-bottom: 1px solid #ebeef5;
}
.bar-inner {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.brand-link {
  display: inline-flex;
  align-items: center;
  border-radius: 10px;
}
.brand-link:hover :deep(.lockup) {
  opacity: 0.82;
}
.account {
  position: relative;
  padding-bottom: 8px;
  margin-bottom: -8px;
}
.account-btn {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  height: 40px;
  padding: 0 10px 0 6px;
  border: 1px solid #e8eef6;
  border-radius: 999px;
  background: #f7f9fc;
  color: #334155;
  cursor: pointer;
}
.account-btn:hover,
.account:hover .account-btn {
  border-color: #d7e3f7;
  background: #eef4ff;
}
.avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #2f6bff;
  color: #fff;
  font-size: 13px;
  font-weight: 700;
}
.meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  line-height: 1.15;
  min-width: 0;
}
.meta strong {
  font-size: 13px;
  font-weight: 650;
  color: #1f2329;
}
.meta em {
  font-style: normal;
  font-size: 11px;
  color: #94a3b8;
}
.caret {
  width: 12px;
  height: 12px;
  color: #94a3b8;
}
.account-menu {
  display: none;
  position: absolute;
  top: calc(100% - 2px);
  right: 0;
  min-width: 148px;
  padding: 8px;
  border: 1px solid #e8eef6;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.08);
  z-index: 30;
}
.account-menu a,
.account-menu button {
  display: block;
  width: 100%;
  height: 36px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: #334155;
  cursor: pointer;
  font-size: 13px;
  line-height: 36px;
  text-align: left;
}
.account-menu a.router-link-active,
.account-menu a:hover,
.account-menu button:hover {
  background: #f4f8ff;
  color: #2f6bff;
}
.account-menu .logout {
  margin-top: 4px;
  color: #b45309;
}
.account-menu .logout:hover {
  background: #fff7ed;
  color: #c2410c;
}
.account:hover .account-menu,
.account:focus-within .account-menu {
  display: block;
}
.main {
  flex: 1;
  padding: 22px 0 28px;
}
</style>
