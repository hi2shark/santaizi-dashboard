<script setup lang="ts">import { ApiReference } from '@scalar/api-reference';import '@scalar/api-reference/style.css'</script>
<template><ClientOnly><ApiReference :configuration="{url:'/openapi/v2.yaml',theme:'saturn',layout:'modern',hideModels:false,hideDownloadButton:false,showSidebar:true,withDefaultFonts:false,authentication:{preferredSecurityScheme:'bearerAuth'}}" /></ClientOnly></template>
