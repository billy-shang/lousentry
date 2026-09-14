<template>
  <div class="lockup" :class="size">
    <BrandMark :size="size" />
    <div class="names">
      <strong>漏哨</strong>
      <span>LouSentry</span>
    </div>
  </div>
</template>

<script setup>
import BrandMark from "./BrandMark.vue";

defineProps({
  size: { type: String, default: "sm" },
});
</script>

<style scoped>
.lockup {
  display: flex;
  align-items: center;
  text-align: left;
}
.names {
  display: flex;
  align-items: baseline;
  min-width: 0;
  white-space: nowrap;
  font-weight: 700;
  color: #1f2329;
  letter-spacing: -0.01em;
  line-height: 1;
}
.names strong,
.names span {
  font-weight: inherit;
  font-size: inherit;
  color: inherit;
}
.lockup.sm { gap: 10px; }
.lockup.sm .names { font-size: 16px; gap: 6px; }
.lockup.md { gap: 12px; }
.lockup.md .names { font-size: 22px; gap: 8px; }
.lockup.lg { gap: 14px; }
.lockup.lg .names { font-size: 28px; gap: 10px; }
</style>
