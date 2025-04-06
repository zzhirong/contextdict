<template>
    {{userid ? userid : "I may not have a userid."}}
</template>
<script setup>
    const prop = defineProps({
        userid: String
    })
</script>
