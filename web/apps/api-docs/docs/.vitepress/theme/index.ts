import DefaultTheme from 'vitepress/theme';import { h } from 'vue';import 'remixicon/fonts/remixicon.css';import './style.css';import ApiReference from './ApiReference.vue'
export default {extends:DefaultTheme,Layout:()=>h(DefaultTheme.Layout),enhanceApp({app}){app.component('ApiReference',ApiReference)}}
