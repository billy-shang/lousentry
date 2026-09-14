<template>
  <div class="stat-grid">
    <div class="stat-item accent">
      <div class="n">{{ stats.today_new ?? 0 }}</div>
      <div class="l">今日新增</div>
    </div>
    <div class="stat-item">
      <div class="n">{{ stats.source_count ?? 0 }}</div>
      <div class="l">数据源</div>
    </div>
    <div class="stat-item">
      <div class="n">{{ stats.pushed ?? 0 }}</div>
      <div class="l">已推送</div>
    </div>
    <div class="stat-item">
      <div class="n">{{ stats.unpushed ?? 0 }}</div>
      <div class="l">未推送</div>
    </div>
    <div class="stat-item">
      <div class="n">{{ formatInterval(stats.interval) }}</div>
      <div class="l">检查间隔</div>
    </div>
  </div>
</template>

<script setup>
import { formatInterval } from "../format";

defineProps({
  stats: { type: Object, default: () => ({}) },
});
</script>
