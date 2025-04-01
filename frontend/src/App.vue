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
          </div>
        </div>
        <div v-if="translation" class="translation-result">
          <div class="markdown-content" v-html="renderedTranslation"></div>
          <button @click="copyMarkdown" class="copy-button">
            Copy Markdown
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

async function translate() {
  const context = inputText.value ?? ""
  if(context == "") return
  let query = `translate?keyword=${encodeURIComponent(context)}`
  if (selectedText.value != ""){
     query = `translate?keyword=${encodeURIComponent(selectedText.value)}` +
      `&context=${encodeURIComponent(context)}`
  }
  isLoading.value = true
  try {
    const response = await axios.get(query)
    translation.value = response.data.result
  } catch (error) {
    console.error('Translation failed:', error)
  }
  isLoading.value = false
}

async function format() {
  try {
    isLoading.value = true
    const response = await axios.get(
      `format?keyword=${encodeURIComponent(inputText.value??"")}`
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
      `summarize?keyword=${encodeURIComponent(inputText.value??"")}`
    )
    translation.value = response.data.result
  } catch (error) {
    console.error('Faield to summerize', error)
  }
  isLoading.value = false
}

function copyMarkdown() {
  navigator.clipboard.writeText(translation.value)
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
</style>
