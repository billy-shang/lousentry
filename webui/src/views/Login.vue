<template>
  <div class="login-page">
    <div class="login-wrap">
      <div class="hero">
        <BrandLockup size="lg" />
        <p>采集高价值漏洞，按周期检查并自动推送到钉钉 / 飞书 / 企业微信。</p>
      </div>

      <form class="login-card" @submit.prevent="onLogin">
        <h2>管理后台登录</h2>
        <p class="sub">Console Login</p>
        <label>
          <span>用户名</span>
          <input v-model.trim="form.username" class="input" autocomplete="username" placeholder="请输入用户名" />
        </label>
        <label>
          <span>密码</span>
          <input
            v-model="form.password"
            class="input"
            type="password"
            autocomplete="current-password"
            placeholder="请输入密码"
          />
        </label>
        <button class="btn login-btn" type="submit" :disabled="loading">
          {{ loading ? "登录中..." : "登录" }}
        </button>
      </form>

      <div class="features">
        <div class="feat">
          <span class="dot blue" />
          <h3>周期检查</h3>
          <p>按设定间隔抓取各漏洞源。启动时先建库，不推送历史漏洞；之后只处理新增或升级的漏洞。</p>
        </div>
        <div class="feat">
          <span class="dot green" />
          <h3>自动告警</h3>
          <p>检查到符合策略的新漏洞后，自动推送到已启用的钉钉、飞书、企业微信 Webhook。</p>
        </div>
        <div class="feat">
          <span class="dot orange" />
          <h3>控制台</h3>
          <p>检索漏洞、手动补推、查看运行日志，并在页面里管理监控与推送配置。</p>
        </div>
      </div>
    </div>
    <AppFooter />
    <div v-if="toast" class="toast-wrap"><div class="toast error">{{ toast }}</div></div>
  </div>
</template>

<script setup>
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import http, { errMsg } from "../api";
import { getStore, setStore } from "../storage";
import BrandLockup from "../components/BrandLockup.vue";
import AppFooter from "../components/AppFooter.vue";

const router = useRouter();
const loading = ref(false);
const toast = ref("");
const form = reactive({
  username: getStore("last-user") || "",
  password: "",
});

async function onLogin() {
  if (!form.username || !form.password) {
    toast.value = "请输入用户名和密码";
    return;
  }
  loading.value = true;
  toast.value = "";
  try {
    const res = await http.post("/login", form);
    setStore("token", res.data.token);
    setStore("user", JSON.stringify(res.data.user));
    setStore("last-user", form.username);
    console.log("[login] 登录成功", form.username);
    router.push("/");
  } catch (e) {
    console.log("[login] 登录失败", errMsg(e));
    toast.value = errMsg(e);
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  background: #f5f7fa;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 48px 20px 16px;
  box-sizing: border-box;
}
.login-wrap {
  width: 100%;
  max-width: 1080px;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 28px;
}
.hero p {
  margin: 14px 0 0;
  color: #8a8f99;
  font-size: 14px;
  line-height: 1.5;
  text-align: center;
}
.login-card {
  width: 100%;
  max-width: 400px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 12px;
  padding: 28px 32px 32px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.04);
}
.login-card h2 {
  margin: 0;
  text-align: center;
  font-size: 22px;
  font-weight: 700;
  color: #1f2329;
}
.sub {
  margin: 6px 0 22px;
  text-align: center;
  color: #a0a4ab;
  font-size: 13px;
}
label {
  display: block;
  margin-bottom: 14px;
}
label span {
  display: block;
  margin-bottom: 6px;
  color: #4e5969;
  font-size: 13px;
  font-weight: 500;
}
.login-card .input {
  height: 40px;
  border-radius: 8px;
}
.login-btn {
  width: 100%;
  height: 42px;
  margin-top: 8px;
  border-radius: 8px;
  background: #2f6bff;
  font-size: 15px;
  font-weight: 600;
}
.login-btn:hover { background: #1f5aee; }
.features {
  width: 100%;
  margin-top: 36px;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}
.feat {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 12px;
  padding: 18px 18px 16px;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.04);
}
.dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-bottom: 10px;
}
.dot.blue { background: #2f6bff; }
.dot.green { background: #22c55e; }
.dot.orange { background: #f59e0b; }
.feat h3 {
  margin: 0 0 8px;
  font-size: 15px;
  font-weight: 650;
  color: #1f2329;
}
.feat p {
  margin: 0;
  font-size: 13px;
  line-height: 1.65;
  color: #8a8f99;
}
@media (max-width: 720px) {
  .login-page { padding: 24px 12px; }
  .hero p { font-size: 12px; white-space: normal; }
  .login-card { padding: 22px 18px 24px; }
  .features { grid-template-columns: 1fr; margin-top: 24px; }
}
</style>
