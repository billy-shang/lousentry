<template>
  <footer class="app-footer">
    <div class="foot-inner">
      <span>{{ APP_TITLE }} {{ APP_VERSION }}</span>
      <a :href="GITHUB_URL" target="_blank" rel="noreferrer">GitHub 项目地址</a>
    </div>
  </footer>
</template>

<script setup>
import { APP_TITLE, APP_VERSION, GITHUB_URL } from "../version";
</script>

<style scoped>
.app-footer {
  width: 100%;
  margin-top: auto;
  background: #eef2f7;
  border-top: 1px solid #e2e8f0;
}
.foot-inner {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 8px 14px;
  min-height: 48px;
  padding: 12px 20px;
  color: #94a3b8;
  font-size: 13px;
}
.app-footer a {
  color: #7aa2ff;
}
.app-footer a:hover {
  color: #2f6bff;
  text-decoration: underline;
}
</style>
