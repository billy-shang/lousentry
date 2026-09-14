<template>
  <div class="shell">
    <header class="topbar">
      <div class="page-wrap bar-inner">
        <BrandLockup size="sm" />
        <nav>
          <router-link to="/">首页</router-link>
          <div class="account">
            <button type="button" class="account-btn">
              {{ username }}<i v-if="!isAdmin">只读</i>
            </button>
            <div class="account-menu">
              <router-link to="/logs">日志</router-link>
              <router-link to="/settings">设置</router-link>
              <button type="button" @click="logout">退出</button>
            </div>
          </div>
        </nav>
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
nav { display: flex; align-items: center; gap: 6px; }
nav > a,
.account-menu a,
.account-menu button,
.account-btn {
  height: 32px;
  padding: 0 12px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: #4e5969;
  cursor: pointer;
  font-size: 14px;
  line-height: 32px;
}
nav > a.router-link-exact-active {
  background: #e8f1ff;
  color: #2f6bff;
  font-weight: 600;
}
nav > a:hover {
  background: #f5f7fa;
  color: #1f2329;
}
.account {
  position: relative;
  padding-bottom: 8px;
  margin-bottom: -8px;
}
.account-btn {
  background: #f5f7fa;
  color: #64748b;
  font-size: 13px;
}
.account-btn i {
  font-style: normal;
  margin-left: 6px;
  color: #2f6bff;
}
.account-menu {
  display: none;
  position: absolute;
  top: 100%;
  right: 0;
  min-width: 120px;
  padding: 6px;
  border: 1px solid #ebeef5;
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
  z-index: 30;
}
.account-menu a,
.account-menu button {
  display: block;
  width: 100%;
  text-align: left;
  background: transparent;
}
.account-menu a.router-link-active,
.account-menu a:hover,
.account-menu button:hover {
  background: #f5f7fa;
  color: #1f2329;
}
.account:hover .account-menu,
.account:focus-within .account-menu {
  display: block;
}
.main {
  flex: 1;
  padding: 22px 0 36px;
}
</style>
