<template>
  <div class="app">
    <main>
      <div class="translation-container">
        <div class="input-section">
          <textarea
            v-model="inputText"
            @select="handleTextSelection"
            @input="clearSelection"
          >
          </textarea>
          <div v-if="selectedText" class="selection-info">
            Selected: {{ selectedText }}
            <button @click="translateSelected" :disabled="isLoading">
              {{ isLoading ? 'Translating...' : 'Translate Selected' }}
            </button>
          </div>
          <button v-else @click="translateFull" :disabled="isLoading">
            {{ isLoading ? 'Translating...' : 'Translate' }}
          </button>
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
const q = params.get('q');
// const text = ref(q)
const inputText = ref(q)

const renderedTranslation = computed(() => {
  return marked(translation.value)
})

function handleTextSelection() {
  const selection = window.getSelection()
  if (selection) {
    selectedText.value = selection.toString()
  }
}

function clearSelection() {
  selectedText.value = ''
}

async function translateFull() {
  try {
    isLoading.value = true
    const response = await axios.get(`/translate?q=${encodeURIComponent(inputText.value??"")}`)
    translation.value = response.data.translation
  } catch (error) {
    console.error('Translation failed:', error)
  }
  isLoading.value = false
}

async function translateSelected() {
  if (!selectedText.value) return
  try {
    isLoading.value = true
    const response = await axios.get(
      `/translate?q=${encodeURIComponent(selectedText.value)}&context=${encodeURIComponent(inputText.value??"")}`
    )
    translation.value = response.data.translation
  } catch (error) {
    console.error('Translation failed:', error)
  }
  isLoading.value = false
}

function copyMarkdown() {
  navigator.clipboard.writeText(translation.value)
}
</script>

<style scoped>
.app {
  max-width: 800px;
  margin: 0 auto;
  padding: 2rem;
}

header {
  text-align: center;
  margin-bottom: 2rem;
}

.translation-container {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.input-section {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

textarea {
  width: 100%;
  min-height: 150px;
  padding: 1rem;
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
  padding: 1rem;
  margin-top: 1rem;
}

.copy-button {
  margin-top: 1rem;
}
button:disabled {
  background-color: #cccccc;
  cursor: not-allowed;
}
</style>
