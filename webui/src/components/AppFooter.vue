<template>
  <footer class="app-footer">
    <span>{{ APP_TITLE }} {{ APP_VERSION }}</span>
    <a :href="GITHUB_URL" target="_blank" rel="noreferrer">GitHub 项目地址</a>
  </footer>
</template>

<script setup>
import { APP_TITLE, APP_VERSION, GITHUB_URL } from "../version";
</script>

<style scoped>
.app-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 8px 12px;
  padding: 16px 20px 22px;
  color: #94a3b8;
  font-size: 13px;
}
.app-footer a {
  color: #2f6bff;
}
.app-footer a:hover {
  text-decoration: underline;
}
</style>
