<template>
  <div class="editor-wrapper">
    <textarea ref="editorEl"></textarea>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import EasyMDE from 'easymde'
import 'easymde/dist/easymde.min.css'

const props = defineProps({
  modelValue: { type: String, default: '' },
  placeholder: { type: String, default: 'Write using Markdown...' },
  minHeight: { type: String, default: '150px' }
})
const emit = defineEmits(['update:modelValue'])

const editorEl = ref(null)
let mde = null

onMounted(() => {
  mde = new EasyMDE({
    element: editorEl.value,
    initialValue: props.modelValue,
    placeholder: props.placeholder,
    spellChecker: false,
    autofocus: false,
    minHeight: props.minHeight,
    toolbar: [
      'bold', 'italic', 'strikethrough', '|',
      'heading', 'quote', 'code', '|',
      'unordered-list', 'ordered-list', '|',
      'link', 'image', '|',
      'preview', 'side-by-side', 'fullscreen', '|',
      'guide'
    ]
  })

  mde.codemirror.on('change', () => {
    emit('update:modelValue', mde.value())
  })
})

watch(() => props.modelValue, (val) => {
  if (mde && mde.value() !== val) {
    mde.value(val)
  }
})

onBeforeUnmount(() => {
  mde?.toTextArea()
  mde = null
})
</script>

<style scoped>
.editor-wrapper { width: 100%; }
</style>
