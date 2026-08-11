import { defineConfig } from 'vitepress'
export default defineConfig({
  lang:'zh-CN',title:'Santaizi API',description:'Santaizi HTTP API v2',base:'/docs/api/',cleanUrls:true,
  outDir:'../../../../resource/web/api-docs',
  themeConfig:{logo:'/static/logo.svg',nav:[{text:'指南',link:'/'},{text:'接口参考',link:'/reference'},{text:'管理后台',link:'/admin/'}],sidebar:[{text:'开始',items:[{text:'概览',link:'/'},{text:'认证与 CSRF',link:'/guides/authentication'},{text:'WebSocket',link:'/guides/websocket'}]},{text:'参考',items:[{text:'API Reference',link:'/reference'}]}],socialLinks:[]},
  vite:{build:{emptyOutDir:true}}
})
