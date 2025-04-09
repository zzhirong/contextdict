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

#### 使用方法
- 在线使用, 在输入框中输入文本, 然后点击相应的功能按钮.
- 集成到第三方阅读软件中, 以 MarginNote 为例, 在"设置 > 研究 > 自定义 URL" 中输入: https://contextdict.zzhirong.com/?text={keyword}

#### 功能
- Translate:
    - 默认是翻译文本
    - 检测到是代码时, 会格式化代码
    - 当有文本被选中时, 则结合当前语境翻译选中的文本
- Format:
    - 如果检测到文本是代码的话, 格式化代码
    - 如果只是普通的文本的话, 则修正其中的错误, 比如去除不必要的空格, 统一符号, 去除换行连字符
- Summarize:
    - 文本摘要, 把里面主要观点提取出来.
- Analyze:
    - 重点在语法层面, 分析句子的句式, 比如有一句长句英语不太好理解, 则可以尝试让 AI 帮你分析一下.
- Explain:
    - 重点在内容层面, 解释当前输入文本, 比如, 有一句话不太懂, 尝试用 AI 帮你详细解释一下.
- Copy:
    - 复制处理结果到剪切板.

#### 注意
- 为避免滥用, 限制了单个请求的文本长度, 超过 3000 个字符会处理失败.
- 单个 IP 每秒最多请求 1 次.
- 由于是 AI 生成的内容, 所以, 请不要将其作为事实依据.
- 处理速度比较慢, 请耐心等待.
- AI 是个黑盒, 所以, 无法保证其准确性.
- 有什么问题, 请联系我, 我的邮箱是: zzhirong@gmail.com.
`
var htmlhelp = marked(mdhelp)
</script>

