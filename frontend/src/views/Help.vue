<template>
  <div class="help" v-html="htmlhelp">
  </div>
  <Footer />
</template>

<script setup lang="ts">
import { marked } from 'marked'
import Footer from '../components/Footer.vue'

var mdhelp = `
### ContextDict
一款利用 AI 辅助阅读的在线工具

#### 使用
以 MarginNote 为例:
在"设置 > 研究 > 自定义 URL" 中输入:
https://contextdict.zzhirong.com/?text={keyword}

#### Features
- Translate:
    - 默认是翻译文本.
    - 检测到是代码时, 会格式化代码.
    - 当有词语被选择的时候, 则解释当前词语在语句中的意思.
- Format:
    - 如果检测到文本是代码的话, 格式化代码
    - 如果只是普通的文本的话, 则修正其中的错误, 比如去除不必要的空格, 符号统一, 去除换行连字符.
- Summarize:
    - 文本摘要
- Analyze:
    - 分析句子的句式.
- Explain:
    - 解释当前输入文本, 比如, 有一句话不太懂, 然后, 尝试用 AI 帮你详细解释一下.
- Copy translation to clipboard
## Installation
1. Install the extension from the Chrome Web Store
`
var htmlhelp = marked(mdhelp)
</script>

