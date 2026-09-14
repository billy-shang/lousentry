<template>
  <span class="mark" :class="size" aria-hidden="true">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M12 3l8 4v6c0 5-3.4 8.4-8 9.5C7.4 21.4 4 18 4 13V7l8-4z" />
      <path d="M9.5 12.2l1.8 1.8 3.4-3.8" />
    </svg>
  </span>
</template>

<script setup>
defineProps({
  size: { type: String, default: "sm" },
});
</script>

<style scoped>
.mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  background: #e8f1ff;
  color: #2f6bff;
  border-radius: 8px;
}
.mark svg { display: block; }
.mark.sm { width: 32px; height: 32px; }
.mark.sm svg { width: 18px; height: 18px; }
.mark.md { width: 40px; height: 40px; }
.mark.md svg { width: 22px; height: 22px; }
.mark.lg { width: 48px; height: 48px; border-radius: 12px; }
.mark.lg svg { width: 26px; height: 26px; }
</style>
