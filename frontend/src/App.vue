<template>
  <div class="app">
    <main>
      <div class="translation-container">
        <div class="input-section">
          <textarea
            v-model="inputText"
            @mouseup="updateSelection"
            @input="clearSelection"
          >
          </textarea>
          <div class="button-group">
            <button @click="translate" :disabled="isLoading">
            {{ isLoading ?
                  'thinking...' : 
                  (selectedText ? 'TranslateSelected' : 'Translate')
            }}
            </button>
            <button @click="format" :disabled="isLoading">
              {{ isLoading ? 'thinking...' : 'Format' }}
            </button>
            <button @click="summarize" :disabled="isLoading">
              {{ isLoading ? 'thinking...' : 'Summarize' }}
            </button>
            <button v-if="isLoading" @click="cancelRequest" class="cancel-button">
              Stop
            </button>
          </div>
        </div>
        <div v-if="translation" class="translation-result">
          <div class="markdown-content" v-html="renderedTranslation"></div>
          <button @click="copyMarkdown" class="copy-button">
            Copy Markdown {{ copyStatus }}
          </button>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { marked } from 'marked'
import axios from 'axios'
import useClipboard from 'vue-clipboard3'

const { toClipboard } = useClipboard()

const selectedText = ref('')
const isLoading = ref(false)
const translation = ref('')
const params = new URLSearchParams(window.location.search);
const q = params.get('keyword');
const inputText = ref(q)
const renderedTranslation = computed(() => {
  return marked(translation.value)
})

function clearSelection() {
  selectedText.value = ''
}

function updateSelection() {
  const selection = window.getSelection()?.toString().trim() || ''
  if (selection) selectedText.value = selection
}

// 在 script setup 顶部添加
const controller = ref<AbortController | null>(null)

// 修改所有请求函数，这里以 translate 为例
async function translate() {
  if (isLoading.value) return
  const context = inputText.value ?? ""
  if(context == "") return
  
  controller.value = new AbortController()
  let query = `api/translate?keyword=${encodeURIComponent(context)}`
  if (selectedText.value != ""){
     query = `translate?keyword=${encodeURIComponent(selectedText.value)}` +
      `&context=${encodeURIComponent(context)}`
  }
  isLoading.value = true
  try {
    const response = await axios.get(query, {
      signal: controller.value.signal
    })
    translation.value = response.data.result
  } catch (error) {
    if (axios.isCancel(error)) {
      console.log('Request canceled')
    } else {
      console.error('Translation failed:', error)
    }
  }
  isLoading.value = false
}

// 添加取消请求的函数
function cancelRequest() {
  if (controller.value) {
    controller.value.abort()
    controller.value = null
    isLoading.value = false
  }
}

async function format() {
  try {
    isLoading.value = true
    const response = await axios.get(
      `/api/format?keyword=${encodeURIComponent(inputText.value??"")}`
    )
    translation.value = response.data.result
  } catch (error) {
    console.error('Failed to format', error)
  }
  isLoading.value = false
}

async function summarize() {
  try {
    isLoading.value = true
    const response = await axios.get(
      `/api/summarize?keyword=${encodeURIComponent(inputText.value??"")}`
    )
    translation.value = response.data.result
  } catch (error) {
    console.error('Faield to summerize', error)
  }
  isLoading.value = false
}

const copyStatus = ref('')

async function copyMarkdown() {
    try {
        await toClipboard(translation.value)
        copyStatus.value = '✓'
        setTimeout(() => {
            copyStatus.value = ''
        }, 2000)
    } catch (error) {
        copyStatus.value = '✗'
        setTimeout(() => {
            copyStatus.value = ''
        }, 2000)
    }
}

// Run translation if query parameter exists
if (q) {
  translate()
}

</script>

<style scoped>
.app {
  max-width: 800px;
  margin: 0 auto;
}

header {
  text-align: center;
  margin-bottom: 2rem;
}

.translation-container {
  display: flex;
  flex-direction: column;
}

.input-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

textarea {
  width: 100%;
  padding: 0.5rem;
  min-height: 150px;
  border: 1px solid #ccc;
  border-radius: 4px;
  resize: vertical;
  font-size: 16px;
}

button {
  padding: 0.5rem 1rem;
  background-color: #4CAF50;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

button:hover {
  background-color: #45a049;
}

.selection-info {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.translation-result {
  border: 1px solid #ccc;
  border-radius: 4px;
  font-size: 16px;
  margin-top: 1rem;
  width: 100%;
  resize: vertical;
  padding: 0.5rem;
  resize: vertical;
}

.copy-button {
  margin-top: 1rem;
}
button:disabled {
  background-color: #cccccc;
  cursor: not-allowed;
}

.button-group {
  display: flex;
  gap: 1rem;
  justify-content: flex-start;
}

.cancel-button {
  background-color: #ff4444;
}

.cancel-button:hover {
  background-color: #cc0000;
}
</style>
